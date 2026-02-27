package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
	"stock-report-analysis/internal/service"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	batchMu             sync.Mutex
	batchCond           *sync.Cond
	batchStatus         models.BatchStatus
	batchFailures       []models.BatchFailure
	batchPending        []int64
	batchChannel        models.AIChannel
	batchPrompt         models.Prompt
	batchMode           string
	batchSnapshotLoaded bool

	qaMu       sync.Mutex
	qaCancel   context.CancelFunc
	qaJobToken int64

	telegraphMu     sync.Mutex
	telegraphStatus models.TelegraphSchedulerStatus
	telegraphCancel context.CancelFunc
	telegraphRunSeq int64
	telegraphOnce   sync.Once
}

func NewApp() *App {
	app := &App{}
	app.batchCond = sync.NewCond(&app.batchMu)
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := db.Init(); err != nil {
		runtime.LogFatal(ctx, "DB init failed: "+err.Error())
	}
	a.startTelegraphScheduler()
}

// --- AI Channels ---

func (a *App) GetChannels() ([]models.AIChannel, error) {
	return service.GetChannels()
}

func (a *App) SaveChannel(ch models.AIChannel) error {
	return service.SaveChannel(ch)
}

func (a *App) DeleteChannel(id int64) error {
	return service.DeleteChannel(id)
}

// --- Prompts ---

func (a *App) GetPrompts() ([]models.Prompt, error) {
	return service.GetPrompts()
}

func (a *App) SavePrompt(p models.Prompt) error {
	return service.SavePrompt(p)
}

func (a *App) DeletePrompt(id int64) error {
	return service.DeletePrompt(id)
}

func (a *App) GetPromptVersions(promptID int64) ([]models.PromptVersion, error) {
	return service.GetPromptVersions(promptID)
}

func (a *App) RestorePromptVersion(promptID int64, versionID int64) error {
	return service.RestorePromptVersion(promptID, versionID)
}

func (a *App) GetTelegraphSchedulerConfig() (models.TelegraphSchedulerConfig, error) {
	return service.GetTelegraphSchedulerConfig()
}

func (a *App) SaveTelegraphSchedulerConfig(cfg models.TelegraphSchedulerConfig) error {
	if err := service.SaveTelegraphSchedulerConfig(cfg); err != nil {
		return err
	}
	if cfg.Enabled == 1 {
		a.triggerTelegraphRun(true)
	} else {
		a.stopTelegraphRun("任务已停止")
	}
	return nil
}

func (a *App) GetTelegraphSchedulerStatus() models.TelegraphSchedulerStatus {
	a.telegraphMu.Lock()
	defer a.telegraphMu.Unlock()
	return a.telegraphStatus
}

func (a *App) RunTelegraphSchedulerNow() error {
	if !a.triggerTelegraphRun(true) {
		return errors.New("自动抓取任务正在运行")
	}
	return nil
}

func (a *App) StopTelegraphScheduler() error {
	cfg, err := service.GetTelegraphSchedulerConfig()
	if err != nil {
		return err
	}
	cfg.Enabled = 0
	if err := service.SaveTelegraphSchedulerConfig(cfg); err != nil {
		return err
	}
	a.stopTelegraphRun("任务已停止")
	return nil
}

// --- Articles ---

func (a *App) GetArticles(keyword string, tagID int64) ([]models.Article, error) {
	return service.GetArticles(keyword, tagID)
}

func (a *App) GetTelegraphArticles(keyword string, tagID int64, order string, watchOnly int) ([]models.TelegraphArticleItem, error) {
	return service.GetTelegraphArticles(keyword, tagID, order, watchOnly)
}

func (a *App) GetTelegraphDashboard() (models.TelegraphDashboard, error) {
	return service.GetTelegraphDashboardByDays(0)
}

func (a *App) GetTelegraphDashboardByDays(days int) (models.TelegraphDashboard, error) {
	return service.GetTelegraphDashboardByDays(days)
}

func (a *App) GetTelegraphDigests(limit int) ([]models.TelegraphDigest, error) {
	return service.GetTelegraphDigests(limit)
}

func (a *App) GetTelegraphWatchlist() ([]models.WatchStock, error) {
	return service.GetTelegraphWatchlist()
}

func (a *App) SaveTelegraphWatchlist(items []models.WatchStock) error {
	if err := service.SaveTelegraphWatchlist(items); err != nil {
		return err
	}
	return service.RebuildTelegraphWatchHits()
}

func (a *App) GetArticle(id int64) (models.Article, error) {
	return service.GetArticle(id)
}

func (a *App) DeleteArticle(id int64) error {
	return service.DeleteArticle(id)
}

func (a *App) ImportArticle() (models.Article, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文章文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "支持格式", Pattern: "*.txt;*.md;*.html;*.htm;*.pdf;*.png;*.jpg;*.jpeg;*.bmp;*.tiff;*.webp;*.doc;*.docx;*.ppt;*.pptx"},
		},
	})
	if err != nil || path == "" {
		return models.Article{}, err
	}
	return service.ImportFile(path)
}

func (a *App) ImportArticles() ([]models.Article, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文章文件（可多选）",
		Filters: []runtime.FileFilter{
			{DisplayName: "支持格式", Pattern: "*.txt;*.md;*.html;*.htm;*.pdf;*.png;*.jpg;*.jpeg;*.bmp;*.tiff;*.webp;*.doc;*.docx;*.ppt;*.pptx"},
		},
	})
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	var articles []models.Article
	var failed []string
	for _, p := range paths {
		art, err := service.ImportFile(p)
		if err == nil {
			articles = append(articles, art)
			continue
		}
		failed = append(failed, fmt.Sprintf("%s: %s", filepath.Base(p), err.Error()))
	}

	if len(articles) == 0 && len(failed) > 0 {
		msg := strings.Join(failed, "；")
		if len(failed) > 3 {
			msg = strings.Join(failed[:3], "；") + fmt.Sprintf("；另有 %d 个文件失败", len(failed)-3)
		}
		return nil, fmt.Errorf("导入失败：%s", msg)
	}
	return articles, nil
}

// --- Tags ---

func (a *App) GetTags() ([]models.Tag, error) {
	return service.GetTags()
}

func (a *App) SaveTag(t models.Tag) error {
	return service.SaveTag(t)
}

func (a *App) DeleteTag(id int64) error {
	return service.DeleteTag(id)
}

func (a *App) GetArticleTags(articleID int64) ([]models.Tag, error) {
	return service.GetArticleTags(articleID)
}

func (a *App) SetArticleTags(articleID int64, tagIDs []int64) error {
	return service.SetArticleTags(articleID, tagIDs)
}

// --- Analysis History ---

func (a *App) GetAnalysisHistory(articleID int64) ([]models.AnalysisHistory, error) {
	return service.GetAnalysisHistory(articleID)
}

func (a *App) GetAnalysisDashboard() (models.AnalysisDashboard, error) {
	return service.GetAnalysisDashboard()
}

func (a *App) GetAnalysisDashboardByDays(days int) (models.AnalysisDashboard, error) {
	return service.GetAnalysisDashboardByDays(days)
}

func (a *App) GetMinerUConfig() (models.MinerUConfig, error) {
	return service.GetMinerUConfig()
}

func (a *App) SaveMinerUConfig(cfg models.MinerUConfig) error {
	return service.SaveMinerUConfig(cfg)
}

// --- Roles ---

func (a *App) GetRoles() ([]models.Role, error) {
	return service.GetRoles()
}

func (a *App) SaveRole(role models.Role) error {
	return service.SaveRole(role)
}

func (a *App) DeleteRole(id int64) error {
	return service.DeleteRole(id)
}

func (a *App) SetDefaultRole(id int64) error {
	return service.SetDefaultRole(id)
}

func (a *App) GetRoleTemplates() []models.RoleTemplate {
	return service.GetRoleTemplates()
}

func (a *App) CreateRoleFromTemplate(templateID string) (models.Role, error) {
	return service.CreateRoleFromTemplate(templateID)
}

// --- QA ---

func (a *App) GetQASessions(articleID int64) ([]models.QASession, error) {
	return service.GetQASessions(articleID)
}

func (a *App) CreateQASession(articleID int64, title string) (models.QASession, error) {
	return service.CreateQASession(articleID, title)
}

func (a *App) RenameQASession(id int64, title string) error {
	return service.RenameQASession(id, title)
}

func (a *App) DeleteQASession(id int64) error {
	return service.DeleteQASession(id)
}

func (a *App) GetQAMessages(sessionID int64) ([]models.QAMessage, error) {
	return service.GetQAMessages(sessionID)
}

func (a *App) GetQAPins(sessionID int64) ([]models.QAPin, error) {
	return service.GetQAPins(sessionID)
}

func (a *App) SaveQAPin(pin models.QAPin) (models.QAPin, error) {
	return service.SaveQAPin(pin)
}

func (a *App) DeleteQAPin(id int64) error {
	return service.DeleteQAPin(id)
}

func (a *App) DebugQAPing(marker string) string {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		marker = "empty-marker"
	}
	log.Printf("[QA][App] ping marker=%s", marker)
	return "pong"
}

func (a *App) AskQuestion(sessionID int64, articleID int64, question string) (int64, error) {
	return a.askQuestionWithFollowUp(sessionID, articleID, question, 0)
}

func (a *App) AskQuestionFollowUp(sessionID int64, articleID int64, question string, followUpMessageID int64) (int64, error) {
	return a.askQuestionWithFollowUp(sessionID, articleID, question, followUpMessageID)
}

func (a *App) askQuestionWithFollowUp(sessionID int64, articleID int64, question string, followUpMessageID int64) (int64, error) {
	trimmed := strings.TrimSpace(question)
	if trimmed == "" {
		return 0, fmt.Errorf("问题不能为空")
	}
	log.Printf("[QA][App] ask request session=%d article=%d follow_up=%d question=%q", sessionID, articleID, followUpMessageID, trimmed)
	ctx, cancel := context.WithCancel(context.Background())
	token := a.setActiveQACancel(cancel)

	callbacks := service.QAStreamCallbacks{
		OnJobStart: func(newSessionID int64, questionMessageID int64, roleCount int) {
			log.Printf("[QA][App] job start session=%d question_message=%d roles=%d", newSessionID, questionMessageID, roleCount)
			runtime.EventsEmit(a.ctx, "qa-job-start", map[string]any{
				"sessionId":         newSessionID,
				"questionMessageId": questionMessageID,
				"roleCount":         roleCount,
			})
		},
		OnRoleStart: func(msg models.QAMessage, _ models.Role) {
			log.Printf("[QA][App] role start session=%d message=%d role=%d(%s)", msg.SessionID, msg.ID, msg.RoleID, msg.RoleName)
			runtime.EventsEmit(a.ctx, "qa-role-start", msg)
		},
		OnRoleChunk: func(messageID int64, roleID int64, roleName string, chunk string) {
			runtime.EventsEmit(a.ctx, "qa-role-chunk", map[string]any{
				"messageId": messageID,
				"roleId":    roleID,
				"roleName":  roleName,
				"chunk":     chunk,
			})
		},
		OnRoleDone: func(msg models.QAMessage) {
			log.Printf("[QA][App] role done session=%d message=%d role=%d(%s)", msg.SessionID, msg.ID, msg.RoleID, msg.RoleName)
			runtime.EventsEmit(a.ctx, "qa-role-done", msg)
		},
		OnRoleError: func(messageID int64, roleID int64, roleName string, errMsg string) {
			log.Printf("[QA][App] role error message=%d role=%d(%s) err=%s", messageID, roleID, roleName, errMsg)
			runtime.EventsEmit(a.ctx, "qa-role-error", map[string]any{
				"messageId": messageID,
				"roleId":    roleID,
				"roleName":  roleName,
				"error":     errMsg,
			})
		},
		OnJobDone: func(doneSessionID int64) {
			log.Printf("[QA][App] job done session=%d", doneSessionID)
			runtime.EventsEmit(a.ctx, "qa-job-done", map[string]any{
				"sessionId": doneSessionID,
			})
		},
	}

	go func() {
		defer a.clearActiveQACancel(token)
		log.Printf("[QA][App] goroutine started input_session=%d article=%d", sessionID, articleID)
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("问答任务异常: %v", r)
				runtime.LogError(a.ctx, errMsg)
				log.Printf("[QA][App] panic recovered: %s", errMsg)
				runtime.EventsEmit(a.ctx, "qa-role-error", map[string]any{
					"messageId": int64(0),
					"roleId":    int64(0),
					"roleName":  "",
					"error":     errMsg,
				})
				runtime.EventsEmit(a.ctx, "qa-job-done", map[string]any{
					"sessionId": sessionID,
				})
			}
		}()
		if _, err := service.AskQuestionWithContextAndFollowUp(ctx, sessionID, articleID, trimmed, followUpMessageID, callbacks); err != nil {
			errMsg := err.Error()
			if errors.Is(err, context.Canceled) {
				errMsg = "已取消本次提问"
			}
			log.Printf("[QA][App] ask failed input_session=%d article=%d err=%s", sessionID, articleID, err.Error())
			runtime.EventsEmit(a.ctx, "qa-role-error", map[string]any{
				"messageId": int64(0),
				"roleId":    int64(0),
				"roleName":  "",
				"error":     errMsg,
			})
			runtime.EventsEmit(a.ctx, "qa-job-done", map[string]any{
				"sessionId": sessionID,
			})
			return
		}
		log.Printf("[QA][App] goroutine finished input_session=%d article=%d", sessionID, articleID)
	}()

	return 0, nil
}

func (a *App) CancelAskQuestion() error {
	a.qaMu.Lock()
	cancel := a.qaCancel
	a.qaMu.Unlock()

	if cancel == nil {
		return errors.New("当前没有进行中的提问任务")
	}
	cancel()
	log.Printf("[QA][App] cancel requested")
	return nil
}

func (a *App) setActiveQACancel(cancel context.CancelFunc) int64 {
	a.qaMu.Lock()
	defer a.qaMu.Unlock()

	if a.qaCancel != nil {
		a.qaCancel()
	}
	a.qaCancel = cancel
	a.qaJobToken++
	return a.qaJobToken
}

func (a *App) clearActiveQACancel(token int64) {
	a.qaMu.Lock()
	defer a.qaMu.Unlock()

	if a.qaJobToken == token {
		a.qaCancel = nil
	}
}

func (a *App) GetQADashboard() (models.QADashboard, error) {
	return service.GetQADashboardByDays(0)
}

func (a *App) GetQADashboardByDays(days int) (models.QADashboard, error) {
	return service.GetQADashboardByDays(days)
}

// --- Export ---

func (a *App) ExportArticle(articleID int64) error {
	article, err := service.GetArticle(articleID)
	if err != nil {
		return err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出文章",
		DefaultFilename: article.Title + ".md",
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown", Pattern: "*.md"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	return service.ExportToFile(article, path)
}

// --- Batch Analysis ---

func (a *App) BatchAnalyze(articleIDs []int64, channelID int64, promptID int64) {
	if err := a.StartBatchAnalyze(articleIDs, channelID, promptID, 1, service.AnalysisModeText); err != nil {
		runtime.EventsEmit(a.ctx, "batch-error", err.Error())
	}
}

func (a *App) StartBatchAnalyze(articleIDs []int64, channelID int64, promptID int64, concurrency int, mode string) error {
	return a.startBatchAnalyze(articleIDs, channelID, promptID, concurrency, mode)
}

func (a *App) PauseBatchAnalyze() error {
	return a.pauseBatchAnalyze()
}

func (a *App) ResumeBatchAnalyze() error {
	return a.resumeBatchAnalyze()
}

func (a *App) RetryFailedBatchAnalyze() error {
	return a.retryFailedBatchAnalyze()
}

func (a *App) GetBatchStatus() models.BatchStatus {
	return a.getBatchStatus()
}

func (a *App) ExportBatchFailures() error {
	return a.exportBatchFailures()
}

// --- AI Analysis ---

func (a *App) AnalyzeArticle(articleID int64, channelID int64, promptID int64) string {
	return a.AnalyzeArticleWithMode(articleID, channelID, promptID, service.AnalysisModeText)
}

func (a *App) AnalyzeArticleWithMode(articleID int64, channelID int64, promptID int64, mode string) string {
	return a.analyzeArticleWithMode(articleID, channelID, promptID, mode)
}

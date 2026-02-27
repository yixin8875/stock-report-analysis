package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
	"stock-report-analysis/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const batchSnapshotConfigKey = "batch_status_snapshot"

type batchSnapshot struct {
	Status    models.BatchStatus `json:"status"`
	ChannelID int64              `json:"channelId"`
	PromptID  int64              `json:"promptId"`
	Mode      string             `json:"mode"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

func (a *App) analyzeArticleWithMode(articleID int64, channelID int64, promptID int64, mode string) string {
	mode = normalizeAnalysisMode(mode)

	article, err := service.GetArticle(articleID)
	if err != nil {
		return "错误: 获取文章失败 - " + err.Error()
	}
	channel, prompt, err := a.getChannelAndPrompt(channelID, promptID)
	if err != nil {
		return "错误: " + err.Error()
	}

	if err := service.UpdateArticleStatus(articleID, 1); err != nil {
		return "错误: 更新状态失败 - " + err.Error()
	}

	startedAt := time.Now()
	result, err := service.AnalyzeArticleDetailed(*channel, prompt.Content, article.Content, mode, func(chunk string) {
		runtime.EventsEmit(a.ctx, "analysis-chunk", chunk)
	})
	if err != nil {
		_ = service.UpdateArticleStatus(articleID, 0)
		a.recordAnalysisRun(articleID, channel, prompt, mode, result, classifyErrorReason(err), startedAt, false)
		return "错误: " + err.Error()
	}

	if err := service.UpdateArticleAnalysis(articleID, result.Text, prompt.Name, channel.Name); err != nil {
		_ = service.UpdateArticleStatus(articleID, 0)
		a.recordAnalysisRun(articleID, channel, prompt, mode, result, "save_error", startedAt, false)
		return "错误: 保存分析结果失败 - " + err.Error()
	}

	a.recordAnalysisRun(articleID, channel, prompt, mode, result, "", startedAt, true)
	return ""
}

func (a *App) startBatchAnalyze(articleIDs []int64, channelID int64, promptID int64, concurrency int, mode string) error {
	if len(articleIDs) == 0 {
		return errors.New("请先选择至少一篇文章")
	}
	channel, prompt, err := a.getChannelAndPrompt(channelID, promptID)
	if err != nil {
		return err
	}

	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 8 {
		concurrency = 8
	}

	mode = normalizeAnalysisMode(mode)
	ids := uniqueArticleIDs(articleIDs)

	a.batchMu.Lock()
	if a.batchStatus.Running {
		a.batchMu.Unlock()
		return errors.New("已有批量任务正在运行")
	}

	a.batchStatus = models.BatchStatus{
		Running:     true,
		Paused:      false,
		Total:       len(ids),
		Completed:   0,
		Success:     0,
		Failed:      0,
		InProgress:  0,
		Concurrency: concurrency,
		Failures:    nil,
	}
	a.batchFailures = nil
	a.batchPending = ids
	a.batchChannel = *channel
	a.batchPrompt = *prompt
	a.batchMode = mode
	a.batchSnapshotLoaded = true

	status := cloneBatchStatus(a.batchStatus)
	snapshotChannelID := a.batchChannel.ID
	snapshotPromptID := a.batchPrompt.ID
	snapshotMode := a.batchMode
	a.batchMu.Unlock()

	a.persistBatchSnapshot(status, snapshotChannelID, snapshotPromptID, snapshotMode)
	go a.emitBatchStatus(status)
	go a.batchDispatchLoop()
	return nil
}

func (a *App) pauseBatchAnalyze() error {
	a.batchMu.Lock()
	if !a.batchStatus.Running {
		a.batchMu.Unlock()
		return errors.New("当前没有运行中的批量任务")
	}
	a.batchStatus.Paused = true
	status := cloneBatchStatus(a.batchStatus)
	snapshotChannelID := a.batchChannel.ID
	snapshotPromptID := a.batchPrompt.ID
	snapshotMode := a.batchMode
	a.batchMu.Unlock()

	a.persistBatchSnapshot(status, snapshotChannelID, snapshotPromptID, snapshotMode)
	go a.emitBatchStatus(status)
	return nil
}

func (a *App) resumeBatchAnalyze() error {
	a.batchMu.Lock()
	if !a.batchStatus.Running {
		a.batchMu.Unlock()
		return errors.New("当前没有运行中的批量任务")
	}
	a.batchStatus.Paused = false
	a.batchCond.Broadcast()
	status := cloneBatchStatus(a.batchStatus)
	snapshotChannelID := a.batchChannel.ID
	snapshotPromptID := a.batchPrompt.ID
	snapshotMode := a.batchMode
	a.batchMu.Unlock()

	a.persistBatchSnapshot(status, snapshotChannelID, snapshotPromptID, snapshotMode)
	go a.emitBatchStatus(status)
	return nil
}

func (a *App) retryFailedBatchAnalyze() error {
	a.batchMu.Lock()
	if !a.batchSnapshotLoaded {
		a.loadBatchSnapshotLocked()
	}
	if a.batchStatus.Running {
		a.batchMu.Unlock()
		return errors.New("请等待当前任务结束后再重试")
	}
	if len(a.batchFailures) == 0 {
		a.batchMu.Unlock()
		return errors.New("没有可重试的失败任务")
	}

	ids := make([]int64, 0, len(a.batchFailures))
	for _, item := range a.batchFailures {
		ids = append(ids, item.ArticleID)
	}
	channelID := a.batchChannel.ID
	promptID := a.batchPrompt.ID
	concurrency := a.batchStatus.Concurrency
	mode := a.batchMode
	a.batchMu.Unlock()

	return a.startBatchAnalyze(ids, channelID, promptID, concurrency, mode)
}

func (a *App) getBatchStatus() models.BatchStatus {
	a.batchMu.Lock()
	defer a.batchMu.Unlock()
	if !a.batchSnapshotLoaded {
		a.loadBatchSnapshotLocked()
	}
	return cloneBatchStatus(a.batchStatus)
}

func (a *App) loadBatchSnapshotLocked() {
	a.batchSnapshotLoaded = true

	var raw string
	err := db.DB.Get(&raw, "SELECT value FROM app_configs WHERE key=?", batchSnapshotConfigKey)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && a.ctx != nil {
			runtime.LogWarning(a.ctx, "load batch snapshot failed: "+err.Error())
		}
		return
	}
	if strings.TrimSpace(raw) == "" {
		return
	}

	var snap batchSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		if a.ctx != nil {
			runtime.LogWarning(a.ctx, "invalid batch snapshot: "+err.Error())
		}
		return
	}

	// after app restart there is no live worker, mark as not running
	snap.Status.Running = false
	snap.Status.Paused = false
	a.batchStatus = snap.Status
	a.batchFailures = cloneBatchStatus(snap.Status).Failures
	a.batchChannel = models.AIChannel{ID: snap.ChannelID}
	a.batchPrompt = models.Prompt{ID: snap.PromptID}
	a.batchMode = normalizeAnalysisMode(snap.Mode)
}

func (a *App) persistBatchSnapshot(status models.BatchStatus, channelID int64, promptID int64, mode string) {
	if !a.batchSnapshotLoaded {
		return
	}

	snap := batchSnapshot{
		Status:    cloneBatchStatus(status),
		ChannelID: channelID,
		PromptID:  promptID,
		Mode:      normalizeAnalysisMode(mode),
		UpdatedAt: time.Now(),
	}
	data, err := json.Marshal(snap)
	if err != nil {
		if a.ctx != nil {
			runtime.LogWarning(a.ctx, "marshal batch snapshot failed: "+err.Error())
		}
		return
	}

	_, err = db.DB.Exec(`
		INSERT INTO app_configs(key, value, updated_at)
		VALUES(?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
	`, batchSnapshotConfigKey, string(data))
	if err != nil && a.ctx != nil {
		runtime.LogWarning(a.ctx, "save batch snapshot failed: "+err.Error())
	}
}

func (a *App) exportBatchFailures() error {
	status := a.getBatchStatus()
	if len(status.Failures) == 0 {
		return errors.New("暂无失败明细")
	}

	filename := fmt.Sprintf("batch_failures_%s.csv", time.Now().Format("20060102_150405"))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出失败明细",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV", Pattern: "*.csv"},
		},
	})
	if err != nil || path == "" {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"article_id", "title", "reason", "time"}); err != nil {
		return err
	}
	for _, item := range status.Failures {
		if err := w.Write([]string{
			fmt.Sprintf("%d", item.ArticleID),
			item.Title,
			item.Reason,
			item.At.Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func (a *App) batchDispatchLoop() {
	for {
		a.batchMu.Lock()

		for a.batchStatus.Running {
			for a.batchStatus.Running && a.batchStatus.Paused {
				a.batchCond.Wait()
			}

			if len(a.batchPending) == 0 && a.batchStatus.InProgress == 0 {
				a.batchStatus.Running = false
				a.batchStatus.Paused = false
				status := cloneBatchStatus(a.batchStatus)
				snapshotChannelID := a.batchChannel.ID
				snapshotPromptID := a.batchPrompt.ID
				snapshotMode := a.batchMode
				a.batchMu.Unlock()
				a.persistBatchSnapshot(status, snapshotChannelID, snapshotPromptID, snapshotMode)
				a.emitBatchStatus(status)
				runtime.EventsEmit(a.ctx, "batch-done", nil)
				return
			}

			dispatched := false
			for len(a.batchPending) > 0 && a.batchStatus.InProgress < a.batchStatus.Concurrency && !a.batchStatus.Paused {
				articleID := a.batchPending[0]
				a.batchPending = a.batchPending[1:]
				a.batchStatus.InProgress++
				dispatched = true
				go a.runBatchArticle(articleID, a.batchChannel, a.batchPrompt, a.batchMode)
			}
			if dispatched {
				continue
			}

			a.batchCond.Wait()
		}

		a.batchMu.Unlock()
		return
	}
}

func (a *App) runBatchArticle(articleID int64, channel models.AIChannel, prompt models.Prompt, mode string) {
	article, err := service.GetArticle(articleID)
	if err != nil {
		a.finishBatchArticle(articleID, "", "获取文章失败: "+err.Error(), channel, prompt, mode, service.AnalysisResult{}, time.Now(), false)
		return
	}

	startedAt := time.Now()
	_ = service.UpdateArticleStatus(articleID, 1)
	result, err := service.AnalyzeArticleDetailed(channel, prompt.Content, article.Content, mode, func(string) {})
	if err != nil {
		_ = service.UpdateArticleStatus(articleID, 0)
		a.finishBatchArticle(articleID, article.Title, "解读失败: "+err.Error(), channel, prompt, mode, result, startedAt, false)
		return
	}

	if err := service.UpdateArticleAnalysis(articleID, result.Text, prompt.Name, channel.Name); err != nil {
		_ = service.UpdateArticleStatus(articleID, 0)
		a.finishBatchArticle(articleID, article.Title, "保存失败: "+err.Error(), channel, prompt, mode, result, startedAt, false)
		return
	}

	a.finishBatchArticle(articleID, article.Title, "", channel, prompt, mode, result, startedAt, true)
}

func (a *App) finishBatchArticle(articleID int64, title string, rawError string, channel models.AIChannel, prompt models.Prompt, mode string, result service.AnalysisResult, startedAt time.Time, success bool) {
	errorReason := ""
	if !success {
		errorReason = classifyErrorReason(errors.New(rawError))
	}
	a.recordAnalysisRun(articleID, &channel, &prompt, mode, result, errorReason, startedAt, success)

	a.batchMu.Lock()
	a.batchStatus.InProgress--
	a.batchStatus.Completed++
	if success {
		a.batchStatus.Success++
	} else {
		a.batchStatus.Failed++
		failure := models.BatchFailure{
			ArticleID: articleID,
			Title:     title,
			Reason:    rawError,
			At:        time.Now(),
		}
		a.batchFailures = append(a.batchFailures, failure)
		a.batchStatus.Failures = append(a.batchStatus.Failures, failure)
	}
	status := cloneBatchStatus(a.batchStatus)
	snapshotChannelID := a.batchChannel.ID
	snapshotPromptID := a.batchPrompt.ID
	snapshotMode := a.batchMode
	a.batchMu.Unlock()
	a.persistBatchSnapshot(status, snapshotChannelID, snapshotPromptID, snapshotMode)

	runtime.EventsEmit(a.ctx, "batch-progress", map[string]int{"current": status.Completed, "total": status.Total})
	if !success {
		runtime.EventsEmit(a.ctx, "batch-error", rawError)
	}
	a.emitBatchStatus(status)

	a.batchMu.Lock()
	a.batchCond.Signal()
	a.batchMu.Unlock()
}

func (a *App) emitBatchStatus(status models.BatchStatus) {
	runtime.EventsEmit(a.ctx, "batch-status", status)
}

func (a *App) getChannelAndPrompt(channelID int64, promptID int64) (*models.AIChannel, *models.Prompt, error) {
	channels, err := service.GetChannels()
	if err != nil {
		return nil, nil, fmt.Errorf("获取渠道失败 - %w", err)
	}
	var channel *models.AIChannel
	for _, c := range channels {
		if c.ID == channelID {
			channel = &c
			break
		}
	}
	if channel == nil {
		return nil, nil, fmt.Errorf("未找到 AI 渠道 (id=%d)，请先在设置中添加", channelID)
	}

	prompts, err := service.GetPrompts()
	if err != nil {
		return nil, nil, fmt.Errorf("获取提示词失败 - %w", err)
	}
	var prompt *models.Prompt
	for _, p := range prompts {
		if p.ID == promptID {
			prompt = &p
			break
		}
	}
	if prompt == nil {
		return nil, nil, fmt.Errorf("未找到提示词 (id=%d)，请先在设置中添加", promptID)
	}

	return channel, prompt, nil
}

func (a *App) recordAnalysisRun(articleID int64, channel *models.AIChannel, prompt *models.Prompt, mode string, result service.AnalysisResult, reason string, startedAt time.Time, success bool) {
	durationMs := result.DurationMs
	if durationMs <= 0 {
		durationMs = time.Since(startedAt).Milliseconds()
	}

	run := models.AnalysisRun{
		ArticleID:        articleID,
		ChannelID:        channel.ID,
		ChannelName:      channel.Name,
		PromptID:         prompt.ID,
		PromptName:       prompt.Name,
		Mode:             normalizeAnalysisMode(mode),
		Success:          boolToInt(success),
		ErrorReason:      reason,
		DurationMs:       durationMs,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		TotalTokens:      result.TotalTokens,
	}
	_ = service.RecordAnalysisRun(run)
}

func normalizeAnalysisMode(mode string) string {
	if strings.EqualFold(mode, service.AnalysisModeStructured) {
		return service.AnalysisModeStructured
	}
	return service.AnalysisModeText
}

func classifyErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "unauthorized"), strings.Contains(msg, "api key"):
		return "auth"
	case strings.Contains(msg, "429"), strings.Contains(msg, "rate"):
		return "rate_limit"
	case strings.Contains(msg, "500"), strings.Contains(msg, "502"), strings.Contains(msg, "503"), strings.Contains(msg, "504"):
		return "server"
	case strings.Contains(msg, "dial"), strings.Contains(msg, "connection"), strings.Contains(msg, "network"):
		return "network"
	case strings.Contains(msg, "save"):
		return "save_error"
	default:
		return "other"
	}
}

func uniqueArticleIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func cloneBatchStatus(status models.BatchStatus) models.BatchStatus {
	copyStatus := status
	if len(status.Failures) == 0 {
		copyStatus.Failures = nil
		return copyStatus
	}
	copyStatus.Failures = make([]models.BatchFailure, len(status.Failures))
	copy(copyStatus.Failures, status.Failures)
	return copyStatus
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

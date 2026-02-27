package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"stock-report-analysis/internal/models"
	"stock-report-analysis/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const telegraphAlertMinScore = 85
const telegraphDigestInterval = 30 * time.Minute

func (a *App) startTelegraphScheduler() {
	a.telegraphOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			// Startup warm-up
			_ = a.triggerTelegraphRun(false)
			for range ticker.C {
				_ = a.triggerTelegraphRun(false)
			}
		}()
	})
}

func (a *App) triggerTelegraphRun(force bool) bool {
	cfg, err := service.GetTelegraphSchedulerConfig()
	if err != nil {
		a.updateTelegraphStatus(func(s *models.TelegraphSchedulerStatus) {
			s.LastError = "读取电报配置失败: " + err.Error()
		})
		return false
	}
	if cfg.Enabled != 1 && !force {
		return false
	}

	a.telegraphMu.Lock()
	if a.telegraphStatus.Running {
		a.telegraphMu.Unlock()
		return false
	}
	if !force && !a.telegraphStatus.LastRunAt.IsZero() {
		nextAt := a.telegraphStatus.LastRunAt.Add(time.Duration(cfg.IntervalMinutes) * time.Minute)
		if time.Now().Before(nextAt) {
			a.telegraphMu.Unlock()
			return false
		}
	}
	runCtx, cancel := context.WithCancel(context.Background())
	a.telegraphRunSeq++
	runSeq := a.telegraphRunSeq
	a.telegraphCancel = cancel
	a.telegraphStatus.Running = true
	a.telegraphMu.Unlock()

	go a.runTelegraphOnce(runCtx, runSeq, cfg)
	return true
}

func (a *App) stopTelegraphRun(reason string) {
	a.telegraphMu.Lock()
	cancel := a.telegraphCancel
	if reason != "" {
		a.telegraphStatus.LastError = reason
	}
	a.telegraphMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (a *App) runTelegraphOnce(ctx context.Context, runSeq int64, cfg models.TelegraphSchedulerConfig) {
	startedAt := time.Now()
	fetched := 0
	imported := 0
	analyzed := 0
	lastErr := ""

	defer func() {
		durationMs := time.Since(startedAt).Milliseconds()
		_ = service.RecordTelegraphRun(startedAt, durationMs, fetched, imported, analyzed, lastErr)

		a.telegraphMu.Lock()
		defer a.telegraphMu.Unlock()
		if a.telegraphRunSeq != runSeq {
			return
		}
		a.telegraphCancel = nil
		s := &a.telegraphStatus
		s.Running = false
		s.LastRunAt = startedAt
		s.LastFetched = fetched
		s.LastImported = imported
		s.LastAnalyzed = analyzed
		s.LastError = lastErr
	}()

	if ctx.Err() != nil {
		lastErr = "任务已停止"
		return
	}

	items, err := service.FetchTelegraphNews(ctx, cfg.SourceURL, cfg.FetchLimit)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			lastErr = "任务已停止"
			return
		}
		lastErr = err.Error()
		log.Printf("[CLS] fetch telegraph failed: %s", err.Error())
		return
	}
	fetched = len(items)
	if len(items) == 0 {
		return
	}

	channel, prompt, err := resolveTelegraphAnalysisTarget(cfg.ChannelID, cfg.AnalysisPrompt)
	if err != nil {
		lastErr = err.Error()
		log.Printf("[CLS] resolve ai target failed: %s", err.Error())
		return
	}

	// Analyze from old to new for chronological readability.
	sort.Slice(items, func(i, j int) bool {
		return items[i].Published.Before(items[j].Published)
	})

	for _, item := range items {
		if ctx.Err() != nil {
			lastErr = "任务已停止"
			break
		}

		article, created, err := service.ImportTelegraphNews(item)
		if err != nil {
			lastErr = "导入电报失败: " + err.Error()
			log.Printf("[CLS] import news failed id=%d err=%s", item.NewsID, err.Error())
			continue
		}
		if !created {
			continue
		}
		imported++
		if err := service.RefreshTelegraphWatchHits(article.ID, article.Title, article.Content); err != nil {
			log.Printf("[CLS] refresh watch hits failed article=%d err=%s", article.ID, err.Error())
		}

		startedRunAt := time.Now()
		_ = service.UpdateArticleStatus(article.ID, 1)

		runCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		result, err := service.AnalyzeArticleDetailedWithContext(runCtx, *channel, prompt.Content, article.Content, service.AnalysisModeText, func(string) {})
		cancel()
		if err != nil {
			if errors.Is(runCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				_ = service.UpdateArticleStatus(article.ID, 0)
				lastErr = "任务已停止"
				break
			}
			a.refreshTelegraphMeta(article, "")
			_ = service.UpdateArticleStatus(article.ID, 0)
			lastErr = "AI 解读失败: " + err.Error()
			a.recordAnalysisRun(article.ID, channel, prompt, service.AnalysisModeText, result, classifyErrorReason(err), startedRunAt, false)
			log.Printf("[CLS] analyze failed article=%d news=%d err=%s", article.ID, item.NewsID, err.Error())
			continue
		}

		if err := service.UpdateArticleAnalysis(article.ID, result.Text, prompt.Name, channel.Name); err != nil {
			a.refreshTelegraphMeta(article, result.Text)
			_ = service.UpdateArticleStatus(article.ID, 0)
			lastErr = "保存解读失败: " + err.Error()
			a.recordAnalysisRun(article.ID, channel, prompt, service.AnalysisModeText, result, "save_error", startedRunAt, false)
			log.Printf("[CLS] save analysis failed article=%d news=%d err=%s", article.ID, item.NewsID, err.Error())
			continue
		}

		a.refreshTelegraphMeta(article, result.Text)
		a.recordAnalysisRun(article.ID, channel, prompt, service.AnalysisModeText, result, "", startedRunAt, true)
		analyzed++
	}

	if ctx.Err() == nil {
		if err := a.maybeGenerateTelegraphDigest(ctx, channel); err != nil {
			log.Printf("[CLS] digest failed: %s", err.Error())
			if lastErr == "" {
				lastErr = "盘中摘要生成失败"
			}
		}
	}

	log.Printf("[CLS] run finished fetched=%d imported=%d analyzed=%d", fetched, imported, analyzed)
}

func (a *App) refreshTelegraphMeta(article models.Article, analysis string) {
	score, direction, level := service.EvaluateTelegraphImportance(article.Title, article.Content, analysis)
	if err := service.UpsertTelegraphMeta(article.ID, score, direction, level); err != nil {
		log.Printf("[CLS] upsert meta failed article=%d err=%s", article.ID, err.Error())
		return
	}
	if err := service.AutoTagTelegraphArticle(article.ID, article.Title, article.Content, direction, level); err != nil {
		log.Printf("[CLS] auto tag failed article=%d err=%s", article.ID, err.Error())
	}

	alerted, err := service.MarkTelegraphAlertedIfNeeded(article.ID, telegraphAlertMinScore)
	if err != nil {
		log.Printf("[CLS] mark alert failed article=%d err=%s", article.ID, err.Error())
		return
	}
	if alerted && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "telegraph-alert", map[string]any{
			"articleId":  article.ID,
			"title":      article.Title,
			"score":      score,
			"direction":  direction,
			"level":      level,
			"createdAt":  article.CreatedAt,
			"sourceType": "news",
		})
	}
}

func (a *App) maybeGenerateTelegraphDigest(ctx context.Context, channel *models.AIChannel) error {
	if channel == nil {
		return nil
	}
	now := time.Now()
	slotEnd := now.Truncate(telegraphDigestInterval)
	slotStart := slotEnd.Add(-telegraphDigestInterval)

	exists, err := service.HasTelegraphDigestSlot(slotStart, slotEnd)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	items, err := service.GetTelegraphDigestSource(slotStart, slotEnd, 5)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("时间窗口: %s - %s\n", slotStart.Format("15:04"), slotEnd.Format("15:04")))
	for i, item := range items {
		b.WriteString(fmt.Sprintf("%d) [%s/%s/%d分] %s\n", i+1, item.ImpactDirection, impactLevelByScore(item.ImportanceScore), item.ImportanceScore, item.Title))
		if item.Analysis != "" {
			b.WriteString("解读: " + trimLocal(item.Analysis, 180) + "\n")
		}
	}

	digestPrompt := `你是A股盘中复盘分析师。请基于给定时间窗口内的高影响新闻，输出：
1) 市场主线（2-3点）
2) 影响路径（政策/行业/公司如何传导）
3) 风险提示（1-2点）
4) 接下来30分钟跟踪信号
要求：简洁、结构化、禁止编造。`

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, err := service.AnalyzeArticleDetailedWithContext(runCtx, *channel, digestPrompt, b.String(), service.AnalysisModeText, func(string) {})
	if err != nil {
		return err
	}

	totalScore := 0
	for _, item := range items {
		totalScore += item.ImportanceScore
	}
	avgScore := 0
	if len(items) > 0 {
		avgScore = totalScore / len(items)
	}
	if err := service.SaveTelegraphDigest(slotStart, slotEnd, res.Text, len(items), avgScore); err != nil {
		return err
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "telegraph-digest", map[string]any{
			"slotStart": slotStart,
			"slotEnd":   slotEnd,
			"summary":   trimLocal(res.Text, 220),
			"topItems":  len(items),
			"avgScore":  avgScore,
		})
	}
	return nil
}

func impactLevelByScore(score int) string {
	if score >= 80 {
		return "高影响"
	}
	if score >= 60 {
		return "中影响"
	}
	return "低影响"
}

func trimLocal(s string, limit int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= limit {
		return string(r)
	}
	return string(r[:limit]) + "..."
}

func resolveTelegraphAnalysisTarget(channelID int64, analysisPrompt string) (*models.AIChannel, *models.Prompt, error) {
	channels, err := service.GetChannels()
	if err != nil {
		return nil, nil, err
	}
	if len(channels) == 0 {
		return nil, nil, fmt.Errorf("请先在设置中配置 AI 渠道")
	}

	var channel *models.AIChannel
	if channelID > 0 {
		for i := range channels {
			if channels[i].ID == channelID {
				channel = &channels[i]
				break
			}
		}
	}
	if channel == nil {
		for i := range channels {
			if channels[i].IsDefault == 1 {
				channel = &channels[i]
				break
			}
		}
	}
	if channel == nil {
		channel = &channels[0]
	}

	promptContent := strings.TrimSpace(analysisPrompt)
	if promptContent == "" {
		cfg := service.DefaultTelegraphPrompt()
		promptContent = cfg
	}
	prompt := &models.Prompt{
		ID:      0,
		Name:    "财联社新闻专用提示词",
		Content: promptContent,
	}

	return channel, prompt, nil
}

func (a *App) updateTelegraphStatus(update func(*models.TelegraphSchedulerStatus)) {
	a.telegraphMu.Lock()
	defer a.telegraphMu.Unlock()
	update(&a.telegraphStatus)
}

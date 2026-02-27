package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

const telegraphSchedulerConfigKey = "telegraph_scheduler_config_v1"

var telegraphHTTPClient = &http.Client{
	Timeout: 20 * time.Second,
}

var stockCodeRegexp = regexp.MustCompile(`\b\d{6}\b`)

type TelegraphNews struct {
	NewsID    int64
	Title     string
	Content   string
	Published time.Time
}

type clsTelegraphPayload struct {
	Props struct {
		InitialState struct {
			RollData []struct {
				ID      int64  `json:"id"`
				CTime   int64  `json:"ctime"`
				Title   string `json:"title"`
				Content string `json:"content"`
				Brief   string `json:"brief"`
			} `json:"roll_data"`
		} `json:"initialState"`
	} `json:"props"`
}

func defaultTelegraphSchedulerConfig() models.TelegraphSchedulerConfig {
	return models.TelegraphSchedulerConfig{
		Enabled:         0,
		SourceURL:       "https://m.cls.cn/telegraph",
		IntervalMinutes: 10,
		FetchLimit:      8,
		ChannelID:       0,
		AnalysisPrompt: `你是资深A股盘中快讯分析师。请基于这条财联社电报，输出：
1) 事件一句话总结
2) 对市场影响方向（利多/利空/中性）与简短理由
3) 可能受影响板块/风格（若无法判断请明确说明）
4) 两个需要继续跟踪的关键信号
要求：仅基于给定内容，不编造。`,
	}
}

func DefaultTelegraphPrompt() string {
	return defaultTelegraphSchedulerConfig().AnalysisPrompt
}

func normalizeTelegraphSchedulerConfig(cfg models.TelegraphSchedulerConfig) models.TelegraphSchedulerConfig {
	def := defaultTelegraphSchedulerConfig()
	cfg.SourceURL = strings.TrimSpace(cfg.SourceURL)
	if cfg.SourceURL == "" {
		cfg.SourceURL = def.SourceURL
	}
	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = def.IntervalMinutes
	}
	if cfg.IntervalMinutes < 1 {
		cfg.IntervalMinutes = 1
	}
	if cfg.IntervalMinutes > 1440 {
		cfg.IntervalMinutes = 1440
	}
	if cfg.FetchLimit <= 0 {
		cfg.FetchLimit = def.FetchLimit
	}
	if cfg.FetchLimit < 1 {
		cfg.FetchLimit = 1
	}
	if cfg.FetchLimit > 50 {
		cfg.FetchLimit = 50
	}
	if cfg.Enabled != 1 {
		cfg.Enabled = 0
	}
	cfg.AnalysisPrompt = strings.TrimSpace(cfg.AnalysisPrompt)
	if cfg.AnalysisPrompt == "" {
		cfg.AnalysisPrompt = def.AnalysisPrompt
	}
	return cfg
}

func GetTelegraphSchedulerConfig() (models.TelegraphSchedulerConfig, error) {
	cfg := defaultTelegraphSchedulerConfig()

	var raw string
	err := db.DB.Get(&raw, "SELECT value FROM app_configs WHERE key=?", telegraphSchedulerConfigKey)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	var stored models.TelegraphSchedulerConfig
	if json.Unmarshal([]byte(raw), &stored) != nil {
		return cfg, nil
	}
	return normalizeTelegraphSchedulerConfig(stored), nil
}

func SaveTelegraphSchedulerConfig(cfg models.TelegraphSchedulerConfig) error {
	cfg = normalizeTelegraphSchedulerConfig(cfg)
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`
		INSERT INTO app_configs(key, value, updated_at)
		VALUES(?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
	`, telegraphSchedulerConfigKey, string(data))
	return err
}

func FetchTelegraphNews(ctx context.Context, sourceURL string, limit int) ([]TelegraphNews, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		sourceURL = defaultTelegraphSchedulerConfig().SourceURL
	}
	if limit <= 0 {
		limit = 8
	}

	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := telegraphHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("抓取失败: HTTP %d, %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	jsonText, err := extractNextDataJSON(string(body))
	if err != nil {
		return nil, err
	}

	var payload clsTelegraphPayload
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, fmt.Errorf("解析 telegraph 数据失败: %w", err)
	}

	rows := payload.Props.InitialState.RollData
	if len(rows) == 0 {
		return nil, errors.New("未获取到滚动电报数据")
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}

	items := make([]TelegraphNews, 0, len(rows))
	for _, row := range rows {
		content := strings.TrimSpace(row.Content)
		if content == "" {
			content = strings.TrimSpace(row.Brief)
		}
		if content == "" || row.ID <= 0 {
			continue
		}
		published := time.Time{}
		if row.CTime > 0 {
			published = time.Unix(row.CTime, 0)
		}
		items = append(items, TelegraphNews{
			NewsID:    row.ID,
			Title:     buildTelegraphTitle(row.Title, content, row.ID),
			Content:   content,
			Published: published,
		})
	}
	return items, nil
}

func ImportTelegraphNews(item TelegraphNews) (models.Article, bool, error) {
	if item.NewsID <= 0 {
		return models.Article{}, false, errors.New("news_id 无效")
	}
	if strings.TrimSpace(item.Content) == "" {
		return models.Article{}, false, errors.New("电报内容为空")
	}

	tx, err := db.DB.Beginx()
	if err != nil {
		return models.Article{}, false, err
	}
	defer tx.Rollback()

	var existingArticleID int64
	err = tx.Get(&existingArticleID, "SELECT article_id FROM telegraph_ingests WHERE news_id=?", item.NewsID)
	if err == nil && existingArticleID > 0 {
		var article models.Article
		if err := tx.Get(&article, "SELECT * FROM articles WHERE id=?", existingArticleID); err != nil {
			return models.Article{}, false, err
		}
		return article, false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return models.Article{}, false, err
	}

	res, err := tx.Exec("INSERT INTO articles(title,content,source,status) VALUES(?,?,?,?)",
		item.Title,
		item.Content,
		fmt.Sprintf("cls-telegraph:%d", item.NewsID),
		0,
	)
	if err != nil {
		return models.Article{}, false, err
	}
	articleID, err := res.LastInsertId()
	if err != nil {
		return models.Article{}, false, err
	}

	var publishedAt any
	if !item.Published.IsZero() {
		publishedAt = item.Published
	}
	if _, err := tx.Exec("INSERT INTO telegraph_ingests(news_id, article_id, published_at) VALUES(?,?,?)", item.NewsID, articleID, publishedAt); err != nil {
		return models.Article{}, false, err
	}

	var article models.Article
	if err := tx.Get(&article, "SELECT * FROM articles WHERE id=?", articleID); err != nil {
		return models.Article{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return models.Article{}, false, err
	}
	return article, true, nil
}

func extractNextDataJSON(html string) (string, error) {
	keyIdx := strings.Index(html, "__NEXT_DATA__")
	if keyIdx < 0 {
		return "", errors.New("页面中未找到 __NEXT_DATA__")
	}

	start := strings.Index(html[keyIdx:], "{")
	if start < 0 {
		return "", errors.New("__NEXT_DATA__ JSON 起始位置无效")
	}
	start += keyIdx

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(html); i++ {
		ch := html[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[start : i+1], nil
			}
		}
	}
	return "", errors.New("未能完整提取 __NEXT_DATA__ JSON")
}

func buildTelegraphTitle(title string, content string, newsID int64) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return trimRunes(title, 80)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Sprintf("财联社电报 #%d", newsID)
	}
	return trimRunes(content, 50)
}

func trimRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "..."
}

func EvaluateTelegraphImportance(title string, content string, analysis string) (int, string, string) {
	joined := strings.ToLower(strings.Join([]string{title, content, analysis}, " "))
	score := 35

	score += hitScore(joined, []string{"央行", "国务院", "财政部", "发改委", "证监会", "美联储", "降准", "降息", "加息", "货币政策", "产业政策"}, 24)
	score += hitScore(joined, []string{"cpi", "ppi", "gdp", "非农", "社融", "m1", "m2", "进出口", "失业率"}, 20)
	score += hitScore(joined, []string{"a股", "沪深", "上证", "创业板", "港股", "美股", "人民币汇率", "国债收益率", "原油", "黄金"}, 16)
	score += hitScore(joined, []string{"半导体", "算力", "ai", "光伏", "锂电", "新能源", "医药", "地产", "军工", "券商", "银行", "煤炭", "有色", "化工", "汽车"}, 12)
	score += hitScore(joined, []string{"公告", "业绩", "预告", "增持", "减持", "并购", "重组", "中标", "回购", "停牌", "复牌"}, 10)
	score += hitScore(joined, []string{"突发", "紧急", "创历史新高", "跌停", "涨停", "大幅", "超预期"}, 14)
	score += hitScore(joined, []string{"%", "％"}, 6)
	if len([]rune(strings.TrimSpace(content))) > 120 {
		score += 4
	}

	if score < 1 {
		score = 1
	}
	if score > 100 {
		score = 100
	}

	direction := detectTelegraphDirection(joined)
	level := "低影响"
	if score >= 80 {
		level = "高影响"
	} else if score >= 60 {
		level = "中影响"
	}
	return score, direction, level
}

func hitScore(text string, keywords []string, score int) int {
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			return score
		}
	}
	return 0
}

func detectTelegraphDirection(text string) string {
	positiveWords := []string{"上调", "增长", "超预期", "回购", "增持", "利好", "盈利", "突破", "提振", "改善", "修复", "上涨", "涨停", "降息", "降准", "中标"}
	negativeWords := []string{"下调", "下滑", "低于预期", "减持", "亏损", "利空", "违约", "处罚", "调查", "风险", "下跌", "跌停", "裁员", "暂停", "暴雷"}

	pos := 0
	neg := 0
	for _, word := range positiveWords {
		pos += strings.Count(text, word)
	}
	for _, word := range negativeWords {
		neg += strings.Count(text, word)
	}

	if pos-neg >= 2 {
		return "利多"
	}
	if neg-pos >= 2 {
		return "利空"
	}
	return "中性"
}

func UpsertTelegraphMeta(articleID int64, score int, direction string, level string) error {
	if articleID <= 0 {
		return nil
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	direction = strings.TrimSpace(direction)
	if direction == "" {
		direction = "中性"
	}
	level = strings.TrimSpace(level)
	if level == "" {
		level = "低影响"
	}

	_, err := db.DB.Exec(`
		INSERT INTO telegraph_meta(article_id, importance_score, impact_direction, impact_level, updated_at)
		VALUES(?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(article_id) DO UPDATE SET
			importance_score=excluded.importance_score,
			impact_direction=excluded.impact_direction,
			impact_level=excluded.impact_level,
			updated_at=CURRENT_TIMESTAMP
	`, articleID, score, direction, level)
	return err
}

func MarkTelegraphAlertedIfNeeded(articleID int64, minScore int) (bool, error) {
	if articleID <= 0 || minScore <= 0 {
		return false, nil
	}
	res, err := db.DB.Exec(`
		UPDATE telegraph_meta
		SET alerted=1, updated_at=CURRENT_TIMESTAMP
		WHERE article_id=? AND alerted=0 AND importance_score>=?
	`, articleID, minScore)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func AutoTagTelegraphArticle(articleID int64, title string, content string, direction string, level string) error {
	if articleID <= 0 {
		return nil
	}

	text := strings.ToLower(strings.Join([]string{title, content}, " "))
	type tagItem struct {
		name  string
		color string
	}
	tags := []tagItem{
		{name: "新闻电报", color: "#14b8a6"},
	}

	macroKeys := []string{"央行", "国务院", "财政部", "发改委", "美联储", "cpi", "ppi", "gdp", "社融"}
	industryKeys := []string{"半导体", "算力", "ai", "光伏", "锂电", "新能源", "医药", "地产", "军工", "券商", "银行", "煤炭", "有色", "化工", "汽车"}
	companyKeys := []string{"股份", "有限公司", "公告", "董事会", "中标", "业绩预告"}

	if hasAny(text, macroKeys) {
		tags = append(tags, tagItem{name: "宏观", color: "#0ea5e9"})
	}
	if hasAny(text, industryKeys) {
		tags = append(tags, tagItem{name: "行业", color: "#8b5cf6"})
	}
	if hasAny(text, companyKeys) || stockCodeRegexp.MatchString(text) {
		tags = append(tags, tagItem{name: "公司", color: "#f59e0b"})
	}

	switch direction {
	case "利多":
		tags = append(tags, tagItem{name: "利多", color: "#10b981"})
	case "利空":
		tags = append(tags, tagItem{name: "利空", color: "#ef4444"})
	default:
		tags = append(tags, tagItem{name: "中性", color: "#6b7280"})
	}
	switch level {
	case "高影响":
		tags = append(tags, tagItem{name: "高影响", color: "#dc2626"})
	case "中影响":
		tags = append(tags, tagItem{name: "中影响", color: "#f97316"})
	default:
		tags = append(tags, tagItem{name: "低影响", color: "#64748b"})
	}

	for _, tag := range tags {
		tagID, err := EnsureTag(tag.name, tag.color)
		if err != nil {
			return err
		}
		if err := AddTagToArticle(articleID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func hasAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func RecordTelegraphRun(startedAt time.Time, durationMs int64, fetched int, imported int, analyzed int, errorReason string) error {
	success := 1
	errorReason = normalizeTelegraphErrorReason(errorReason)
	if errorReason != "" {
		success = 0
	}
	_, err := db.DB.Exec(`
		INSERT INTO telegraph_runs(
			started_at, duration_ms, fetched, imported, analyzed, success, error_reason
		) VALUES(?,?,?,?,?,?,?)
	`, startedAt, durationMs, fetched, imported, analyzed, success, errorReason)
	return err
}

func normalizeTelegraphErrorReason(reason string) string {
	r := strings.ToLower(strings.TrimSpace(reason))
	if r == "" {
		return ""
	}
	switch {
	case strings.Contains(r, "停止"), strings.Contains(r, "cancel"), strings.Contains(r, "canceled"):
		return "stopped"
	case strings.Contains(r, "超时"), strings.Contains(r, "timeout"), strings.Contains(r, "deadline"):
		return "timeout"
	case strings.Contains(r, "401"), strings.Contains(r, "403"), strings.Contains(r, "unauthorized"), strings.Contains(r, "api"):
		return "auth"
	case strings.Contains(r, "429"), strings.Contains(r, "rate"):
		return "rate_limit"
	case strings.Contains(r, "500"), strings.Contains(r, "502"), strings.Contains(r, "503"), strings.Contains(r, "504"):
		return "server"
	case strings.Contains(r, "network"), strings.Contains(r, "connection"), strings.Contains(r, "dial"):
		return "network"
	case strings.Contains(r, "解析"), strings.Contains(r, "json"), strings.Contains(r, "抓取"):
		return "fetch_parse"
	default:
		return "other"
	}
}

func GetTelegraphDashboardByDays(days int) (models.TelegraphDashboard, error) {
	dashboard := models.TelegraphDashboard{}
	if days < 0 {
		days = 0
	}

	clause, args := buildTelegraphTimeFilter(days)
	total := struct {
		TotalRuns    int64         `db:"total_runs"`
		TotalFetched sql.NullInt64 `db:"total_fetched"`
		TotalImport  sql.NullInt64 `db:"total_imported"`
		TotalAnalyze sql.NullInt64 `db:"total_analyzed"`
		AvgDuration  sql.NullInt64 `db:"avg_duration_ms"`
	}{}
	if err := db.DB.Get(&total, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_runs,
			SUM(fetched) AS total_fetched,
			SUM(imported) AS total_imported,
			SUM(analyzed) AS total_analyzed,
			AVG(duration_ms) AS avg_duration_ms
		FROM telegraph_runs
		%s
	`, clause), args...); err != nil {
		return dashboard, err
	}

	dashboard.TotalRuns = int(total.TotalRuns)
	dashboard.TotalFetched = int(total.TotalFetched.Int64)
	dashboard.TotalImported = int(total.TotalImport.Int64)
	dashboard.TotalAnalyzed = int(total.TotalAnalyze.Int64)
	dashboard.AvgDurationMs = total.AvgDuration.Int64
	if dashboard.TotalImported <= 0 {
		dashboard.SuccessRate = "0%"
	} else {
		dashboard.SuccessRate = fmt.Sprintf("%.1f%%", float64(dashboard.TotalAnalyzed)*100/float64(dashboard.TotalImported))
	}

	reasonRows := []struct {
		Reason string `db:"reason"`
		Count  int64  `db:"count"`
	}{}
	reasonClause, reasonArgs := buildTelegraphFailureFilter(days)
	if err := db.DB.Select(&reasonRows, fmt.Sprintf(`
		SELECT error_reason AS reason, COUNT(*) AS count
		FROM telegraph_runs
		%s
		GROUP BY error_reason
		ORDER BY count DESC, reason ASC
		LIMIT 8
	`, reasonClause), reasonArgs...); err != nil {
		return dashboard, err
	}

	dashboard.FailureTop = make([]models.FailureReasonMetric, 0, len(reasonRows))
	for _, row := range reasonRows {
		dashboard.FailureTop = append(dashboard.FailureTop, models.FailureReasonMetric{
			Reason: row.Reason,
			Count:  int(row.Count),
		})
	}

	return dashboard, nil
}

func buildTelegraphTimeFilter(days int) (string, []any) {
	if days <= 0 {
		return "", nil
	}
	return "WHERE started_at >= datetime('now', ?)", []any{fmt.Sprintf("-%d day", days)}
}

func buildTelegraphFailureFilter(days int) (string, []any) {
	if days <= 0 {
		return "WHERE success = 0 AND error_reason <> ''", nil
	}
	return "WHERE started_at >= datetime('now', ?) AND success = 0 AND error_reason <> ''", []any{fmt.Sprintf("-%d day", days)}
}

type TelegraphDigestSource struct {
	ArticleID       int64     `db:"article_id"`
	Title           string    `db:"title"`
	Content         string    `db:"content"`
	Analysis        string    `db:"analysis"`
	ImportanceScore int       `db:"importance_score"`
	ImpactDirection string    `db:"impact_direction"`
	CreatedAt       time.Time `db:"created_at"`
}

func GetTelegraphDigestSource(slotStart time.Time, slotEnd time.Time, limit int) ([]TelegraphDigestSource, error) {
	if limit <= 0 {
		limit = 5
	}
	var rows []TelegraphDigestSource
	err := db.DB.Select(&rows, `
		SELECT
			a.id AS article_id,
			a.title,
			a.content,
			a.analysis,
			COALESCE(tm.importance_score, 0) AS importance_score,
			COALESCE(tm.impact_direction, '中性') AS impact_direction,
			a.created_at
		FROM articles a
		LEFT JOIN telegraph_meta tm ON a.id = tm.article_id
		WHERE a.source LIKE ?
			AND a.created_at >= ?
			AND a.created_at < ?
		ORDER BY COALESCE(tm.importance_score, 0) DESC, a.id DESC
		LIMIT ?
	`, telegraphSourcePrefixLike, slotStart, slotEnd, limit)
	return rows, err
}

func SaveTelegraphDigest(slotStart time.Time, slotEnd time.Time, summary string, topItems int, avgScore int) error {
	_, err := db.DB.Exec(`
		INSERT INTO telegraph_digests(slot_start, slot_end, summary, top_items, avg_score)
		VALUES(?,?,?,?,?)
		ON CONFLICT(slot_start, slot_end) DO UPDATE SET
			summary=excluded.summary,
			top_items=excluded.top_items,
			avg_score=excluded.avg_score,
			created_at=CURRENT_TIMESTAMP
	`, slotStart, slotEnd, strings.TrimSpace(summary), topItems, avgScore)
	return err
}

func HasTelegraphDigestSlot(slotStart time.Time, slotEnd time.Time) (bool, error) {
	var n int
	if err := db.DB.Get(&n, "SELECT COUNT(1) FROM telegraph_digests WHERE slot_start=? AND slot_end=?", slotStart, slotEnd); err != nil {
		return false, err
	}
	return n > 0, nil
}

func GetTelegraphDigests(limit int) ([]models.TelegraphDigest, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 50 {
		limit = 50
	}
	var items []models.TelegraphDigest
	err := db.DB.Select(&items, `
		SELECT id, slot_start, slot_end, summary, top_items, avg_score, created_at
		FROM telegraph_digests
		ORDER BY slot_end DESC, id DESC
		LIMIT ?
	`, limit)
	return items, err
}

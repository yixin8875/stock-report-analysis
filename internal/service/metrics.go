package service

import (
	"database/sql"
	"fmt"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

func RecordAnalysisRun(run models.AnalysisRun) error {
	_, err := db.DB.Exec(`
		INSERT INTO analysis_runs(
			article_id, channel_id, channel_name, prompt_id, prompt_name, mode,
			success, error_reason, duration_ms, prompt_tokens, completion_tokens, total_tokens
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		run.ArticleID,
		run.ChannelID,
		run.ChannelName,
		run.PromptID,
		run.PromptName,
		run.Mode,
		run.Success,
		run.ErrorReason,
		run.DurationMs,
		run.PromptTokens,
		run.CompletionTokens,
		run.TotalTokens,
	)
	return err
}

func GetAnalysisDashboard() (models.AnalysisDashboard, error) {
	return GetAnalysisDashboardByDays(0)
}

func GetAnalysisDashboardByDays(days int) (models.AnalysisDashboard, error) {
	dashboard := models.AnalysisDashboard{}
	if days < 0 {
		days = 0
	}
	clause, args := buildTimeFilter(days)

	totals := struct {
		TotalRuns     int64         `db:"total_runs"`
		SuccessRuns   sql.NullInt64 `db:"success_runs"`
		FailedRuns    sql.NullInt64 `db:"failed_runs"`
		AvgDurationMs sql.NullInt64 `db:"avg_duration_ms"`
		TotalTokens   sql.NullInt64 `db:"total_tokens"`
		PromptTokens  sql.NullInt64 `db:"prompt_tokens"`
		OutputTokens  sql.NullInt64 `db:"output_tokens"`
	}{}
	if err := db.DB.Get(&totals, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_runs,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS success_runs,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) AS failed_runs,
			AVG(duration_ms) AS avg_duration_ms,
			SUM(total_tokens) AS total_tokens,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS output_tokens
		FROM analysis_runs
		%s
	`, clause), args...); err != nil {
		return dashboard, err
	}

	dashboard.TotalRuns = int(totals.TotalRuns)
	dashboard.SuccessRuns = int(totals.SuccessRuns.Int64)
	dashboard.FailedRuns = int(totals.FailedRuns.Int64)
	dashboard.AvgDurationMs = totals.AvgDurationMs.Int64
	dashboard.TotalTokens = totals.TotalTokens.Int64
	dashboard.PromptTokens = totals.PromptTokens.Int64
	dashboard.OutputTokens = totals.OutputTokens.Int64
	dashboard.SuccessRate = percentage(dashboard.SuccessRuns, dashboard.TotalRuns)

	channelRows := []struct {
		ChannelID    int64         `db:"channel_id"`
		ChannelName  string        `db:"channel_name"`
		TotalRuns    int64         `db:"total_runs"`
		SuccessRuns  sql.NullInt64 `db:"success_runs"`
		FailedRuns   sql.NullInt64 `db:"failed_runs"`
		AvgDuration  sql.NullInt64 `db:"avg_duration"`
		TotalTokens  sql.NullInt64 `db:"total_tokens"`
		PromptTokens sql.NullInt64 `db:"prompt_tokens"`
		OutputTokens sql.NullInt64 `db:"output_tokens"`
	}{}
	if err := db.DB.Select(&channelRows, fmt.Sprintf(`
		SELECT
			channel_id,
			channel_name,
			COUNT(*) AS total_runs,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS success_runs,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) AS failed_runs,
			AVG(duration_ms) AS avg_duration,
			SUM(total_tokens) AS total_tokens,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS output_tokens
		FROM analysis_runs
		%s
		GROUP BY channel_id, channel_name
		ORDER BY total_runs DESC, channel_name ASC
	`, clause), args...); err != nil {
		return dashboard, err
	}

	dashboard.ByChannel = make([]models.ChannelMetric, 0, len(channelRows))
	for _, row := range channelRows {
		totalRuns := int(row.TotalRuns)
		successRuns := int(row.SuccessRuns.Int64)
		dashboard.ByChannel = append(dashboard.ByChannel, models.ChannelMetric{
			ChannelID:    row.ChannelID,
			ChannelName:  row.ChannelName,
			TotalRuns:    totalRuns,
			SuccessRuns:  successRuns,
			FailedRuns:   int(row.FailedRuns.Int64),
			SuccessRate:  percentage(successRuns, totalRuns),
			AvgDuration:  row.AvgDuration.Int64,
			TotalTokens:  row.TotalTokens.Int64,
			PromptTokens: row.PromptTokens.Int64,
			OutputTokens: row.OutputTokens.Int64,
		})
	}

	reasonRows := []struct {
		Reason string `db:"reason"`
		Count  int64  `db:"count"`
	}{}
	reasonClause, reasonArgs := buildFailureFilter(days)
	if err := db.DB.Select(&reasonRows, fmt.Sprintf(`
		SELECT error_reason AS reason, COUNT(*) AS count
		FROM analysis_runs
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

func percentage(num int, den int) string {
	if den <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", float64(num)*100/float64(den))
}

func buildTimeFilter(days int) (string, []any) {
	if days <= 0 {
		return "", nil
	}
	return "WHERE created_at >= datetime('now', ?)", []any{fmt.Sprintf("-%d day", days)}
}

func buildFailureFilter(days int) (string, []any) {
	if days <= 0 {
		return "WHERE success = 0 AND error_reason <> ''", nil
	}
	return "WHERE created_at >= datetime('now', ?) AND success = 0 AND error_reason <> ''", []any{fmt.Sprintf("-%d day", days)}
}

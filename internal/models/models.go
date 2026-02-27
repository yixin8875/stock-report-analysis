package models

import "time"

type AIChannel struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	BaseURL   string    `db:"base_url" json:"baseUrl"`
	APIKey    string    `db:"api_key" json:"apiKey"`
	Model     string    `db:"model" json:"model"`
	IsDefault int       `db:"is_default" json:"isDefault"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type Prompt struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Content   string    `db:"content" json:"content"`
	IsDefault int       `db:"is_default" json:"isDefault"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type PromptVersion struct {
	ID        int64     `db:"id" json:"id"`
	PromptID  int64     `db:"prompt_id" json:"promptId"`
	VersionNo int       `db:"version_no" json:"versionNo"`
	Name      string    `db:"name" json:"name"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type TelegraphSchedulerConfig struct {
	Enabled         int    `json:"enabled"`
	SourceURL       string `json:"sourceUrl"`
	IntervalMinutes int    `json:"intervalMinutes"`
	FetchLimit      int    `json:"fetchLimit"`
	ChannelID       int64  `json:"channelId"`
	AnalysisPrompt  string `json:"analysisPrompt"`
}

type TelegraphSchedulerStatus struct {
	Running      bool      `json:"running"`
	LastRunAt    time.Time `json:"lastRunAt"`
	LastError    string    `json:"lastError"`
	LastFetched  int       `json:"lastFetched"`
	LastImported int       `json:"lastImported"`
	LastAnalyzed int       `json:"lastAnalyzed"`
}

type TelegraphArticleItem struct {
	ID              int64                 `db:"id" json:"id"`
	Title           string                `db:"title" json:"title"`
	Source          string                `db:"source" json:"source"`
	Status          int                   `db:"status" json:"status"`
	CreatedAt       time.Time             `db:"created_at" json:"createdAt"`
	AnalyzedAt      *time.Time            `db:"analyzed_at" json:"analyzedAt"`
	ImportanceScore int                   `db:"importance_score" json:"importanceScore"`
	ImpactDirection string                `db:"impact_direction" json:"impactDirection"`
	ImpactLevel     string                `db:"impact_level" json:"impactLevel"`
	WatchMatched    int                   `db:"watch_matched" json:"watchMatched"`
	WatchMatches    []TelegraphWatchMatch `db:"-" json:"watchMatches"`
	Tags            []Tag                 `db:"-" json:"tags"`
}

type WatchStock struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type TelegraphWatchMatch struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type TelegraphDigest struct {
	ID        int64     `db:"id" json:"id"`
	SlotStart time.Time `db:"slot_start" json:"slotStart"`
	SlotEnd   time.Time `db:"slot_end" json:"slotEnd"`
	Summary   string    `db:"summary" json:"summary"`
	TopItems  int       `db:"top_items" json:"topItems"`
	AvgScore  int       `db:"avg_score" json:"avgScore"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type TelegraphDashboard struct {
	TotalRuns     int                   `json:"totalRuns"`
	TotalFetched  int                   `json:"totalFetched"`
	TotalImported int                   `json:"totalImported"`
	TotalAnalyzed int                   `json:"totalAnalyzed"`
	SuccessRate   string                `json:"successRate"`
	AvgDurationMs int64                 `json:"avgDurationMs"`
	FailureTop    []FailureReasonMetric `json:"failureTop"`
}

type Article struct {
	ID          int64      `db:"id" json:"id"`
	Title       string     `db:"title" json:"title"`
	Content     string     `db:"content" json:"content"`
	Source      string     `db:"source" json:"source"`
	Analysis    string     `db:"analysis" json:"analysis"`
	PromptUsed  string     `db:"prompt_used" json:"promptUsed"`
	ChannelUsed string     `db:"channel_used" json:"channelUsed"`
	Status      int        `db:"status" json:"status"` // 0=待解读 1=解读中 2=已解读
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	AnalyzedAt  *time.Time `db:"analyzed_at" json:"analyzedAt"`
	Tags        []Tag      `db:"-" json:"tags"`
}

type Tag struct {
	ID    int64  `db:"id" json:"id"`
	Name  string `db:"name" json:"name"`
	Color string `db:"color" json:"color"`
}

type AnalysisHistory struct {
	ID          int64     `db:"id" json:"id"`
	ArticleID   int64     `db:"article_id" json:"articleId"`
	Analysis    string    `db:"analysis" json:"analysis"`
	PromptUsed  string    `db:"prompt_used" json:"promptUsed"`
	ChannelUsed string    `db:"channel_used" json:"channelUsed"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

type BatchFailure struct {
	ArticleID int64     `json:"articleId"`
	Title     string    `json:"title"`
	Reason    string    `json:"reason"`
	At        time.Time `json:"at"`
}

type BatchStatus struct {
	Running     bool           `json:"running"`
	Paused      bool           `json:"paused"`
	Total       int            `json:"total"`
	Completed   int            `json:"completed"`
	Success     int            `json:"success"`
	Failed      int            `json:"failed"`
	InProgress  int            `json:"inProgress"`
	Concurrency int            `json:"concurrency"`
	Failures    []BatchFailure `json:"failures"`
}

type AnalysisRun struct {
	ID               int64     `db:"id" json:"id"`
	ArticleID        int64     `db:"article_id" json:"articleId"`
	ChannelID        int64     `db:"channel_id" json:"channelId"`
	ChannelName      string    `db:"channel_name" json:"channelName"`
	PromptID         int64     `db:"prompt_id" json:"promptId"`
	PromptName       string    `db:"prompt_name" json:"promptName"`
	Mode             string    `db:"mode" json:"mode"`
	Success          int       `db:"success" json:"success"`
	ErrorReason      string    `db:"error_reason" json:"errorReason"`
	DurationMs       int64     `db:"duration_ms" json:"durationMs"`
	PromptTokens     int       `db:"prompt_tokens" json:"promptTokens"`
	CompletionTokens int       `db:"completion_tokens" json:"completionTokens"`
	TotalTokens      int       `db:"total_tokens" json:"totalTokens"`
	CreatedAt        time.Time `db:"created_at" json:"createdAt"`
}

type ChannelMetric struct {
	ChannelID    int64  `json:"channelId"`
	ChannelName  string `json:"channelName"`
	TotalRuns    int    `json:"totalRuns"`
	SuccessRuns  int    `json:"successRuns"`
	FailedRuns   int    `json:"failedRuns"`
	SuccessRate  string `json:"successRate"`
	AvgDuration  int64  `json:"avgDuration"`
	TotalTokens  int64  `json:"totalTokens"`
	PromptTokens int64  `json:"promptTokens"`
	OutputTokens int64  `json:"outputTokens"`
}

type FailureReasonMetric struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type AnalysisDashboard struct {
	TotalRuns     int                   `json:"totalRuns"`
	SuccessRuns   int                   `json:"successRuns"`
	FailedRuns    int                   `json:"failedRuns"`
	SuccessRate   string                `json:"successRate"`
	AvgDurationMs int64                 `json:"avgDurationMs"`
	TotalTokens   int64                 `json:"totalTokens"`
	PromptTokens  int64                 `json:"promptTokens"`
	OutputTokens  int64                 `json:"outputTokens"`
	ByChannel     []ChannelMetric       `json:"byChannel"`
	FailureTop    []FailureReasonMetric `json:"failureTop"`
}

type Role struct {
	ID            int64     `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	Alias         string    `db:"alias" json:"alias"`
	DomainTags    string    `db:"domain_tags" json:"domainTags"`
	SystemPrompt  string    `db:"system_prompt" json:"systemPrompt"`
	ModelOverride string    `db:"model_override" json:"modelOverride"`
	Temperature   float64   `db:"temperature" json:"temperature"`
	MaxTokens     int       `db:"max_tokens" json:"maxTokens"`
	Enabled       int       `db:"enabled" json:"enabled"`
	IsDefault     int       `db:"is_default" json:"isDefault"`
	CreatedAt     time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time `db:"updated_at" json:"updatedAt"`
}

type QASession struct {
	ID        int64     `db:"id" json:"id"`
	ArticleID int64     `db:"article_id" json:"articleId"`
	Title     string    `db:"title" json:"title"`
	Summary   string    `db:"summary" json:"summary"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type QAPin struct {
	ID              int64     `db:"id" json:"id"`
	SessionID       int64     `db:"session_id" json:"sessionId"`
	ArticleID       int64     `db:"article_id" json:"articleId"`
	SourceMessageID int64     `db:"source_message_id" json:"sourceMessageId"`
	Content         string    `db:"content" json:"content"`
	CreatedAt       time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `db:"updated_at" json:"updatedAt"`
}

type QAEvidence struct {
	ID         int64  `db:"id" json:"id"`
	MessageID  int64  `db:"message_id" json:"messageId"`
	ChunkIndex int    `db:"chunk_index" json:"chunkIndex"`
	Quote      string `db:"quote" json:"quote"`
	Reason     string `db:"reason" json:"reason"`
}

type QAMessage struct {
	ID               int64        `db:"id" json:"id"`
	SessionID        int64        `db:"session_id" json:"sessionId"`
	ArticleID        int64        `db:"article_id" json:"articleId"`
	ParentID         int64        `db:"parent_id" json:"parentId"`
	RoleType         string       `db:"role_type" json:"roleType"`
	RoleID           int64        `db:"role_id" json:"roleId"`
	RoleName         string       `db:"role_name" json:"roleName"`
	Content          string       `db:"content" json:"content"`
	Status           string       `db:"status" json:"status"`
	ErrorReason      string       `db:"error_reason" json:"errorReason"`
	DurationMs       int64        `db:"duration_ms" json:"durationMs"`
	PromptTokens     int          `db:"prompt_tokens" json:"promptTokens"`
	CompletionTokens int          `db:"completion_tokens" json:"completionTokens"`
	TotalTokens      int          `db:"total_tokens" json:"totalTokens"`
	CreatedAt        time.Time    `db:"created_at" json:"createdAt"`
	Evidences        []QAEvidence `db:"-" json:"evidences"`
}

type QARun struct {
	ID               int64     `db:"id" json:"id"`
	SessionID        int64     `db:"session_id" json:"sessionId"`
	MessageID        int64     `db:"message_id" json:"messageId"`
	ArticleID        int64     `db:"article_id" json:"articleId"`
	RoleID           int64     `db:"role_id" json:"roleId"`
	RoleName         string    `db:"role_name" json:"roleName"`
	Success          int       `db:"success" json:"success"`
	ErrorReason      string    `db:"error_reason" json:"errorReason"`
	DurationMs       int64     `db:"duration_ms" json:"durationMs"`
	PromptTokens     int       `db:"prompt_tokens" json:"promptTokens"`
	CompletionTokens int       `db:"completion_tokens" json:"completionTokens"`
	TotalTokens      int       `db:"total_tokens" json:"totalTokens"`
	CreatedAt        time.Time `db:"created_at" json:"createdAt"`
}

type QARoleMetric struct {
	RoleID       int64  `json:"roleId"`
	RoleName     string `json:"roleName"`
	TotalRuns    int    `json:"totalRuns"`
	SuccessRuns  int    `json:"successRuns"`
	FailedRuns   int    `json:"failedRuns"`
	SuccessRate  string `json:"successRate"`
	AvgDuration  int64  `json:"avgDuration"`
	TotalTokens  int64  `json:"totalTokens"`
	PromptTokens int64  `json:"promptTokens"`
	OutputTokens int64  `json:"outputTokens"`
}

type QADashboard struct {
	TotalRuns     int                   `json:"totalRuns"`
	SuccessRuns   int                   `json:"successRuns"`
	FailedRuns    int                   `json:"failedRuns"`
	SuccessRate   string                `json:"successRate"`
	AvgDurationMs int64                 `json:"avgDurationMs"`
	TotalTokens   int64                 `json:"totalTokens"`
	PromptTokens  int64                 `json:"promptTokens"`
	OutputTokens  int64                 `json:"outputTokens"`
	ByRole        []QARoleMetric        `json:"byRole"`
	FailureTop    []FailureReasonMetric `json:"failureTop"`
}

type MinerUConfig struct {
	Enabled        int    `json:"enabled"`
	BaseURL        string `json:"baseUrl"`
	APIToken       string `json:"apiToken"`
	ModelVersion   string `json:"modelVersion"`
	IsOCR          int    `json:"isOCR"`
	PollIntervalMs int    `json:"pollIntervalMs"`
	TimeoutSec     int    `json:"timeoutSec"`
}

type RoleTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Alias        string `json:"alias"`
	DomainTags   string `json:"domainTags"`
	SystemPrompt string `json:"systemPrompt"`
}

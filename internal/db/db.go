package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var DB *sqlx.DB

func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".stock-report-analysis")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", filepath.Join(dir, "data.db"))
	DB, err = sqlx.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	DB.SetMaxOpenConns(1)
	if err := DB.Ping(); err != nil {
		return err
	}

	return migrate()
}

func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS ai_channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		base_url TEXT NOT NULL,
		api_key TEXT NOT NULL,
		model TEXT NOT NULL,
		is_default INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS prompts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		content TEXT NOT NULL,
		is_default INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS prompt_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt_id INTEGER NOT NULL REFERENCES prompts(id) ON DELETE CASCADE,
		version_no INTEGER NOT NULL,
		name TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(prompt_id, version_no)
	);
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		source TEXT DEFAULT '',
		analysis TEXT DEFAULT '',
		prompt_used TEXT DEFAULT '',
		channel_used TEXT DEFAULT '',
		status INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		analyzed_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		color TEXT DEFAULT '#6b7280'
	);
	CREATE TABLE IF NOT EXISTS article_tags (
		article_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		PRIMARY KEY(article_id, tag_id),
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE,
		FOREIGN KEY(tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS analysis_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		analysis TEXT NOT NULL,
		prompt_used TEXT DEFAULT '',
		channel_used TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS analysis_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		channel_id INTEGER NOT NULL,
		channel_name TEXT NOT NULL,
		prompt_id INTEGER NOT NULL,
		prompt_name TEXT NOT NULL,
		mode TEXT DEFAULT 'text',
		success INTEGER NOT NULL,
		error_reason TEXT DEFAULT '',
		duration_ms INTEGER DEFAULT 0,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS app_configs (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS roles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		alias TEXT DEFAULT '',
		domain_tags TEXT DEFAULT '',
		system_prompt TEXT NOT NULL,
		model_override TEXT DEFAULT '',
		temperature REAL DEFAULT 0.2,
		max_tokens INTEGER DEFAULT 1200,
		enabled INTEGER DEFAULT 1,
		is_default INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS qa_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		title TEXT DEFAULT '',
		summary TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS qa_pins (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL REFERENCES qa_sessions(id) ON DELETE CASCADE,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		source_message_id INTEGER DEFAULT 0,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS qa_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL REFERENCES qa_sessions(id) ON DELETE CASCADE,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		parent_id INTEGER DEFAULT 0,
		role_type TEXT NOT NULL,
		role_id INTEGER DEFAULT 0,
		content TEXT NOT NULL,
		status TEXT DEFAULT 'done',
		error_reason TEXT DEFAULT '',
		duration_ms INTEGER DEFAULT 0,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS qa_evidences (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id INTEGER NOT NULL REFERENCES qa_messages(id) ON DELETE CASCADE,
		chunk_index INTEGER NOT NULL,
		quote TEXT DEFAULT '',
		reason TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS qa_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL REFERENCES qa_sessions(id) ON DELETE CASCADE,
		message_id INTEGER NOT NULL REFERENCES qa_messages(id) ON DELETE CASCADE,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
		role_name TEXT NOT NULL,
		success INTEGER NOT NULL,
		error_reason TEXT DEFAULT '',
		duration_ms INTEGER DEFAULT 0,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS telegraph_ingests (
		news_id INTEGER PRIMARY KEY,
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		published_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS telegraph_meta (
		article_id INTEGER PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
		importance_score INTEGER DEFAULT 0,
		impact_direction TEXT DEFAULT '中性',
		impact_level TEXT DEFAULT '低',
		alerted INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS telegraph_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at DATETIME NOT NULL,
		duration_ms INTEGER DEFAULT 0,
		fetched INTEGER DEFAULT 0,
		imported INTEGER DEFAULT 0,
		analyzed INTEGER DEFAULT 0,
		success INTEGER DEFAULT 1,
		error_reason TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS telegraph_digests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slot_start DATETIME NOT NULL,
		slot_end DATETIME NOT NULL,
		summary TEXT NOT NULL,
		top_items INTEGER DEFAULT 0,
		avg_score INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(slot_start, slot_end)
	);
	CREATE TABLE IF NOT EXISTS telegraph_watch_hits (
		article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
		stock_code TEXT NOT NULL,
		stock_name TEXT NOT NULL,
		match_type TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(article_id, stock_code)
	);
	CREATE INDEX IF NOT EXISTS idx_article_tags_article_id ON article_tags(article_id);
	CREATE INDEX IF NOT EXISTS idx_article_tags_tag_id ON article_tags(tag_id);
	CREATE INDEX IF NOT EXISTS idx_analysis_history_article_id ON analysis_history(article_id);
	CREATE INDEX IF NOT EXISTS idx_analysis_runs_created_at ON analysis_runs(created_at);
	CREATE INDEX IF NOT EXISTS idx_analysis_runs_channel_id ON analysis_runs(channel_id);
	CREATE INDEX IF NOT EXISTS idx_analysis_runs_success ON analysis_runs(success);
	CREATE INDEX IF NOT EXISTS idx_prompt_versions_prompt_id ON prompt_versions(prompt_id);
	CREATE INDEX IF NOT EXISTS idx_roles_enabled_default ON roles(enabled, is_default);
	CREATE INDEX IF NOT EXISTS idx_qa_sessions_article_id ON qa_sessions(article_id);
	CREATE INDEX IF NOT EXISTS idx_qa_pins_session_id ON qa_pins(session_id);
	CREATE INDEX IF NOT EXISTS idx_qa_messages_session_id ON qa_messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_qa_messages_created_at ON qa_messages(created_at);
	CREATE INDEX IF NOT EXISTS idx_qa_evidences_message_id ON qa_evidences(message_id);
	CREATE INDEX IF NOT EXISTS idx_qa_runs_created_at ON qa_runs(created_at);
	CREATE INDEX IF NOT EXISTS idx_qa_runs_role_id ON qa_runs(role_id);
	CREATE INDEX IF NOT EXISTS idx_qa_runs_success ON qa_runs(success);
	CREATE INDEX IF NOT EXISTS idx_telegraph_ingests_article_id ON telegraph_ingests(article_id);
	CREATE INDEX IF NOT EXISTS idx_telegraph_meta_score ON telegraph_meta(importance_score);
	CREATE INDEX IF NOT EXISTS idx_telegraph_meta_level ON telegraph_meta(impact_level);
	CREATE INDEX IF NOT EXISTS idx_telegraph_runs_started_at ON telegraph_runs(started_at);
	CREATE INDEX IF NOT EXISTS idx_telegraph_runs_success ON telegraph_runs(success);
	CREATE INDEX IF NOT EXISTS idx_telegraph_digests_slot_end ON telegraph_digests(slot_end);
	CREATE INDEX IF NOT EXISTS idx_telegraph_watch_hits_code ON telegraph_watch_hits(stock_code);
	CREATE INDEX IF NOT EXISTS idx_telegraph_watch_hits_article_id ON telegraph_watch_hits(article_id);
	INSERT INTO prompt_versions(prompt_id, version_no, name, content)
	SELECT p.id, 1, p.name, p.content
	FROM prompts p
	WHERE NOT EXISTS (
		SELECT 1
		FROM prompt_versions pv
		WHERE pv.prompt_id = p.id
	);
	INSERT INTO roles(name, alias, domain_tags, system_prompt, model_override, enabled, is_default)
	SELECT
		'通用分析师',
		'general',
		'通用,基本面',
		'你是资深股票研究分析师。请基于用户提供的报告上下文回答问题。禁止编造事实；若证据不足要明确说明。回答要结构清晰，先结论后理由。',
		'',
		1,
		1
	WHERE NOT EXISTS (SELECT 1 FROM roles);
	UPDATE roles
	SET is_default = 1, updated_at = CURRENT_TIMESTAMP
	WHERE id = (
		SELECT id FROM roles WHERE enabled = 1 ORDER BY is_default DESC, id ASC LIMIT 1
	)
	AND NOT EXISTS (SELECT 1 FROM roles WHERE enabled = 1 AND is_default = 1);`
	_, err := DB.Exec(schema)
	return err
}

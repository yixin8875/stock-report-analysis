package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

const appUpdateConfigKey = "app_update_config_v1"

var githubRepoRegexp = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func defaultAppUpdateConfig() models.AppUpdateConfig {
	cfg := models.AppUpdateConfig{
		GitHubRepo: normalizeGitHubRepo(strings.TrimSpace(os.Getenv("APP_UPDATE_GITHUB_REPO"))),
	}
	return cfg
}

func normalizeGitHubRepo(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimPrefix(repo, "github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.Trim(repo, "/")
	return repo
}

func validateGitHubRepo(repo string) error {
	if strings.TrimSpace(repo) == "" {
		return errors.New("GitHub 仓库不能为空，例如 owner/repo")
	}
	if !githubRepoRegexp.MatchString(repo) {
		return fmt.Errorf("GitHub 仓库格式无效: %s（示例: owner/repo）", repo)
	}
	return nil
}

func GetAppUpdateConfig() (models.AppUpdateConfig, error) {
	cfg := defaultAppUpdateConfig()

	var raw string
	err := db.DB.Get(&raw, "SELECT value FROM app_configs WHERE key=?", appUpdateConfigKey)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	var stored models.AppUpdateConfig
	if json.Unmarshal([]byte(raw), &stored) != nil {
		return cfg, nil
	}
	stored.GitHubRepo = normalizeGitHubRepo(stored.GitHubRepo)
	if stored.GitHubRepo == "" {
		return cfg, nil
	}
	return stored, nil
}

func SaveAppUpdateConfig(cfg models.AppUpdateConfig) error {
	cfg.GitHubRepo = normalizeGitHubRepo(cfg.GitHubRepo)
	if err := validateGitHubRepo(cfg.GitHubRepo); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`
		INSERT INTO app_configs(key, value, updated_at)
		VALUES(?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
	`, appUpdateConfigKey, string(data))
	return err
}

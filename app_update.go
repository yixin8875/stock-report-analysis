package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	goRuntime "runtime"
	"strconv"
	"strings"
	"time"

	"stock-report-analysis/internal/models"
	"stock-report-analysis/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt time.Time            `json:"published_at"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func (a *App) GetAppVersion() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		return "dev"
	}
	return v
}

func (a *App) GetAppUpdateConfig() (models.AppUpdateConfig, error) {
	return service.GetAppUpdateConfig()
}

func (a *App) SaveAppUpdateConfig(cfg models.AppUpdateConfig) error {
	return service.SaveAppUpdateConfig(cfg)
}

func (a *App) CheckAppUpdate() (models.AppUpdateResult, error) {
	cfg, err := service.GetAppUpdateConfig()
	if err != nil {
		return models.AppUpdateResult{}, err
	}
	repo := strings.TrimSpace(cfg.GitHubRepo)
	if repo == "" {
		return models.AppUpdateResult{}, errors.New("请先在设置中填写 GitHub 仓库（owner/repo）")
	}

	owner, name, err := parseGitHubRepo(repo)
	if err != nil {
		return models.AppUpdateResult{}, err
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, name)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return models.AppUpdateResult{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "stock-report-analysis-updater")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return models.AppUpdateResult{}, fmt.Errorf("请求 GitHub 失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.AppUpdateResult{}, fmt.Errorf("GitHub 返回异常: HTTP %d, %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var rel githubRelease
	if err := json.Unmarshal(bodyBytes, &rel); err != nil {
		return models.AppUpdateResult{}, fmt.Errorf("解析 GitHub Release 失败: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return models.AppUpdateResult{}, errors.New("未读取到有效的最新版本 tag")
	}

	current := a.GetAppVersion()
	latest := strings.TrimSpace(rel.TagName)
	hasUpdate := isVersionNewer(latest, current)

	selected := selectReleaseAsset(rel.Assets, goRuntime.GOOS, goRuntime.GOARCH)
	result := models.AppUpdateResult{
		Repo:           repo,
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      hasUpdate,
		ReleaseName:    strings.TrimSpace(rel.Name),
		ReleaseNotes:   strings.TrimSpace(rel.Body),
		ReleaseURL:     strings.TrimSpace(rel.HTMLURL),
		PublishedAt:    rel.PublishedAt,
		OS:             goRuntime.GOOS,
		Arch:           goRuntime.GOARCH,
		CheckedAt:      time.Now(),
	}
	if selected != nil {
		result.DownloadName = selected.Name
		result.DownloadURL = selected.BrowserDownloadURL
		result.DownloadSize = selected.Size
	}

	switch {
	case !hasUpdate:
		result.Message = "已是最新版本"
	case hasUpdate && result.DownloadURL == "":
		result.Message = "发现新版本，但未匹配到当前系统安装包"
	default:
		result.Message = "发现新版本"
	}

	log.Printf("[UPDATE] check repo=%s current=%s latest=%s hasUpdate=%v asset=%s", repo, current, latest, hasUpdate, result.DownloadName)
	return result, nil
}

func (a *App) OpenURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("URL 不能为空")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return errors.New("仅支持 http/https URL")
	}
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

func (a *App) DownloadAndInstallAppUpdate(downloadURL string, downloadName string) (string, error) {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return "", errors.New("下载地址不能为空")
	}
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		return "", errors.New("仅支持 http/https 下载地址")
	}

	if goRuntime.GOOS != "windows" {
		return "", errors.New("当前仅 Windows 支持一键安装，请点击“打开发布页”手动更新")
	}

	fileName := normalizeDownloadFileName(downloadURL, downloadName)
	if !strings.HasSuffix(strings.ToLower(fileName), ".exe") {
		return "", fmt.Errorf("Windows 一键安装仅支持 .exe 安装包，当前文件: %s", fileName)
	}

	tmpDir, err := os.MkdirTemp("", "stock-report-update-*")
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(tmpDir, fileName)

	log.Printf("[UPDATE] download start url=%s file=%s", downloadURL, targetPath)
	if err := downloadToFile(downloadURL, targetPath); err != nil {
		return "", err
	}

	cmd := exec.Command(targetPath)
	cmd.Dir = tmpDir
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动安装包失败: %w", err)
	}

	log.Printf("[UPDATE] installer started pid=%d path=%s", cmd.Process.Pid, targetPath)
	go func() {
		time.Sleep(600 * time.Millisecond)
		runtime.Quit(a.ctx)
	}()
	return "安装包已启动，应用将自动退出并进入安装流程", nil
}

func parseGitHubRepo(repo string) (string, string, error) {
	repo = strings.TrimSpace(repo)
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("GitHub 仓库格式无效: %s（示例: owner/repo）", repo)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func selectReleaseAsset(assets []githubReleaseAsset, osName, arch string) *githubReleaseAsset {
	if len(assets) == 0 {
		return nil
	}

	type scored struct {
		idx   int
		score int
	}
	best := scored{idx: -1, score: -1}
	for i, asset := range assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if name == "" || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		score := 0
		switch osName {
		case "windows":
			if strings.Contains(name, "windows") || strings.Contains(name, "win") {
				score += 50
			}
			if strings.Contains(name, "installer") {
				score += 90
			}
			if strings.HasSuffix(name, ".exe") {
				score += 40
			}
		case "darwin":
			if strings.Contains(name, "mac") || strings.Contains(name, "macos") || strings.Contains(name, "darwin") || strings.Contains(name, "osx") {
				score += 50
			}
		}
		switch arch {
		case "amd64":
			if strings.Contains(name, "amd64") || strings.Contains(name, "x64") {
				score += 30
			}
		case "arm64":
			if strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") {
				score += 30
			}
		}
		if osName == "darwin" && strings.Contains(name, "universal") {
			score += 40
		}
		if strings.HasSuffix(name, ".zip") {
			score += 5
		}
		if score > best.score {
			best = scored{idx: i, score: score}
		}
	}
	if best.idx >= 0 && best.score > 0 {
		return &assets[best.idx]
	}
	return nil
}

func isVersionNewer(latest, current string) bool {
	latest = strings.TrimSpace(latest)
	current = strings.TrimSpace(current)
	if latest == "" {
		return false
	}
	if current == "" {
		return true
	}

	lv, lok := parseSemver(latest)
	cv, cok := parseSemver(current)
	if lok && cok {
		if lv[0] != cv[0] {
			return lv[0] > cv[0]
		}
		if lv[1] != cv[1] {
			return lv[1] > cv[1]
		}
		return lv[2] > cv[2]
	}
	if lok && !cok {
		return true
	}
	return latest != current
}

func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return out, false
	}
	if i := strings.IndexAny(v, "+-"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return out, false
	}
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			out[i] = 0
			continue
		}
		part := strings.TrimSpace(parts[i])
		if part == "" {
			return out, false
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func normalizeDownloadFileName(rawURL, suggested string) string {
	name := strings.TrimSpace(suggested)
	if name == "" {
		if u, err := url.Parse(rawURL); err == nil {
			name = path.Base(strings.TrimSpace(u.Path))
		}
	}
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "update-installer.bin"
	}
	return name
}

func downloadToFile(downloadURL, targetPath string) error {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "stock-report-analysis-updater")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("下载失败: HTTP %d, %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("写入安装包失败: %w", err)
	}
	if written <= 0 {
		return errors.New("下载文件为空")
	}
	log.Printf("[UPDATE] download done bytes=%d path=%s", written, targetPath)
	return nil
}

package service

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

const mineruConfigKey = "mineru_config"

func defaultMinerUConfig() models.MinerUConfig {
	return models.MinerUConfig{
		Enabled:        1,
		BaseURL:        "https://mineru.net",
		APIToken:       "",
		ModelVersion:   "vlm",
		IsOCR:          1,
		PollIntervalMs: 2000,
		TimeoutSec:     300,
	}
}

func GetMinerUConfig() (models.MinerUConfig, error) {
	cfg := defaultMinerUConfig()

	var raw string
	err := db.DB.Get(&raw, "SELECT value FROM app_configs WHERE key=?", mineruConfigKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return cfg, err
	}
	if err == nil && strings.TrimSpace(raw) != "" {
		if unmarshalErr := json.Unmarshal([]byte(raw), &cfg); unmarshalErr != nil {
			return cfg, unmarshalErr
		}
	}

	if strings.TrimSpace(cfg.APIToken) == "" {
		cfg.APIToken = strings.TrimSpace(os.Getenv("MINERU_API_TOKEN"))
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		cfg.APIToken = strings.TrimSpace(os.Getenv("MINERU_API_KEY"))
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = strings.TrimSpace(os.Getenv("MINERU_BASE_URL"))
	}
	normalizeMinerUConfig(&cfg)
	return cfg, nil
}

func SaveMinerUConfig(cfg models.MinerUConfig) error {
	normalizeMinerUConfig(&cfg)
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	_, err = db.DB.Exec(`
		INSERT INTO app_configs(key, value, updated_at)
		VALUES(?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
	`, mineruConfigKey, string(payload))
	return err
}

func normalizeMinerUConfig(cfg *models.MinerUConfig) {
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.APIToken = strings.TrimSpace(cfg.APIToken)
	cfg.ModelVersion = strings.TrimSpace(strings.ToLower(cfg.ModelVersion))
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://mineru.net"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.ModelVersion == "" {
		cfg.ModelVersion = "vlm"
	}
	if cfg.ModelVersion != "pipeline" && cfg.ModelVersion != "vlm" {
		cfg.ModelVersion = "vlm"
	}
	if cfg.IsOCR != 1 {
		cfg.IsOCR = 0
	}
	if cfg.Enabled != 1 {
		cfg.Enabled = 0
	}
	if cfg.PollIntervalMs < 500 {
		cfg.PollIntervalMs = 500
	}
	if cfg.TimeoutSec < 30 {
		cfg.TimeoutSec = 30
	}
}

type mineruEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type mineruBatchCreateResp struct {
	BatchID  string   `json:"batch_id"`
	FileURLs []string `json:"file_urls"`
}

type mineruExtractResultItem struct {
	FileName   string `json:"file_name"`
	State      string `json:"state"`
	ErrMsg     string `json:"err_msg"`
	FullZipURL string `json:"full_zip_url"`
}

type mineruBatchResultResp struct {
	BatchID       string                    `json:"batch_id"`
	ExtractResult []mineruExtractResultItem `json:"extract_result"`
}

func ParseFileWithMinerU(filePath string) (string, error) {
	cfg, err := GetMinerUConfig()
	if err != nil {
		return "", err
	}
	if cfg.Enabled != 1 {
		return "", errors.New("MinerU 解析未启用，请在设置中启用")
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		return "", errors.New("缺少 MinerU API Token，请在设置中配置")
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	fileName := filepath.Base(filePath)
	if strings.TrimSpace(fileName) == "" {
		fileName = fmt.Sprintf("report-%d", time.Now().Unix())
	}

	batchID, uploadURL, err := createMinerUBatch(cfg, fileName)
	if err != nil {
		return "", err
	}
	if err := uploadMinerUFile(uploadURL, fileData); err != nil {
		return "", err
	}

	zipURL, err := waitMinerUResult(cfg, batchID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(zipURL) == "" {
		return "", errors.New("MinerU 返回成功但缺少结果下载地址")
	}

	markdown, err := downloadAndExtractMarkdown(zipURL)
	if err != nil {
		return "", err
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", errors.New("MinerU 解析结果为空")
	}
	return markdown, nil
}

func createMinerUBatch(cfg models.MinerUConfig, fileName string) (string, string, error) {
	dataID := fmt.Sprintf("%d", time.Now().UnixNano())
	payload := map[string]any{
		"files": []map[string]any{
			{
				"name":    fileName,
				"data_id": dataID,
				"is_ocr":  cfg.IsOCR == 1,
			},
		},
		"model_version": cfg.ModelVersion,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+"/api/v4/file-urls/batch", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("MinerU 创建任务失败: HTTP %d, %s", resp.StatusCode, string(respBytes))
	}

	var env mineruEnvelope
	if err := json.Unmarshal(respBytes, &env); err != nil {
		return "", "", err
	}
	if env.Code != 0 {
		return "", "", fmt.Errorf("MinerU 创建任务失败: %s (code=%d)", env.Msg, env.Code)
	}

	var data mineruBatchCreateResp
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", "", err
	}
	if data.BatchID == "" || len(data.FileURLs) == 0 {
		return "", "", errors.New("MinerU 返回的任务信息不完整")
	}
	return data.BatchID, data.FileURLs[0], nil
}

func uploadMinerUFile(uploadURL string, fileData []byte) error {
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(fileData))
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("上传文件到 MinerU 失败: HTTP %d, %s", resp.StatusCode, string(body))
	}
	return nil
}

func waitMinerUResult(cfg models.MinerUConfig, batchID string) (string, error) {
	deadline := time.Now().Add(time.Duration(cfg.TimeoutSec) * time.Second)
	client := &http.Client{Timeout: 30 * time.Second}
	url := cfg.BaseURL + "/api/v4/extract-results/batch/" + batchID

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.APIToken)

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(time.Duration(cfg.PollIntervalMs) * time.Millisecond)
			continue
		}
		respBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("查询 MinerU 结果失败: HTTP %d, %s", resp.StatusCode, string(respBytes))
		}

		var env mineruEnvelope
		if err := json.Unmarshal(respBytes, &env); err != nil {
			return "", err
		}
		if env.Code != 0 {
			return "", fmt.Errorf("查询 MinerU 结果失败: %s (code=%d)", env.Msg, env.Code)
		}

		var data mineruBatchResultResp
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return "", err
		}
		if len(data.ExtractResult) == 0 {
			time.Sleep(time.Duration(cfg.PollIntervalMs) * time.Millisecond)
			continue
		}

		allDone := true
		for _, item := range data.ExtractResult {
			state := strings.ToLower(strings.TrimSpace(item.State))
			switch state {
			case "done":
				if strings.TrimSpace(item.FullZipURL) != "" {
					return item.FullZipURL, nil
				}
			case "failed":
				errMsg := strings.TrimSpace(item.ErrMsg)
				if errMsg == "" {
					errMsg = "未知错误"
				}
				return "", fmt.Errorf("MinerU 解析失败: %s", errMsg)
			case "pending", "running", "converting", "waiting-file", "waiting_file":
				allDone = false
			default:
				allDone = false
			}
		}
		if allDone {
			break
		}
		time.Sleep(time.Duration(cfg.PollIntervalMs) * time.Millisecond)
	}

	return "", fmt.Errorf("等待 MinerU 结果超时（%d 秒）", cfg.TimeoutSec)
}

type zipCandidate struct {
	Name string
	Size uint64
}

func downloadAndExtractMarkdown(zipURL string) (string, error) {
	resp, err := http.Get(zipURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("下载 MinerU 结果失败: HTTP %d, %s", resp.StatusCode, string(body))
	}

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", err
	}

	candidates := make([]zipCandidate, 0)
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".md") {
			candidates = append(candidates, zipCandidate{Name: f.Name, Size: f.UncompressedSize64})
		}
	}
	if len(candidates) == 0 {
		return "", errors.New("MinerU 结果压缩包中未找到 markdown 文件")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		ni := strings.ToLower(candidates[i].Name)
		nj := strings.ToLower(candidates[j].Name)
		bi := filepath.Base(ni)
		bj := filepath.Base(nj)

		pi := 0
		pj := 0
		if strings.HasSuffix(bi, ".md") {
			pi += 2
		}
		if strings.HasSuffix(bj, ".md") {
			pj += 2
		}
		if strings.Contains(ni, "content_list") || strings.Contains(ni, "layout") {
			pi--
		}
		if strings.Contains(nj, "content_list") || strings.Contains(nj, "layout") {
			pj--
		}
		if pi != pj {
			return pi > pj
		}
		if len(candidates[i].Name) != len(candidates[j].Name) {
			return len(candidates[i].Name) < len(candidates[j].Name)
		}
		return candidates[i].Size > candidates[j].Size
	})

	target := candidates[0].Name
	for _, f := range reader.File {
		if f.Name != target {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		mdBytes, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return string(mdBytes), nil
	}
	return "", errors.New("无法读取 MinerU markdown 文件")
}

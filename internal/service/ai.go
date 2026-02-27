package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"stock-report-analysis/internal/models"
)

const (
	AnalysisModeText       = "text"
	AnalysisModeStructured = "structured"
)

type AnalysisResult struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMs       int64
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Stream         bool              `json:"stream"`
	StreamOptions  *streamOptions    `json:"stream_options,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type streamDelta struct {
	Content string `json:"content"`
}

type streamChoice struct {
	Message      streamDelta `json:"message"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
	Text         string      `json:"text"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamChunk struct {
	Choices []streamChoice `json:"choices"`
	Usage   *usage         `json:"usage,omitempty"`
	Error   *apiError      `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
}

var aiHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
}

func AnalyzeArticle(channel models.AIChannel, prompt, content string, onChunk func(string)) (string, error) {
	res, err := AnalyzeArticleDetailed(channel, prompt, content, AnalysisModeText, onChunk)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

func AnalyzeArticleDetailed(channel models.AIChannel, prompt, content, mode string, onChunk func(string)) (AnalysisResult, error) {
	return AnalyzeArticleDetailedWithContext(context.Background(), channel, prompt, content, mode, onChunk)
}

func AnalyzeArticleDetailedWithContext(ctx context.Context, channel models.AIChannel, prompt, content, mode string, onChunk func(string)) (AnalysisResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if onChunk == nil {
		onChunk = func(string) {}
	}

	startedAt := time.Now()

	reqBody := chatRequest{
		Model: channel.Model,
		Messages: []chatMessage{
			{Role: "system", Content: buildSystemPrompt(prompt, mode)},
			{Role: "user", Content: content},
		},
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	if mode == AnalysisModeStructured {
		reqBody.ResponseFormat = map[string]string{"type": "json_object"}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return AnalysisResult{}, err
	}

	url := strings.TrimRight(channel.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return AnalysisResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+channel.APIKey)

	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return AnalysisResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return AnalysisResult{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		result, err := parseJSONResponse(ctx, resp.Body, onChunk)
		if err != nil {
			return AnalysisResult{}, err
		}
		result.DurationMs = time.Since(startedAt).Milliseconds()
		return result, nil
	}
	if !strings.Contains(ct, "text/event-stream") {
		b, _ := io.ReadAll(resp.Body)
		return AnalysisResult{}, fmt.Errorf("非预期的响应类型 %s: %s", ct, string(b[:min(len(b), 200)]))
	}

	result, err := parseSSEResponse(ctx, resp.Body, onChunk)
	if err != nil {
		return AnalysisResult{}, err
	}
	result.DurationMs = time.Since(startedAt).Milliseconds()
	return result, nil
}

func parseSSEResponse(ctx context.Context, r io.Reader, onChunk func(string)) (AnalysisResult, error) {
	var full strings.Builder
	var usageData usage
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return AnalysisResult{}, ctx.Err()
		default:
		}

		data := parseSSEDataLine(scanner.Text())
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return AnalysisResult{}, errors.New(chunk.Error.Message)
		}
		if chunk.Usage != nil {
			usageData = *chunk.Usage
		}

		text := pickChoiceContent(chunk.Choices)
		if text == "" {
			continue
		}
		full.WriteString(text)
		onChunk(text)
	}
	if err := scanner.Err(); err != nil {
		return AnalysisResult{}, err
	}
	if full.Len() == 0 {
		return AnalysisResult{}, errors.New("API 返回内容为空")
	}

	return AnalysisResult{
		Text:             full.String(),
		PromptTokens:     usageData.PromptTokens,
		CompletionTokens: usageData.CompletionTokens,
		TotalTokens:      usageData.TotalTokens,
	}, nil
}

func parseJSONResponse(ctx context.Context, r io.Reader, onChunk func(string)) (AnalysisResult, error) {
	select {
	case <-ctx.Done():
		return AnalysisResult{}, ctx.Err()
	default:
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return AnalysisResult{}, err
	}

	var resp streamChunk
	if err := json.Unmarshal(body, &resp); err != nil {
		return AnalysisResult{}, fmt.Errorf("解析 JSON 响应失败: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return AnalysisResult{}, errors.New(resp.Error.Message)
	}

	text := pickChoiceContent(resp.Choices)
	if text == "" {
		return AnalysisResult{}, errors.New("API 返回内容为空")
	}
	onChunk(text)

	result := AnalysisResult{
		Text: text,
	}
	if resp.Usage != nil {
		result.PromptTokens = resp.Usage.PromptTokens
		result.CompletionTokens = resp.Usage.CompletionTokens
		result.TotalTokens = resp.Usage.TotalTokens
	}
	return result, nil
}

func buildSystemPrompt(prompt, mode string) string {
	if mode != AnalysisModeStructured {
		return prompt
	}
	return prompt + `

请严格以 JSON 输出，且必须是一个可解析的 JSON 对象，不要使用 Markdown 代码块。
JSON Schema:
{
  "summary": "string",
  "risks": ["string"],
  "catalysts": ["string"],
  "valuationView": "string"
}`
}

func parseSSEDataLine(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:"))
}

func pickChoiceContent(choices []streamChoice) string {
	if len(choices) == 0 {
		return ""
	}
	choice := choices[0]
	switch {
	case choice.Delta.Content != "":
		return choice.Delta.Content
	case choice.Message.Content != "":
		return choice.Message.Content
	case choice.Text != "":
		return choice.Text
	default:
		return ""
	}
}

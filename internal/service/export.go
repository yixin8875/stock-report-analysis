package service

import (
	"fmt"
	"os"

	"stock-report-analysis/internal/models"
)

func ExportMarkdown(article models.Article) string {
	md := fmt.Sprintf("# %s\n\n## 原文\n\n%s\n", article.Title, article.Content)
	if article.Analysis != "" {
		md += fmt.Sprintf("\n## AI 解读\n\n> 渠道: %s | 提示词: %s\n\n%s\n", article.ChannelUsed, article.PromptUsed, article.Analysis)
	}
	return md
}

func ExportToFile(article models.Article, path string) error {
	return os.WriteFile(path, []byte(ExportMarkdown(article)), 0644)
}

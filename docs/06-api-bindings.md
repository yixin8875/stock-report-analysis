# Wails API 绑定速查

本页用于前后端联调时快速查询 `App` 暴露方法与实时事件。

## 1. 绑定来源

- 绑定入口: `main.go` 的 `Bind: []interface{}{ app }`
- 方法实现: `app.go` / `app_batch_analysis.go` / `app_update.go`
- 前端声明（自动生成）: `frontend/wailsjs/go/main/App.d.ts`

说明:

- 仅 `func (a *App) 方法名...` 且方法名首字母大写的方法会暴露给前端
- `frontend/wailsjs/go/main/App.d.ts` 是当前对前端的接口真值

## 2. 方法清单（按模块）

### 2.1 文章、标签、分析

- `GetArticles(keyword, tagID)`
- `GetArticle(id)`
- `DeleteArticle(id)`
- `ImportArticle()`
- `ImportArticles()`
- `AnalyzeArticle(articleID, channelID, promptID)`
- `AnalyzeArticleWithMode(articleID, channelID, promptID, mode)`
- `GetAnalysisHistory(articleID)`
- `GetAnalysisDashboard()`
- `GetAnalysisDashboardByDays(days)`
- `GetTags()`
- `SaveTag(tag)`
- `DeleteTag(id)`
- `GetArticleTags(articleID)`
- `SetArticleTags(articleID, tagIDs)`
- `ExportArticle(articleID)`

### 2.2 AI 渠道与提示词

- `GetChannels()`
- `SaveChannel(channel)`
- `DeleteChannel(id)`
- `GetPrompts()`
- `SavePrompt(prompt)`
- `DeletePrompt(id)`
- `GetPromptVersions(promptID)`
- `RestorePromptVersion(promptID, versionID)`

### 2.3 批量分析

- `BatchAnalyze(articleIDs, channelID, promptID)`
- `StartBatchAnalyze(articleIDs, channelID, promptID, concurrency, mode)`
- `PauseBatchAnalyze()`
- `ResumeBatchAnalyze()`
- `RetryFailedBatchAnalyze()`
- `GetBatchStatus()`
- `ExportBatchFailures()`

### 2.4 QA 问答

- `GetRoles()`
- `SaveRole(role)`
- `DeleteRole(id)`
- `SetDefaultRole(id)`
- `GetRoleTemplates()`
- `CreateRoleFromTemplate(templateID)`
- `GetQASessions(articleID)`
- `CreateQASession(articleID, title)`
- `RenameQASession(id, title)`
- `DeleteQASession(id)`
- `GetQAMessages(sessionID)`
- `GetQAPins(sessionID)`
- `SaveQAPin(pin)`
- `DeleteQAPin(id)`
- `AskQuestion(sessionID, articleID, question)`
- `AskQuestionFollowUp(sessionID, articleID, question, followUpMessageID)`
- `CancelAskQuestion()`
- `GetQADashboard()`
- `GetQADashboardByDays(days)`
- `DebugQAPing(marker)`

### 2.5 财联社电报与自选股

- `GetTelegraphSchedulerConfig()`
- `SaveTelegraphSchedulerConfig(cfg)`
- `GetTelegraphSchedulerStatus()`
- `RunTelegraphSchedulerNow()`
- `StopTelegraphScheduler()`
- `GetTelegraphArticles(keyword, tagID, order, watchOnly)`
- `GetTelegraphDashboard()`
- `GetTelegraphDashboardByDays(days)`
- `GetTelegraphDigests(limit)`
- `GetTelegraphWatchlist()`
- `SaveTelegraphWatchlist(items)`

### 2.6 MinerU

- `GetMinerUConfig()`
- `SaveMinerUConfig(cfg)`

### 2.7 应用更新

- `GetAppVersion()`
- `GetAppUpdateConfig()`
- `SaveAppUpdateConfig(cfg)`
- `CheckAppUpdate()`
- `OpenURL(url)`
- `DownloadAndInstallAppUpdate(downloadURL, downloadName)`

说明:

- `DownloadAndInstallAppUpdate` 当前仅 Windows 生效
- 自动更新依赖 GitHub Release 与版本 tag（`v*`）

## 3. 前端调用示例

```ts
import {
  CheckAppUpdate,
  StartBatchAnalyze,
  AskQuestion,
} from "../../wailsjs/go/main/App";

const update = await CheckAppUpdate();
await StartBatchAnalyze([1, 2, 3], 1, 1, 3, "text");
const sessionID = await AskQuestion(0, 12, "请总结这篇报告核心结论");
```

## 4. 后端实时事件（Events）

问答相关:

- `qa-job-start`
- `qa-role-start`
- `qa-role-chunk`
- `qa-role-done`
- `qa-role-error`
- `qa-job-done`

批量分析相关:

- `analysis-chunk`
- `batch-status`
- `batch-progress`
- `batch-error`
- `batch-done`

财联社相关:

- `telegraph-alert`
- `telegraph-digest`

## 5. 更新接口后的注意事项

1. 修改后端 `App` 暴露方法后，需要重新生成绑定:

```bash
wails generate module
```

2. 提交时请同步包含:

- `frontend/wailsjs/go/main/App.d.ts`
- `frontend/wailsjs/go/main/App.js`
- `frontend/wailsjs/go/models.ts`（若模型有变更）


# 前后端事件契约（Events Contract）

本页约定 Wails 事件总线中各事件的 payload 结构，避免前后端字段不一致。

## 1. 基本约定

- 后端通过 `runtime.EventsEmit(a.ctx, eventName, payload)` 发送
- 前端通过 `EventsOn(eventName, (...args) => { const payload = args[0] })` 订阅
- 未特殊说明时，payload 为一个对象；`batch-error` 为字符串；`batch-done` 无 payload
- Go 的 `time.Time` 在前端按字符串/可序列化时间处理

## 2. QA 事件

### 2.1 `qa-job-start`

来源:

- `app.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `sessionId` | `number` | 会话 ID |
| `questionMessageId` | `number` | 用户问题消息 ID |
| `roleCount` | `number` | 本次参与角色数 |

### 2.2 `qa-role-start`

来源:

- `app.go`

Payload:

- `models.QAMessage` 全量对象（见 `frontend/wailsjs/go/models.ts` 的 `QAMessage`）

关键字段:

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | `number` | 消息 ID |
| `sessionId` | `number` | 会话 ID |
| `roleId` | `number` | 角色 ID |
| `roleName` | `string` | 角色名称 |
| `content` | `string` | 当前内容（起始时可能为空） |
| `status` | `string` | `running/done/failed` |
| `createdAt` | `string` | 创建时间 |

### 2.3 `qa-role-chunk`

来源:

- `app.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `messageId` | `number` | 当前消息 ID |
| `roleId` | `number` | 角色 ID |
| `roleName` | `string` | 角色名称 |
| `chunk` | `string` | 增量文本 |

### 2.4 `qa-role-done`

来源:

- `app.go`

Payload:

- `models.QAMessage` 全量对象（最终态）

### 2.5 `qa-role-error`

来源:

- `app.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `messageId` | `number` | 消息 ID，若为 `0` 表示任务级错误 |
| `roleId` | `number` | 角色 ID，任务级错误时为 `0` |
| `roleName` | `string` | 角色名，任务级错误时可能为空 |
| `error` | `string` | 错误信息 |

约定:

- 前端看到 `messageId=0` 时，应作为全局错误处理并结束“提问中”状态

### 2.6 `qa-job-done`

来源:

- `app.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `sessionId` | `number` | 完成的会话 ID |

## 3. 批量分析事件

### 3.1 `analysis-chunk`

来源:

- `app_batch_analysis.go`

Payload:

- `string`（单篇分析流式增量文本）

### 3.2 `batch-status`

来源:

- `app_batch_analysis.go`

Payload:

- `models.BatchStatus` 对象

关键字段:

| 字段 | 类型 | 说明 |
|---|---|---|
| `running` | `boolean` | 是否运行中 |
| `paused` | `boolean` | 是否暂停 |
| `total` | `number` | 总任务数 |
| `completed` | `number` | 已完成数 |
| `success` | `number` | 成功数 |
| `failed` | `number` | 失败数 |
| `inProgress` | `number` | 并发执行中数量 |
| `concurrency` | `number` | 并发度 |
| `failures` | `BatchFailure[]` | 失败明细 |

### 3.3 `batch-progress`

来源:

- `app_batch_analysis.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `current` | `number` | 当前完成进度 |
| `total` | `number` | 总任务数 |

### 3.4 `batch-error`

来源:

- `app.go` / `app_batch_analysis.go`

Payload:

- `string`（错误信息）

### 3.5 `batch-done`

来源:

- `app_batch_analysis.go`

Payload:

- `null`（可按无 payload 处理）

## 4. 财联社事件

### 4.1 `telegraph-alert`

来源:

- `app_telegraph_scheduler.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `articleId` | `number` | 新闻文章 ID |
| `title` | `string` | 标题 |
| `score` | `number` | 影响分 |
| `direction` | `string` | 影响方向 |
| `level` | `string` | 影响等级 |
| `createdAt` | `string` | 创建时间 |
| `sourceType` | `string` | 当前固定为 `news` |

### 4.2 `telegraph-digest`

来源:

- `app_telegraph_scheduler.go`

Payload:

| 字段 | 类型 | 说明 |
|---|---|---|
| `slotStart` | `string` | 摘要窗口开始时间 |
| `slotEnd` | `string` | 摘要窗口结束时间 |
| `summary` | `string` | 摘要文本（截断版） |
| `topItems` | `number` | 入选新闻数量 |
| `avgScore` | `number` | 平均影响分 |

## 5. 前端接入建议

- 对 `args[0]` 做空值保护，避免事件参数异常导致崩溃
- 对数字字段统一 `Number(payload.xxx || 0)` 处理
- 对任务结束类事件（如 `qa-job-done`）始终做 UI 状态收敛
- 新增事件前，先更新本文件与 `docs/06-api-bindings.md`


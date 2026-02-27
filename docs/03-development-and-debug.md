# 开发与调试

## 1. 本地开发

前置:

- Go 1.24+
- Node.js 20+
- Wails CLI v2.11+

安装依赖:

```bash
cd frontend
npm install
```

启动开发:

```bash
wails dev
```

常用命令:

```bash
go test ./...
cd frontend && npm run build
wails build
```

## 2. 日志与定位

`wails dev` 下后端日志直接输出到终端（`stdout`），关键前缀：

- `[BOOT]`: 启动日志
- `[QA][App]` / `[QA]`: 问答链路日志
- `[CLS]`: 财联社抓取与分析日志
- `[UPDATE]`: 更新检查与下载日志

建议排查顺序:

1. 先确认是否有 `[BOOT] logger ready: stdout`
2. 再看请求是否进入后端（例如 `[QA][App] ask request ...`）
3. 再看服务执行是否有报错（例如 role failed、fetch failed）

## 3. 常见问题

### 3.1 按钮一直“提问中”

- 检查控制台是否有 `[QA][App] ask request`
- 若有前端异常（如构造函数错误），先修复前端运行时错误
- 检查渠道、API Key、模型配置是否可用

### 3.2 财联社任务停不下来

- 使用设置页“立即停止”
- 检查 `GetTelegraphSchedulerConfig` 的 `enabled` 是否已置 `0`
- 观察 `[CLS] run finished` 是否出现

### 3.3 发布后客户端不显示更新

- 确认版本是 tag 发布（`v*`）
- 确认 Release 有当前系统对应资产
- 检查设置页 `GitHub 仓库` 是否是 `owner/repo`


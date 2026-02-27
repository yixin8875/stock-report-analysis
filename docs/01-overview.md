# 项目概览

## 1. 项目定位

`stock-report-analysis` 是一个桌面端股票研究与快讯解读工具，支持：

- 导入研究报告与新闻内容
- 调用多 AI 渠道做结构化分析
- 对财联社电报做定时抓取、自动解读与监控
- 提供问答、标签、看板、导出与版本更新能力

## 2. 技术栈

- 桌面框架: `Wails v2`
- 后端: `Go 1.24`
- 前端: `React + TypeScript + Vite + Tailwind`
- 数据库: `SQLite` (`modernc.org/sqlite`)

## 3. 核心目录

- `main.go`: 应用入口、日志初始化、Wails 启动
- `app.go`: 绑定给前端调用的后端方法
- `app_batch_analysis.go`: 批量分析任务
- `app_telegraph_scheduler.go`: 财联社定时任务执行器
- `app_update.go`: 检查更新与下载安装（Windows）
- `internal/service/*`: 业务逻辑层（文章、问答、财联社、更新等）
- `internal/db/db.go`: SQLite 初始化与迁移
- `frontend/src/pages/*`: 页面层（文章、详情、新闻、设置）
- `.github/workflows/*`: CI/CD 工作流

## 4. 应用数据位置

- 数据库路径: `~/.stock-report-analysis/data.db`

SQLite 运行参数:

- `WAL`
- `foreign_keys=1`
- `busy_timeout=5000`

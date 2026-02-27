# 数据库与配置说明

## 1. 数据库基础

- 文件路径: `~/.stock-report-analysis/data.db`
- 初始化入口: `internal/db/db.go`
- 特性: `WAL` + 外键约束 + busy timeout

## 2. 主要表

文章分析相关:

- `articles`: 原文与分析结果
- `analysis_history`: 历史分析快照
- `analysis_runs`: 每次分析运行指标
- `tags` / `article_tags`: 标签体系

问答相关:

- `roles`: 问答角色配置
- `qa_sessions`: 会话
- `qa_messages`: 消息
- `qa_evidences`: 证据引用
- `qa_runs`: 运行质量指标
- `qa_pins`: 置顶内容

新闻电报相关:

- `telegraph_ingests`: 财联社 news_id 去重映射
- `telegraph_meta`: 重要性与影响方向
- `telegraph_runs`: 调度运行记录
- `telegraph_digests`: 30 分钟摘要
- `telegraph_watch_hits`: 新闻与自选股命中关系

## 3. 配置存储（app_configs）

以下配置通过 `app_configs(key,value)` 保存:

- `telegraph_scheduler_config_v1`: 财联社调度配置
- `telegraph_watchlist_v1`: 自选股池
- `mineru_config`: MinerU 文档解析配置
- `app_update_config_v1`: 自动更新仓库配置

## 4. 迁移策略

- 采用 `CREATE TABLE IF NOT EXISTS` 与 `CREATE INDEX IF NOT EXISTS`
- 通过默认插入与补齐逻辑保证老库可平滑升级
- 迁移在应用启动时执行

## 5. 运维建议

- 升级版本前先备份 `data.db`
- 异常退出后优先重启应用让 SQLite 自恢复 WAL
- 如果需要导出分析数据，优先走应用内导出能力，避免直接改库


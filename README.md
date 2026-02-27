# 股票报告 AI 解读

桌面端工具（Wails + Go + React），用于导入股票研究报告并调用 AI 模型做结构化解读。

## 功能

- 导入单篇或多篇文章（`.txt` / `.md` / `.html`）
- 文章搜索、标签过滤、标签管理
- 配置多 AI 渠道（Base URL / API Key / Model）
- 配置提示词模板并设置默认项
- 单篇流式解读、批量解读
- 批量任务中心：并发设置、暂停/继续、失败重试、失败明细导出
- 结构化解读模式：JSON 输出并卡片化展示（结论/风险/催化剂/估值观点）
- 保存解读历史，支持历史切换查看
- 一键导出 Markdown（原文 + AI 解读）
- 运行质量看板：成功率、耗时、Token、失败原因、按渠道统计

## 技术栈

- 后端：Go 1.24、Wails v2、SQLite（modernc）
- 前端：React + TypeScript + Vite + Tailwind CSS v4

## 本地开发

### 1. 安装依赖

```bash
cd frontend
npm install
```

### 2. 启动开发模式

在项目根目录执行：

```bash
wails dev
```

## 构建

在项目根目录执行：

```bash
wails build
```

## 数据存储

应用数据默认保存在：

```text
~/.stock-report-analysis/data.db
```

SQLite 已开启：

- `WAL` 日志模式
- `foreign_keys`
- `busy_timeout`

## 项目结构

```text
.
├── app.go                     # Wails 绑定方法
├── main.go                    # Wails 启动入口
├── internal
│   ├── db                     # 数据库初始化与迁移
│   ├── models                 # 数据模型
│   └── service                # 业务逻辑（文章、标签、AI、导出、配置）
└── frontend
    └── src/pages              # Articles / ArticleDetail / Settings 页面
```

## 常见问题

- 批量解读失败：请先在「设置」中确认 AI 渠道、API Key 和提示词已配置。
- 解读超时或空结果：检查模型可用性、Base URL 是否兼容 OpenAI Chat Completions 接口。
- 搜索性能问题：建议避免在单篇里放入超长原文，或按主题打标签后再筛选。

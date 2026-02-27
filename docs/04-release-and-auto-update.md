# 发布与自动更新

## 1. 工作流说明

### 1.1 自动发布工作流

文件: `.github/workflows/release.yml`

触发条件:

- `push tag v*`（仅版本 tag 自动触发）

行为:

1. 分别在 `macos-latest` 与 `windows-latest` 构建
2. 注入版本号: `-ldflags "-X main.Version=<tag>"`
3. mac 打包为 `.app.zip`
4. windows 通过 `-nsis` 产出 `*installer.exe`
5. 生成 `.sha256`
6. 上传并发布 GitHub Release

### 1.2 手动构建工作流

文件: `.github/workflows/build-wails.yml`

触发条件:

- `workflow_dispatch`（手动触发，不会在普通提交自动运行）

## 2. 发布步骤

1. 确认代码已推送主分支
2. 创建并推送 tag

```bash
git tag v1.0.0
git push origin v1.0.0
```

3. 在 GitHub Actions 查看 `Release Wails (Win + Mac)` 成功
4. 在 GitHub Releases 确认资产完整

## 3. 客户端更新机制

后端入口位于 `app_update.go`：

- `GetAppVersion`: 读取当前版本
- `CheckAppUpdate`: 调 GitHub `releases/latest` 比较版本并匹配资产
- `DownloadAndInstallAppUpdate`: Windows 下载并拉起安装包

前端入口位于设置页“应用更新”：

1. 配置 `owner/repo`
2. 点击“检查更新”
3. 有更新时可打开发布页或下载安装

## 4. 平台差异

Windows:

- 支持“一键下载安装并重启”
- 仅接受 `*installer.exe` 资产

macOS:

- 当前为打开发布页手动安装
- 后续如要自动替换，需要额外做签名、公证与更新框架接入

## 5. 失败排查

### 5.1 Windows 找不到 installer

- 确认 workflow 已安装 `NSIS`
- 看 `Debug Windows Build Outputs` 是否有 `*installer.exe`
- 若无，优先检查 `wails build -nsis` 输出日志

### 5.2 检查更新失败

- 仓库格式必须是 `owner/repo`
- Release 必须存在且 tag 合法（建议 `v1.2.3`）
- 可能受 GitHub API 速率限制，稍后重试

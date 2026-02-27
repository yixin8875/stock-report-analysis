import { useEffect, useMemo, useState } from 'react'
import {
  CheckAppUpdate,
  CreateRoleFromTemplate,
  DeleteChannel,
  DownloadAndInstallAppUpdate,
  DeletePrompt,
  DeleteRole,
  DeleteTag,
  GetAnalysisDashboard,
  GetAnalysisDashboardByDays,
  GetAppUpdateConfig,
  GetAppVersion,
  GetChannels,
  GetMinerUConfig,
  GetPromptVersions,
  GetPrompts,
  GetRoleTemplates,
  GetRoles,
  GetTags,
  GetTelegraphSchedulerConfig,
  GetTelegraphSchedulerStatus,
  GetTelegraphWatchlist,
  OpenURL,
  RestorePromptVersion,
  RunTelegraphSchedulerNow,
  SaveChannel,
  SaveAppUpdateConfig,
  SaveMinerUConfig,
  SavePrompt,
  SaveRole,
  SaveTag,
  SaveTelegraphSchedulerConfig,
  SaveTelegraphWatchlist,
  StopTelegraphScheduler,
  SetDefaultRole,
} from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'

const inputCls = 'w-full px-3 py-2.5 bg-white border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-shadow'

const TAG_COLORS = ['#3b82f6', '#ef4444', '#f59e0b', '#10b981', '#8b5cf6', '#ec4899', '#6366f1', '#14b8a6']

type TabKey = 'channels' | 'prompts' | 'roles' | 'mineru' | 'telegraph' | 'updater' | 'watchlist' | 'tags' | 'dashboard'
type DashboardRange = 0 | 7 | 30

type AppUpdateConfigData = {
  githubRepo: string
}

type WatchStockFormData = {
  code: string
  name: string
  aliasesText: string
}

type TelegraphConfigData = {
  enabled: number
  sourceUrl: string
  intervalMinutes: number
  fetchLimit: number
  channelId: number
  analysisPrompt: string
}

type TelegraphStatusData = {
  running: boolean
  lastRunAt: unknown
  lastError: string
  lastFetched: number
  lastImported: number
  lastAnalyzed: number
}

type ChannelFormData = {
  id: number
  name: string
  baseUrl: string
  apiKey: string
  model: string
  isDefault: number
}

type PromptFormData = {
  id: number
  name: string
  content: string
  isDefault: number
}

type TagFormData = {
  id: number
  name: string
  color: string
}

type RoleFormData = {
  id: number
  name: string
  alias: string
  domainTags: string
  systemPrompt: string
  modelOverride: string
  temperature: number
  maxTokens: number
  enabled: number
  isDefault: number
}

const DEFAULT_TELEGRAPH_CONFIG: TelegraphConfigData = {
  enabled: 0,
  sourceUrl: 'https://m.cls.cn/telegraph',
  intervalMinutes: 10,
  fetchLimit: 8,
  channelId: 0,
  analysisPrompt: '你是资深A股盘中快讯分析师。请基于这条财联社电报，输出：1) 事件一句话总结 2) 市场影响方向与理由 3) 受影响板块 4) 后续跟踪信号。禁止编造。',
}

const DEFAULT_TELEGRAPH_STATUS: TelegraphStatusData = {
  running: false,
  lastRunAt: '',
  lastError: '',
  lastFetched: 0,
  lastImported: 0,
  lastAnalyzed: 0,
}

const toChannelFormData = (item: models.AIChannel): ChannelFormData => ({
  id: item.id,
  name: item.name,
  baseUrl: item.baseUrl,
  apiKey: item.apiKey,
  model: item.model,
  isDefault: item.isDefault,
})

const toPromptFormData = (item: models.Prompt): PromptFormData => ({
  id: item.id,
  name: item.name,
  content: item.content,
  isDefault: item.isDefault,
})

const toTagFormData = (item: models.Tag): TagFormData => ({
  id: item.id,
  name: item.name,
  color: item.color,
})

const toRoleFormData = (item: models.Role): RoleFormData => ({
  id: item.id,
  name: item.name,
  alias: item.alias,
  domainTags: item.domainTags,
  systemPrompt: item.systemPrompt,
  modelOverride: item.modelOverride,
  temperature: item.temperature,
  maxTokens: item.maxTokens,
  enabled: item.enabled,
  isDefault: item.isDefault,
})

export default function Settings() {
  const [channels, setChannels] = useState<models.AIChannel[]>([])
  const [prompts, setPrompts] = useState<models.Prompt[]>([])
  const [promptVersions, setPromptVersions] = useState<models.PromptVersion[]>([])
  const [roles, setRoles] = useState<models.Role[]>([])
  const [tags, setTags] = useState<models.Tag[]>([])
  const [dashboard, setDashboard] = useState<models.AnalysisDashboard | null>(null)
  const [dashboardRange, setDashboardRange] = useState<DashboardRange>(0)
  const [tab, setTab] = useState<TabKey>('channels')
  const [editCh, setEditCh] = useState<ChannelFormData | null>(null)
  const [editPr, setEditPr] = useState<PromptFormData | null>(null)
  const [versionPrompt, setVersionPrompt] = useState<models.Prompt | null>(null)
  const [diffVersionID, setDiffVersionID] = useState(0)
  const [restoringPromptVersionID, setRestoringPromptVersionID] = useState(0)
  const [editRole, setEditRole] = useState<RoleFormData | null>(null)
  const [editTag, setEditTag] = useState<TagFormData | null>(null)
  const [roleTemplates, setRoleTemplates] = useState<models.RoleTemplate[]>([])
  const [creatingTemplateID, setCreatingTemplateID] = useState('')
  const [mineruCfg, setMineruCfg] = useState<models.MinerUConfig>(new models.MinerUConfig({
    enabled: 1,
    baseUrl: 'https://mineru.net',
    apiToken: '',
    modelVersion: 'vlm',
    isOCR: 1,
    pollIntervalMs: 2000,
    timeoutSec: 300,
  }))
  const [mineruSaving, setMineruSaving] = useState(false)
  const [mineruSavedTip, setMineruSavedTip] = useState('')
  const [telegraphCfg, setTelegraphCfg] = useState<TelegraphConfigData>(DEFAULT_TELEGRAPH_CONFIG)
  const [telegraphStatus, setTelegraphStatus] = useState<TelegraphStatusData>(DEFAULT_TELEGRAPH_STATUS)
  const [telegraphSaving, setTelegraphSaving] = useState(false)
  const [telegraphRunningNow, setTelegraphRunningNow] = useState(false)
  const [telegraphTip, setTelegraphTip] = useState('')
  const [updateCfg, setUpdateCfg] = useState<AppUpdateConfigData>({ githubRepo: '' })
  const [updateResult, setUpdateResult] = useState<models.AppUpdateResult | null>(null)
  const [updateSaving, setUpdateSaving] = useState(false)
  const [updateChecking, setUpdateChecking] = useState(false)
  const [updateInstalling, setUpdateInstalling] = useState(false)
  const [updateTip, setUpdateTip] = useState('')
  const [appVersion, setAppVersion] = useState('')
  const [watchStocks, setWatchStocks] = useState<WatchStockFormData[]>([])
  const [watchSaving, setWatchSaving] = useState(false)
  const [watchTip, setWatchTip] = useState('')

  const loadChannels = () => GetChannels().then((list) => setChannels(list || []))
  const loadPrompts = () => GetPrompts().then((list) => setPrompts(list || []))
  const loadPromptVersions = (promptID: number) => GetPromptVersions(promptID).then((list) => setPromptVersions(list || []))
  const loadRoles = () => GetRoles().then((list) => setRoles(list || []))
  const loadTags = () => GetTags().then((list) => setTags(list || []))
  const loadRoleTemplates = () => GetRoleTemplates().then((list) => setRoleTemplates(list || []))
  const loadMinerU = () => GetMinerUConfig().then((cfg) => setMineruCfg(new models.MinerUConfig(cfg)))
  const loadTelegraphCfg = () => GetTelegraphSchedulerConfig().then((cfg) => {
    setTelegraphCfg({
      enabled: cfg?.enabled === 1 ? 1 : 0,
      sourceUrl: cfg?.sourceUrl || DEFAULT_TELEGRAPH_CONFIG.sourceUrl,
      intervalMinutes: Number(cfg?.intervalMinutes || DEFAULT_TELEGRAPH_CONFIG.intervalMinutes),
      fetchLimit: Number(cfg?.fetchLimit || DEFAULT_TELEGRAPH_CONFIG.fetchLimit),
      channelId: Number(cfg?.channelId || 0),
      analysisPrompt: cfg?.analysisPrompt || DEFAULT_TELEGRAPH_CONFIG.analysisPrompt,
    })
  })
  const loadTelegraphStatus = () => GetTelegraphSchedulerStatus().then((status) => {
    setTelegraphStatus({
      running: !!status?.running,
      lastRunAt: status?.lastRunAt || '',
      lastError: status?.lastError || '',
      lastFetched: Number(status?.lastFetched || 0),
      lastImported: Number(status?.lastImported || 0),
      lastAnalyzed: Number(status?.lastAnalyzed || 0),
    })
  })
  const loadUpdateCfg = () => GetAppUpdateConfig().then((cfg) => {
    setUpdateCfg({
      githubRepo: cfg?.githubRepo || '',
    })
  })
  const loadAppVersion = () => GetAppVersion().then((version) => setAppVersion(version || ''))
  const loadWatchlist = () => GetTelegraphWatchlist().then((list) => {
    setWatchStocks((list || []).map((item) => ({
      code: item.code || '',
      name: item.name || '',
      aliasesText: (item.aliases || []).join(', '),
    })))
  })
  const loadDashboard = (range: DashboardRange = dashboardRange) => {
    if (range === 0) {
      return GetAnalysisDashboard().then((data) => setDashboard(data || null))
    }
    return GetAnalysisDashboardByDays(range).then((data) => setDashboard(data || null))
  }

  useEffect(() => {
    loadChannels()
    loadPrompts()
    loadRoles()
    loadRoleTemplates()
    loadTags()
    loadMinerU()
    loadTelegraphCfg()
    loadTelegraphStatus()
    loadUpdateCfg()
    loadAppVersion()
    loadWatchlist()
    loadDashboard()
  }, [])

  useEffect(() => {
    if (tab === 'dashboard') {
      loadDashboard(dashboardRange)
    }
  }, [tab, dashboardRange])

  useEffect(() => {
    if (tab !== 'telegraph') {
      return
    }
    void loadTelegraphStatus()
    const timer = window.setInterval(() => {
      void loadTelegraphStatus()
    }, 4000)
    return () => window.clearInterval(timer)
  }, [tab])

  const saveCh = async () => {
    if (!editCh?.name || !editCh.baseUrl || !editCh.apiKey || !editCh.model) {
      return
    }
    await SaveChannel(new models.AIChannel(editCh))
    setEditCh(null)
    await loadChannels()
  }

  const savePr = async () => {
    if (!editPr?.name || !editPr.content) {
      return
    }
    await SavePrompt(new models.Prompt(editPr))
    setEditPr(null)
    await loadPrompts()
    if (versionPrompt && versionPrompt.id === editPr.id && editPr.id > 0) {
      await loadPromptVersions(editPr.id)
    }
  }

  const openPromptVersions = async (prompt: models.Prompt) => {
    setVersionPrompt(prompt)
    setDiffVersionID(0)
    await loadPromptVersions(prompt.id)
  }

  const restorePromptVersion = async (versionID: number) => {
    if (!versionPrompt || restoringPromptVersionID) {
      return
    }
    setRestoringPromptVersionID(versionID)
    try {
      await RestorePromptVersion(versionPrompt.id, versionID)
      await loadPrompts()
      await loadPromptVersions(versionPrompt.id)
      setEditPr(null)
    } finally {
      setRestoringPromptVersionID(0)
    }
  }

  const saveTagItem = async () => {
    if (!editTag?.name) {
      return
    }
    await SaveTag(new models.Tag(editTag))
    setEditTag(null)
    await loadTags()
  }

  const saveRoleItem = async () => {
    if (!editRole?.name || !editRole.systemPrompt) {
      return
    }
    const payload = new models.Role(editRole)
    if (payload.maxTokens <= 0) {
      payload.maxTokens = 1200
    }
    if (payload.temperature <= 0) {
      payload.temperature = 0.2
    }
    await SaveRole(payload)
    setEditRole(null)
    await loadRoles()
  }

  const createByTemplate = async (templateID: string) => {
    if (!templateID) {
      return
    }
    setCreatingTemplateID(templateID)
    try {
      await CreateRoleFromTemplate(templateID)
      await loadRoles()
    } finally {
      setCreatingTemplateID('')
    }
  }

  const saveMinerU = async () => {
    setMineruSaving(true)
    setMineruSavedTip('')
    try {
      await SaveMinerUConfig(new models.MinerUConfig(mineruCfg))
      setMineruSavedTip('已保存')
    } finally {
      setMineruSaving(false)
    }
  }

  const saveTelegraphScheduler = async () => {
    setTelegraphSaving(true)
    setTelegraphTip('')
    try {
      await SaveTelegraphSchedulerConfig(new models.TelegraphSchedulerConfig(telegraphCfg))
      setTelegraphTip('已保存')
      await loadTelegraphCfg()
      await loadTelegraphStatus()
    } catch (err) {
      setTelegraphTip(`保存失败: ${toErrorMessage(err)}`)
    } finally {
      setTelegraphSaving(false)
    }
  }

  const runTelegraphNow = async () => {
    setTelegraphRunningNow(true)
    setTelegraphTip('')
    try {
      await RunTelegraphSchedulerNow()
      setTelegraphTip('已触发执行')
      await loadTelegraphStatus()
    } catch (err) {
      setTelegraphTip(`执行失败: ${toErrorMessage(err)}`)
    } finally {
      setTelegraphRunningNow(false)
    }
  }

  const saveUpdateCfg = async () => {
    setUpdateSaving(true)
    setUpdateTip('')
    try {
      await SaveAppUpdateConfig(new models.AppUpdateConfig(updateCfg))
      setUpdateTip('已保存')
      await loadUpdateCfg()
    } catch (err) {
      setUpdateTip(`保存失败: ${toErrorMessage(err)}`)
    } finally {
      setUpdateSaving(false)
    }
  }

  const checkUpdateNow = async () => {
    setUpdateChecking(true)
    setUpdateTip('')
    try {
      const result = await CheckAppUpdate()
      const normalized = result ? new models.AppUpdateResult(result) : null
      setUpdateResult(normalized)
      setUpdateTip(normalized?.message || '检查完成')
      await loadAppVersion()
    } catch (err) {
      setUpdateTip(`检查失败: ${toErrorMessage(err)}`)
    } finally {
      setUpdateChecking(false)
    }
  }

  const openExternalURL = async (url: string) => {
    try {
      await OpenURL(url)
    } catch (err) {
      setUpdateTip(`打开失败: ${toErrorMessage(err)}`)
    }
  }

  const installUpdateNow = async () => {
    if (!updateResult?.downloadUrl) {
      return
    }
    setUpdateInstalling(true)
    setUpdateTip('')
    try {
      const msg = await DownloadAndInstallAppUpdate(updateResult.downloadUrl, updateResult.downloadName || '')
      setUpdateTip(msg || '已启动安装')
    } catch (err) {
      setUpdateTip(`安装失败: ${toErrorMessage(err)}`)
    } finally {
      setUpdateInstalling(false)
    }
  }

  const stopTelegraphNow = async () => {
    setTelegraphRunningNow(true)
    setTelegraphTip('')
    try {
      await StopTelegraphScheduler()
      setTelegraphTip('已停止并关闭自动任务')
      await loadTelegraphCfg()
      await loadTelegraphStatus()
    } catch (err) {
      setTelegraphTip(`停止失败: ${toErrorMessage(err)}`)
    } finally {
      setTelegraphRunningNow(false)
    }
  }

  const saveWatchlist = async () => {
    setWatchSaving(true)
    setWatchTip('')
    try {
      const payload = watchStocks
        .map((item) => {
          const aliases = (item.aliasesText || '')
            .split(/[,\uff0c]/g)
            .map((part) => part.trim())
            .filter(Boolean)
          return new models.WatchStock({
            code: item.code.trim(),
            name: item.name.trim(),
            aliases,
          })
        })
        .filter((item) => item.code)
      await SaveTelegraphWatchlist(payload)
      setWatchTip('已保存并重建历史新闻映射')
      await loadWatchlist()
    } catch (err) {
      setWatchTip(`保存失败: ${toErrorMessage(err)}`)
    } finally {
      setWatchSaving(false)
    }
  }

  const activePrompt = useMemo(
    () => (versionPrompt ? (prompts.find((item) => item.id === versionPrompt.id) || versionPrompt) : null),
    [prompts, versionPrompt],
  )

  const diffVersion = useMemo(
    () => promptVersions.find((item) => item.id === diffVersionID) || null,
    [promptVersions, diffVersionID],
  )

  const promptDiffLines = useMemo(
    () => buildLineDiff(diffVersion?.content || '', activePrompt?.content || ''),
    [diffVersion?.content, activePrompt?.content],
  )

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <h2 className="text-xl font-semibold text-gray-800 mb-5">设置</h2>
      <div className="flex gap-1 mb-6 bg-gray-100 p-1 rounded-lg w-fit">
        <TabButton tab={tab} value="channels" onClick={setTab} label="AI 渠道" />
        <TabButton tab={tab} value="prompts" onClick={setTab} label="提示词模板" />
        <TabButton tab={tab} value="roles" onClick={setTab} label="问答角色" />
        <TabButton tab={tab} value="mineru" onClick={setTab} label="MinerU 解析" />
        <TabButton tab={tab} value="telegraph" onClick={setTab} label="财联社电报" />
        <TabButton tab={tab} value="updater" onClick={setTab} label="应用更新" />
        <TabButton tab={tab} value="watchlist" onClick={setTab} label="自选股池" />
        <TabButton tab={tab} value="tags" onClick={setTab} label="标签管理" />
        <TabButton tab={tab} value="dashboard" onClick={setTab} label="运行看板" />
      </div>

      {tab === 'channels' && (
        <div>
          <button
            onClick={() => setEditCh({ id: 0, name: '', baseUrl: '', apiKey: '', model: '', isDefault: 0 })}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg mb-4 hover:bg-blue-600 shadow-sm transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
            添加渠道
          </button>
          {editCh && <ChannelForm ch={editCh} onChange={setEditCh} onSave={saveCh} onCancel={() => setEditCh(null)} />}
          <div className="space-y-2">
            {channels.map((c) => (
              <div key={c.id} className="group flex items-center justify-between p-4 bg-white rounded-xl border border-gray-200/80 hover:border-gray-300 transition-colors">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-800">{c.name}</span>
                    <span className="text-xs text-gray-400 bg-gray-100 px-2 py-0.5 rounded-md">{c.model}</span>
                    {c.isDefault === 1 && <span className="text-xs bg-blue-50 text-blue-600 px-2 py-0.5 rounded-full ring-1 ring-blue-200 font-medium">默认</span>}
                  </div>
                  <div className="text-xs text-gray-400 mt-1.5 truncate">{c.baseUrl}</div>
                </div>
                <div className="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button onClick={() => setEditCh(toChannelFormData(c))} className="text-xs text-blue-500 hover:text-blue-700 font-medium">编辑</button>
                  <button onClick={async () => { await DeleteChannel(c.id); await loadChannels() }} className="text-xs text-red-400 hover:text-red-600 font-medium">删除</button>
                </div>
              </div>
            ))}
            {channels.length === 0 && !editCh && (
              <div className="text-center py-12 text-sm text-gray-400">暂无 AI 渠道，点击上方按钮添加</div>
            )}
          </div>
        </div>
      )}

      {tab === 'prompts' && (
        <div>
          <button
            onClick={() => setEditPr({ id: 0, name: '', content: '', isDefault: 0 })}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg mb-4 hover:bg-blue-600 shadow-sm transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
            添加提示词
          </button>
          {editPr && <PromptForm pr={editPr} onChange={setEditPr} onSave={savePr} onCancel={() => setEditPr(null)} />}
          <div className="space-y-2">
            {prompts.map((p) => (
              <div key={p.id} className="group flex items-center justify-between p-4 bg-white rounded-xl border border-gray-200/80 hover:border-gray-300 transition-colors">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-800">{p.name}</span>
                    {p.isDefault === 1 && <span className="text-xs bg-blue-50 text-blue-600 px-2 py-0.5 rounded-full ring-1 ring-blue-200 font-medium">默认</span>}
                  </div>
                  <div className="text-xs text-gray-400 mt-1.5 truncate max-w-lg">{p.content}</div>
                </div>
                <div className="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0 ml-4">
                  <button onClick={() => void openPromptVersions(p)} className="text-xs text-indigo-500 hover:text-indigo-700 font-medium">版本</button>
                  <button onClick={() => setEditPr(toPromptFormData(p))} className="text-xs text-blue-500 hover:text-blue-700 font-medium">编辑</button>
                  <button
                    onClick={async () => {
                      await DeletePrompt(p.id)
                      await loadPrompts()
                      if (versionPrompt?.id === p.id) {
                        setVersionPrompt(null)
                        setPromptVersions([])
                        setDiffVersionID(0)
                      }
                    }}
                    className="text-xs text-red-400 hover:text-red-600 font-medium"
                  >
                    删除
                  </button>
                </div>
              </div>
            ))}
            {prompts.length === 0 && !editPr && (
              <div className="text-center py-12 text-sm text-gray-400">暂无提示词模板，点击上方按钮添加</div>
            )}
          </div>
          {versionPrompt && (
            <div className="mt-4 p-4 bg-white rounded-xl border border-indigo-200">
              <div className="flex items-center justify-between mb-3">
                <div>
                  <div className="text-sm font-semibold text-gray-800">版本历史：{versionPrompt.name}</div>
                  <div className="text-xs text-gray-500 mt-1">每次修改名称或内容会自动生成新版本，可一键恢复。</div>
                </div>
                <button
                  onClick={() => {
                    setVersionPrompt(null)
                    setPromptVersions([])
                    setDiffVersionID(0)
                  }}
                  className="px-2.5 py-1.5 text-xs bg-gray-100 text-gray-600 rounded-md hover:bg-gray-200"
                >
                  关闭
                </button>
              </div>
              <div className="space-y-2 max-h-72 overflow-auto pr-1">
                {promptVersions.map((version) => (
                  <div key={version.id} className="border border-gray-200 rounded-lg p-3">
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-xs text-gray-700">
                        <span className="font-semibold">v{version.versionNo}</span>
                        <span className="text-gray-400 ml-2">{formatDateTime(version.createdAt)}</span>
                      </div>
                      <button
                        onClick={() => void restorePromptVersion(version.id)}
                        disabled={restoringPromptVersionID === version.id}
                        className="px-2.5 py-1 text-xs bg-indigo-500 text-white rounded-md hover:bg-indigo-600 disabled:opacity-50"
                      >
                        {restoringPromptVersionID === version.id ? '恢复中...' : '恢复此版本'}
                      </button>
                      <button
                        onClick={() => setDiffVersionID((prev) => (prev === version.id ? 0 : version.id))}
                        className="px-2.5 py-1 text-xs bg-white border border-indigo-200 text-indigo-600 rounded-md hover:bg-indigo-50"
                      >
                        {diffVersionID === version.id ? '收起差异' : '查看差异'}
                      </button>
                    </div>
                    <div className="mt-2 text-xs text-gray-500">名称：{version.name}</div>
                    <div className="mt-1 text-xs text-gray-500 whitespace-pre-wrap line-clamp-3">{version.content}</div>
                  </div>
                ))}
                {promptVersions.length === 0 && (
                  <div className="text-xs text-gray-400 py-4 text-center">暂无版本记录</div>
                )}
              </div>
              {diffVersion && activePrompt && (
                <div className="mt-4 border border-indigo-200 rounded-lg bg-indigo-50/40 p-3">
                  <div className="text-xs text-indigo-700 font-semibold">
                    差异对比：v{diffVersion.versionNo} vs 当前
                  </div>
                  <div className="mt-1 text-xs text-gray-600">
                    名称：{diffVersion.name === activePrompt.name ? '无变化' : `“${diffVersion.name}” -> “${activePrompt.name}”`}
                  </div>
                  <div className="mt-2 max-h-72 overflow-auto rounded-md border border-indigo-100 bg-white">
                    {promptDiffLines.length > 0 ? (
                      promptDiffLines.map((line, idx) => (
                        <div
                          key={`${idx}-${line.text}`}
                          className={`px-2 py-0.5 text-[11px] font-mono whitespace-pre-wrap break-words ${
                            line.type === 'add'
                              ? 'bg-emerald-50 text-emerald-700'
                              : line.type === 'remove'
                                ? 'bg-rose-50 text-rose-700'
                                : 'text-gray-500'
                          }`}
                        >
                          {line.type === 'add' ? '+' : line.type === 'remove' ? '-' : ' '} {line.text}
                        </div>
                      ))
                    ) : (
                      <div className="px-2 py-1 text-[11px] text-gray-400 font-mono">无内容</div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {tab === 'roles' && (
        <div>
          {roleTemplates.length > 0 && (
            <div className="p-4 mb-4 bg-amber-50 border border-amber-200 rounded-xl">
              <div className="text-sm font-semibold text-amber-800 mb-2">角色模板库</div>
              <div className="text-xs text-amber-700 mb-3">一键创建内置专业角色，你也可以在创建后继续编辑提示词。</div>
              <div className="grid grid-cols-2 gap-2">
                {roleTemplates.map((tpl) => (
                  <div key={tpl.id} className="bg-white border border-amber-100 rounded-lg p-3">
                    <div className="text-sm font-medium text-gray-800">{tpl.name}</div>
                    <div className="text-xs text-gray-500 mt-1">{tpl.domainTags}</div>
                    <button
                      onClick={() => createByTemplate(tpl.id)}
                      disabled={creatingTemplateID === tpl.id}
                      className="mt-2 px-3 py-1.5 text-xs bg-amber-500 text-white rounded-md hover:bg-amber-600 disabled:opacity-50"
                    >
                      {creatingTemplateID === tpl.id ? '创建中...' : '一键创建'}
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          <button
            onClick={() => setEditRole({ id: 0, name: '', alias: '', domainTags: '', systemPrompt: '', modelOverride: '', temperature: 0.2, maxTokens: 1200, enabled: 1, isDefault: 0 })}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg mb-4 hover:bg-blue-600 shadow-sm transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
            添加角色
          </button>
          {editRole && <RoleForm role={editRole} onChange={setEditRole} onSave={saveRoleItem} onCancel={() => setEditRole(null)} />}
          <div className="space-y-2">
            {roles.map((role) => (
              <div key={role.id} className="group flex items-center justify-between p-4 bg-white rounded-xl border border-gray-200/80 hover:border-gray-300 transition-colors">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-800">{role.name}</span>
                    {role.alias && <span className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded-md">@{role.alias}</span>}
                    {role.enabled === 1 ? (
                      <span className="text-xs bg-emerald-50 text-emerald-600 px-2 py-0.5 rounded-full ring-1 ring-emerald-200 font-medium">启用</span>
                    ) : (
                      <span className="text-xs bg-gray-100 text-gray-500 px-2 py-0.5 rounded-full ring-1 ring-gray-200 font-medium">停用</span>
                    )}
                    {role.isDefault === 1 && <span className="text-xs bg-blue-50 text-blue-600 px-2 py-0.5 rounded-full ring-1 ring-blue-200 font-medium">默认</span>}
                  </div>
                  <div className="text-xs text-gray-400 mt-1.5 truncate">{role.domainTags || '未设置领域标签'}</div>
                  <div className="text-xs text-gray-400 mt-0.5 truncate">模型覆盖: {role.modelOverride || '跟随默认渠道'}</div>
                </div>
                <div className="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0 ml-4">
                  {role.isDefault !== 1 && (
                    <button
                      onClick={async () => {
                        await SetDefaultRole(role.id)
                        await loadRoles()
                      }}
                      className="text-xs text-indigo-500 hover:text-indigo-700 font-medium"
                    >
                      设默认
                    </button>
                  )}
                  <button onClick={() => setEditRole(toRoleFormData(role))} className="text-xs text-blue-500 hover:text-blue-700 font-medium">编辑</button>
                  <button onClick={async () => { await DeleteRole(role.id); await loadRoles() }} className="text-xs text-red-400 hover:text-red-600 font-medium">删除</button>
                </div>
              </div>
            ))}
            {roles.length === 0 && !editRole && (
              <div className="text-center py-12 text-sm text-gray-400">暂无角色，点击上方按钮添加</div>
            )}
          </div>
        </div>
      )}

      {tab === 'mineru' && (
        <div className="bg-white rounded-xl border border-gray-200 p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-gray-800">MinerU 文档解析</h3>
            <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
              <input
                type="checkbox"
                checked={mineruCfg.enabled === 1}
                onChange={(e) => setMineruCfg({ ...mineruCfg, enabled: e.target.checked ? 1 : 0 })}
                className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
              />
              启用
            </label>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">Base URL</label>
              <input
                value={mineruCfg.baseUrl}
                onChange={(e) => setMineruCfg({ ...mineruCfg, baseUrl: e.target.value })}
                placeholder="https://mineru.net"
                className={inputCls}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">模型版本</label>
              <select
                value={mineruCfg.modelVersion}
                onChange={(e) => setMineruCfg({ ...mineruCfg, modelVersion: e.target.value })}
                className={inputCls}
              >
                <option value="vlm">vlm（推荐）</option>
                <option value="pipeline">pipeline</option>
              </select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">API Token</label>
            <input
              type="password"
              value={mineruCfg.apiToken}
              onChange={(e) => setMineruCfg({ ...mineruCfg, apiToken: e.target.value })}
              placeholder="在 MinerU 控制台创建的 API Token"
              className={inputCls}
            />
          </div>

          <div className="grid grid-cols-3 gap-3">
            <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer mt-6">
              <input
                type="checkbox"
                checked={mineruCfg.isOCR === 1}
                onChange={(e) => setMineruCfg({ ...mineruCfg, isOCR: e.target.checked ? 1 : 0 })}
                className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
              />
              图片 OCR
            </label>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">轮询间隔(ms)</label>
              <input
                type="number"
                min={500}
                step={100}
                value={mineruCfg.pollIntervalMs}
                onChange={(e) => setMineruCfg({ ...mineruCfg, pollIntervalMs: Number(e.target.value) })}
                className={inputCls}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">超时(秒)</label>
              <input
                type="number"
                min={30}
                step={10}
                value={mineruCfg.timeoutSec}
                onChange={(e) => setMineruCfg({ ...mineruCfg, timeoutSec: Number(e.target.value) })}
                className={inputCls}
              />
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={saveMinerU}
              disabled={mineruSaving}
              className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors disabled:opacity-50"
            >
              {mineruSaving ? '保存中...' : '保存配置'}
            </button>
            {mineruSavedTip && <span className="text-xs text-emerald-600">{mineruSavedTip}</span>}
          </div>

          <div className="text-xs text-gray-500 leading-relaxed">
            导入 PDF/图片/Office 文档时会先调用 MinerU 解析，再将 Markdown 内容写入文章原文。
          </div>
        </div>
      )}

      {tab === 'telegraph' && (
        <div className="bg-white rounded-xl border border-gray-200 p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-gray-800">财联社 24H 电报自动解读</h3>
            <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
              <input
                type="checkbox"
                checked={telegraphCfg.enabled === 1}
                onChange={(e) => setTelegraphCfg({ ...telegraphCfg, enabled: e.target.checked ? 1 : 0 })}
                className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
              />
              启用
            </label>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">抓取地址</label>
              <input
                value={telegraphCfg.sourceUrl}
                onChange={(e) => setTelegraphCfg({ ...telegraphCfg, sourceUrl: e.target.value })}
                placeholder="https://m.cls.cn/telegraph"
                className={inputCls}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">抓取条数</label>
              <input
                type="number"
                min={1}
                max={50}
                value={telegraphCfg.fetchLimit}
                onChange={(e) => setTelegraphCfg({ ...telegraphCfg, fetchLimit: Number(e.target.value) })}
                className={inputCls}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">执行间隔(分钟)</label>
              <input
                type="number"
                min={1}
                max={1440}
                value={telegraphCfg.intervalMinutes}
                onChange={(e) => setTelegraphCfg({ ...telegraphCfg, intervalMinutes: Number(e.target.value) })}
                className={inputCls}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">AI 渠道</label>
              <select
                value={telegraphCfg.channelId}
                onChange={(e) => setTelegraphCfg({ ...telegraphCfg, channelId: Number(e.target.value) })}
                className={inputCls}
              >
                <option value={0}>自动（默认渠道）</option>
                {channels.map((channel) => (
                  <option key={channel.id} value={channel.id}>
                    {channel.name} / {channel.model}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">新闻解读专用提示词</label>
            <textarea
              rows={5}
              value={telegraphCfg.analysisPrompt}
              onChange={(e) => setTelegraphCfg({ ...telegraphCfg, analysisPrompt: e.target.value })}
              className={`${inputCls} resize-y`}
              placeholder="该提示词只用于财联社新闻自动解读，不影响普通文章解读。"
            />
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={saveTelegraphScheduler}
              disabled={telegraphSaving}
              className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors disabled:opacity-50"
            >
              {telegraphSaving ? '保存中...' : '保存配置'}
            </button>
            <button
              onClick={runTelegraphNow}
              disabled={telegraphRunningNow || telegraphStatus.running}
              className="px-4 py-2 bg-emerald-500 text-white text-sm rounded-lg hover:bg-emerald-600 shadow-sm transition-colors disabled:opacity-50"
            >
              {telegraphRunningNow ? '触发中...' : telegraphStatus.running ? '运行中...' : '立即执行'}
            </button>
            <button
              onClick={stopTelegraphNow}
              disabled={telegraphRunningNow}
              className="px-4 py-2 bg-rose-500 text-white text-sm rounded-lg hover:bg-rose-600 shadow-sm transition-colors disabled:opacity-50"
            >
              立即停止
            </button>
            <button
              onClick={() => void loadTelegraphStatus()}
              className="px-4 py-2 bg-white border border-gray-200 text-gray-700 text-sm rounded-lg hover:bg-gray-50 transition-colors"
            >
              刷新状态
            </button>
            {telegraphTip && <span className="text-xs text-emerald-600">{telegraphTip}</span>}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div className="text-xs text-gray-500">运行状态</div>
              <div className={`mt-1 text-sm font-semibold ${telegraphStatus.running ? 'text-emerald-600' : 'text-gray-700'}`}>
                {telegraphStatus.running ? '执行中' : '空闲'}
              </div>
            </div>
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div className="text-xs text-gray-500">上次执行时间</div>
              <div className="mt-1 text-sm font-semibold text-gray-700">{formatDateTime(telegraphStatus.lastRunAt)}</div>
            </div>
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div className="text-xs text-gray-500">上次抓取/导入</div>
              <div className="mt-1 text-sm font-semibold text-gray-700">
                {telegraphStatus.lastFetched} / {telegraphStatus.lastImported}
              </div>
            </div>
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div className="text-xs text-gray-500">上次 AI 解读数</div>
              <div className="mt-1 text-sm font-semibold text-gray-700">{telegraphStatus.lastAnalyzed}</div>
            </div>
          </div>

          {telegraphStatus.lastError && (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700 break-all">
              最近错误：{telegraphStatus.lastError}
            </div>
          )}

          <div className="text-xs text-gray-500 leading-relaxed">
            定时任务会抓取电报、自动去重入库并调用 AI 解读；新内容会按时间顺序处理并写入“新闻电报”页面。
          </div>
        </div>
      )}

      {tab === 'updater' && (
        <div className="bg-white rounded-xl border border-gray-200 p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-gray-800">应用更新（GitHub Release）</h3>
            <span className="text-xs text-gray-500">当前版本：{appVersion || '-'}</span>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">GitHub 仓库</label>
            <input
              value={updateCfg.githubRepo}
              onChange={(e) => setUpdateCfg({ githubRepo: e.target.value })}
              placeholder="owner/repo，例如 boohee/stock-report-analysis"
              className={inputCls}
            />
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={saveUpdateCfg}
              disabled={updateSaving}
              className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors disabled:opacity-50"
            >
              {updateSaving ? '保存中...' : '保存更新源'}
            </button>
            <button
              onClick={checkUpdateNow}
              disabled={updateChecking}
              className="px-4 py-2 bg-emerald-500 text-white text-sm rounded-lg hover:bg-emerald-600 shadow-sm transition-colors disabled:opacity-50"
            >
              {updateChecking ? '检查中...' : '检查更新'}
            </button>
            {updateResult?.releaseUrl && (
              <button
                onClick={() => void openExternalURL(updateResult.releaseUrl)}
                className="px-4 py-2 bg-white border border-gray-200 text-gray-700 text-sm rounded-lg hover:bg-gray-50 transition-colors"
              >
                打开发布页
              </button>
            )}
            {updateResult?.downloadUrl && (
              <button
                onClick={() => void openExternalURL(updateResult.downloadUrl)}
                className="px-4 py-2 bg-white border border-gray-200 text-gray-700 text-sm rounded-lg hover:bg-gray-50 transition-colors"
              >
                下载此平台安装包
              </button>
            )}
            {updateResult?.hasUpdate && updateResult?.os === 'windows' && updateResult?.downloadUrl && (
              <button
                onClick={installUpdateNow}
                disabled={updateInstalling}
                className="px-4 py-2 bg-indigo-600 text-white text-sm rounded-lg hover:bg-indigo-700 shadow-sm transition-colors disabled:opacity-50"
              >
                {updateInstalling ? '下载安装中...' : '一键下载安装并重启'}
              </button>
            )}
            {updateTip && <span className="text-xs text-emerald-600">{updateTip}</span>}
          </div>

          {updateResult && (
            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs text-gray-500">最新版本</div>
                <div className="mt-1 text-sm font-semibold text-gray-700">{updateResult.latestVersion || '-'}</div>
              </div>
              <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs text-gray-500">更新状态</div>
                <div className={`mt-1 text-sm font-semibold ${updateResult.hasUpdate ? 'text-emerald-600' : 'text-gray-700'}`}>
                  {updateResult.hasUpdate ? '有新版本' : '已是最新'}
                </div>
              </div>
              <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs text-gray-500">适配平台</div>
                <div className="mt-1 text-sm font-semibold text-gray-700">
                  {updateResult.os}/{updateResult.arch}
                </div>
              </div>
              <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs text-gray-500">发布时间</div>
                <div className="mt-1 text-sm font-semibold text-gray-700">{formatDateTime(updateResult.publishedAt)}</div>
              </div>
              <div className="col-span-2 rounded-lg border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs text-gray-500">安装包</div>
                <div className="mt-1 text-sm font-semibold text-gray-700 break-all">{updateResult.downloadName || '未匹配到当前系统安装包'}</div>
              </div>
            </div>
          )}

          <div className="text-xs text-gray-500 leading-relaxed">
            发布新版本流程：推送 `v1.2.3` 这类 tag 后，GitHub Actions 会自动构建 Win/Mac 包并发布到 Release，客户端在此页可检查并跳转下载。
          </div>
        </div>
      )}

      {tab === 'watchlist' && (
        <div className="bg-white rounded-xl border border-gray-200 p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-gray-800">自选股池</h3>
            <button
              onClick={() => setWatchStocks((prev) => [...prev, { code: '', name: '', aliasesText: '' }])}
              className="px-3 py-1.5 text-xs bg-blue-500 text-white rounded-md hover:bg-blue-600"
            >
              添加一行
            </button>
          </div>

          <div className="text-xs text-gray-500 leading-relaxed">
            配置后，新闻会自动映射到你的自选股；新闻页可“仅看自选”并支持“自选优先”排序。
          </div>

          <div className="space-y-2">
            {watchStocks.map((item, idx) => (
              <div key={`watch-${idx}`} className="grid grid-cols-12 gap-2">
                <input
                  value={item.code}
                  onChange={(e) => {
                    const value = e.target.value.replace(/\D/g, '').slice(0, 6)
                    setWatchStocks((prev) => prev.map((row, i) => (i === idx ? { ...row, code: value } : row)))
                  }}
                  placeholder="股票代码 600519"
                  className="col-span-2 px-3 py-2 border border-gray-200 rounded-lg text-sm"
                />
                <input
                  value={item.name}
                  onChange={(e) => setWatchStocks((prev) => prev.map((row, i) => (i === idx ? { ...row, name: e.target.value } : row)))}
                  placeholder="股票名称 贵州茅台"
                  className="col-span-3 px-3 py-2 border border-gray-200 rounded-lg text-sm"
                />
                <input
                  value={item.aliasesText}
                  onChange={(e) => setWatchStocks((prev) => prev.map((row, i) => (i === idx ? { ...row, aliasesText: e.target.value } : row)))}
                  placeholder="别名（逗号分隔） 茅台, Moutai"
                  className="col-span-6 px-3 py-2 border border-gray-200 rounded-lg text-sm"
                />
                <button
                  onClick={() => setWatchStocks((prev) => prev.filter((_, i) => i !== idx))}
                  className="col-span-1 px-2 py-2 text-xs bg-rose-50 text-rose-600 rounded-lg hover:bg-rose-100"
                >
                  删除
                </button>
              </div>
            ))}
            {watchStocks.length === 0 && (
              <div className="text-center py-8 text-sm text-gray-400">暂无自选股，点击“添加一行”开始配置</div>
            )}
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={saveWatchlist}
              disabled={watchSaving}
              className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors disabled:opacity-50"
            >
              {watchSaving ? '保存中...' : '保存自选股池'}
            </button>
            {watchTip && <span className="text-xs text-emerald-600">{watchTip}</span>}
          </div>
        </div>
      )}

      {tab === 'tags' && (
        <div>
          <button
            onClick={() => setEditTag({ id: 0, name: '', color: TAG_COLORS[0] })}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg mb-4 hover:bg-blue-600 shadow-sm transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
            添加标签
          </button>
          {editTag && (
            <div className="p-5 bg-white rounded-xl border border-blue-200 mb-4 space-y-3 shadow-sm">
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1.5">标签名称</label>
                <input placeholder="如 宏观、个股、行业" value={editTag.name} onChange={(e) => setEditTag({ ...editTag, name: e.target.value })} className={inputCls} />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1.5">颜色</label>
                <div className="flex gap-2">
                  {TAG_COLORS.map((color) => (
                    <button
                      key={color}
                      onClick={() => setEditTag({ ...editTag, color })}
                      className={`w-7 h-7 rounded-full transition-transform ${editTag.color === color ? 'ring-2 ring-offset-2 ring-blue-400 scale-110' : 'hover:scale-110'}`}
                      style={{ backgroundColor: color }}
                    />
                  ))}
                </div>
              </div>
              <div className="flex gap-2 pt-1">
                <button onClick={saveTagItem} className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors">保存</button>
                <button onClick={() => setEditTag(null)} className="px-4 py-2 bg-gray-100 text-gray-600 text-sm rounded-lg hover:bg-gray-200 transition-colors">取消</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {tags.map((t) => (
              <div key={t.id} className="group flex items-center justify-between p-4 bg-white rounded-xl border border-gray-200/80 hover:border-gray-300 transition-colors">
                <div className="flex items-center gap-3">
                  <span className="w-4 h-4 rounded-full shrink-0" style={{ backgroundColor: t.color }} />
                  <span className="text-sm font-medium text-gray-800">{t.name}</span>
                </div>
                <div className="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button onClick={() => setEditTag(toTagFormData(t))} className="text-xs text-blue-500 hover:text-blue-700 font-medium">编辑</button>
                  <button onClick={async () => { await DeleteTag(t.id); await loadTags() }} className="text-xs text-red-400 hover:text-red-600 font-medium">删除</button>
                </div>
              </div>
            ))}
            {tags.length === 0 && !editTag && (
              <div className="text-center py-12 text-sm text-gray-400">暂无标签，点击上方按钮添加</div>
            )}
          </div>
        </div>
      )}

      {tab === 'dashboard' && (
        <DashboardPanel data={dashboard} range={dashboardRange} onRangeChange={setDashboardRange} onRefresh={() => loadDashboard(dashboardRange)} />
      )}
    </div>
  )
}

function formatDateTime(value: unknown): string {
  if (!value) {
    return '-'
  }
  const dt = new Date(String(value))
  if (Number.isNaN(dt.getTime())) {
    return String(value)
  }
  return dt.toLocaleString()
}

function toErrorMessage(err: unknown): string {
  if (err instanceof Error && err.message) {
    return err.message
  }
  if (typeof err === 'string') {
    return err
  }
  return '未知错误'
}

type DiffLine = {
  type: 'same' | 'add' | 'remove'
  text: string
}

function splitLines(text: string): string[] {
  const normalized = (text || '').replace(/\r\n/g, '\n')
  if (!normalized) {
    return []
  }
  return normalized.split('\n')
}

function buildLineDiff(fromText: string, toText: string): DiffLine[] {
  const from = splitLines(fromText)
  const to = splitLines(toText)
  const m = from.length
  const n = to.length

  if (m === 0 && n === 0) {
    return []
  }

  const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0))
  for (let i = m - 1; i >= 0; i -= 1) {
    for (let j = n - 1; j >= 0; j -= 1) {
      if (from[i] === to[j]) {
        dp[i][j] = dp[i + 1][j + 1] + 1
      } else {
        dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1])
      }
    }
  }

  const result: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < m && j < n) {
    if (from[i] === to[j]) {
      result.push({ type: 'same', text: from[i] })
      i += 1
      j += 1
      continue
    }
    if (dp[i + 1][j] >= dp[i][j + 1]) {
      result.push({ type: 'remove', text: from[i] })
      i += 1
    } else {
      result.push({ type: 'add', text: to[j] })
      j += 1
    }
  }
  while (i < m) {
    result.push({ type: 'remove', text: from[i] })
    i += 1
  }
  while (j < n) {
    result.push({ type: 'add', text: to[j] })
    j += 1
  }
  return result
}

function TabButton({ tab, value, onClick, label }: { tab: TabKey; value: TabKey; onClick: (key: TabKey) => void; label: string }) {
  return (
    <button
      onClick={() => onClick(value)}
      className={`px-4 py-2 text-sm rounded-md transition-colors ${tab === value ? 'bg-white text-gray-800 font-medium shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
    >
      {label}
    </button>
  )
}

function DashboardPanel({
  data,
  range,
  onRangeChange,
  onRefresh,
}: {
  data: models.AnalysisDashboard | null
  range: DashboardRange
  onRangeChange: (next: DashboardRange) => void
  onRefresh: () => void
}) {
  const summaryItems = [
    { label: '总运行次数', value: data?.totalRuns ?? 0 },
    { label: '成功率', value: data?.successRate ?? '0%' },
    { label: '平均耗时', value: `${data?.avgDurationMs ?? 0} ms` },
    { label: '总 Token', value: data?.totalTokens ?? 0 },
  ]

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold text-gray-800">分析运行看板</h3>
        <div className="flex items-center gap-2">
          <select
            value={range}
            onChange={(e) => onRangeChange(Number(e.target.value) as DashboardRange)}
            className="text-xs bg-white border border-gray-200 rounded-lg px-2 py-1.5"
          >
            <option value={0}>全部</option>
            <option value={7}>近 7 天</option>
            <option value={30}>近 30 天</option>
          </select>
          <button onClick={onRefresh} className="px-3 py-1.5 text-xs bg-white border border-gray-200 rounded-lg hover:bg-gray-50">刷新</button>
        </div>
      </div>

      <div className="grid grid-cols-4 gap-3">
        {summaryItems.map((item) => (
          <div key={item.label} className="bg-white border border-gray-200 rounded-xl p-4">
            <div className="text-xs text-gray-500 mb-1">{item.label}</div>
            <div className="text-lg font-semibold text-gray-800">{item.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <section className="bg-white border border-gray-200 rounded-xl p-4">
          <h4 className="text-sm font-semibold text-gray-800 mb-3">按渠道质量统计</h4>
          <div className="overflow-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-gray-500 border-b border-gray-100">
                  <th className="text-left py-2 pr-2">渠道</th>
                  <th className="text-right py-2 pr-2">运行</th>
                  <th className="text-right py-2 pr-2">成功率</th>
                  <th className="text-right py-2 pr-2">均耗时</th>
                  <th className="text-right py-2">Token</th>
                </tr>
              </thead>
              <tbody>
                {(data?.byChannel || []).map((row) => (
                  <tr key={`${row.channelId}-${row.channelName}`} className="border-b border-gray-50 last:border-0">
                    <td className="py-2 pr-2 text-gray-700">{row.channelName}</td>
                    <td className="py-2 pr-2 text-right text-gray-700">{row.totalRuns}</td>
                    <td className="py-2 pr-2 text-right text-gray-700">{row.successRate}</td>
                    <td className="py-2 pr-2 text-right text-gray-700">{row.avgDuration} ms</td>
                    <td className="py-2 text-right text-gray-700">{row.totalTokens}</td>
                  </tr>
                ))}
                {(data?.byChannel?.length || 0) === 0 && (
                  <tr>
                    <td colSpan={5} className="py-6 text-center text-gray-400">暂无数据</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section className="bg-white border border-gray-200 rounded-xl p-4">
          <h4 className="text-sm font-semibold text-gray-800 mb-3">失败原因 Top</h4>
          <div className="space-y-2">
            {(data?.failureTop || []).map((item) => (
              <div key={item.reason} className="flex items-center justify-between px-3 py-2 bg-red-50 border border-red-100 rounded-lg">
                <span className="text-xs text-red-700">{item.reason}</span>
                <span className="text-xs font-semibold text-red-600">{item.count}</span>
              </div>
            ))}
            {(data?.failureTop?.length || 0) === 0 && (
              <div className="text-xs text-gray-400">暂无失败原因统计</div>
            )}
          </div>

          <div className="mt-4 text-xs text-gray-500 space-y-1">
            <div>成功: {data?.successRuns ?? 0}</div>
            <div>失败: {data?.failedRuns ?? 0}</div>
            <div>输入 Token: {data?.promptTokens ?? 0}</div>
            <div>输出 Token: {data?.outputTokens ?? 0}</div>
          </div>
        </section>
      </div>
    </div>
  )
}

type ChannelFormProps = {
  ch: ChannelFormData
  onChange: (value: ChannelFormData) => void
  onSave: () => void
  onCancel: () => void
}

function ChannelForm({ ch, onChange, onSave, onCancel }: ChannelFormProps) {
  const setField = <K extends keyof ChannelFormData>(key: K, value: ChannelFormData[K]) => {
    onChange({ ...ch, [key]: value })
  }

  return (
    <div className="p-5 bg-white rounded-xl border border-blue-200 mb-4 space-y-3 shadow-sm">
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">渠道名称</label>
          <input placeholder="如 DeepSeek" value={ch.name} onChange={(e) => setField('name', e.target.value)} className={inputCls} />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">模型名称</label>
          <input placeholder="如 deepseek-chat" value={ch.model} onChange={(e) => setField('model', e.target.value)} className={inputCls} />
        </div>
      </div>
      <div>
        <label className="block text-xs font-medium text-gray-500 mb-1.5">API Base URL</label>
        <input placeholder="如 https://api.deepseek.com/v1" value={ch.baseUrl} onChange={(e) => setField('baseUrl', e.target.value)} className={inputCls} />
      </div>
      <div>
        <label className="block text-xs font-medium text-gray-500 mb-1.5">API Key</label>
        <input placeholder="sk-..." type="password" value={ch.apiKey} onChange={(e) => setField('apiKey', e.target.value)} className={inputCls} />
      </div>
      <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
        <input
          type="checkbox"
          checked={ch.isDefault === 1}
          onChange={(e) => setField('isDefault', e.target.checked ? 1 : 0)}
          className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
        />
        设为默认渠道
      </label>
      <div className="flex gap-2 pt-1">
        <button onClick={onSave} className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors">保存</button>
        <button onClick={onCancel} className="px-4 py-2 bg-gray-100 text-gray-600 text-sm rounded-lg hover:bg-gray-200 transition-colors">取消</button>
      </div>
    </div>
  )
}

type PromptFormProps = {
  pr: PromptFormData
  onChange: (value: PromptFormData) => void
  onSave: () => void
  onCancel: () => void
}

function PromptForm({ pr, onChange, onSave, onCancel }: PromptFormProps) {
  const setField = <K extends keyof PromptFormData>(key: K, value: PromptFormData[K]) => {
    onChange({ ...pr, [key]: value })
  }

  return (
    <div className="p-5 bg-white rounded-xl border border-blue-200 mb-4 space-y-3 shadow-sm">
      <div>
        <label className="block text-xs font-medium text-gray-500 mb-1.5">提示词名称</label>
        <input placeholder="如 股票分析" value={pr.name} onChange={(e) => setField('name', e.target.value)} className={inputCls} />
      </div>
      <div>
        <label className="block text-xs font-medium text-gray-500 mb-1.5">提示词内容</label>
        <textarea
          placeholder="输入提示词内容，AI 将根据此提示词解读文章..."
          value={pr.content}
          onChange={(e) => setField('content', e.target.value)}
          rows={6}
          className={`${inputCls} resize-none`}
        />
      </div>
      <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
        <input
          type="checkbox"
          checked={pr.isDefault === 1}
          onChange={(e) => setField('isDefault', e.target.checked ? 1 : 0)}
          className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
        />
        设为默认提示词
      </label>
      <div className="flex gap-2 pt-1">
        <button onClick={onSave} className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors">保存</button>
        <button onClick={onCancel} className="px-4 py-2 bg-gray-100 text-gray-600 text-sm rounded-lg hover:bg-gray-200 transition-colors">取消</button>
      </div>
    </div>
  )
}

type RoleFormProps = {
  role: RoleFormData
  onChange: (value: RoleFormData) => void
  onSave: () => void
  onCancel: () => void
}

function RoleForm({ role, onChange, onSave, onCancel }: RoleFormProps) {
  const setField = <K extends keyof RoleFormData>(key: K, value: RoleFormData[K]) => {
    onChange({ ...role, [key]: value })
  }

  return (
    <div className="p-5 bg-white rounded-xl border border-blue-200 mb-4 space-y-3 shadow-sm">
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">角色名称</label>
          <input placeholder="如 财务分析师" value={role.name} onChange={(e) => setField('name', e.target.value)} className={inputCls} />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">角色别名（@用）</label>
          <input placeholder="如 finance" value={role.alias} onChange={(e) => setField('alias', e.target.value)} className={inputCls} />
        </div>
      </div>
      <div>
        <label className="block text-xs font-medium text-gray-500 mb-1.5">领域标签</label>
        <input placeholder="如 财报,估值,基本面" value={role.domainTags} onChange={(e) => setField('domainTags', e.target.value)} className={inputCls} />
      </div>
      <div>
        <label className="block text-xs font-medium text-gray-500 mb-1.5">系统提示词</label>
        <textarea
          placeholder="定义角色能力和回答风格"
          value={role.systemPrompt}
          onChange={(e) => setField('systemPrompt', e.target.value)}
          rows={5}
          className={`${inputCls} resize-none`}
        />
      </div>
      <div className="grid grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">模型覆盖（可空）</label>
          <input placeholder="如 deepseek-chat" value={role.modelOverride} onChange={(e) => setField('modelOverride', e.target.value)} className={inputCls} />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">温度</label>
          <input
            type="number"
            min={0}
            max={2}
            step={0.1}
            value={role.temperature}
            onChange={(e) => setField('temperature', Number(e.target.value))}
            className={inputCls}
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1.5">最大 Token</label>
          <input
            type="number"
            min={100}
            step={100}
            value={role.maxTokens}
            onChange={(e) => setField('maxTokens', Number(e.target.value))}
            className={inputCls}
          />
        </div>
      </div>
      <div className="flex items-center gap-5">
        <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
          <input
            type="checkbox"
            checked={role.enabled === 1}
            onChange={(e) => setField('enabled', e.target.checked ? 1 : 0)}
            className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
          />
          启用角色
        </label>
        <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
          <input
            type="checkbox"
            checked={role.isDefault === 1}
            onChange={(e) => setField('isDefault', e.target.checked ? 1 : 0)}
            className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500/20"
          />
          设为默认角色
        </label>
      </div>
      <div className="flex gap-2 pt-1">
        <button onClick={onSave} className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors">保存</button>
        <button onClick={onCancel} className="px-4 py-2 bg-gray-100 text-gray-600 text-sm rounded-lg hover:bg-gray-200 transition-colors">取消</button>
      </div>
    </div>
  )
}

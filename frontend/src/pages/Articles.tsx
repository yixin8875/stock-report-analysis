import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  DeleteArticle,
  ExportBatchFailures,
  GetArticles,
  GetBatchStatus,
  GetChannels,
  GetPrompts,
  GetTags,
  ImportArticles,
  PauseBatchAnalyze,
  ResumeBatchAnalyze,
  RetryFailedBatchAnalyze,
  StartBatchAnalyze,
} from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const statusMap: Record<number, { text: string; color: string }> = {
  0: { text: '待解读', color: 'bg-amber-50 text-amber-600 ring-1 ring-amber-200' },
  1: { text: '解读中', color: 'bg-blue-50 text-blue-600 ring-1 ring-blue-200' },
  2: { text: '已解读', color: 'bg-emerald-50 text-emerald-600 ring-1 ring-emerald-200' },
}

const modeOptions = [
  { value: 'text', label: '普通文本' },
  { value: 'structured', label: '结构化 JSON' },
] as const

type AnalysisMode = (typeof modeOptions)[number]['value']

export default function Articles() {
  const [articles, setArticles] = useState<models.Article[]>([])
  const [keyword, setKeyword] = useState('')
  const [debouncedKeyword, setDebouncedKeyword] = useState('')
  const [tags, setTags] = useState<models.Tag[]>([])
  const [filterTag, setFilterTag] = useState(0)
  const [selected, setSelected] = useState<Set<number>>(new Set())

  const [taskCenterOpen, setTaskCenterOpen] = useState(false)
  const [channels, setChannels] = useState<models.AIChannel[]>([])
  const [prompts, setPrompts] = useState<models.Prompt[]>([])
  const [batchChId, setBatchChId] = useState(0)
  const [batchPrId, setBatchPrId] = useState(0)
  const [batchMode, setBatchMode] = useState<AnalysisMode>('text')
  const [batchConcurrency, setBatchConcurrency] = useState(2)
  const [batchStatus, setBatchStatus] = useState<models.BatchStatus | null>(null)
  const [batchError, setBatchError] = useState('')
  const [importError, setImportError] = useState('')
  const [importing, setImporting] = useState(false)

  const navigate = useNavigate()

  const load = (kw: string = debouncedKeyword, tagId: number = filterTag) =>
    GetArticles(kw, tagId).then((list) => setArticles(list || []))
  const loadTags = () => GetTags().then((list) => setTags(list || []))

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedKeyword(keyword)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [keyword])

  useEffect(() => {
    load()
  }, [debouncedKeyword, filterTag])

  useEffect(() => {
    loadTags()
    GetBatchStatus().then((status) => setBatchStatus(status || null))
  }, [])

  useEffect(() => {
    const offS = EventsOn('batch-status', (...args: unknown[]) => {
      const payload = args[0] as models.BatchStatus | undefined
      if (payload && typeof payload.total === 'number') {
        setBatchStatus(payload)
      }
    })
    const offE = EventsOn('batch-error', (...args: unknown[]) => {
      const msg = args[0]
      setBatchError(typeof msg === 'string' && msg.trim() ? msg : '批量任务执行失败')
    })
    const offD = EventsOn('batch-done', () => {
      load()
    })

    return () => {
      offS()
      offE()
      offD()
    }
  }, [debouncedKeyword, filterTag])

  const handleImport = async () => {
    if (importing) {
      return
    }
    setImportError('')
    setImporting(true)
    try {
      const list = await ImportArticles()
      if (list?.length) {
        await load()
      } else {
        setImportError('未导入到任何文章，请检查文件格式或 MinerU 配置')
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err || '')
      if (msg.toLowerCase().includes('cancel')) {
        return
      }
      setImportError(msg || '导入失败，请检查 MinerU 配置或网络连接')
    } finally {
      setImporting(false)
    }
  }

  const handleDelete = async (e: React.MouseEvent, id: number) => {
    e.stopPropagation()
    await DeleteArticle(id)
    await load()
  }

  const toggleSelect = (e: React.MouseEvent, id: number) => {
    e.stopPropagation()
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const openTaskCenter = () => {
    setBatchError('')
    GetChannels().then((list) => {
      setChannels(list || [])
      if (!list?.length) {
        return
      }
      const defaultChannel = list.find((c) => c.isDefault === 1)
      setBatchChId(defaultChannel?.id || list[0].id)
    })
    GetPrompts().then((list) => {
      setPrompts(list || [])
      if (!list?.length) {
        return
      }
      const defaultPrompt = list.find((p) => p.isDefault === 1)
      setBatchPrId(defaultPrompt?.id || list[0].id)
    })
    setTaskCenterOpen(true)
  }

  const startBatch = async () => {
    setBatchError('')
    if (!batchChId || !batchPrId || selected.size === 0) {
      return
    }
    try {
      await StartBatchAnalyze(Array.from(selected), batchChId, batchPrId, batchConcurrency, batchMode)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '启动批量任务失败'
      setBatchError(msg)
    }
  }

  const pauseOrResume = async () => {
    if (!batchStatus?.running) {
      return
    }
    try {
      if (batchStatus.paused) {
        await ResumeBatchAnalyze()
      } else {
        await PauseBatchAnalyze()
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '操作失败'
      setBatchError(msg)
    }
  }

  const retryFailed = async () => {
    setBatchError('')
    try {
      await RetryFailedBatchAnalyze()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '重试失败'
      setBatchError(msg)
    }
  }

  const exportFailures = async () => {
    try {
      await ExportBatchFailures()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '导出失败'
      setBatchError(msg)
    }
  }

  const activeStatus = batchStatus || {
    running: false,
    paused: false,
    total: 0,
    completed: 0,
    success: 0,
    failed: 0,
    inProgress: 0,
    concurrency: batchConcurrency,
    failures: [],
  }

  const showTaskCenter = taskCenterOpen || activeStatus.running || activeStatus.total > 0

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="flex items-center justify-between mb-5">
        <h2 className="text-xl font-semibold text-gray-800">文章列表</h2>
        <div className="flex items-center gap-2">
          {selected.size > 0 && !showTaskCenter && (
            <button
              onClick={openTaskCenter}
              className="inline-flex items-center gap-1.5 px-4 py-2 bg-emerald-500 text-white text-sm rounded-lg hover:bg-emerald-600 shadow-sm transition-colors"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
              打开任务中心 ({selected.size})
            </button>
          )}
          <button
            onClick={handleImport}
            disabled={importing}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 shadow-sm transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {importing ? (
              <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
            )}
            {importing ? '导入中...' : '导入文章'}
          </button>
        </div>
      </div>

      {importing && (
        <div className="mb-4 text-sm text-blue-700 bg-blue-50 border border-blue-200 rounded-lg px-4 py-2.5">
          正在导入并解析文档（PDF/图片 可能需要几十秒），请稍候...
        </div>
      )}

      {importError && (
        <div className="mb-4 text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-4 py-2.5">
          {importError}
        </div>
      )}

      {showTaskCenter && (
        <div className="mb-4 p-4 bg-emerald-50 border border-emerald-200 rounded-xl">
          <div className="flex items-center justify-between mb-3">
            <span className="text-sm font-medium text-emerald-700">批量任务中心</span>
            {!activeStatus.running && (
              <button onClick={() => setTaskCenterOpen(false)} className="text-xs text-gray-500 hover:text-gray-700">收起</button>
            )}
          </div>

          <div className="grid grid-cols-5 gap-2">
            <select
              value={batchChId}
              onChange={(e) => setBatchChId(Number(e.target.value))}
              disabled={activeStatus.running}
              className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 disabled:opacity-60"
            >
              {channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
            <select
              value={batchPrId}
              onChange={(e) => setBatchPrId(Number(e.target.value))}
              disabled={activeStatus.running}
              className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 disabled:opacity-60"
            >
              {prompts.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
            <select
              value={batchMode}
              onChange={(e) => setBatchMode(e.target.value as AnalysisMode)}
              disabled={activeStatus.running}
              className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 disabled:opacity-60"
            >
              {modeOptions.map((opt) => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
            </select>
            <select
              value={batchConcurrency}
              onChange={(e) => setBatchConcurrency(Number(e.target.value))}
              disabled={activeStatus.running}
              className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 disabled:opacity-60"
            >
              {[1, 2, 3, 4, 5].map((n) => <option key={n} value={n}>并发 {n}</option>)}
            </select>
            <button
              onClick={startBatch}
              disabled={activeStatus.running || selected.size === 0}
              className="px-4 py-2 bg-emerald-500 text-white text-sm rounded-lg hover:bg-emerald-600 disabled:opacity-50"
            >
              开始任务
            </button>
          </div>

          <div className="mt-3 flex items-center gap-2 text-xs text-gray-600">
            <span>总计 {activeStatus.total}</span>
            <span>完成 {activeStatus.completed}</span>
            <span>成功 {activeStatus.success}</span>
            <span>失败 {activeStatus.failed}</span>
            <span>进行中 {activeStatus.inProgress}</span>
          </div>

          {activeStatus.total > 0 && (
            <div className="mt-2 h-2 bg-emerald-100 rounded-full overflow-hidden">
              <div
                className="h-full bg-emerald-500 transition-all rounded-full"
                style={{ width: `${(activeStatus.completed / activeStatus.total) * 100}%` }}
              />
            </div>
          )}

          <div className="mt-3 flex items-center gap-2">
            {activeStatus.running && (
              <button
                onClick={pauseOrResume}
                className="px-3 py-1.5 text-xs bg-white border border-gray-200 rounded-lg hover:bg-gray-50"
              >
                {activeStatus.paused ? '继续' : '暂停'}
              </button>
            )}
            {!activeStatus.running && activeStatus.failed > 0 && (
              <button onClick={retryFailed} className="px-3 py-1.5 text-xs bg-white border border-gray-200 rounded-lg hover:bg-gray-50">重试失败项</button>
            )}
            {activeStatus.failures.length > 0 && (
              <button onClick={exportFailures} className="px-3 py-1.5 text-xs bg-white border border-gray-200 rounded-lg hover:bg-gray-50">导出失败明细</button>
            )}
          </div>

          {batchError && (
            <div className="mt-3 text-xs text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
              {batchError}
            </div>
          )}

          {activeStatus.failures.length > 0 && (
            <div className="mt-3 max-h-40 overflow-auto bg-white border border-red-100 rounded-lg">
              {activeStatus.failures.map((item, idx) => (
                <div key={`${item.articleId}-${idx}`} className="px-3 py-2 border-b border-gray-100 last:border-0">
                  <div className="text-xs text-gray-700">#{item.articleId} {item.title || '未知标题'}</div>
                  <div className="text-xs text-red-600 mt-0.5">{item.reason}</div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tags.length > 0 && (
        <div className="flex items-center gap-2 mb-4 flex-wrap">
          <button
            onClick={() => setFilterTag(0)}
            className={`text-xs px-3 py-1.5 rounded-full transition-colors ${filterTag === 0 ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-500 hover:bg-gray-200'}`}
          >
            全部
          </button>
          {tags.map((t) => (
            <button
              key={t.id}
              onClick={() => setFilterTag(t.id)}
              className={`text-xs px-3 py-1.5 rounded-full transition-colors ${filterTag === t.id ? 'text-white' : 'text-gray-600 hover:opacity-80'}`}
              style={filterTag === t.id ? { backgroundColor: t.color } : { backgroundColor: `${t.color}20`, color: t.color }}
            >
              {t.name}
            </button>
          ))}
        </div>
      )}

      <div className="relative mb-5">
        <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
        <input
          type="text"
          placeholder="搜索文章标题或内容..."
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          className="w-full pl-10 pr-4 py-2.5 bg-white border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-shadow"
        />
      </div>

      <div className="space-y-2">
        {articles.map((a) => (
          <div
            key={a.id}
            onClick={() => navigate(`/article/${a.id}`)}
            className="group flex items-center gap-3 p-4 bg-white rounded-xl border border-gray-200/80 cursor-pointer hover:border-blue-300 hover:shadow-sm transition-all"
          >
            <input
              type="checkbox"
              checked={selected.has(a.id)}
              onClick={(e) => toggleSelect(e, a.id)}
              onChange={() => {}}
              className="w-4 h-4 rounded border-gray-300 text-blue-500 shrink-0"
            />
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-gray-800 truncate group-hover:text-blue-600 transition-colors">{a.title}</span>
                {a.tags?.map((t) => (
                  <span
                    key={t.id}
                    className="text-xs px-2 py-0.5 rounded-full shrink-0"
                    style={{ backgroundColor: `${t.color}20`, color: t.color }}
                  >
                    {t.name}
                  </span>
                ))}
              </div>
              <div className="text-xs text-gray-400 mt-1.5">{new Date(a.createdAt).toLocaleString()}</div>
            </div>
            <div className="flex items-center gap-3 ml-4 shrink-0">
              <span className={`text-xs px-2.5 py-1 rounded-full font-medium ${statusMap[a.status]?.color}`}>
                {statusMap[a.status]?.text}
              </span>
              <button
                onClick={(e) => handleDelete(e, a.id)}
                className="text-xs text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-all"
              >
                删除
              </button>
            </div>
          </div>
        ))}
        {articles.length === 0 && (
          <div className="text-center py-20">
            <svg className="w-12 h-12 text-gray-300 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
            <p className="text-sm text-gray-400">暂无文章</p>
            <p className="text-xs text-gray-300 mt-1">点击右上角"导入文章"开始</p>
          </div>
        )}
      </div>
    </div>
  )
}

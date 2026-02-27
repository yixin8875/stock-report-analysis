import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  GetTags,
  GetTelegraphArticles,
  GetTelegraphDashboardByDays,
  GetTelegraphDigests,
  GetTelegraphSchedulerStatus,
} from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'

const statusMap: Record<number, { text: string; color: string }> = {
  0: { text: '待解读', color: 'bg-amber-50 text-amber-600 ring-1 ring-amber-200' },
  1: { text: '解读中', color: 'bg-blue-50 text-blue-600 ring-1 ring-blue-200' },
  2: { text: '已解读', color: 'bg-emerald-50 text-emerald-600 ring-1 ring-emerald-200' },
}

type SortOrder = 'latest' | 'score_desc' | 'watch_first'

export default function News() {
  const [items, setItems] = useState<models.TelegraphArticleItem[]>([])
  const [digests, setDigests] = useState<models.TelegraphDigest[]>([])
  const [dashboard, setDashboard] = useState<models.TelegraphDashboard | null>(null)
  const [tags, setTags] = useState<models.Tag[]>([])
  const [status, setStatus] = useState<models.TelegraphSchedulerStatus | null>(null)

  const [keyword, setKeyword] = useState('')
  const [debouncedKeyword, setDebouncedKeyword] = useState('')
  const [filterTag, setFilterTag] = useState(0)
  const [order, setOrder] = useState<SortOrder>('score_desc')
  const [watchOnly, setWatchOnly] = useState(false)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const loadNews = async (kw: string = debouncedKeyword, tagID: number = filterTag, sortOrder: SortOrder = order, onlyWatch: boolean = watchOnly) => {
    setLoading(true)
    try {
      const list = await GetTelegraphArticles(kw, tagID, sortOrder, onlyWatch ? 1 : 0)
      setItems(list || [])
    } finally {
      setLoading(false)
    }
  }

  const loadPanels = async () => {
    const [dashboardData, digestData, tagData] = await Promise.all([
      GetTelegraphDashboardByDays(1),
      GetTelegraphDigests(6),
      GetTags(),
    ])
    setDashboard(dashboardData || null)
    setDigests(digestData || [])
    setTags(tagData || [])
  }

  const loadStatus = async () => {
    const next = await GetTelegraphSchedulerStatus()
    setStatus(next || null)
  }

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedKeyword(keyword)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [keyword])

  useEffect(() => {
    void loadNews()
  }, [debouncedKeyword, filterTag, order, watchOnly])

  useEffect(() => {
    void loadPanels()
    void loadStatus()
    const timer = window.setInterval(() => {
      void loadStatus()
      void loadPanels()
    }, 20000)
    return () => window.clearInterval(timer)
  }, [])

  const summaryItems = useMemo(() => {
    return [
      { label: '今日抓取', value: dashboard?.totalFetched ?? 0 },
      { label: '今日入库', value: dashboard?.totalImported ?? 0 },
      { label: '今日解读', value: dashboard?.totalAnalyzed ?? 0 },
      { label: '解读成功率', value: dashboard?.successRate ?? '0%' },
    ]
  }, [dashboard])

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-gray-800">新闻电报</h2>
        <button
          onClick={() => {
            void loadNews()
            void loadPanels()
            void loadStatus()
          }}
          className="px-3 py-1.5 text-xs bg-white border border-gray-200 rounded-lg hover:bg-gray-50"
        >
          刷新
        </button>
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
          <div className="text-sm font-semibold text-gray-800 mb-2">失败原因 Top</div>
          <div className="space-y-2">
            {(dashboard?.failureTop || []).map((item) => (
              <div key={item.reason} className="flex items-center justify-between rounded-lg bg-rose-50 border border-rose-100 px-3 py-2">
                <span className="text-xs text-rose-700">{item.reason}</span>
                <span className="text-xs font-semibold text-rose-600">{item.count}</span>
              </div>
            ))}
            {(dashboard?.failureTop?.length || 0) === 0 && (
              <div className="text-xs text-gray-400">暂无失败记录</div>
            )}
          </div>
        </section>
        <section className="bg-white border border-gray-200 rounded-xl p-4">
          <div className="text-sm font-semibold text-gray-800 mb-2">盘中摘要（每30分钟）</div>
          <div className="space-y-2 max-h-44 overflow-auto pr-1">
            {digests.map((item) => (
              <div key={item.id} className="border border-gray-100 rounded-lg p-2.5">
                <div className="text-[11px] text-gray-500">
                  {formatTime(item.slotStart)} - {formatTime(item.slotEnd)} / 平均影响 {item.avgScore}
                </div>
                <div className="text-xs text-gray-700 mt-1 whitespace-pre-wrap">{item.summary}</div>
              </div>
            ))}
            {digests.length === 0 && (
              <div className="text-xs text-gray-400">暂无摘要（等第一轮定时任务完成后生成）</div>
            )}
          </div>
        </section>
      </div>

      <div className="bg-white border border-gray-200 rounded-xl p-3 grid grid-cols-5 gap-3">
        <input
          placeholder="搜索电报标题或内容"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          className="col-span-2 px-3 py-2.5 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400"
        />
        <select
          value={filterTag}
          onChange={(e) => setFilterTag(Number(e.target.value))}
          className="px-3 py-2.5 border border-gray-200 rounded-lg text-sm"
        >
          <option value={0}>全部标签</option>
          {tags.map((tag) => (
            <option key={tag.id} value={tag.id}>{tag.name}</option>
          ))}
        </select>
        <select
          value={order}
          onChange={(e) => setOrder(e.target.value as SortOrder)}
          className="px-3 py-2.5 border border-gray-200 rounded-lg text-sm"
        >
          <option value="score_desc">按重要性</option>
          <option value="watch_first">自选优先</option>
          <option value="latest">按时间</option>
        </select>
        <div className="px-3 py-2 border border-gray-200 rounded-lg text-xs text-gray-600 flex items-center justify-between gap-3">
          <label className="flex items-center gap-1.5 cursor-pointer">
            <input
              type="checkbox"
              checked={watchOnly}
              onChange={(e) => setWatchOnly(e.target.checked)}
              className="w-3.5 h-3.5 rounded border-gray-300"
            />
            仅看自选
          </label>
          <span className={status?.running ? 'text-emerald-600 font-semibold' : 'text-gray-700'}>
            {status?.running ? '运行中' : '空闲'}
          </span>
        </div>
      </div>

      {status?.lastError && (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700 break-all">
          最近错误：{status.lastError}
        </div>
      )}

      <div className="space-y-2">
        {items.map((item) => (
          <div
            key={item.id}
            onClick={() => navigate(`/article/${item.id}`)}
            className="group cursor-pointer bg-white border border-gray-200 rounded-xl p-4 hover:border-blue-300 hover:shadow-sm transition-all"
          >
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-sm font-medium text-gray-800 line-clamp-1">{item.title}</h3>
              <div className="flex items-center gap-2 shrink-0">
                <span className={`text-xs px-2 py-0.5 rounded-full ${levelColor(item.impactLevel)}`}>{item.impactLevel}</span>
                <span className={`text-xs px-2 py-0.5 rounded-full ${directionColor(item.impactDirection)}`}>{item.impactDirection}</span>
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-100 text-slate-700">影响分 {item.importanceScore}</span>
                <span className={`text-xs px-2 py-0.5 rounded-full ${statusMap[item.status]?.color || 'bg-gray-100 text-gray-500'}`}>
                  {statusMap[item.status]?.text || '未知'}
                </span>
              </div>
            </div>
            <div className="mt-2 flex items-center gap-1.5 flex-wrap">
              {(item.watchMatches || []).slice(0, 4).map((w) => (
                <span key={`${item.id}-${w.code}`} className="text-[11px] px-2 py-0.5 rounded-full bg-blue-100 text-blue-700">
                  自选 {w.code} {w.name}
                </span>
              ))}
              {(item.tags || []).slice(0, 6).map((tag) => (
                <span key={tag.id} className="text-[11px] px-2 py-0.5 rounded-full" style={{ backgroundColor: `${tag.color}20`, color: tag.color }}>
                  {tag.name}
                </span>
              ))}
            </div>
            <div className="mt-2 text-xs text-gray-400">
              入库时间：{formatDateTime(item.createdAt)} {item.analyzedAt ? `| 解读时间：${formatDateTime(item.analyzedAt)}` : ''}
            </div>
          </div>
        ))}
        {items.length === 0 && !loading && (
          <div className="text-center py-12 text-sm text-gray-400">暂无新闻电报</div>
        )}
        {loading && (
          <div className="text-center py-12 text-sm text-gray-400">加载中...</div>
        )}
      </div>
    </div>
  )
}

function levelColor(level: string): string {
  if (level === '高影响') {
    return 'bg-rose-100 text-rose-700'
  }
  if (level === '中影响') {
    return 'bg-amber-100 text-amber-700'
  }
  return 'bg-slate-100 text-slate-700'
}

function directionColor(direction: string): string {
  if (direction === '利多') {
    return 'bg-emerald-100 text-emerald-700'
  }
  if (direction === '利空') {
    return 'bg-rose-100 text-rose-700'
  }
  return 'bg-gray-100 text-gray-600'
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

function formatTime(value: unknown): string {
  if (!value) {
    return '-'
  }
  const dt = new Date(String(value))
  if (Number.isNaN(dt.getTime())) {
    return String(value)
  }
  return `${String(dt.getHours()).padStart(2, '0')}:${String(dt.getMinutes()).padStart(2, '0')}`
}

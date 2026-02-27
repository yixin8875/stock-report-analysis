import { useEffect, useState } from 'react'
import { HashRouter, Routes, Route, NavLink } from 'react-router-dom'
import Articles from './pages/Articles'
import ArticleDetail from './pages/ArticleDetail'
import News from './pages/News'
import Settings from './pages/Settings'
import { EventsOn } from '../wailsjs/runtime/runtime'

const navCls = ({isActive}: {isActive: boolean}) =>
  `flex items-center gap-2.5 px-3 py-2.5 rounded-lg text-sm transition-colors ${isActive ? 'bg-blue-50 text-blue-600 font-medium' : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700'}`

type AppToast = {
  id: number
  title: string
  body: string
  tone: 'info' | 'warn'
}

function App() {
  const [toasts, setToasts] = useState<AppToast[]>([])

  const pushToast = (title: string, body: string, tone: 'info' | 'warn') => {
    const id = Date.now() + Math.floor(Math.random() * 1000)
    setToasts((prev) => [...prev.slice(-3), { id, title, body, tone }])
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((item) => item.id !== id))
    }, 6000)
  }

  useEffect(() => {
    const notifyDesktop = (title: string, body: string, tag: string) => {
      if (typeof window === 'undefined' || !('Notification' in window)) {
        return
      }
      const doNotify = () => {
        try {
          // Browser/WebView notification for high-priority telegraph events.
          new Notification(title, { body, tag, silent: false })
        } catch {
          // ignore
        }
      }
      if (Notification.permission === 'granted') {
        doNotify()
      } else if (Notification.permission === 'default') {
        Notification.requestPermission().then((permission) => {
          if (permission === 'granted') {
            doNotify()
          }
        }).catch(() => undefined)
      }
    }

    const offAlert = EventsOn('telegraph-alert', (...args: unknown[]) => {
      const payload = (args[0] || {}) as Record<string, unknown>
      const title = String(payload.title || '高影响新闻')
      const score = Number(payload.score || 0)
      const direction = String(payload.direction || '中性')
      const level = String(payload.level || '高影响')
      const body = `${level} ${direction}，影响分 ${score}`
      pushToast('关键新闻提醒', `${title} · ${body}`, 'warn')
      notifyDesktop('关键新闻提醒', `${title}\n${body}`, `telegraph-alert-${payload.articleId || ''}`)
    })

    const offDigest = EventsOn('telegraph-digest', (...args: unknown[]) => {
      const payload = (args[0] || {}) as Record<string, unknown>
      const slotStart = String(payload.slotStart || '')
      const slotEnd = String(payload.slotEnd || '')
      const summary = String(payload.summary || '盘中摘要已更新')
      pushToast('盘中摘要已生成', summary, 'info')
      notifyDesktop('盘中摘要已生成', `${slotStart} - ${slotEnd}`, `telegraph-digest-${slotEnd}`)
    })

    return () => {
      offAlert()
      offDigest()
    }
  }, [])

  return (
    <HashRouter>
      <div className="flex h-screen bg-gray-100/80">
        <nav className="w-52 bg-white border-r border-gray-200/80 flex flex-col shrink-0">
          <div className="px-5 py-5 border-b border-gray-100">
            <h1 className="text-base font-semibold text-gray-800 tracking-tight">
              <span className="text-blue-500">AI</span> 解读助手
            </h1>
            <p className="text-xs text-gray-400 mt-0.5">股票报告智能分析</p>
          </div>
          <div className="flex flex-col gap-1 p-3 flex-1">
            <NavLink to="/" end className={navCls}>
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" /></svg>
              文章列表
            </NavLink>
            <NavLink to="/news" className={navCls}>
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 4H5a2 2 0 00-2 2v10a2 2 0 002 2h4l3 3 3-3h4a2 2 0 002-2V6a2 2 0 00-2-2z" /></svg>
              新闻电报
            </NavLink>
            <NavLink to="/settings" className={navCls}>
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
              设置
            </NavLink>
          </div>
        </nav>
        <main className="flex-1 overflow-auto">
          <Routes>
            <Route path="/" element={<Articles />} />
            <Route path="/news" element={<News />} />
            <Route path="/article/:id" element={<ArticleDetail />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
        <div className="fixed top-3 right-3 z-50 space-y-2">
          {toasts.map((toast) => (
            <div
              key={toast.id}
              className={`w-80 rounded-lg border px-3 py-2 shadow-md ${
                toast.tone === 'warn'
                  ? 'bg-rose-50 border-rose-200 text-rose-800'
                  : 'bg-sky-50 border-sky-200 text-sky-800'
              }`}
            >
              <div className="text-xs font-semibold">{toast.title}</div>
              <div className="text-xs mt-1 break-words">{toast.body}</div>
            </div>
          ))}
        </div>
      </div>
    </HashRouter>
  )
}

export default App

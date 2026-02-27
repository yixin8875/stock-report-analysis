import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import {
  AnalyzeArticleWithMode,
  AskQuestion,
  AskQuestionFollowUp,
  CancelAskQuestion,
  CreateQASession,
  DeleteQAPin,
  DeleteQASession,
  ExportArticle,
  GetAnalysisHistory,
  GetArticle,
  GetArticleTags,
  GetChannels,
  GetPrompts,
  GetRoles,
  GetQAMessages,
  GetQAPins,
  GetQASessions,
  GetTags,
  SaveQAPin,
  SetArticleTags,
} from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const analysisModes = [
  { value: 'text', label: '普通文本' },
  { value: 'structured', label: '结构化 JSON' },
] as const

type AnalysisMode = (typeof analysisModes)[number]['value']
type DetailTab = 'analysis' | 'qa'

type StructuredAnalysis = {
  summary: string
  risks: string[]
  catalysts: string[]
  valuationView: string
}

export default function ArticleDetail() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [article, setArticle] = useState<models.Article | null>(null)
  const [channels, setChannels] = useState<models.AIChannel[]>([])
  const [prompts, setPrompts] = useState<models.Prompt[]>([])
  const [channelId, setChannelId] = useState(0)
  const [promptId, setPromptId] = useState(0)
  const [analysisMode, setAnalysisMode] = useState<AnalysisMode>('text')
  const [streaming, setStreaming] = useState('')
  const [analyzing, setAnalyzing] = useState(false)
  const [error, setError] = useState('')
  const [allTags, setAllTags] = useState<models.Tag[]>([])
  const [articleTags, setArticleTags] = useState<models.Tag[]>([])
  const [showTagPicker, setShowTagPicker] = useState(false)
  const [history, setHistory] = useState<models.AnalysisHistory[]>([])
  const [historyId, setHistoryId] = useState(0)

  const [detailTab, setDetailTab] = useState<DetailTab>('analysis')
  const [qaSessions, setQaSessions] = useState<models.QASession[]>([])
  const [qaSessionId, setQaSessionId] = useState(0)
  const [qaMessages, setQaMessages] = useState<models.QAMessage[]>([])
  const [qaPins, setQaPins] = useState<models.QAPin[]>([])
  const [pinDraft, setPinDraft] = useState('')
  const [qaQuestion, setQaQuestion] = useState('')
  const [asking, setAsking] = useState(false)
  const [cancelingAsk, setCancelingAsk] = useState(false)
  const [qaError, setQaError] = useState('')
  const [pinSaving, setPinSaving] = useState(false)
  const [qaRoles, setQaRoles] = useState<models.Role[]>([])
  const [mentionOpen, setMentionOpen] = useState(false)
  const [mentionKeyword, setMentionKeyword] = useState('')
  const [mentionRange, setMentionRange] = useState<{ start: number; end: number } | null>(null)
  const [mentionActiveIndex, setMentionActiveIndex] = useState(0)

  const pendingQuestionRef = useRef('')
  const activeSessionRef = useRef(0)
  const qaInputRef = useRef<HTMLTextAreaElement | null>(null)
  const askSeqRef = useRef(0)
  const askHardTimeoutRef = useRef<number | null>(null)
  const pendingFollowUpMessageRef = useRef(0)
  const [followUpMessage, setFollowUpMessage] = useState<models.QAMessage | null>(null)

  const aid = Number(id)

  const clearAskTimers = () => {
    if (askHardTimeoutRef.current !== null) {
      window.clearTimeout(askHardTimeoutRef.current)
      askHardTimeoutRef.current = null
    }
  }

  const upsertQAMessage = (incoming: models.QAMessage) => {
    setQaMessages((prev) => {
      let base = prev
      if (incoming.roleType === 'user') {
        const tempIdx = base.findIndex((item) => item.id < 0 && item.roleType === 'user' && item.content === incoming.content)
        if (tempIdx >= 0) {
          base = [...base.slice(0, tempIdx), ...base.slice(tempIdx + 1)]
        }
      }
      const idx = base.findIndex((item) => item.id === incoming.id)
      if (idx < 0) {
        return [...base, incoming]
      }
      const next = [...base]
      next[idx] = new models.QAMessage({
        ...next[idx],
        ...incoming,
        evidences: incoming.evidences?.length ? incoming.evidences : next[idx].evidences,
      })
      return next
    })
  }

  const applyServerQAMessages = (list: models.QAMessage[], targetSessionID: number) => {
    const server = list || []
    setQaMessages((prev) => {
      const temps = prev.filter((item) => item.id < 0 && item.roleType === 'user' && (item.sessionId === 0 || item.sessionId === targetSessionID))
      if (temps.length === 0) {
        return server
      }
      const serverUsers = new Set(server.filter((item) => item.roleType === 'user').map((item) => normalizeComparableText(item.content || '')))
      const remainTemps = temps.filter((item) => !serverUsers.has(normalizeComparableText(item.content || '')))
      return [...server, ...remainTemps]
    })
  }

  const loadQASessions = async (preferredSessionId = 0) => {
    if (!aid) {
      setQaSessions([])
      setQaSessionId(0)
      setQaMessages([])
      return
    }
    const list = (await GetQASessions(aid)) || []
    setQaSessions(list)
    setQaSessionId((prev) => {
      const target = preferredSessionId || prev
      if (target && list.some((s) => s.id === target)) {
        return target
      }
      return list[0]?.id || 0
    })
    if (list.length === 0) {
      setQaMessages([])
    }
  }

  useEffect(() => {
    activeSessionRef.current = qaSessionId
  }, [qaSessionId])

  useEffect(() => {
    if (!aid) {
      return
    }

    GetArticle(aid).then(setArticle)
    GetChannels().then((list) => {
      setChannels(list || [])
      const defaultChannel = (list || []).find((c) => c.isDefault === 1)
      if (defaultChannel) {
        setChannelId(defaultChannel.id)
      } else if (list?.length) {
        setChannelId(list[0].id)
      }
    })
    GetPrompts().then((list) => {
      setPrompts(list || [])
      const defaultPrompt = (list || []).find((p) => p.isDefault === 1)
      if (defaultPrompt) {
        setPromptId(defaultPrompt.id)
      } else if (list?.length) {
        setPromptId(list[0].id)
      }
    })
    GetRoles().then((list) => setQaRoles(list || []))
    GetTags().then((list) => setAllTags(list || []))
    GetArticleTags(aid).then((list) => setArticleTags(list || []))
    GetAnalysisHistory(aid).then((list) => setHistory(list || []))
    loadQASessions()
  }, [aid])

  useEffect(() => {
    const off = EventsOn('analysis-chunk', (...args: unknown[]) => {
      const chunk = args[0]
      setStreaming((prev) => prev + (typeof chunk === 'string' ? chunk : ''))
    })
    return () => {
      off()
    }
  }, [aid])

  useEffect(() => {
    if (!qaSessionId) {
      setQaMessages([])
      setQaPins([])
      setFollowUpMessage(null)
      return
    }
    setFollowUpMessage(null)
    GetQAMessages(qaSessionId).then((list) => applyServerQAMessages(list || [], qaSessionId))
    GetQAPins(qaSessionId).then((list) => setQaPins(list || []))
  }, [qaSessionId])

  useEffect(() => {
    const offJobStart = EventsOn('qa-job-start', (...args: unknown[]) => {
      const payload = (args[0] || {}) as Record<string, unknown>
      const sessionID = Number(payload.sessionId || 0)
      const questionMessageID = Number(payload.questionMessageId || 0)
      const questionText = pendingQuestionRef.current.trim()

      if (sessionID > 0) {
        if (activeSessionRef.current === 0 || activeSessionRef.current === sessionID) {
          setQaSessionId(sessionID)
        }
        loadQASessions(sessionID)
      }

      if (questionMessageID > 0 && questionText) {
        upsertQAMessage(new models.QAMessage({
          id: questionMessageID,
          sessionId: sessionID,
          articleId: aid,
          parentId: pendingFollowUpMessageRef.current || 0,
          roleType: 'user',
          roleId: 0,
          roleName: '你',
          content: questionText,
          status: 'done',
          errorReason: '',
          durationMs: 0,
          promptTokens: 0,
          completionTokens: 0,
          totalTokens: 0,
          createdAt: new Date().toISOString(),
          evidences: [],
        }))
        setQaQuestion('')
        setFollowUpMessage(null)
        pendingFollowUpMessageRef.current = 0
        setMentionOpen(false)
        setMentionKeyword('')
        setMentionRange(null)
        setMentionActiveIndex(0)
      }
    })

    const offRoleStart = EventsOn('qa-role-start', (...args: unknown[]) => {
      const raw = args[0]
      if (!raw || typeof raw !== 'object') {
        return
      }
      const msg = new models.QAMessage(raw)
      if (!msg.sessionId || (activeSessionRef.current && msg.sessionId !== activeSessionRef.current)) {
        return
      }
      upsertQAMessage(msg)
    })

    const offRoleChunk = EventsOn('qa-role-chunk', (...args: unknown[]) => {
      const payload = (args[0] || {}) as Record<string, unknown>
      const messageID = Number(payload.messageId || 0)
      const chunk = typeof payload.chunk === 'string' ? payload.chunk : ''
      if (!messageID || !chunk) {
        return
      }
      setQaMessages((prev) => {
        const idx = prev.findIndex((item) => item.id === messageID)
        if (idx < 0) {
          return prev
        }
        const next = [...prev]
        next[idx] = new models.QAMessage({
          ...next[idx],
          content: `${next[idx].content || ''}${chunk}`,
          status: 'running',
        })
        return next
      })
    })

    const offRoleDone = EventsOn('qa-role-done', (...args: unknown[]) => {
      const raw = args[0]
      if (!raw || typeof raw !== 'object') {
        return
      }
      const msg = new models.QAMessage(raw)
      if (!msg.sessionId || (activeSessionRef.current && msg.sessionId !== activeSessionRef.current)) {
        return
      }
      upsertQAMessage(msg)
    })

    const offRoleError = EventsOn('qa-role-error', (...args: unknown[]) => {
      const payload = (args[0] || {}) as Record<string, unknown>
      const messageID = Number(payload.messageId || 0)
      const errMsg = typeof payload.error === 'string' ? payload.error : '回答失败'
      if (!messageID) {
        setQaError(errMsg)
        setAsking(false)
        setCancelingAsk(false)
        pendingQuestionRef.current = ''
        pendingFollowUpMessageRef.current = 0
        clearAskTimers()
        return
      }
      setQaMessages((prev) => {
        const idx = prev.findIndex((item) => item.id === messageID)
        if (idx < 0) {
          return prev
        }
        const next = [...prev]
        next[idx] = new models.QAMessage({
          ...next[idx],
          status: 'failed',
          errorReason: errMsg,
        })
        return next
      })
    })

    const offJobDone = EventsOn('qa-job-done', (...args: unknown[]) => {
      const payload = (args[0] || {}) as Record<string, unknown>
      const sessionID = Number(payload.sessionId || 0)
      setAsking(false)
      setCancelingAsk(false)
      pendingQuestionRef.current = ''
      pendingFollowUpMessageRef.current = 0
      clearAskTimers()
      if (sessionID > 0) {
        loadQASessions(sessionID)
        if (activeSessionRef.current === 0 || activeSessionRef.current === sessionID) {
          GetQAMessages(sessionID).then((list) => applyServerQAMessages(list || [], sessionID))
        }
      }
    })

    return () => {
      offJobStart()
      offRoleStart()
      offRoleChunk()
      offRoleDone()
      offRoleError()
      offJobDone()
    }
  }, [aid])

  useEffect(() => {
    return () => {
      clearAskTimers()
    }
  }, [])

  const handleAnalyze = async () => {
    if (!aid) {
      return
    }
    if (!channelId || !promptId) {
      setError('请先配置 AI 渠道和提示词')
      return
    }

    setError('')
    setStreaming('')
    setAnalyzing(true)
    setHistoryId(0)

    try {
      const errMsg = await AnalyzeArticleWithMode(aid, channelId, promptId, analysisMode)
      if (errMsg) {
        setError(errMsg)
      } else {
        GetArticle(aid).then(setArticle)
        GetAnalysisHistory(aid).then((list) => setHistory(list || []))
      }
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : '分析失败'
      setError(message)
    }

    setAnalyzing(false)
  }

  const toggleTag = async (tagId: number) => {
    if (!aid) {
      return
    }
    const has = articleTags.some((t) => t.id === tagId)
    const newIds = has ? articleTags.filter((t) => t.id !== tagId).map((t) => t.id) : [...articleTags.map((t) => t.id), tagId]
    await SetArticleTags(aid, newIds)
    GetArticleTags(aid).then((list) => setArticleTags(list || []))
  }

  const handleCreateSession = async () => {
    if (!aid) {
      return
    }
    const session = await CreateQASession(aid, '问答会话')
    await loadQASessions(session.id)
    setQaSessionId(session.id)
    setQaMessages([])
    setQaPins([])
  }

  const handleDeleteSession = async () => {
    if (!qaSessionId) {
      return
    }
    await DeleteQASession(qaSessionId)
    await loadQASessions()
    setQaPins([])
    setFollowUpMessage(null)
  }

  const handleSavePin = async (content: string, sourceMessageID: number = 0) => {
    const text = content.trim()
    if (!qaSessionId || !aid || !text) {
      return
    }
    setPinSaving(true)
    try {
      const pin = await SaveQAPin(new models.QAPin({
        id: 0,
        sessionId: qaSessionId,
        articleId: aid,
        sourceMessageId: sourceMessageID,
        content: text,
      }))
      setQaPins((prev) => [pin, ...prev.filter((p) => p.id !== pin.id)])
      if (!sourceMessageID) {
        setPinDraft('')
      }
    } finally {
      setPinSaving(false)
    }
  }

  const handleDeletePin = async (id: number) => {
    await DeleteQAPin(id)
    setQaPins((prev) => prev.filter((p) => p.id !== id))
  }

  const enabledRoles = useMemo(
    () => (qaRoles || []).filter((role) => role.enabled === 1),
    [qaRoles],
  )

  const mentionCandidates = useMemo(() => {
    const key = mentionKeyword.trim().toLowerCase()
    if (!key) {
      return enabledRoles
    }
    return enabledRoles.filter((role) => {
      const name = (role.name || '').toLowerCase()
      const alias = (role.alias || '').toLowerCase()
      return name.includes(key) || alias.includes(key)
    })
  }, [enabledRoles, mentionKeyword])

  const syncMentionState = (text: string, caret: number) => {
    const mention = findMentionAtCaret(text, caret)
    if (!mention) {
      setMentionOpen(false)
      setMentionKeyword('')
      setMentionRange(null)
      setMentionActiveIndex(0)
      return
    }
    const mentionChanged =
      !mentionOpen ||
      !mentionRange ||
      mentionRange.start !== mention.start ||
      mentionRange.end !== mention.end ||
      mentionKeyword !== mention.keyword

    setMentionOpen(true)
    setMentionKeyword(mention.keyword)
    setMentionRange({ start: mention.start, end: mention.end })
    if (mentionChanged) {
      setMentionActiveIndex(0)
    }
  }

  const handleQuestionInput = (nextText: string, caret: number) => {
    setQaQuestion(nextText)
    syncMentionState(nextText, caret)
  }

  const insertMentionRole = (role: models.Role) => {
    const input = qaInputRef.current
    const current = qaQuestion
    const fallbackCaret = input?.selectionStart ?? current.length
    const resolvedRange = mentionRange || findMentionAtCaret(current, fallbackCaret)
    const picked = `@${(role.alias || role.name || '').trim()}`
    if (!picked || picked === '@') {
      return
    }

    let nextText = current
    let nextCaret = current.length
    if (resolvedRange) {
      const before = current.slice(0, resolvedRange.start)
      const after = current.slice(resolvedRange.end)
      const needSpace = !!after && !/^\s/.test(after)
      nextText = `${before}${picked}${needSpace ? ' ' : ''}${after}`
      nextCaret = before.length + picked.length + (needSpace ? 1 : 0)
    } else {
      const suffix = current && !/\s$/.test(current) ? ' ' : ''
      nextText = `${current}${suffix}${picked} `
      nextCaret = nextText.length
    }

    setQaQuestion(nextText)
    setMentionOpen(false)
    setMentionKeyword('')
    setMentionRange(null)
    setMentionActiveIndex(0)

    requestAnimationFrame(() => {
      if (!qaInputRef.current) {
        return
      }
      qaInputRef.current.focus()
      qaInputRef.current.setSelectionRange(nextCaret, nextCaret)
    })
  }

  useEffect(() => {
    if (!mentionOpen) {
      return
    }
    if (mentionCandidates.length === 0) {
      setMentionActiveIndex(0)
      return
    }
    if (mentionActiveIndex >= mentionCandidates.length) {
      setMentionActiveIndex(0)
    }
  }, [mentionOpen, mentionCandidates, mentionActiveIndex])

  const handleAskQuestion = async () => {
    if (!aid || asking) {
      return
    }
    const question = qaQuestion.trim()
    if (!question) {
      return
    }
    const followUpMessageID = followUpMessage?.id || 0

    setQaError('')
    pendingQuestionRef.current = question
    pendingFollowUpMessageRef.current = followUpMessageID
    askSeqRef.current += 1
    const currentSeq = askSeqRef.current

    try {
      const tempMessageID = -Date.now()
      upsertQAMessage(new models.QAMessage({
        id: tempMessageID,
        sessionId: qaSessionId || 0,
        articleId: aid,
        parentId: followUpMessageID,
        roleType: 'user',
        roleId: 0,
        roleName: '你',
        content: question,
        status: 'done',
        errorReason: '',
        durationMs: 0,
        promptTokens: 0,
        completionTokens: 0,
        totalTokens: 0,
        createdAt: new Date().toISOString(),
        evidences: [],
      }))
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : '前端状态更新失败'
      setQaError(message)
      setAsking(false)
      pendingQuestionRef.current = ''
      clearAskTimers()
      return
    }

    setAsking(true)
    setCancelingAsk(false)
    setFollowUpMessage(null)
    clearAskTimers()
    askHardTimeoutRef.current = window.setTimeout(() => {
      if (askSeqRef.current !== currentSeq) {
        return
      }
      setQaError('提问超时（90 秒），请稍后重试或更换角色/模型')
      setAsking(false)
      setCancelingAsk(false)
    }, 90000)

    let rpcPromise: Promise<number>
    try {
      rpcPromise = followUpMessageID > 0
        ? AskQuestionFollowUp(qaSessionId, aid, question, followUpMessageID)
        : AskQuestion(qaSessionId, aid, question)
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : '提问失败（调用异常）'
      setQaError(message)
      setAsking(false)
      setCancelingAsk(false)
      pendingQuestionRef.current = ''
      pendingFollowUpMessageRef.current = 0
      clearAskTimers()
      return
    }

    const ackTimer = window.setTimeout(() => {
      if (askSeqRef.current !== currentSeq) {
        return
      }
      setQaError('后端调用未返回，请重启 wails dev 后重试')
      setAsking(false)
      setCancelingAsk(false)
      pendingQuestionRef.current = ''
      pendingFollowUpMessageRef.current = 0
      clearAskTimers()
    }, 10000)

    void rpcPromise.then((questionMessageID) => {
      window.clearTimeout(ackTimer)
      void questionMessageID
    }).catch((e: unknown) => {
      window.clearTimeout(ackTimer)
      if (askSeqRef.current !== currentSeq) {
        return
      }
      const message = e instanceof Error ? e.message : '提问失败'
      setQaError(message)
      setAsking(false)
      setCancelingAsk(false)
      pendingQuestionRef.current = ''
      pendingFollowUpMessageRef.current = 0
      clearAskTimers()
    })
  }

  const handlePickFollowUpMessage = (msg: models.QAMessage) => {
    if (msg.roleType !== 'assistant' || !msg.id) {
      return
    }
    setFollowUpMessage(msg)
    setQaQuestion((prev) => prev || '基于你上面的回答，')
    requestAnimationFrame(() => {
      if (!qaInputRef.current) {
        return
      }
      qaInputRef.current.focus()
      const len = qaInputRef.current.value.length
      qaInputRef.current.setSelectionRange(len, len)
    })
  }

  const handleCancelAskQuestion = async () => {
    if (!asking || cancelingAsk) {
      return
    }
    setCancelingAsk(true)
    setQaError('')
    clearAskTimers()
    askHardTimeoutRef.current = window.setTimeout(() => {
      setQaError('取消超时，请稍后重试')
      setAsking(false)
      setCancelingAsk(false)
      pendingQuestionRef.current = ''
      clearAskTimers()
    }, 15000)
    try {
      await CancelAskQuestion()
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : '取消失败'
      setQaError(message)
      setCancelingAsk(false)
    }
  }

  const selectedHistory = historyId ? history.find((h) => h.id === historyId) : null
  const analysis = analyzing ? streaming : (selectedHistory ? selectedHistory.analysis : (article?.analysis || streaming))

  const structured = useMemo(() => parseStructuredAnalysis(analysis), [analysis])
  const useStructuredCards = analysisMode === 'structured' && !!structured
  const articleContentIsMarkdown = useMemo(() => looksLikeMarkdown(article?.content || ''), [article?.content])

  const sortedQAMessages = useMemo(
    () => [...qaMessages].sort((a, b) => normalizeMessageOrderID(a.id) - normalizeMessageOrderID(b.id)),
    [qaMessages],
  )
  const defaultRoleName = useMemo(
    () => enabledRoles.find((role) => role.isDefault === 1)?.name || '默认角色',
    [enabledRoles],
  )

  if (!article) {
    return <div className="flex items-center justify-center h-full text-gray-400 text-sm">加载中...</div>
  }

  const backPath = article.source?.startsWith('cls-telegraph:') ? '/news' : '/'

  return (
    <div className="p-6 h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3 min-w-0 flex-1">
          <button
            onClick={() => navigate(backPath)}
            className="inline-flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 transition-colors shrink-0"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" /></svg>
            返回
          </button>
          <div className="h-4 w-px bg-gray-200" />
          <h2 className="text-base font-semibold text-gray-800 truncate">{article.title}</h2>
          <div className="flex items-center gap-1.5 shrink-0 relative">
            {articleTags.map((t) => (
              <span key={t.id} className="text-xs px-2 py-0.5 rounded-full" style={{ backgroundColor: `${t.color}20`, color: t.color }}>
                {t.name}
              </span>
            ))}
            <button
              onClick={() => setShowTagPicker(!showTagPicker)}
              className="w-5 h-5 rounded-full bg-gray-100 hover:bg-gray-200 flex items-center justify-center text-gray-400 text-xs transition-colors"
            >
              +
            </button>
            {showTagPicker && (
              <div className="absolute top-7 left-0 z-10 bg-white border border-gray-200 rounded-lg shadow-lg p-2 min-w-[140px]">
                {allTags.map((t) => (
                  <label key={t.id} className="flex items-center gap-2 px-2 py-1.5 hover:bg-gray-50 rounded cursor-pointer">
                    <input
                      type="checkbox"
                      checked={articleTags.some((at) => at.id === t.id)}
                      onChange={() => toggleTag(t.id)}
                      className="w-3.5 h-3.5 rounded border-gray-300"
                    />
                    <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ backgroundColor: t.color }} />
                    <span className="text-xs text-gray-700">{t.name}</span>
                  </label>
                ))}
                {allTags.length === 0 && <div className="text-xs text-gray-400 px-2 py-1">请先在设置中添加标签</div>}
              </div>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <select
            value={channelId}
            onChange={(e) => setChannelId(Number(e.target.value))}
            className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 focus:outline-none"
          >
            {channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
          <select
            value={promptId}
            onChange={(e) => setPromptId(Number(e.target.value))}
            className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 focus:outline-none"
          >
            {prompts.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
          <select
            value={analysisMode}
            onChange={(e) => setAnalysisMode(e.target.value as AnalysisMode)}
            className="text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 focus:outline-none"
          >
            {analysisModes.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
          </select>
          <button
            onClick={handleAnalyze}
            disabled={analyzing}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed shadow-sm transition-colors"
          >
            {analyzing ? (
              <><svg className="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>解读中...</>
            ) : (
              <><svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>AI 解读</>
            )}
          </button>
          <button
            onClick={() => ExportArticle(aid)}
            className="inline-flex items-center gap-1.5 px-3 py-2 bg-white border border-gray-200 text-gray-600 text-sm rounded-lg hover:bg-gray-50 shadow-sm transition-colors"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
            导出
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 px-4 py-2.5 mb-4 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
          <svg className="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          {error}
        </div>
      )}

      <div className="flex-1 grid grid-cols-2 gap-4 min-h-0">
        <div className="bg-white rounded-xl border border-gray-200/80 flex flex-col overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-100 bg-gray-50/50">
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider">原文</h3>
          </div>
          <div className="p-4 overflow-auto flex-1">
            {articleContentIsMarkdown ? (
              <div className="text-sm text-gray-700 leading-relaxed prose prose-sm max-w-none prose-headings:text-gray-800 prose-a:text-blue-500">
                <ReactMarkdown>{article.content}</ReactMarkdown>
              </div>
            ) : (
              <div className="text-sm text-gray-700 leading-relaxed whitespace-pre-wrap">{article.content}</div>
            )}
          </div>
        </div>

        <div className="bg-white rounded-xl border border-gray-200/80 flex flex-col overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-100 bg-gray-50/50 flex items-center justify-between gap-3">
            <div className="inline-flex bg-gray-100 rounded-lg p-1">
              <button
                onClick={() => setDetailTab('analysis')}
                className={`px-3 py-1.5 text-xs rounded-md transition-colors ${detailTab === 'analysis' ? 'bg-white text-gray-700 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
              >
                AI 解读
              </button>
              <button
                onClick={() => setDetailTab('qa')}
                className={`px-3 py-1.5 text-xs rounded-md transition-colors ${detailTab === 'qa' ? 'bg-white text-gray-700 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
              >
                报告问答
              </button>
            </div>

            {detailTab === 'analysis' && history.length > 0 && (
              <select
                value={historyId}
                onChange={(e) => setHistoryId(Number(e.target.value))}
                className="text-xs bg-white border border-gray-200 rounded-md px-2 py-1 focus:outline-none"
              >
                <option value={0}>最新解读</option>
                {history.map((h) => (
                  <option key={h.id} value={h.id}>{new Date(h.createdAt).toLocaleString()} - {h.channelUsed}/{h.promptUsed}</option>
                ))}
              </select>
            )}

            {detailTab === 'qa' && (
              <div className="flex items-center gap-2">
                <select
                  value={qaSessionId}
                  onChange={(e) => setQaSessionId(Number(e.target.value))}
                  className="text-xs bg-white border border-gray-200 rounded-md px-2 py-1 focus:outline-none min-w-[170px]"
                >
                  {qaSessions.length === 0 && <option value={0}>暂无会话</option>}
                  {qaSessions.map((s) => (
                    <option key={s.id} value={s.id}>{s.title}</option>
                  ))}
                </select>
                <button onClick={handleCreateSession} className="px-2 py-1 text-xs bg-white border border-gray-200 rounded-md hover:bg-gray-50">新建</button>
                <button onClick={handleDeleteSession} disabled={!qaSessionId || asking} className="px-2 py-1 text-xs bg-white border border-gray-200 rounded-md hover:bg-gray-50 disabled:opacity-50">删除</button>
              </div>
            )}
          </div>

          {detailTab === 'analysis' ? (
            <div className="p-4 overflow-auto flex-1">
              {analysis ? (
                useStructuredCards && structured ? (
                  <StructuredCards data={structured} />
                ) : (
                  <div className="text-sm text-gray-700 leading-relaxed prose prose-sm max-w-none prose-headings:text-gray-800 prose-a:text-blue-500">
                    <ReactMarkdown>{analysis}</ReactMarkdown>
                  </div>
                )
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-gray-300">
                  <svg className="w-10 h-10 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                  <p className="text-sm">点击"AI 解读"开始分析</p>
                </div>
              )}
            </div>
          ) : (
            <div className="p-4 flex-1 min-h-0 flex flex-col">
              <div className="mb-3 border border-amber-200 bg-amber-50 rounded-lg p-3">
                <div className="text-xs font-medium text-amber-800 mb-2">会话记忆（Pin）</div>
                <div className="space-y-2 max-h-28 overflow-auto pr-1">
                  {qaPins.map((pin) => (
                    <div key={pin.id} className="flex items-start justify-between gap-2 bg-white border border-amber-100 rounded-md px-2 py-1.5">
                      <div className="text-xs text-gray-700 whitespace-pre-wrap">{pin.content}</div>
                      <button
                        onClick={() => handleDeletePin(pin.id)}
                        className="text-[11px] text-red-500 hover:text-red-700 shrink-0"
                      >
                        删除
                      </button>
                    </div>
                  ))}
                  {qaPins.length === 0 && <div className="text-xs text-amber-700">暂无固定记忆。可从回答中“固定到记忆”，或手动添加。</div>}
                </div>
                <div className="mt-2 flex items-center gap-2">
                  <input
                    value={pinDraft}
                    onChange={(e) => setPinDraft(e.target.value)}
                    placeholder="手动添加一条固定记忆"
                    className="flex-1 text-xs px-2 py-1.5 bg-white border border-amber-200 rounded-md focus:outline-none"
                  />
                  <button
                    onClick={() => handleSavePin(pinDraft)}
                    disabled={pinSaving || !pinDraft.trim() || !qaSessionId}
                    className="px-2.5 py-1.5 text-xs bg-amber-500 text-white rounded-md hover:bg-amber-600 disabled:opacity-50"
                  >
                    固定
                  </button>
                </div>
              </div>

              <div className="flex-1 overflow-auto space-y-3 pr-1">
                {sortedQAMessages.map((msg) => {
                  const isUser = msg.roleType === 'user'
                  return (
                    <div key={msg.id} className={`max-w-[92%] rounded-lg border px-3 py-2 ${isUser ? 'ml-auto bg-blue-50 border-blue-100' : 'bg-gray-50 border-gray-200'}`}>
                      <div className="flex items-center justify-between gap-2 text-xs mb-1">
                        <span className="font-medium text-gray-700">{isUser ? '你' : (msg.roleName || '助手')}</span>
                        <span className="text-gray-400">{msg.status === 'running' ? '生成中' : msg.status === 'failed' ? '失败' : ''}</span>
                      </div>
                      <div className="text-sm text-gray-700 leading-relaxed prose prose-sm max-w-none prose-headings:text-gray-800 prose-a:text-blue-500">
                        <ReactMarkdown>{msg.content || (msg.status === 'running' ? '正在生成回答...' : '')}</ReactMarkdown>
                      </div>
                      {msg.errorReason && (
                        <div className="text-xs text-red-500 mt-1">{msg.errorReason}</div>
                      )}
                      {!isUser && (msg.evidences?.length || 0) > 0 && (
                        <div className="mt-2 pt-2 border-t border-gray-200">
                          <div className="text-xs text-gray-500 mb-1">参考片段</div>
                          <div className="space-y-1">
                            {(msg.evidences || []).slice(0, 4).map((ev) => (
                              <div key={ev.id} className="text-xs text-gray-600 leading-relaxed">[{ev.chunkIndex}] {ev.quote}</div>
                            ))}
                          </div>
                        </div>
                      )}
                      {!isUser && msg.content.trim() && (
                        <div className="mt-2 pt-2 border-t border-gray-200 flex justify-end">
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => handlePickFollowUpMessage(msg)}
                              disabled={asking}
                              className="text-xs px-2 py-1 bg-indigo-100 text-indigo-700 rounded-md hover:bg-indigo-200 disabled:opacity-50"
                            >
                              继续追问
                            </button>
                            <button
                              onClick={() => handleSavePin(msg.content, msg.id)}
                              disabled={pinSaving || !qaSessionId}
                              className="text-xs px-2 py-1 bg-amber-100 text-amber-700 rounded-md hover:bg-amber-200 disabled:opacity-50"
                            >
                              固定到记忆
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  )
                })}
                {sortedQAMessages.length === 0 && (
                  <div className="flex flex-col items-center justify-center h-full text-gray-300">
                    <svg className="w-10 h-10 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-4l-3 3-3-3z" /></svg>
                    <p className="text-sm">输入问题并可用 @角色名 指定答复角色</p>
                  </div>
                )}
              </div>

              {qaError && (
                <div className="mt-3 text-xs text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
                  {qaError}
                </div>
              )}

              <div className="mt-3 border border-gray-200 rounded-lg p-3 bg-white relative">
                {followUpMessage && (
                  <div className="mb-2 flex items-start justify-between gap-2 rounded-md border border-indigo-200 bg-indigo-50 px-2 py-1.5">
                    <div className="min-w-0">
                      <div className="text-[11px] font-medium text-indigo-700">
                        继续追问来源：{followUpMessage.roleName || '助手'}（#{followUpMessage.id}）
                      </div>
                      <div className="text-[11px] text-indigo-600 whitespace-pre-wrap line-clamp-2">
                        {followUpMessage.content || ''}
                      </div>
                    </div>
                    <button
                      onClick={() => setFollowUpMessage(null)}
                      className="shrink-0 text-[11px] px-2 py-1 bg-white border border-indigo-200 text-indigo-600 rounded-md hover:bg-indigo-100"
                    >
                      取消
                    </button>
                  </div>
                )}
                <textarea
                  ref={qaInputRef}
                  value={qaQuestion}
                  onChange={(e) => handleQuestionInput(e.target.value, e.target.selectionStart ?? e.target.value.length)}
                  onClick={(e) => syncMentionState(e.currentTarget.value, e.currentTarget.selectionStart ?? e.currentTarget.value.length)}
                  onKeyUp={(e) => syncMentionState(e.currentTarget.value, e.currentTarget.selectionStart ?? e.currentTarget.value.length)}
                  onBlur={() => {
                    window.setTimeout(() => setMentionOpen(false), 100)
                  }}
                  onKeyDown={(e) => {
                    if ((e.nativeEvent as KeyboardEvent).isComposing) {
                      return
                    }
                    if (mentionOpen && mentionCandidates.length > 0) {
                      if (e.key === 'ArrowDown') {
                        e.preventDefault()
                        setMentionActiveIndex((prev) => (prev + 1) % mentionCandidates.length)
                        return
                      }
                      if (e.key === 'ArrowUp') {
                        e.preventDefault()
                        setMentionActiveIndex((prev) => (prev - 1 + mentionCandidates.length) % mentionCandidates.length)
                        return
                      }
                      if ((e.key === 'Enter' || e.key === 'Tab') && !e.shiftKey) {
                        e.preventDefault()
                        insertMentionRole(mentionCandidates[mentionActiveIndex] || mentionCandidates[0])
                        return
                      }
                    }
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault()
                      handleAskQuestion()
                    }
                  }}
                  rows={3}
                  placeholder="输入 @ 可选择已启用角色，例如：@finance 请从现金流角度解释这份报告"
                  className="w-full text-sm text-gray-700 placeholder-gray-400 bg-transparent focus:outline-none resize-none"
                />
                {mentionOpen && (
                  <div className="absolute left-3 right-3 bottom-[74px] max-h-44 overflow-auto rounded-md border border-gray-200 bg-white shadow-lg z-20">
                    {mentionCandidates.length > 0 ? (
                      mentionCandidates.map((role, idx) => (
                        <button
                          key={role.id}
                          onMouseDown={(e) => {
                            e.preventDefault()
                            insertMentionRole(role)
                          }}
                          className={`w-full px-3 py-2 text-left text-xs border-b border-gray-100 last:border-b-0 ${idx === mentionActiveIndex ? 'bg-blue-50 text-blue-700' : 'hover:bg-gray-50 text-gray-700'}`}
                        >
                          <span className="font-medium">{role.name}</span>
                          {role.alias && <span className="ml-2 text-gray-500">@{role.alias}</span>}
                          {role.isDefault === 1 && <span className="ml-2 text-[10px] text-blue-600 bg-blue-100 px-1.5 py-0.5 rounded">默认</span>}
                        </button>
                      ))
                    ) : (
                      <div className="px-3 py-2 text-xs text-gray-500">没有匹配的已启用角色</div>
                    )}
                  </div>
                )}
                <div className="mt-2 flex items-center justify-between">
                  <span className="text-xs text-gray-400">未使用 @ 时，将由 {defaultRoleName} 回答</span>
                  <div className="flex items-center gap-2">
                    {asking && (
                      <button
                        onClick={handleCancelAskQuestion}
                        disabled={cancelingAsk}
                        className="px-3 py-1.5 bg-rose-500 text-white text-xs rounded-md hover:bg-rose-600 disabled:opacity-50"
                      >
                        {cancelingAsk ? '停止中...' : '停止生成'}
                      </button>
                    )}
                    <button
                      onClick={handleAskQuestion}
                      disabled={asking || !qaQuestion.trim()}
                      className="px-3 py-1.5 bg-blue-500 text-white text-xs rounded-md hover:bg-blue-600 disabled:opacity-50"
                    >
                      {asking ? '提问中...' : '发送提问'}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function StructuredCards({ data }: { data: StructuredAnalysis }) {
  return (
    <div className="space-y-3 text-sm text-gray-700">
      <section className="rounded-lg border border-gray-200 bg-gray-50 p-3">
        <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">核心结论</h4>
        <p className="leading-relaxed">{data.summary || '无'}</p>
      </section>

      <section className="rounded-lg border border-red-200 bg-red-50 p-3">
        <h4 className="text-xs font-semibold text-red-600 uppercase mb-2">主要风险</h4>
        {data.risks.length > 0 ? (
          <ul className="list-disc pl-4 space-y-1">
            {data.risks.map((item, idx) => <li key={`${item}-${idx}`}>{item}</li>)}
          </ul>
        ) : <p>无</p>}
      </section>

      <section className="rounded-lg border border-emerald-200 bg-emerald-50 p-3">
        <h4 className="text-xs font-semibold text-emerald-600 uppercase mb-2">催化因素</h4>
        {data.catalysts.length > 0 ? (
          <ul className="list-disc pl-4 space-y-1">
            {data.catalysts.map((item, idx) => <li key={`${item}-${idx}`}>{item}</li>)}
          </ul>
        ) : <p>无</p>}
      </section>

      <section className="rounded-lg border border-blue-200 bg-blue-50 p-3">
        <h4 className="text-xs font-semibold text-blue-600 uppercase mb-2">估值观点</h4>
        <p className="leading-relaxed">{data.valuationView || '无'}</p>
      </section>
    </div>
  )
}

function parseStructuredAnalysis(raw: string): StructuredAnalysis | null {
  const cleaned = extractJSONObject(raw)
  if (!cleaned) {
    return null
  }
  try {
    const parsed = JSON.parse(cleaned) as Partial<StructuredAnalysis>
    return {
      summary: typeof parsed.summary === 'string' ? parsed.summary : '',
      risks: Array.isArray(parsed.risks) ? parsed.risks.filter((x): x is string => typeof x === 'string') : [],
      catalysts: Array.isArray(parsed.catalysts) ? parsed.catalysts.filter((x): x is string => typeof x === 'string') : [],
      valuationView: typeof parsed.valuationView === 'string' ? parsed.valuationView : '',
    }
  } catch {
    return null
  }
}

function extractJSONObject(raw: string): string {
  const text = raw.trim().replace(/^```json\s*/i, '').replace(/^```\s*/i, '').replace(/```$/i, '').trim()
  const start = text.indexOf('{')
  const end = text.lastIndexOf('}')
  if (start < 0 || end <= start) {
    return ''
  }
  return text.slice(start, end + 1)
}

function looksLikeMarkdown(content: string): boolean {
  const text = content || ''
  if (!text.trim()) {
    return false
  }
  const rules = [
    /(^|\n)#{1,6}\s+\S+/,
    /(^|\n)\s*[-*+]\s+\S+/,
    /(^|\n)\s*\d+\.\s+\S+/,
    /\[.+\]\(.+\)/,
    /`[^`\n]+`/,
    /(^|\n)>\s+\S+/,
    /\*\*[^*\n]+\*\*/,
    /\|.+\|.+\|/,
  ]
  return rules.some((rule) => rule.test(text))
}

function findMentionAtCaret(text: string, caret: number): { start: number; end: number; keyword: string } | null {
  if (!text || caret < 0) {
    return null
  }
  const clippedCaret = Math.min(caret, text.length)
  let i = clippedCaret - 1
  for (; i >= 0; i--) {
    const ch = text[i]
    if (ch === '@') {
      break
    }
    if (/[\s,，。:：;；!！?？()（）[\]{}]/.test(ch)) {
      return null
    }
  }
  if (i < 0 || text[i] !== '@') {
    return null
  }

  let end = clippedCaret
  for (; end < text.length; end++) {
    if (/[\s,，。:：;；!！?？()（）[\]{}]/.test(text[end])) {
      break
    }
  }
  const keyword = text.slice(i + 1, clippedCaret).trim()
  return { start: i, end, keyword }
}

function normalizeMessageOrderID(id: number): number {
  if (id >= 0) {
    return id
  }
  return 9_000_000_000_000 + Math.abs(id)
}

function normalizeComparableText(text: string): string {
  return (text || '').trim().replace(/\s+/g, ' ')
}

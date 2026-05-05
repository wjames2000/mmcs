import { useState, useEffect, useCallback, useRef } from 'react'
import type { StreamMessage } from '../types'

interface UseSSEResult {
  messages: StreamMessage[]
  status: string
  isConnected: boolean
  currentRound: number
  clearMessages: () => void
  addMessage: (msg: StreamMessage) => void
  setMessages: React.Dispatch<React.SetStateAction<StreamMessage[]>>
}

/**
 * SSE Hook — 双模式支持
 * Wails 桌面模式使用定时轮询 MessageStore（绕过 runtime.EventsOn 的 unreliability）
 * Web 浏览器模式使用 EventSource
 *
 * Wails 环境检测：优先使用 go.main.App（与 api.ts 一致），降级使用 runtime.EventsOn
 */
export function useSSE(sessionId: string | null): UseSSEResult {
  const [messages, setMessages] = useState<StreamMessage[]>([])
  const [status, setStatus] = useState<string>('')
  const [isConnected, setIsConnected] = useState(false)
  const [currentRound, setCurrentRound] = useState(0)
  const eventSourceRef = useRef<EventSource | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const lastMsgIdRef = useRef(0)

  const clearMessages = useCallback(() => {
    setMessages([])
    setCurrentRound(0)
    lastMsgIdRef.current = 0
  }, [])

  const addMessage = useCallback((msg: StreamMessage) => {
    setMessages(prev => [...prev, msg])
  }, [])

  const handleEvent = useCallback((eventData: any) => {
    const data = typeof eventData === 'string' ? JSON.parse(eventData) : eventData
    const eventType = data.type || data.event || 'message'
    const payload = data.data || data

    const message: StreamMessage = {
      type: eventType,
      node_name: payload.node_name,
      role_name: payload.role_name,
      content: payload.content || payload.message || '',
      metadata: payload.metadata,
      timestamp: payload.timestamp || new Date().toISOString(),
      error: payload.error,
    }

    switch (eventType) {
      case 'round.start':
        const roundMatch = payload?.node_name?.match(/round_(\d+)/)
        if (roundMatch) setCurrentRound(parseInt(roundMatch[1]))
        break
      case 'session.starting': setStatus('starting'); break
      case 'session.paused': setStatus('paused'); break
      case 'session.resumed': setStatus('running'); break
      case 'session.ended': setStatus('ended'); break
      case 'error': setStatus('error'); break
    }

    setMessages(prev => [...prev, message])
  }, [])

  useEffect(() => {
    if (!sessionId) {
      setMessages([])
      setStatus('')
      setIsConnected(false)
      return
    }

    // Wails 环境检测：一致使用 go.main.App（与 api.ts 对齐）
    const isWailsEnv = typeof window !== 'undefined' && !!(window as any).go?.main?.App

    if (isWailsEnv) {
      // Wails 桌面模式：轮询 MessageStore（每 1.5 秒），绕过 runtime.EventsOn 的不可靠性
      setIsConnected(true)

      const pollFn = async () => {
        try {
          const allMsgs = await (window as any).go?.main?.App?.GetSessionMessages(sessionId)
          if (!Array.isArray(allMsgs)) {
            // allMsgs 不是数组（可能是 undefined/null），不清空已有消息
            return
          }

          // 全量替换消息（包括空数组时的清空）
          setMessages(() => {
            return allMsgs.map((m: any) => ({
              type: 'role.speak' as const,
              role_name: m.role_name || m.RoleName || '',
              content: m.content || m.Content || '',
              timestamp: m.created_at || m.CreatedAt || m.timestamp || new Date().toISOString(),
            }))
          })
          if (allMsgs.length > 0) {
            setCurrentRound(prev => {
              const maxRound = Math.max(...allMsgs.map((m: any) => m.round || m.Round || 0), prev)
              return maxRound || prev
            })
          }
        } catch {
          // silently ignore poll errors
        }
      }

      // 首次立即执行 + 定时轮询
      pollFn()
      pollTimerRef.current = setInterval(pollFn, 1500)

      // 尝试额外监听 runtime.EventsOn（双通道冗余，副通道）
      let unlisten: (() => void) | null = null
      try {
        if ((window as any).runtime?.EventsOn) {
          const eventName = `session:${sessionId}`
          unlisten = (window as any).runtime.EventsOn(eventName, (eventData: any) => {
            handleEvent(eventData)
          })
        }
      } catch {
        // EventsOn 不可用不影响功能（轮询是主通道）
      }

      return () => {
        if (pollTimerRef.current) {
          clearInterval(pollTimerRef.current)
          pollTimerRef.current = null
        }
        if (unlisten) unlisten()
        setIsConnected(false)
      }
    }

    // Web 浏览器模式：使用 EventSource
    const es = new EventSource(`http://localhost:8080/api/v1/sessions/${sessionId}/stream`)
    eventSourceRef.current = es

    es.addEventListener('connected', () => setIsConnected(true))

    const eventTypes = ['round.start', 'role.speak', 'role.done', 'round.eval', 'session.paused', 'session.resumed', 'session.ended']
    eventTypes.forEach(et => {
      es.addEventListener(et, (e) => handleEvent({ ...JSON.parse(e.data), type: et } as any))
    })

    es.addEventListener('error', () => {
      handleEvent({ type: 'error', error: 'SSE 连接错误', timestamp: new Date().toISOString() })
    })

    es.onerror = () => setIsConnected(false)

    return () => {
      es.close()
      eventSourceRef.current = null
      setIsConnected(false)
    }
  }, [sessionId, handleEvent])

  return { messages, status, isConnected, currentRound, clearMessages, addMessage, setMessages }
}

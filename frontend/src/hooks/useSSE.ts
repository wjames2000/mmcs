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
 * Wails 桌面模式使用 runtime.EventsOn
 * Web 浏览器模式使用 EventSource
 */
export function useSSE(sessionId: string | null): UseSSEResult {
  const [messages, setMessages] = useState<StreamMessage[]>([])
  const [status, setStatus] = useState<string>('')
  const [isConnected, setIsConnected] = useState(false)
  const [currentRound, setCurrentRound] = useState(0)
  const eventSourceRef = useRef<EventSource | null>(null)
  const unlistenRef = useRef<(() => void) | null>(null)

  const clearMessages = useCallback(() => {
    setMessages([])
    setCurrentRound(0)
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
        // 从 node_name 提取轮次号，如 "round_3"
        const roundMatch = payload?.node_name?.match(/round_(\d+)/)
        if (roundMatch) {
          setCurrentRound(parseInt(roundMatch[1]))
        }
        break
      case 'session.starting':
        setStatus('starting')
        break
      case 'session.paused':
        setStatus('paused')
        break
      case 'session.resumed':
        setStatus('running')
        break
      case 'session.ended':
        setStatus('ended')
        break
      case 'error':
        setStatus('error')
        break
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

    const isWailsEnv = typeof window !== 'undefined' && !!(window as any).runtime?.EventsOn

    if (isWailsEnv) {
      // Wails 桌面模式：使用 runtime.EventsOn
      const eventName = `session:${sessionId}`
      const unlisten = (window as any).runtime.EventsOn(eventName, (eventData: any) => {
        handleEvent(eventData)
      })
      unlistenRef.current = unlisten
      setIsConnected(true)

      return () => {
        if (unlistenRef.current) {
          unlistenRef.current()
          unlistenRef.current = null
        }
        setIsConnected(false)
      }
    }

    // Web 浏览器模式：使用 EventSource
    const es = new EventSource(`http://localhost:8080/api/v1/sessions/${sessionId}/stream`)
    eventSourceRef.current = es

    es.addEventListener('connected', () => {
      setIsConnected(true)
    })

    es.addEventListener('round.start', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'round.start' })
    })

    es.addEventListener('role.speak', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'role.speak' })
    })

    es.addEventListener('role.done', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'role.done' })
    })

    es.addEventListener('round.eval', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'round.eval' })
    })

    es.addEventListener('session.paused', (e) => {
      setStatus('paused')
      handleEvent({ ...JSON.parse(e.data), type: 'session.paused' })
    })

    es.addEventListener('session.resumed', (e) => {
      setStatus('running')
      handleEvent({ ...JSON.parse(e.data), type: 'session.resumed' })
    })

    es.addEventListener('session.ended', (e) => {
      setStatus('ended')
      handleEvent({ ...JSON.parse(e.data), type: 'session.ended' })
    })

    es.addEventListener('error', () => {
      console.error('SSE connection error')
      handleEvent({ type: 'error', error: 'SSE 连接错误', timestamp: new Date().toISOString() })
    })

    es.onerror = () => {
      setIsConnected(false)
    }

    return () => {
      es.close()
      eventSourceRef.current = null
      setIsConnected(false)
    }
  }, [sessionId, handleEvent])

  return { messages, status, isConnected, currentRound, clearMessages, addMessage, setMessages }
}

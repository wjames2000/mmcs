import { useState, useEffect, useCallback, useRef } from 'react'
import type { StreamMessage } from '../types'

interface UseSSEResult {
  messages: StreamMessage[]
  status: string
  isConnected: boolean
  clearMessages: () => void
  addMessage: (msg: StreamMessage) => void
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
  const eventSourceRef = useRef<EventSource | null>(null)
  const unlistenRef = useRef<(() => void) | null>(null)

  const clearMessages = useCallback(() => {
    setMessages([])
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

    es.addEventListener('message', (e) => {
      handleEvent(JSON.parse(e.data))
    })

    es.addEventListener('node_start', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'node_start' })
    })

    es.addEventListener('node_end', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'node_end' })
    })

    es.addEventListener('evaluation', (e) => {
      handleEvent({ ...JSON.parse(e.data), type: 'round.eval' })
    })

    es.addEventListener('session.paused', () => {
      setStatus('paused')
    })

    es.addEventListener('session.resumed', () => {
      setStatus('running')
    })

    es.addEventListener('session.ended', () => {
      setStatus('ended')
    })

    es.addEventListener('error', (e) => {
      console.error('SSE error:', e)
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

  return { messages, status, isConnected, clearMessages, addMessage }
}

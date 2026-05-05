import { useState } from 'react'
import { api } from '../../lib/api'

interface Props {
  sessionId: string
  status: string
  onStatusChange: (newStatus: string) => void
}

export default function SessionControls({ sessionId, status, onStatusChange }: Props) {
  const [error, setError] = useState('')
  const [loading, setLoading] = useState('')

  const handleAction = async (action: string) => {
    setError('')
    setLoading(action)
    try {
      switch (action) {
        case 'start':
          await api.startSession(sessionId)
          onStatusChange('running')
          break
        case 'pause':
          await api.pauseSession(sessionId, 'user_request', '用户暂停讨论')
          onStatusChange('paused')
          break
        case 'resume':
          await api.resumeSession(sessionId, '用户恢复讨论')
          onStatusChange('running')
          break
        case 'terminate':
          await api.terminateSession(sessionId)
          onStatusChange('ended')
          break
      }
    } catch (err: any) {
      const msg = typeof err === 'string' ? err : err?.message || err?.error || '操作失败'
      setError(msg)
      console.error('Session action failed:', err)
    } finally {
      setLoading('')
    }
  }

  const btn = (action: string, label: string, color: string) => (
    <button
      onClick={() => handleAction(action)}
      disabled={loading !== ''}
      className={`px-3 py-1.5 text-xs font-medium text-white rounded-lg transition-colors disabled:opacity-50 ${color}`}
    >
      {loading === action ? '处理中...' : label}
    </button>
  )

  return (
    <div className="flex items-center gap-2">
      {status === 'draft' && btn('start', '▶ 启动', 'bg-green-600 hover:bg-green-700')}

      {status === 'running' && (
        <>
          {btn('pause', '⏸ 暂停', 'bg-yellow-600 hover:bg-yellow-700')}
          {btn('terminate', '⏹ 终止', 'bg-red-600 hover:bg-red-700')}
        </>
      )}

      {status === 'paused' && (
        <>
          {btn('resume', '▶ 恢复', 'bg-green-600 hover:bg-green-700')}
          {btn('terminate', '⏹ 终止', 'bg-red-600 hover:bg-red-700')}
        </>
      )}

      {error && <span className="text-xs text-red-600 ml-2">{error}</span>}
    </div>
  )
}

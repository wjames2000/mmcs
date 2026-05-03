import { api } from '../../lib/api'

interface Props {
  sessionId: string
  status: string
  onStatusChange: (newStatus: string) => void
}

export default function SessionControls({ sessionId, status, onStatusChange }: Props) {
  const handleAction = async (action: string) => {
    try {
      switch (action) {
        case 'start':
          await api.startSession(sessionId)
          onStatusChange('running')
          break
        case 'pause':
          await api.pauseSession(sessionId)
          onStatusChange('paused')
          break
        case 'resume':
          await api.resumeSession(sessionId)
          onStatusChange('running')
          break
        case 'terminate':
          await api.terminateSession(sessionId)
          onStatusChange('ended')
          break
      }
    } catch (err) {
      console.error('Session action failed:', err)
    }
  }

  const buttonStyle = (color: string) =>
    `px-3 py-1.5 text-xs font-medium text-white rounded-lg transition-colors ${color}`

  return (
    <div className="flex items-center gap-2">
      {status === 'draft' && (
        <button
          onClick={() => handleAction('start')}
          className={buttonStyle('bg-green-600 hover:bg-green-700')}
        >
          ▶ 启动
        </button>
      )}

      {status === 'running' && (
        <>
          <button
            onClick={() => handleAction('pause')}
            className={buttonStyle('bg-yellow-600 hover:bg-yellow-700')}
          >
            ⏸ 暂停
          </button>
          <button
            onClick={() => handleAction('terminate')}
            className={buttonStyle('bg-red-600 hover:bg-red-700')}
          >
            ⏹ 终止
          </button>
        </>
      )}

      {status === 'paused' && (
        <>
          <button
            onClick={() => handleAction('resume')}
            className={buttonStyle('bg-green-600 hover:bg-green-700')}
          >
            ▶ 恢复
          </button>
          <button
            onClick={() => handleAction('terminate')}
            className={buttonStyle('bg-red-600 hover:bg-red-700')}
          >
            ⏹ 终止
          </button>
        </>
      )}
    </div>
  )
}

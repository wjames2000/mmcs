import { useNavigate } from 'react-router-dom'
import type { Session } from '../../types'
import { PARADIGM_LABELS, PARADIGM_ICONS, STATUS_COLORS } from '../../styles/colors'

interface Props {
  session: Session
  workspaceId: string
}

export default function SessionCard({ session, workspaceId }: Props) {
  const navigate = useNavigate()

  const statusLabel: Record<string, string> = {
    draft: '草稿',
    running: '运行中',
    paused: '已暂停',
    ended: '已结束',
    failed: '失败',
  }

  const handleAction = (e: React.MouseEvent) => {
    e.stopPropagation()
    switch (session.status) {
      case 'draft':
        // Start - handled by parent via API
        break
      case 'running':
      case 'paused':
        navigate(`/sessions/${session.id}`)
        break
      case 'ended':
        navigate(`/sessions/${session.id}/minutes`)
        break
    }
  }

  const formatDuration = () => {
    if (!session.started_at) return null
    const start = new Date(session.started_at)
    const end = session.ended_at ? new Date(session.ended_at) : new Date()
    const diff = Math.round((end.getTime() - start.getTime()) / 60000)
    return diff > 0 ? `${diff} 分钟` : '刚刚开始'
  }

  const formatTime = (t: string) => {
    const d = new Date(t)
    return `${d.getMonth()+1}/${d.getDate()} ${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}`
  }

  return (
    <div
      className="bg-white rounded-lg border border-gray-200 p-4 hover:shadow-sm hover:border-blue-200 cursor-pointer transition-all"
      onClick={() => {
        if (session.status === 'ended') navigate(`/sessions/${session.id}/minutes`)
        else navigate(`/sessions/${session.id}`)
      }}
    >
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-lg">{PARADIGM_ICONS[session.paradigm] || '💬'}</span>
          <h4 className="text-sm font-semibold text-gray-900 truncate max-w-[240px]">
            {session.title}
          </h4>
        </div>
        <span
          className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium shrink-0"
          style={{
            backgroundColor:
              session.status === 'running' ? '#f0fdf4'
              : session.status === 'paused' ? '#fefce8'
              : session.status === 'ended' ? '#f3f4f6'
              : '#f3f4f6',
            color: STATUS_COLORS[session.status] || '#6b7280',
          }}
        >
          <span className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: STATUS_COLORS[session.status] }} />
          {statusLabel[session.status] || session.status}
        </span>
      </div>

      <div className="flex items-center gap-3 text-xs text-gray-400 mb-3">
        <span>{PARADIGM_LABELS[session.paradigm] || session.paradigm}</span>
        {session.started_at && <span>{formatDuration()}</span>}
        <span>最大 {session.max_rounds} 轮</span>
      </div>

      <div className="flex items-center gap-3 text-xs text-gray-400">
        <span>开始: {session.started_at ? formatTime(session.started_at) : '-'}</span>
        <span>结束: {session.ended_at ? formatTime(session.ended_at) : '-'}</span>
      </div>

      {session.status === 'ended' && (
        <button
          onClick={(e) => { e.stopPropagation(); navigate(`/sessions/${session.id}/minutes`) }}
          className="text-xs text-blue-600 hover:text-blue-700 font-medium"
        >
          查看纪要 →
        </button>
      )}
    </div>
  )
}

export function SessionCardSkeleton() {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <div className="skeleton h-5 w-48 mb-2" />
      <div className="skeleton h-3 w-32 mb-3" />
    </div>
  )
}

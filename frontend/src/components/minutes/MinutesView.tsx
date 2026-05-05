import type { MeetingMinutes } from '../../types'
import DecisionList from './DecisionList'
import ScoreMatrix from './ScoreMatrix'
import ReasoningChain from './ReasoningChain'
import MaterialList from './MaterialList'
import { PARADIGM_LABELS, PARADIGM_ICONS } from '../../styles/colors'

interface Props {
  minutes: MeetingMinutes
  loading?: boolean
}

export default function MinutesView({ minutes, loading }: Props) {
  if (loading) {
    return (
      <div className="space-y-6">
        <div className="skeleton h-8 w-64 mb-4" />
        <div className="skeleton h-4 w-48" />
        <div className="skeleton h-32 w-full" />
        <div className="skeleton h-48 w-full" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 mb-2">{minutes.title} - 会议纪要</h1>
        <div className="flex items-center gap-4 text-sm text-gray-500">
          <span className="flex items-center gap-1">
            {PARADIGM_ICONS[minutes.paradigm] || '💬'}
            {PARADIGM_LABELS[minutes.paradigm] || minutes.paradigm}
          </span>
          <span>参与角色: {minutes.participants?.length || 0} 个</span>
          {minutes.started_at && minutes.ended_at && (
            <span>
              {Math.round(
                (new Date(minutes.ended_at).getTime() - new Date(minutes.started_at).getTime()) / 60000
              )} 分钟
            </span>
          )}
        </div>
      </div>

      {/* Decisions */}
      <DecisionList decisions={minutes.decisions || []} disagreements={minutes.disagreements || []} />

      {/* Score Matrix */}
      {minutes.score_matrix && <ScoreMatrix matrix={minutes.score_matrix} />}

      {/* Reasoning Chain */}
      <ReasoningChain rounds={minutes.rounds || []} />

      {/* Conclusion */}
      {minutes.conclusion && (
        <div className="bg-blue-50 border border-blue-200 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-blue-800 mb-2">📝 讨论结论</h3>
          <p className="text-sm text-blue-700 whitespace-pre-wrap">{minutes.conclusion}</p>
        </div>
      )}

      {/* Attachments */}
      {minutes.materials && minutes.materials.length > 0 && (
        <MaterialList materials={minutes.materials} />
      )}
    </div>
  )
}

export function MinutesEmpty() {
  return (
    <div className="text-center py-16 text-gray-400">
      <p className="text-4xl mb-3">📄</p>
      <p className="text-sm">暂无会议纪要</p>
      <p className="text-xs mt-1">会话结束后将自动生成</p>
    </div>
  )
}

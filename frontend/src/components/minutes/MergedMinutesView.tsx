import type { MergedMinutes } from '../../types'
import DecisionList from './DecisionList'
import ReasoningChain from './ReasoningChain'
import MaterialList from './MaterialList'

interface Props {
  merged: MergedMinutes
  loading?: boolean
}

export default function MergedMinutesView({ merged, loading }: Props) {
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
        <h1 className="text-2xl font-bold text-gray-900 mb-2">合并会议纪要</h1>
        <div className="flex items-center gap-2 text-sm text-gray-500 mb-1">
          <span className="text-gray-700 font-medium">{merged.original_title}</span>
          <span className="text-gray-400">→</span>
          <span className="text-gray-700 font-medium">{merged.new_title}</span>
        </div>
        <p className="text-xs text-gray-400">
          包含原会话和新会话的完整讨论记录
        </p>
      </div>

      {/* Original session link */}
      <div className="bg-blue-50 border border-blue-200 rounded-xl p-4">
        <h3 className="text-sm font-semibold text-blue-800 mb-1">📋 原会话</h3>
        <p className="text-sm text-blue-700">{merged.original_title}</p>
        {/* Only show original rounds if available */}
        {merged.original_minutes?.rounds && merged.original_minutes.rounds.length > 0 && (
          <p className="text-xs text-blue-500 mt-1">
            {merged.original_minutes.rounds.length} 轮讨论 ·{' '}
            {merged.original_minutes.participants?.length || 0} 个角色参与
          </p>
        )}
      </div>

      {/* Merged Decisions */}
      <DecisionList decisions={merged.merged_decisions || []} disagreements={[]} />

      {/* Original session details */}
      {merged.original_minutes && (
        <div className="bg-white border border-gray-200 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-gray-700 mb-3">📜 原会话讨论记录</h3>
          {merged.original_minutes.conclusion && (
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-3 mb-3">
              <p className="text-xs font-medium text-gray-500 mb-1">原会话结论</p>
              <p className="text-sm text-gray-700 whitespace-pre-wrap">{merged.original_minutes.conclusion}</p>
            </div>
          )}
          <ReasoningChain rounds={merged.original_minutes.rounds || []} />
        </div>
      )}

      {/* New session details */}
      {merged.new_minutes && (
        <div className="bg-white border border-gray-200 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-gray-700 mb-3">🆕 新会话讨论记录</h3>
          {merged.new_minutes.conclusion && (
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-3 mb-3">
              <p className="text-xs font-medium text-gray-500 mb-1">新会话结论</p>
              <p className="text-sm text-gray-700 whitespace-pre-wrap">{merged.new_minutes.conclusion}</p>
            </div>
          )}
          <ReasoningChain rounds={merged.new_minutes.rounds || []} />
        </div>
      )}

      {/* Merged Conclusion */}
      {merged.merged_conclusion && (
        <div className="bg-blue-50 border border-blue-200 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-blue-800 mb-2">📝 合并结论</h3>
          <p className="text-sm text-blue-700 whitespace-pre-wrap">{merged.merged_conclusion}</p>
        </div>
      )}

      {/* Merged Materials */}
      {merged.merged_materials && merged.merged_materials.length > 0 && (
        <MaterialList materials={merged.merged_materials} />
      )}
    </div>
  )
}

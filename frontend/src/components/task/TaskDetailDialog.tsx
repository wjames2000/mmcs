import type { Task } from '../../types'
import { STATUS_COLORS } from '../../styles/colors'

interface Props {
  task: Task
  onClose: () => void
}

const STATUS_LABELS: Record<string, string> = {
  pending: '待分配',
  in_progress: '进行中',
  reviewing: '待验证',
  completed: '已完成',
  rejected: '未通过',
}

const PRIORITY_LABELS: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '紧急',
}

export default function TaskDetailDialog({ task, onClose }: Props) {
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-start justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-900">{task.title}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-lg">&times;</button>
        </div>

        <div className="space-y-4">
          <div className="flex gap-3">
            <span
              className="px-2 py-0.5 rounded-full text-xs font-medium"
              style={{
                backgroundColor:
                  task.priority === 'critical' ? '#fef2f2'
                  : task.priority === 'high' ? '#fff7ed'
                  : task.priority === 'medium' ? '#fefce8'
                  : '#f3f4f6',
                color: STATUS_COLORS[task.priority],
              }}
            >
              {PRIORITY_LABELS[task.priority] || task.priority}
            </span>
            <span
              className="px-2 py-0.5 rounded-full text-xs font-medium"
              style={{
                backgroundColor:
                  task.status === 'completed' ? '#f0fdf4'
                  : task.status === 'rejected' ? '#fef2f2'
                  : '#f3f4f6',
                color: STATUS_COLORS[task.status],
              }}
            >
              {STATUS_LABELS[task.status] || task.status}
            </span>
          </div>

          <div>
            <h4 className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">描述</h4>
            <p className="text-sm text-gray-700">{task.description || '暂无描述'}</p>
          </div>

          {task.acceptance_criteria && (
            <div>
              <h4 className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">验收标准</h4>
              <p className="text-sm text-gray-700 whitespace-pre-wrap">{task.acceptance_criteria}</p>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-xs text-gray-400">分配 Agent</p>
              <p className="text-gray-700">{task.assigned_agent || '未分配'}</p>
            </div>
            <div>
              <p className="text-xs text-gray-400">来源轮次</p>
              <p className="text-gray-700">第 {task.source_round} 轮</p>
            </div>
            <div>
              <p className="text-xs text-gray-400">创建时间</p>
              <p className="text-gray-700">{new Date(task.created_at).toLocaleString('zh-CN')}</p>
            </div>
            {task.completed_at && (
              <div>
                <p className="text-xs text-gray-400">完成时间</p>
                <p className="text-gray-700">{new Date(task.completed_at).toLocaleString('zh-CN')}</p>
              </div>
            )}
          </div>

          {task.validation_result && (
            <div className="border-t pt-3 mt-3">
              <h4 className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-2">验证结果</h4>
              <div className="bg-gray-50 rounded-lg p-3">
                <p className="text-sm font-medium">
                  判定：{task.validation_result.verdict === 'passed' ? '✅ 通过'
                    : task.validation_result.verdict === 'needs_revision' ? '⚠️ 需要修改'
                    : '❌ 拒绝'}
                </p>
                <p className="text-sm text-gray-600 mt-1">{task.validation_result.reason}</p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

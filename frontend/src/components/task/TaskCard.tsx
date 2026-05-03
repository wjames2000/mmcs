import type { Task } from '../../types'
import { STATUS_COLORS } from '../../styles/colors'

interface Props {
  task: Task
  onDragStart: (e: React.DragEvent, taskId: string) => void
  onClick: (task: Task) => void
}

const PRIORITY_LABELS: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '紧急',
}

export default function TaskCard({ task, onDragStart, onClick }: Props) {
  return (
    <div
      draggable
      onDragStart={(e) => onDragStart(e, task.id)}
      onClick={() => onClick(task)}
      className="bg-white rounded-lg border border-gray-200 p-3 hover:shadow-md hover:border-blue-200 cursor-grab active:cursor-grabbing transition-all"
    >
      <p className="text-sm font-medium text-gray-900 mb-2 line-clamp-2">{task.title}</p>

      <div className="flex items-center justify-between">
        <span
          className="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium"
          style={{
            backgroundColor:
              task.priority === 'critical' ? '#fef2f2'
              : task.priority === 'high' ? '#fff7ed'
              : task.priority === 'medium' ? '#fefce8'
              : '#f3f4f6',
            color: STATUS_COLORS[task.priority] || '#6b7280',
          }}
        >
          {PRIORITY_LABELS[task.priority] || task.priority}
        </span>

        {task.assigned_agent && (
          <span className="text-xs text-gray-400 truncate max-w-[80px]">
            {task.assigned_agent}
          </span>
        )}
      </div>
    </div>
  )
}

export function TaskCardSkeleton() {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-3">
      <div className="skeleton h-4 w-full mb-2" />
      <div className="skeleton h-4 w-3/4 mb-2" />
      <div className="skeleton h-3 w-16" />
    </div>
  )
}

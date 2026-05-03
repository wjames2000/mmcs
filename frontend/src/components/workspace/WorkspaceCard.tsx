import { useNavigate } from 'react-router-dom'
import type { Workspace } from '../../types'
import { STATUS_COLORS } from '../../styles/colors'

interface Props {
  workspace: Workspace
}

export default function WorkspaceCard({ workspace }: Props) {
  const navigate = useNavigate()

  return (
    <div
      className="bg-white rounded-xl border border-gray-200 p-5 hover:shadow-md hover:border-blue-200 cursor-pointer transition-all"
      onClick={() => navigate(`/workspaces/${workspace.id}`)}
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <span className="text-2xl">📁</span>
          <h3 className="text-base font-semibold text-gray-900 truncate max-w-[200px]">
            {workspace.name}
          </h3>
        </div>
        <span
          className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium"
          style={{
            backgroundColor: workspace.status === 'active' ? '#f0fdf4' : '#f3f4f6',
            color: STATUS_COLORS[workspace.status] || '#6b7280',
          }}
        >
          <span
            className="w-1.5 h-1.5 rounded-full"
            style={{ backgroundColor: STATUS_COLORS[workspace.status] || '#6b7280' }}
          />
          {workspace.status === 'active' ? '活跃' : '已归档'}
        </span>
      </div>

      <p className="text-sm text-gray-500 mb-4 line-clamp-2 min-h-[40px]">
        {workspace.description || '暂无描述'}
      </p>

      <div className="flex items-center gap-3 text-xs text-gray-400">
        <span className="inline-flex items-center gap-1">
          <span className="w-1.5 h-1.5 rounded-full bg-blue-500" />
          {workspace.mode === 'standalone' ? '独立模式' : '共享模式'}
        </span>
        <span>创建于 {new Date(workspace.created_at).toLocaleDateString('zh-CN')}</span>
      </div>
    </div>
  )
}

export function WorkspaceCardSkeleton() {
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5">
      <div className="flex items-center gap-2 mb-3">
        <div className="skeleton w-8 h-8" />
        <div className="skeleton h-5 w-32" />
      </div>
      <div className="skeleton h-4 w-full mb-2" />
      <div className="skeleton h-4 w-3/4 mb-4" />
      <div className="skeleton h-3 w-40" />
    </div>
  )
}

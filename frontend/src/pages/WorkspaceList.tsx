import { useState, useEffect } from 'react'
import { useAuth } from '../lib/auth'
import { api } from '../lib/api'
import WorkspaceCard, { WorkspaceCardSkeleton } from '../components/workspace/WorkspaceCard'
import CreateWorkspaceDialog from '../components/workspace/CreateWorkspaceDialog'
import type { Workspace } from '../types'

export default function WorkspaceList() {
  const { user } = useAuth()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showCreate, setShowCreate] = useState(false)

  const fetchWorkspaces = async () => {
    if (!user) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getWorkspaces(user.id)
      setWorkspaces(data || [])
    } catch (err: any) {
      setError(err.message || '加载工作区列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchWorkspaces()
  }, [user])

  const handleCreate = async (name: string, description: string, mode: string) => {
    if (!user) return
    try {
      const ws = await api.createWorkspace(user.id, name, description, mode)
      if (!ws || !ws.id) throw new Error('返回数据异常')
      setWorkspaces(prev => [ws, ...prev])
    } catch (err: any) {
      console.error('[WorkspaceList] createWorkspace error:', err)
      throw new Error(typeof err === 'string' ? err : err.message || JSON.stringify(err))
    }
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">工作区</h1>
          <p className="text-sm text-gray-500 mt-1">管理您的所有工作区和讨论会话</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors flex items-center gap-2"
        >
          <span>+</span>
          <span>新建工作区</span>
        </button>
      </div>

      {/* Content */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3, 4, 5, 6].map(i => <WorkspaceCardSkeleton key={i} />)}
        </div>
      ) : error ? (
        <div className="text-center py-16">
          <p className="text-4xl mb-3">😵</p>
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <button
            onClick={fetchWorkspaces}
            className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg"
          >
            重试
          </button>
        </div>
      ) : workspaces.length === 0 ? (
        <div className="text-center py-16">
          <p className="text-4xl mb-3">📁</p>
          <p className="text-sm text-gray-400 mb-2">暂无工作区</p>
          <p className="text-xs text-gray-400 mb-4">创建您的第一个工作区开始协作</p>
          <button
            onClick={() => setShowCreate(true)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
          >
            + 新建工作区
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {workspaces.map(ws => (
            <WorkspaceCard key={ws.id} workspace={ws} />
          ))}
        </div>
      )}

      <CreateWorkspaceDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onSubmit={handleCreate}
      />
    </div>
  )
}

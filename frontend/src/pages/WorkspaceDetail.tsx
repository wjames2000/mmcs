import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../lib/auth'
import { api } from '../lib/api'
import SessionCard, { SessionCardSkeleton } from '../components/session/SessionCard'
import CreateSessionDialog from '../components/session/CreateSessionDialog'
import type { Workspace, Session } from '../types'

export default function WorkspaceDetail() {
  const { id } = useParams<{ id: string }>()
  const { user } = useAuth()
  const navigate = useNavigate()

  const [workspace, setWorkspace] = useState<Workspace | null>(null)
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showCreate, setShowCreate] = useState(false)

  const fetchData = async () => {
    if (!id || !user) return
    setLoading(true)
    setError('')
    try {
      const [ws, sess] = await Promise.all([
        api.getWorkspaceDetail(id, user.id),
        api.getSessions(id),
      ])
      setWorkspace(ws)
      setSessions(sess || [])
    } catch (err: any) {
      setError(err.message || '加载工作区失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [id, user])

  const handleCreateSession = async (title: string, paradigm: string, maxRounds: number, roleIds: string[]) => {
    if (!user || !id) return
    const resp = await api.createSession(user.id, id, title, paradigm, maxRounds, roleIds)
    const newSession = resp.session || resp
    setSessions(prev => [newSession, ...prev])
  }

  // Stats
  const stats = {
    total: sessions.length,
    running: sessions.filter(s => s.status === 'running').length,
    paused: sessions.filter(s => s.status === 'paused').length,
    ended: sessions.filter(s => s.status === 'ended').length,
  }

  if (loading) {
    return (
      <div>
        <div className="skeleton h-8 w-48 mb-4" />
        <div className="grid grid-cols-4 gap-4 mb-6">
          {[1, 2, 3, 4].map(i => <div key={i} className="skeleton h-20 rounded-xl" />)}
        </div>
        <div className="space-y-3">
          {[1, 2, 3].map(i => <div key={i} className="skeleton h-16 rounded-lg" />)}
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-center py-16">
        <p className="text-4xl mb-3">😵</p>
        <p className="text-sm text-red-500 mb-4">{error}</p>
        <button onClick={fetchData} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">重试</button>
      </div>
    )
  }

  if (!workspace) {
    return (
      <div className="text-center py-16">
        <p className="text-4xl mb-3">🔍</p>
        <p className="text-sm text-gray-400">工作区不存在</p>
      </div>
    )
  }

  return (
    <div>
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-gray-400 mb-4">
        <Link to="/" className="hover:text-blue-600">工作区</Link>
        <span>/</span>
        <span className="text-gray-700">{workspace.name}</span>
      </div>

      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{workspace.name}</h1>
          <p className="text-sm text-gray-500 mt-1">{workspace.description || '暂无描述'}</p>
        </div>
        <div className="flex gap-2">
          <Link
            to={`/workspaces/${id}/tasks`}
            className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
          >
            📋 任务看板
          </Link>
          <button
            onClick={() => setShowCreate(true)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
          >
            + 新建会话
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        {[
          { label: '全部会话', value: stats.total, color: 'text-gray-900' },
          { label: '运行中', value: stats.running, color: 'text-green-600' },
          { label: '已暂停', value: stats.paused, color: 'text-yellow-600' },
          { label: '已结束', value: stats.ended, color: 'text-gray-500' },
        ].map(stat => (
          <div key={stat.label} className="bg-white rounded-xl border border-gray-200 p-4">
            <p className="text-sm text-gray-500">{stat.label}</p>
            <p className={`text-2xl font-bold ${stat.color}`}>{stat.value}</p>
          </div>
        ))}
      </div>

      {/* Sessions */}
      <div>
        <h2 className="text-base font-semibold text-gray-900 mb-3">会话列表</h2>
        {sessions.length === 0 ? (
          <div className="text-center py-12 bg-white rounded-xl border border-gray-200">
            <p className="text-4xl mb-3">💬</p>
            <p className="text-sm text-gray-400 mb-3">暂无会话</p>
            <button
              onClick={() => setShowCreate(true)}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
            >
              + 新建会话
            </button>
          </div>
        ) : (
          <div className="space-y-2">
            {sessions.map(session => (
              <SessionCard key={session.id} session={session} workspaceId={id!} />
            ))}
          </div>
        )}
      </div>

      <CreateSessionDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onSubmit={handleCreateSession}
      />
    </div>
  )
}

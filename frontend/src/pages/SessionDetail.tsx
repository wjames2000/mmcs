import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useSSE } from '../hooks/useSSE'
import ChatWindow from '../components/session/ChatWindow'
import SessionControls from '../components/session/SessionControls'
import { PARADIGM_LABELS, PARADIGM_ICONS, STATUS_COLORS, getRoleColor, getRoleDotStyle } from '../styles/colors'
import type { Session, SessionRole } from '../types'

export default function SessionDetail() {
  const { id: sessionId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [session, setSession] = useState<Session | null>(null)
  const [sessionRoles, setSessionRoles] = useState<SessionRole[]>([])
  const [roles, setRoles] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [sessionStatus, setSessionStatus] = useState('')

  const { messages, isConnected, clearMessages } = useSSE(sessionId || null)

  const fetchSession = async () => {
    if (!sessionId) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getSessionDetail(sessionId)
      setSession(data.session || data)
      setSessionRoles(data.roles || [])
      setSessionStatus(data.session?.status || data.status || '')

      // Load role details
      const roleIds = (data.roles || []).map((r: SessionRole) => r.role_id)
      if (roleIds.length > 0 && data.session?.creator_id) {
        // In Wails mode, need user ID
        const allRoles = await api.getRoles(data.session.creator_id)
        setRoles(allRoles.filter((r: any) => roleIds.includes(r.id)))
      }
    } catch (err: any) {
      setError(err.message || '加载会话失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchSession()
    clearMessages()
  }, [sessionId])

  const handleStatusChange = (newStatus: string) => {
    setSessionStatus(newStatus)
    if (newStatus === 'ended') {
      // Reload session data after termination
      fetchSession()
    }
  }

  // Find active speaker from recent messages
  const lastMessage = messages.filter(m =>
    m.type === 'role.speak' || m.type === 'message'
  ).pop()
  const activeSpeaker = lastMessage?.role_name

  const isStreaming = sessionStatus === 'running' && isConnected

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="skeleton h-10 w-48 mb-4" />
        <div className="flex-1 skeleton" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-center py-16">
        <p className="text-sm text-red-500 mb-4">{error}</p>
        <button onClick={fetchSession} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">重试</button>
      </div>
    )
  }

  if (!session) {
    return (
      <div className="text-center py-16">
        <p className="text-4xl mb-3">🔍</p>
        <p className="text-sm text-gray-400">会话不存在</p>
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col">
      {/* Breadcrumb & Header */}
      <div className="flex items-center justify-between mb-4 shrink-0">
        <div>
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-1">
            <Link to={`/workspaces/${session.workspace_id}`} className="hover:text-blue-600">工作区</Link>
            <span>/</span>
            <span className="text-gray-700">{session.title}</span>
          </div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-bold text-gray-900">{session.title}</h1>
            <span className="flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full"
              style={{
                backgroundColor: sessionStatus === 'running' ? '#f0fdf4'
                  : sessionStatus === 'paused' ? '#fefce8'
                  : sessionStatus === 'ended' ? '#f3f4f6'
                  : '#f3f4f6',
                color: STATUS_COLORS[sessionStatus] || '#6b7280',
              }}
            >
              <span className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: STATUS_COLORS[sessionStatus] }} />
              {sessionStatus === 'draft' ? '草稿'
                : sessionStatus === 'running' ? '运行中'
                : sessionStatus === 'paused' ? '已暂停'
                : sessionStatus === 'ended' ? '已结束'
                : session.status}
            </span>
            <span className="text-xs text-gray-400">
              {PARADIGM_ICONS[session.paradigm]} {PARADIGM_LABELS[session.paradigm]}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <SessionControls
            sessionId={session.id}
            status={sessionStatus}
            onStatusChange={handleStatusChange}
          />
          {sessionStatus === 'ended' && (
            <button
              onClick={() => navigate(`/sessions/${session.id}/minutes`)}
              className="px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 rounded-lg"
            >
              📄 查看纪要
            </button>
          )}
        </div>
      </div>

      {/* Main content: chat + participants sidebar */}
      <div className="flex-1 flex gap-4 min-h-0">
        {/* Chat area */}
        <div className="flex-1 bg-white rounded-xl border border-gray-200 flex flex-col overflow-hidden">
          <ChatWindow
            messages={messages}
            activeSpeaker={activeSpeaker}
            isStreaming={isStreaming}
          />

          {/* Interrupt input (only when paused) */}
          {sessionStatus === 'paused' && (
            <div className="border-t border-gray-200 p-3 shrink-0">
              <div className="flex gap-2">
                <input
                  type="text"
                  placeholder="输入介入消息..."
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                />
                <button className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg">
                  发送
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Participant sidebar */}
        <div className="w-48 shrink-0 bg-white rounded-xl border border-gray-200 p-4">
          <h3 className="text-xs font-semibold text-gray-500 uppercase mb-3">参与者</h3>
          {roles.length === 0 ? (
            <p className="text-xs text-gray-400">加载中...</p>
          ) : (
            <div className="space-y-3">
              {roles.map(role => {
                const color = getRoleColor(role.name)
                return (
                  <div key={role.id} className="flex items-center gap-2">
                    <span
                      className="w-2.5 h-2.5 rounded-full shrink-0"
                      style={{ backgroundColor: color.dot }}
                    />
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-gray-900 truncate">{role.name}</p>
                      <p className="text-xs text-gray-400 truncate">{role.title}</p>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

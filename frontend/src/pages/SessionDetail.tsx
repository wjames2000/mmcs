import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useSSE } from '../hooks/useSSE'
import ChatWindow from '../components/session/ChatWindow'
import SessionControls from '../components/session/SessionControls'
import MaterialUploader from '../components/session/MaterialUploader'
import RestartSessionDialog from '../components/session/RestartSessionDialog'
import { PARADIGM_LABELS, PARADIGM_ICONS, STATUS_COLORS, getRoleColor, getRoleDotStyle } from '../styles/colors'
import type { Session, SessionRole, Role, ModelProvider, Material } from '../types'

export default function SessionDetail() {
  const { id: sessionId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [session, setSession] = useState<Session | null>(null)
  const [sessionRoles, setSessionRoles] = useState<SessionRole[]>([])
  const [roles, setRoles] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [sessionStatus, setSessionStatus] = useState('')
  const [actionError, setActionError] = useState('') // 删除/归档等操作的错误通知

  // 角色管理状态
  const [showAddRole, setShowAddRole] = useState(false)
  const [allRoles, setAllRoles] = useState<Role[]>([])
  const [modelProviders, setModelProviders] = useState<ModelProvider[]>([])
  const [selectedNewRoleId, setSelectedNewRoleId] = useState<string>('')
  const [newRoleProvider, setNewRoleProvider] = useState<string>('')
  const [newRoleModel, setNewRoleModel] = useState<string>('')

  const { messages, status: sseStatus, isConnected, currentRound, clearMessages, setMessages } = useSSE(sessionId || null)

  // Sync SSE status changes to sessionStatus
  useEffect(() => {
    if (sseStatus === 'ended' || sseStatus === 'paused' || sseStatus === 'running') {
      setSessionStatus(sseStatus)
    }
  }, [sseStatus])

  // 材料管理
  const [materials, setMaterials] = useState<Material[]>([])

  // 重启会议状态
  const [showRestartDialog, setShowRestartDialog] = useState(false)

  const fetchSession = async () => {
    if (!sessionId) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getSessionDetail(sessionId)
      setSession(data.session || data)
      setSessionRoles(data.roles || [])
      setSessionStatus(data.session?.status || data.status || '')

      // Load materials
      try {
        const materialData = await api.getSessionMaterials(sessionId)
        setMaterials(materialData || [])
      } catch {
        setMaterials([])
      }

      // Load historical messages for ended sessions
      try {
        const msgs = await api.getSessionMessages(sessionId)
        if (msgs && msgs.length > 0) {
          setMessages(msgs.map((m: any) => ({
            type: 'role.speak',
            role_name: m.role_name || '',
            content: m.content || '',
            timestamp: m.created_at || new Date().toISOString(),
          })))
        }
      } catch {
        // silently ignore - messages may not be available
      }

      // Load role details
      const roleIds = (data.roles || []).map((r: SessionRole) => r.role_id)
      if (data.session?.creator_id) {
        const userId = data.session.creator_id
        // Load all roles for add-role dialog, filter current roles
        const allRolesData = await api.getRoles(userId)
        setAllRoles(allRolesData)
        setRoles(allRolesData.filter((r: any) => roleIds.includes(r.id)))

        // Load model providers for model selection
        try {
          const providers = await api.getModelProviders()
          setModelProviders(providers || [])
        } catch {
          setModelProviders([])
        }
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

  // ===== 角色管理 =====

  const handleAddRole = async () => {
    if (!sessionId || !selectedNewRoleId) return
    setError('')
    try {
      const modelOverride = newRoleProvider && newRoleModel
        ? { provider: newRoleProvider, model_name: newRoleModel }
        : undefined
      await api.addSessionRole(sessionId, selectedNewRoleId, modelOverride)
      // 刷新会话数据
      await fetchSession()
      setShowAddRole(false)
      setSelectedNewRoleId('')
      setNewRoleProvider('')
      setNewRoleModel('')
    } catch (err: any) {
      setError(err.message || '添加角色失败')
    }
  }

  const handleRemoveRole = async (roleId: string) => {
    if (!sessionId) return
    setError('')
    try {
      await api.removeSessionRole(sessionId, roleId)
      // 刷新会话数据
      await fetchSession()
    } catch (err: any) {
      setError(err.message || '移除角色失败')
    }
  }

  const handleNewRoleSelect = (roleId: string) => {
    setSelectedNewRoleId(roleId)
    // 选中角色时自动选择默认模型
    const role = allRoles.find(r => r.id === roleId)
    if (role && role.default_model && typeof role.default_model === 'object') {
      const dm = role.default_model as any
      setNewRoleProvider(dm.provider || '')
      setNewRoleModel(dm.model_name || '')
    } else {
      // 使用第一个启用的模型
      const firstProv = modelProviders.find(p => p.enabled)
      if (firstProv) {
        setNewRoleProvider(firstProv.name)
        setNewRoleModel(firstProv.default_model || '')
      }
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
            <>
              <button
                onClick={() => navigate(`/sessions/${session.id}/minutes`)}
                className="px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 rounded-lg"
              >
                📄 查看纪要
              </button>
              <button
                onClick={() => setShowRestartDialog(true)}
                className="px-3 py-1.5 text-xs font-medium text-green-600 hover:bg-green-50 rounded-lg"
              >
                🔄 重启会议
              </button>
              <button
                onClick={async () => {
                  try {
                    await api.archiveSession(session.id)
                    setSessionStatus('archived')
                    setActionError('')
                  } catch (err: any) {
                    setActionError(err.message || '归档失败')
                  }
                }}
                className="px-3 py-1.5 text-xs font-medium text-gray-600 hover:bg-gray-100 rounded-lg"
              >
                📦 归档
              </button>
              <button
                onClick={async () => {
                  try {
                    await api.deleteSession(session.id)
                    navigate(`/workspaces/${session.workspace_id}`)
                  } catch (err: any) {
                    setActionError(err.message || '删除失败')
                  }
                }}
                className="px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 rounded-lg"
              >
                🗑 删除
              </button>
            </>
          )}
        </div>
      </div>

      {/* 操作错误通知 */}
      {actionError && (
        <div className="shrink-0 mb-2 px-4 py-2 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700 flex items-center justify-between">
          <span>{actionError}</span>
          <button onClick={() => setActionError('')} className="text-red-400 hover:text-red-600 ml-2">&times;</button>
        </div>
      )}

      {/* Main content: chat + participants sidebar */}
      <div className="flex-1 flex gap-4 min-h-0">
        {/* Chat area */}
        <div className="flex-1 bg-white rounded-xl border border-gray-200 flex flex-col overflow-hidden">
          {/* 当前轮次指示器 */}
          {currentRound > 0 && isStreaming && (
            <div className="shrink-0 bg-blue-50 border-b border-blue-100 px-4 py-1.5 text-center">
              <span className="text-xs font-medium text-blue-700">
                第 {currentRound} 轮讨论中...
              </span>
            </div>
          )}
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
                  id="interrupt-input"
                  placeholder="输入介入消息..."
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                  onKeyDown={async (e) => {
                    if (e.key === 'Enter' && e.currentTarget.value.trim()) {
                      const msg = e.currentTarget.value
                      e.currentTarget.value = ''
                      try {
                        await api.resumeSession(session.id, msg)
                        setSessionStatus('running')
                      } catch (err: any) {
                        setError(err.message || '恢复会话失败')
                      }
                    }
                  }}
                />
                <button
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
                  onClick={async () => {
                    const input = document.getElementById('interrupt-input') as HTMLInputElement
                    if (!input || !input.value.trim()) return
                    const msg = input.value
                    input.value = ''
                    try {
                      await api.resumeSession(session.id, msg)
                      setSessionStatus('running')
                    } catch (err: any) {
                      setError(err.message || '恢复会话失败')
                    }
                  }}
                >
                  发送
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Participant sidebar */}
        <div className="w-56 shrink-0 bg-white rounded-xl border border-gray-200 p-4 flex flex-col">
          <h3 className="text-xs font-semibold text-gray-500 uppercase mb-3">参与者</h3>
          {roles.length === 0 ? (
            <p className="text-xs text-gray-400">加载中...</p>
          ) : (
            <div className="space-y-3 flex-1">
              {roles.map(role => {
                // 查找对应的 sessionRole 获取模型信息
                const sr = sessionRoles.find(sr => sr.role_id === role.id)
                const modelInfo = sr?.model_override
                const color = getRoleColor(role.name)
                return (
                  <div key={role.id} className="group flex items-start gap-2">
                    <span
                      className="w-2.5 h-2.5 rounded-full shrink-0 mt-1"
                      style={{ backgroundColor: color.dot }}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1">
                        <p className="text-sm font-medium text-gray-900 truncate">{role.name}</p>
                        {/* 草稿状态下的移除按钮 */}
                        {sessionStatus === 'draft' && (
                          <button
                            onClick={() => handleRemoveRole(role.id)}
                            className="opacity-0 group-hover:opacity-100 text-gray-400 hover:text-red-500 transition-opacity p-0.5"
                            title="移除此角色"
                          >
                            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          </button>
                        )}
                      </div>
                      <p className="text-xs text-gray-400 truncate">{role.title}</p>
                      {/* 显示模型信息 */}
                      {modelInfo && (
                        <p className="text-[10px] text-gray-300 truncate mt-0.5">
                          {modelInfo.provider && modelInfo.model_name
                            ? `${modelInfo.provider}: ${modelInfo.model_name}`
                            : modelInfo.provider || modelInfo.model_name || ''}
                        </p>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}

          {/* 草稿状态：添加角色按钮 */}
          {sessionStatus === 'draft' && (
            <button
              onClick={() => setShowAddRole(true)}
              className="mt-3 w-full px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 border border-dashed border-blue-300 rounded-lg transition-colors"
            >
              + 添加角色
            </button>
          )}

          {/* 会议材料 */}
          <MaterialUploader
            sessionId={sessionId || ''}
            sessionStatus={sessionStatus}
            materials={materials}
            onMaterialsChange={setMaterials}
          />
        </div>

        {/* 添加角色弹窗 */}
        {showAddRole && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowAddRole(false)}>
            <div
              className="bg-white rounded-xl shadow-xl w-full max-w-md mx-4 p-6"
              onClick={e => e.stopPropagation()}
            >
              <h3 className="text-base font-semibold text-gray-900 mb-4">添加角色</h3>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">选择角色</label>
                  <div className="max-h-48 overflow-y-auto space-y-1">
                    {allRoles
                      .filter(r => !sessionRoles.some(sr => sr.role_id === r.id))
                      .map(role => (
                        <label
                          key={role.id}
                          className={`flex items-center gap-3 px-3 py-2 rounded-lg border cursor-pointer transition-all ${
                            selectedNewRoleId === role.id
                              ? 'border-blue-500 bg-blue-50'
                              : 'border-gray-200 hover:border-gray-300'
                          }`}
                        >
                          <input
                            type="radio"
                            name="new-role"
                            checked={selectedNewRoleId === role.id}
                            onChange={() => handleNewRoleSelect(role.id)}
                            className="text-blue-600"
                          />
                          <div>
                            <p className="text-sm font-medium text-gray-900">{role.name}</p>
                            <p className="text-xs text-gray-400">{role.title}</p>
                          </div>
                        </label>
                      ))}
                    {allRoles.filter(r => !sessionRoles.some(sr => sr.role_id === r.id)).length === 0 && (
                      <p className="text-sm text-gray-400 text-center py-4">所有角色已添加</p>
                    )}
                  </div>
                </div>

                {/* 模型选择 */}
                {selectedNewRoleId && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">模型选择（可选）</label>
                    <div className="flex items-center gap-2">
                      <select
                        value={newRoleProvider}
                        onChange={e => {
                          setNewRoleProvider(e.target.value)
                          const prov = modelProviders.find(p => p.name === e.target.value)
                          setNewRoleModel(prov?.default_model || '')
                        }}
                        className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                      >
                        <option value="">默认模型</option>
                        {modelProviders.filter(p => p.enabled).map(p => (
                          <option key={p.id || p.name} value={p.name}>{p.name}</option>
                        ))}
                      </select>
                      {newRoleProvider && (
                        <input
                          type="text"
                          value={newRoleModel}
                          onChange={e => setNewRoleModel(e.target.value)}
                          placeholder="模型名称"
                          className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                        />
                      )}
                    </div>
                  </div>
                )}
              </div>

              {error && <p className="text-sm text-red-600 mt-3">{error}</p>}

              <div className="flex justify-end gap-3 mt-6">
                <button
                  onClick={() => {
                    setShowAddRole(false)
                    setSelectedNewRoleId('')
                    setNewRoleProvider('')
                    setNewRoleModel('')
                    setError('')
                  }}
                  className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-lg"
                >
                  取消
                </button>
                <button
                  onClick={handleAddRole}
                  disabled={!selectedNewRoleId}
                  className="px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
                >
                  添加
                </button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* 重启会议弹窗 */}
      {showRestartDialog && session && (
        <RestartSessionDialog
          sessionId={session.id}
          sessionTitle={session.title}
          originalRoles={roles}
          originalSessionRoles={sessionRoles}
          creatorId={session.creator_id}
          onClose={() => setShowRestartDialog(false)}
          onSuccess={(newSessionId: string) => {
            setShowRestartDialog(false)
            navigate(`/sessions/${newSessionId}`)
          }}
        />
      )}
    </div>
  )
}

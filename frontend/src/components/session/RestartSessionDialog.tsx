import { useState, useEffect } from 'react'
import { api } from '../../lib/api'
import type { Role, SessionRole } from '../../types'

interface Props {
  sessionId: string
  sessionTitle: string
  originalRoles: Role[]
  originalSessionRoles: SessionRole[]
  creatorId: string
  onClose: () => void
  onSuccess: (newSessionId: string) => void
}

export default function RestartSessionDialog({
  sessionId,
  sessionTitle,
  originalRoles,
  originalSessionRoles,
  creatorId,
  onClose,
  onSuccess,
}: Props) {
  const [title, setTitle] = useState('')
  const [topic, setTopic] = useState('')
  const [allRoles, setAllRoles] = useState<Role[]>([])
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [materials, setMaterials] = useState<any[]>([])
  const [uploading, setUploading] = useState(false)

  // Pre-fill with original session data
  useEffect(() => {
    const titleFromOriginal = sessionTitle
      ? `${sessionTitle} - 续会`
      : '续会讨论'
    setTitle(titleFromOriginal)

    // Pre-select original roles
    setSelectedRoleIds(originalRoles.map(r => r.id))

    // Load all available roles
    const fetchRoles = async () => {
      try {
        const roles = await api.getRoles(creatorId)
        setAllRoles(roles || [])
      } catch {
        setAllRoles(originalRoles)
      }
    }
    fetchRoles()
  }, [originalRoles, creatorId])

  const toggleRole = (roleId: string) => {
    setSelectedRoleIds(prev =>
      prev.includes(roleId)
        ? prev.filter(id => id !== roleId)
        : [...prev, roleId]
    )
  }

  const handleRestart = async () => {
    if (!title.trim()) {
      setError('请输入新会话标题')
      return
    }
    if (selectedRoleIds.length === 0) {
      setError('请至少选择一个角色')
      return
    }

    setLoading(true)
    setError('')
    try {
      const roleBindings = selectedRoleIds.map(roleId => {
        const sr = originalSessionRoles.find(sr => sr.role_id === roleId)
        return {
          role_id: roleId,
          model_override: sr?.model_override || undefined,
        }
      })

      const resp = await api.restartSession(sessionId, creatorId, title.trim(), topic.trim(), selectedRoleIds, roleBindings)
      const newSessionId = resp?.session?.id
      if (!newSessionId) {
        throw new Error('未获取到新会话 ID')
      }

      // Upload new materials (files added during restart, not copied from original)
      for (const mat of materials) {
        if (mat._isNew) {
          try {
            await api.uploadSessionMaterial(newSessionId, mat.file_name, mat.mime_type, mat._data)
          } catch (e) {
            console.warn('上传材料失败:', mat.file_name, e)
          }
        }
      }

      onSuccess(newSessionId)
    } catch (err: any) {
      const msg = typeof err === 'string' ? err : err?.message || '重启会议失败'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      const base64 = (reader.result as string).split(',')[1] || reader.result
      setMaterials(prev => [...prev, {
        _isNew: true,
        file_name: file.name,
        mime_type: file.type,
        file_size: file.size,
        _data: base64,
      }])
    }
    reader.readAsDataURL(file)
    e.target.value = ''
  }

  const removeMaterial = (idx: number) => {
    setMaterials(prev => prev.filter((_, i) => i !== idx))
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6"
        onClick={e => e.stopPropagation()}
      >
        <h3 className="text-base font-semibold text-gray-900 mb-2">🔄 重启会议</h3>
        <p className="text-xs text-gray-500 mb-4">
          将创建一个新会话，原会话的角色和材料将自动复制到新会话中。
        </p>

        <div className="space-y-4">
          {/* 新标题 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">新会话标题 *</label>
            <input
              type="text"
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder="输入新会话标题"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
            />
          </div>

          {/* 新主题描述 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">主题描述（可选）</label>
            <textarea
              value={topic}
              onChange={e => setTopic(e.target.value)}
              placeholder="输入讨论主题或背景描述..."
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none resize-none"
            />
          </div>

          {/* 角色选择 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              选择参与角色（{selectedRoleIds.length} 个已选）
            </label>
            <div className="max-h-40 overflow-y-auto space-y-1 border border-gray-200 rounded-lg p-2">
              {allRoles.length === 0 ? (
                <p className="text-xs text-gray-400 text-center py-2">加载中...</p>
              ) : (
                allRoles.map(role => (
                  <label
                    key={role.id}
                    className={`flex items-center gap-3 px-3 py-2 rounded-lg border cursor-pointer transition-all ${
                      selectedRoleIds.includes(role.id)
                        ? 'border-blue-500 bg-blue-50'
                        : 'border-gray-100 hover:border-gray-300'
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={selectedRoleIds.includes(role.id)}
                      onChange={() => toggleRole(role.id)}
                      className="text-blue-600 rounded"
                    />
                    <div>
                      <p className="text-sm font-medium text-gray-900">{role.name}</p>
                      <p className="text-xs text-gray-400">{role.title}</p>
                    </div>
                  </label>
                ))
              )}
            </div>
          </div>
          </div>

          {/* 附件上传 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">新增附件材料</label>
            <label htmlFor="restart-file-input"
              className="block border-2 border-dashed border-gray-300 rounded-lg p-4 text-center hover:border-blue-400 transition-colors cursor-pointer">
              <p className="text-xs text-gray-500">📎 点击选择文件上传</p>
            </label>
            <input id="restart-file-input" type="file" className="hidden" onChange={handleFileSelect} />
            {materials.length > 0 && (
              <div className="mt-2 space-y-1">
                {materials.map((m, i) => (
                  <div key={i} className="flex items-center justify-between px-3 py-1.5 bg-gray-50 rounded-lg text-xs">
                    <span className="text-gray-700 truncate">{m.file_name}</span>
                    <button onClick={() => removeMaterial(i)} className="text-red-500 hover:text-red-700 ml-2">✕</button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {error && <p className="text-sm text-red-600">{error}</p>}

        <div className="flex justify-end gap-3 mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-lg"
          >
            取消
          </button>
          <button
            onClick={handleRestart}
            disabled={loading || !title.trim() || selectedRoleIds.length === 0}
            className="px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
          >
            {loading ? '创建中...' : '确认重启'}
          </button>
        </div>
      </div>
    </div>
  )
}

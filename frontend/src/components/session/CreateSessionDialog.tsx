import { useState, useEffect } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../lib/auth'
import { PARADIGM_LABELS, PARADIGM_ICONS } from '../../styles/colors'
import type { Role } from '../../types'

interface Props {
  open: boolean
  onClose: () => void
  onSubmit: (title: string, paradigm: string, maxRounds: number, roleIds: string[]) => Promise<void>
}

const PARADIGMS = ['round_robin', 'court', 'evaluation', 'free_chat'] as const

export default function CreateSessionDialog({ open, onClose, onSubmit }: Props) {
  const { user } = useAuth()
  const [title, setTitle] = useState('')
  const [paradigm, setParadigm] = useState<string>('round_robin')
  const [maxRounds, setMaxRounds] = useState(10)
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [loadingRoles, setLoadingRoles] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open && user) {
      setLoadingRoles(true)
      api.getRoles(user.id)
        .then(setRoles)
        .catch(() => setRoles([]))
        .finally(() => setLoadingRoles(false))
    }
  }, [open, user])

  if (!open) return null

  const toggleRole = (roleId: string) => {
    setSelectedRoleIds(prev =>
      prev.includes(roleId) ? prev.filter(id => id !== roleId) : [...prev, roleId]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) { setError('会话标题不能为空'); return }
    if (selectedRoleIds.length === 0) { setError('至少选择一个角色'); return }

    setSubmitting(true)
    setError('')
    try {
      await onSubmit(title.trim(), paradigm, maxRounds, selectedRoleIds)
      setTitle('')
      setParadigm('round_robin')
      setMaxRounds(10)
      setSelectedRoleIds([])
      onClose()
    } catch (err: any) {
      setError(err.message || '创建会话失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-gray-900 mb-4">新建会话</h2>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">会话标题 *</label>
            <input
              type="text"
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder="例如：用户登录模块代码评审"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
              autoFocus
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">讨论范式</label>
            <div className="grid grid-cols-2 gap-2">
              {PARADIGMS.map(p => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setParadigm(p)}
                  className={`flex items-center gap-2 px-3 py-2.5 rounded-lg border text-sm transition-all ${
                    paradigm === p
                      ? 'border-blue-500 bg-blue-50 text-blue-700'
                      : 'border-gray-200 hover:border-gray-300 text-gray-600'
                  }`}
                >
                  <span>{PARADIGM_ICONS[p]}</span>
                  <span>{PARADIGM_LABELS[p]}</span>
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">最大轮数</label>
            <input
              type="number"
              value={maxRounds}
              onChange={e => setMaxRounds(Math.max(1, parseInt(e.target.value) || 1))}
              min={1}
              max={100}
              className="w-24 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">选择角色 *</label>
            {loadingRoles ? (
              <div className="space-y-2">
                {[1, 2, 3].map(i => <div key={i} className="skeleton h-10 w-full" />)}
              </div>
            ) : roles.length === 0 ? (
              <p className="text-sm text-gray-400">暂无可用角色，请先在角色管理中创建</p>
            ) : (
              <div className="space-y-1.5 max-h-48 overflow-y-auto">
                {roles.map(role => (
                  <label
                    key={role.id}
                    className={`flex items-center gap-3 px-3 py-2 rounded-lg border cursor-pointer transition-all ${
                      selectedRoleIds.includes(role.id)
                        ? 'border-blue-500 bg-blue-50'
                        : 'border-gray-200 hover:border-gray-300'
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
                ))}
              </div>
            )}
          </div>

          {error && <p className="text-sm text-red-600">{error}</p>}

          <div className="flex justify-end gap-3 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-lg">
              取消
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
            >
              {submitting ? '创建中...' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

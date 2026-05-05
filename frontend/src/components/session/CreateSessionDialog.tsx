import { useState, useEffect, useCallback } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../lib/auth'
import { PARADIGM_LABELS, PARADIGM_ICONS } from '../../styles/colors'
import type { Role, ModelProvider } from '../../types'

interface RoleBinding {
  role_id: string
  model_override?: { provider: string; model_name: string }
}

interface Props {
  open: boolean
  onClose: () => void
  onSubmit: (title: string, paradigm: string, maxRounds: number, roleIds: string[], roleBindings?: RoleBinding[], topic?: string) => Promise<void>
}

const PARADIGMS = ['round_robin', 'court', 'evaluation', 'free_chat'] as const

export default function CreateSessionDialog({ open, onClose, onSubmit }: Props) {
  const { user } = useAuth()
  const [title, setTitle] = useState('')
  const [topic, setTopic] = useState('')
  const [paradigm, setParadigm] = useState<string>('round_robin')
  const [maxRounds, setMaxRounds] = useState(10)
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [loadingRoles, setLoadingRoles] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  // 模型相关状态
  const [modelProviders, setModelProviders] = useState<ModelProvider[]>([])
  const [roleModels, setRoleModels] = useState<Record<string, { provider: string; modelName: string }>>({})

  // 角色默认模型映射
  const getDefaultModel = useCallback((role: Role): { provider: string; modelName: string } => {
    if (role.default_model && typeof role.default_model === 'object') {
      return {
        provider: (role.default_model as any).provider || '',
        modelName: (role.default_model as any).model_name || '',
      }
    }
    return { provider: '', modelName: '' }
  }, [])

  useEffect(() => {
    if (open && user) {
      setLoadingRoles(true)
      Promise.all([
        api.getRoles(user.id),
        api.getModelProviders(),
      ])
        .then(([rolesData, providers]) => {
          setRoles(rolesData)
          setModelProviders(providers || [])
        })
        .catch(() => {
          setRoles([])
          setModelProviders([])
        })
        .finally(() => setLoadingRoles(false))
    }
  }, [open, user])

  if (!open) return null

  const toggleRole = (roleId: string) => {
    setSelectedRoleIds(prev => {
      if (prev.includes(roleId)) {
        // 移除角色时清除其模型选择
        const newModels = { ...roleModels }
        delete newModels[roleId]
        setRoleModels(newModels)
        return prev.filter(id => id !== roleId)
      } else {
        // 选中角色时设置默认模型
        const role = roles.find(r => r.id === roleId)
        if (role) {
          const defaultModel = getDefaultModel(role)
          // 如果有默认模型且没有手动设置过，则使用默认值
          if (defaultModel.provider && defaultModel.modelName) {
            setRoleModels(prev => ({
              ...prev,
              [roleId]: defaultModel,
            }))
          } else {
            // 如果角色没有默认模型，使用第一个启用的模型提供商
            const firstEnabled = modelProviders.find(p => p.enabled)
            if (firstEnabled) {
              setRoleModels(prev => ({
                ...prev,
                [roleId]: { provider: firstEnabled.name, modelName: firstEnabled.default_model || '' },
              }))
            }
          }
        }
        return [...prev, roleId]
      }
    })
  }

  const updateRoleModel = (roleId: string, provider: string, modelName: string) => {
    setRoleModels(prev => ({
      ...prev,
      [roleId]: { provider, modelName },
    }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) { setError('会话标题不能为空'); return }
    if (selectedRoleIds.length === 0) { setError('至少选择一个角色'); return }

    setSubmitting(true)
    setError('')
    try {
      // 构建 roleBindings
      const roleBindings: RoleBinding[] = selectedRoleIds.map(roleId => {
        const modelInfo = roleModels[roleId]
        if (modelInfo && modelInfo.provider && modelInfo.modelName) {
          return {
            role_id: roleId,
            model_override: { provider: modelInfo.provider, model_name: modelInfo.modelName },
          }
        }
        return { role_id: roleId }
      })

      await onSubmit(title.trim(), paradigm, maxRounds, selectedRoleIds, roleBindings, topic.trim())
      setTitle('')
      setTopic('')
      setParadigm('round_robin')
      setMaxRounds(10)
      setSelectedRoleIds([])
      setRoleModels({})
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
            <label className="block text-sm font-medium text-gray-700 mb-1">讨论主题描述</label>
            <textarea
              value={topic}
              onChange={e => setTopic(e.target.value)}
              placeholder="描述本次讨论的具体内容、背景和期望达成的目标..."
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none resize-none"
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
              <div className="space-y-1.5 max-h-64 overflow-y-auto">
                {roles.map(role => {
                  const isSelected = selectedRoleIds.includes(role.id)
                  return (
                    <div key={role.id}>
                      <label
                        className={`flex items-center gap-3 px-3 py-2 rounded-lg border cursor-pointer transition-all ${
                          isSelected
                            ? 'border-blue-500 bg-blue-50'
                            : 'border-gray-200 hover:border-gray-300'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => toggleRole(role.id)}
                          className="text-blue-600 rounded"
                        />
                        <div>
                          <p className="text-sm font-medium text-gray-900">{role.name}</p>
                          <p className="text-xs text-gray-400">{role.title}</p>
                        </div>
                      </label>
                      {/* 选中角色时显示模型选择 */}
                      {isSelected && (
                        <div className="ml-8 mt-1 mb-2 p-2 bg-gray-50 rounded-lg border border-gray-100">
                          <ModelSelector
                            providers={modelProviders}
                            roleId={role.id}
                            roleName={role.name}
                            value={roleModels[role.id] || { provider: '', modelName: '' }}
                            onChange={updateRoleModel}
                          />
                        </div>
                      )}
                    </div>
                  )
                })}
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

// ==================== ModelSelector 子组件 ====================

interface ModelSelectorProps {
  providers: ModelProvider[]
  roleId: string
  roleName: string
  value: { provider: string; modelName: string }
  onChange: (roleId: string, provider: string, modelName: string) => void
}

function ModelSelector({ providers, roleId, roleName, value, onChange }: ModelSelectorProps) {
  const enabledProviders = providers.filter(p => p.enabled)

  const handleProviderChange = (providerName: string) => {
    const prov = enabledProviders.find(p => p.name === providerName)
    onChange(roleId, providerName, prov?.default_model || '')
  }

  const currentProvider = enabledProviders.find(p => p.name === value.provider)

  // 构建可选的模型列表：默认使用 default_model，用户可自定义输入
  const defaultModel = currentProvider?.default_model || ''
  // 如果当前选中的模型名与 default_model 不同，也加入到选项中
  const modelOptions: string[] = [defaultModel]
  if (value.modelName && value.modelName !== defaultModel) {
    modelOptions.push(value.modelName)
  }
  // 过滤空值
  const filteredModelOptions = modelOptions.filter(Boolean)

  if (enabledProviders.length === 0) {
    return (
      <div className="flex items-center gap-2">
        <span className="text-xs text-gray-400">暂无可用模型提供商</span>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-gray-500 whitespace-nowrap">模型：</span>
      <select
        value={value.provider}
        onChange={e => handleProviderChange(e.target.value)}
        className="text-xs px-2 py-1 border border-gray-200 rounded bg-white focus:ring-1 focus:ring-blue-500 outline-none"
        onClick={e => e.stopPropagation()}
      >
        <option value="">选择提供商</option>
        {enabledProviders.map(p => (
          <option key={p.id || p.name} value={p.name}>
            {p.name}
          </option>
        ))}
      </select>
      {value.provider && (
        <div className="flex items-center gap-1">
          {filteredModelOptions.length > 0 ? (
            <select
              value={value.modelName}
              onChange={e => onChange(roleId, value.provider, e.target.value)}
              className="text-xs px-2 py-1 border border-gray-200 rounded bg-white focus:ring-1 focus:ring-blue-500 outline-none max-w-[160px]"
              onClick={e => e.stopPropagation()}
            >
              <option value="">选择模型</option>
              {filteredModelOptions.map(m => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
          ) : (
            <input
              type="text"
              value={value.modelName}
              onChange={e => onChange(roleId, value.provider, e.target.value)}
              placeholder="模型名称"
              className="text-xs px-2 py-1 border border-gray-200 rounded bg-white focus:ring-1 focus:ring-blue-500 outline-none w-[120px]"
              onClick={e => e.stopPropagation()}
            />
          )}
        </div>
      )}
    </div>
  )
}

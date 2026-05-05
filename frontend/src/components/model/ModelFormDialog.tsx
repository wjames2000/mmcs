import { useState, useEffect, useCallback } from 'react'
import type { ModelProvider, CreateModelProviderReq } from '../../types'

interface Props {
  open: boolean
  editItem?: ModelProvider | null
  onClose: () => void
  onSubmit: (data: CreateModelProviderReq) => Promise<void>
}

const PROVIDER_TYPES = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'ark', label: 'Ark' },
  { value: 'custom', label: '自定义' },
]

export default function ModelFormDialog({ open, editItem, onClose, onSubmit }: Props) {
  const [name, setName] = useState('')
  const [provider, setProvider] = useState('openai')
  const [baseUrl, setBaseUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [showApiKey, setShowApiKey] = useState(false)
  const [defaultModel, setDefaultModel] = useState('')
  const [availableModels, setAvailableModels] = useState<string[]>([])
  const [refreshing, setRefreshing] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const isEdit = !!editItem

  useEffect(() => {
    if (editItem) {
      setName(editItem.name)
      setProvider(editItem.provider)
      setBaseUrl(editItem.base_url)
      setApiKey(editItem.api_key)
      setShowApiKey(false)
      setDefaultModel(editItem.default_model)
    } else {
      setName('')
      setProvider('openai')
      setBaseUrl('')
      setApiKey('')
      setShowApiKey(false)
      setDefaultModel('')
    }
    setAvailableModels([])
    setError('')
  }, [editItem, open])

  const fetchModels = useCallback(async () => {
    if (!baseUrl.trim()) { setError('请先填写 API 地址'); return }
    setRefreshing(true)
    setError('')
    try {
      const models = await (window as any).go?.main?.App?.RefreshModelsFromProvider
        ? (window as any).go.main.App.RefreshModelsFromProvider(name || provider)
        : fetch(`${baseUrl.replace(/\/$/, '')}/models`, {
            headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {},
          }).then(r => r.json()).then(r => (r.data || []).map((m: any) => m.id || m))

      const list = Array.isArray(models) ? models : []
      setAvailableModels(list)
      if (list.length > 0 && !defaultModel) {
        setDefaultModel(list[0])
      }
    } catch (err: any) {
      setError('获取模型列表失败: ' + (err.message || err))
    } finally {
      setRefreshing(false)
    }
  }, [baseUrl, apiKey, name, provider, defaultModel])

  if (!open) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('提供商名称不能为空'); return }
    if (!baseUrl.trim()) { setError('API 地址不能为空'); return }
    if (!apiKey.trim() && !isEdit) { setError('API Key 不能为空'); return }

    setSubmitting(true)
    setError('')
    try {
      await onSubmit({
        name: name.trim(),
        provider,
        api_key: apiKey.trim(),
        base_url: baseUrl.trim(),
        default_model: defaultModel.trim(),
      })
      if (!isEdit) {
        setName(''); setProvider('openai'); setBaseUrl('')
        setApiKey(''); setDefaultModel(''); setAvailableModels([])
      }
      onClose()
    } catch (err: any) {
      setError(err.message || (isEdit ? '更新失败' : '添加失败'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-xl w-full max-w-md mx-4 p-6"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-gray-900 mb-4">
          {isEdit ? '编辑模型配置' : '添加模型配置'}
        </h2>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">提供商名称 *</label>
            <input type="text" value={name} onChange={e => setName(e.target.value)}
              placeholder="例如：my-openai, local-ollama"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
              autoFocus disabled={isEdit} />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">提供商类型 *</label>
            <select value={provider} onChange={e => setProvider(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none bg-white">
              {PROVIDER_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">API 地址 *</label>
            <input type="text" value={baseUrl} onChange={e => setBaseUrl(e.target.value)}
              placeholder="例如：https://api.openai.com/v1"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
          </div>

          {/* API Key with show/hide toggle */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              API Key {!isEdit && '*'}
            </label>
            <div className="relative">
              <input
                type={showApiKey ? 'text' : 'password'}
                value={apiKey}
                onChange={e => setApiKey(e.target.value)}
                placeholder={isEdit ? '留空则保持不变' : 'sk-...'}
                className="w-full px-3 py-2 pr-10 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none font-mono"
              />
              <button
                type="button"
                onClick={() => setShowApiKey(!showApiKey)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                title={showApiKey ? '隐藏' : '显示'}
              >
                {showApiKey ? '🙈' : '👁️'}
              </button>
            </div>
          </div>

          {/* Default model with refresh */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="block text-sm font-medium text-gray-700">默认模型</label>
              <button
                type="button"
                onClick={fetchModels}
                disabled={refreshing}
                className="text-xs text-blue-600 hover:text-blue-700 disabled:text-gray-400"
              >
                {refreshing ? '刷新中...' : '↻ 刷新模型列表'}
              </button>
            </div>
            {availableModels.length > 0 ? (
              <select
                value={defaultModel}
                onChange={e => setDefaultModel(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none bg-white"
              >
                {availableModels.map(m => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                value={defaultModel}
                onChange={e => setDefaultModel(e.target.value)}
                placeholder="先点击「刷新模型列表」获取，或手动输入"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
              />
            )}
            {availableModels.length > 0 && (
              <p className="text-xs text-green-600 mt-1">已加载 {availableModels.length} 个模型</p>
            )}
          </div>

          {error && <p className="text-sm text-red-600">{error}</p>}

          <div className="flex justify-end gap-3 pt-2">
            <button type="button" onClick={onClose}
              className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-lg">
              取消
            </button>
            <button type="submit" disabled={submitting}
              className="px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50">
              {submitting ? '保存中...' : (isEdit ? '保存' : '添加')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

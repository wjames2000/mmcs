import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import ModelFormDialog from '../components/model/ModelFormDialog'
import type { ModelProvider, CreateModelProviderReq } from '../types'

export default function ModelConfig() {
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editItem, setEditItem] = useState<ModelProvider | null>(null)

  const fetchProviders = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.getModelProviders()
      setProviders(data || [])
    } catch (err: any) {
      setError(err.message || '加载模型配置失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchProviders()
  }, [fetchProviders])

  const handleAdd = async (data: CreateModelProviderReq) => {
    await api.createModelProvider(data)
    await fetchProviders()
  }

  const handleEdit = async (data: CreateModelProviderReq) => {
    if (!editItem) return
    await api.updateModelProvider(editItem.id, data)
    setEditItem(null)
    await fetchProviders()
  }

  const handleDelete = async (id: string) => {
    if (!window.confirm('确定要删除此模型提供商配置吗？')) return
    try {
      await api.deleteModelProvider(id)
      await fetchProviders()
    } catch (err: any) {
      alert(err.message || '删除失败')
    }
  }

  const handleToggle = async (id: string) => {
    try {
      await api.toggleModelProvider(id)
      setProviders(prev =>
        prev.map(p => (p.id === id ? { ...p, enabled: !p.enabled } : p))
      )
    } catch (err: any) {
      alert(err.message || '切换状态失败')
    }
  }

  // 脱敏显示 API Key
  const maskApiKey = (key: string): string => {
    if (!key || key.length < 8) return '••••••••'
    return key.slice(0, 4) + '••••' + key.slice(-4)
  }

  // 判断 provider 类型标签颜色
  const providerBadgeClass = (provider: string) => {
    const map: Record<string, string> = {
      openai: 'bg-green-100 text-green-700',
      openrouter: 'bg-blue-100 text-blue-700',
      claude: 'bg-orange-100 text-orange-700',
      anthropic: 'bg-orange-100 text-orange-700',
      ollama: 'bg-yellow-100 text-yellow-700',
      ark: 'bg-blue-100 text-blue-700',
    }
    return map[provider] || 'bg-gray-100 text-gray-700'
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">模型配置</h1>
          <p className="text-sm text-gray-500 mt-1">管理 AI 模型提供商配置</p>
        </div>
        <button
          onClick={() => { setEditItem(null); setShowForm(true) }}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
        >
          + 添加模型
        </button>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-40 rounded-xl bg-gray-100 animate-pulse" />
          ))}
        </div>
      ) : error ? (
        <div className="text-center py-16">
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <button onClick={fetchProviders} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">
            重试
          </button>
        </div>
      ) : providers.length === 0 ? (
        <div className="text-center py-16">
          <p className="text-4xl mb-3">⚙️</p>
          <p className="text-sm text-gray-400 mb-3">暂无模型配置</p>
          <button
            onClick={() => { setEditItem(null); setShowForm(true) }}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
          >
            + 添加模型
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {providers.map(p => (
            <div
              key={p.id}
              className={`bg-white rounded-xl border p-5 transition-all hover:shadow-md ${
                p.enabled ? 'border-gray-200' : 'border-gray-100 opacity-60'
              }`}
            >
              {/* 头部：名称 + 类型标签 */}
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg bg-blue-50 flex items-center justify-center text-blue-600 font-bold text-sm">
                    {p.name.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <h3 className="text-base font-semibold text-gray-900">{p.name}</h3>
                    <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${providerBadgeClass(p.provider)}`}>
                      {p.provider}
                    </span>
                  </div>
                </div>
                {/* 启用/禁用开关 */}
                <button
                  onClick={() => handleToggle(p.id)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    p.enabled ? 'bg-green-500' : 'bg-gray-300'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      p.enabled ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {/* 配置信息 */}
              <div className="space-y-1.5 text-sm">
                <div className="flex items-center gap-2">
                  <span className="text-gray-400 w-16 shrink-0">API 地址</span>
                  <span className="text-gray-700 truncate" title={p.base_url}>
                    {p.base_url}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-gray-400 w-16 shrink-0">API Key</span>
                  <span className="text-gray-700 font-mono">{maskApiKey(p.api_key)}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-gray-400 w-16 shrink-0">默认模型</span>
                  <span className="text-gray-700">{p.default_model || '-'}</span>
                </div>
              </div>

              {/* 操作按钮 */}
              <div className="flex items-center gap-2 mt-4 pt-3 border-t border-gray-100">
                <button
                  onClick={() => { setEditItem(p); setShowForm(true) }}
                  className="px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                >
                  编辑
                </button>
                <button
                  onClick={() => handleDelete(p.id)}
                  className="px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 rounded-lg transition-colors ml-auto"
                >
                  删除
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 创建表单对话框 */}
      <ModelFormDialog
        open={showForm && !editItem}
        editItem={null}
        onClose={() => setShowForm(false)}
        onSubmit={handleAdd}
      />

      {/* 编辑表单对话框 */}
      <ModelFormDialog
        open={showForm && !!editItem}
        editItem={editItem}
        onClose={() => { setShowForm(false); setEditItem(null) }}
        onSubmit={handleEdit}
      />
    </div>
  )
}

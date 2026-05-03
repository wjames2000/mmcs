import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import ModelFormDialog from '../components/model/ModelFormDialog'

export default function ModelConfig() {
  const [models, setModels] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)

  const fetchModels = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.getModels()
      setModels(data || [])
    } catch (err: any) {
      setError(err.message || '加载模型配置失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchModels()
  }, [])

  const handleAddModel = async (name: string, baseUrl: string, defaultModel: string) => {
    // In a real implementation, this would call an API to save the model config
    setModels(prev => [...prev, name])
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">模型配置</h1>
          <p className="text-sm text-gray-500 mt-1">管理 AI 模型提供商配置</p>
        </div>
        <button
          onClick={() => setShowForm(true)}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
        >
          + 添加模型
        </button>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map(i => <div key={i} className="skeleton h-20 rounded-xl" />)}
        </div>
      ) : error ? (
        <div className="text-center py-16">
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <button onClick={fetchModels} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">重试</button>
        </div>
      ) : models.length === 0 ? (
        <div className="text-center py-16">
          <p className="text-4xl mb-3">⚙️</p>
          <p className="text-sm text-gray-400 mb-3">暂无模型配置</p>
          <button onClick={() => setShowForm(true)} className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg">
            + 添加模型
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {models.map(name => (
            <div key={name} className="bg-white rounded-xl border border-gray-200 p-5 hover:shadow-md transition-all">
              <div className="flex items-center gap-3 mb-2">
                <div className="w-10 h-10 rounded-lg bg-blue-50 flex items-center justify-center text-blue-600 font-bold text-sm">
                  {name.charAt(0).toUpperCase()}
                </div>
                <div>
                  <h3 className="text-base font-semibold text-gray-900">{name}</h3>
                  <p className="text-xs text-gray-400">模型提供商</p>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      <ModelFormDialog
        open={showForm}
        onClose={() => setShowForm(false)}
        onSubmit={handleAddModel}
      />
    </div>
  )
}

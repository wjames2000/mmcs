import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import TaskBoardComponent from '../components/task/TaskBoard'
import type { Task, TaskStatus } from '../types'

export default function TaskBoard() {
  const { id: workspaceId } = useParams<{ id: string }>()
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchTasks = async () => {
    if (!workspaceId) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getTasks(workspaceId)
      setTasks(data || [])
    } catch (err: any) {
      setError(err.message || '加载任务失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTasks()
  }, [workspaceId])

  const handleStatusChange = async (taskId: string, newStatus: TaskStatus) => {
    // Optimistic update
    setTasks(prev =>
      prev.map(t => (t.id === taskId ? { ...t, status: newStatus } : t))
    )

    try {
      await api.updateTaskStatus(taskId, newStatus)
    } catch (err: any) {
      // Rollback on failure
      fetchTasks()
      console.error('Status update failed:', err)
    }
  }

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-400 mb-4">
        <Link to={`/workspaces/${workspaceId}`} className="hover:text-blue-600">工作区</Link>
        <span>/</span>
        <span className="text-gray-700">任务看板</span>
      </div>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">任务看板</h1>
          <p className="text-sm text-gray-500 mt-1">拖拽卡片变更任务状态</p>
        </div>
        <button
          onClick={fetchTasks}
          className="px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100 rounded-lg"
        >
          刷新
        </button>
      </div>

      {error ? (
        <div className="text-center py-16">
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <button onClick={fetchTasks} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">重试</button>
        </div>
      ) : (
        <TaskBoardComponent
          tasks={tasks}
          loading={loading}
          onStatusChange={handleStatusChange}
        />
      )}
    </div>
  )
}

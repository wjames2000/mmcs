import { useState } from 'react'
import type { Task, TaskStatus } from '../../types'
import TaskCard from './TaskCard'
import TaskDetailDialog from './TaskDetailDialog'

interface Props {
  tasks: Task[]
  loading?: boolean
  onStatusChange: (taskId: string, newStatus: TaskStatus) => void
}

const COLUMNS: { status: TaskStatus; label: string; color: string }[] = [
  { status: 'pending', label: '待分配', color: '#6b7280' },
  { status: 'in_progress', label: '进行中', color: '#2563eb' },
  { status: 'reviewing', label: '待验证', color: '#ca8a04' },
  { status: 'completed', label: '已完成', color: '#16a34a' },
  { status: 'rejected', label: '未通过', color: '#dc2626' },
]

export default function TaskBoard({ tasks, loading, onStatusChange }: Props) {
  const [dragOverColumn, setDragOverColumn] = useState<string | null>(null)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)

  const getTasksByStatus = (status: TaskStatus) =>
    tasks.filter(t => t.status === status)

  const handleDragStart = (e: React.DragEvent, taskId: string) => {
    e.dataTransfer.setData('text/plain', taskId)
    e.dataTransfer.effectAllowed = 'move'
  }

  const handleDragOver = (e: React.DragEvent, status: string) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOverColumn(status)
  }

  const handleDragLeave = () => {
    setDragOverColumn(null)
  }

  const handleDrop = (e: React.DragEvent, newStatus: TaskStatus) => {
    e.preventDefault()
    setDragOverColumn(null)
    const taskId = e.dataTransfer.getData('text/plain')
    if (taskId) onStatusChange(taskId, newStatus)
  }

  if (loading) {
    return (
      <div className="grid grid-cols-5 gap-4">
        {COLUMNS.map(col => (
          <div key={col.status} className="space-y-3">
            <div className="skeleton h-8 w-24 mb-3" />
            {[1, 2].map(i => <div key={i} className="skeleton h-24 w-full" />)}
          </div>
        ))}
      </div>
    )
  }

  if (tasks.length === 0) {
    return (
      <div className="text-center py-16 text-gray-400">
        <p className="text-4xl mb-3">📋</p>
        <p className="text-sm">暂无任务</p>
      </div>
    )
  }

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-4">
        {COLUMNS.map(col => {
          const columnTasks = getTasksByStatus(col.status)
          return (
            <div
              key={col.status}
              onDragOver={(e) => handleDragOver(e, col.status)}
              onDragLeave={handleDragLeave}
              onDrop={(e) => handleDrop(e, col.status)}
              className={`rounded-xl p-3 min-h-[200px] transition-colors ${
                dragOverColumn === col.status ? 'kanban-column-dragover' : 'bg-gray-50'
              }`}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2">
                  <span
                    className="w-2.5 h-2.5 rounded-full"
                    style={{ backgroundColor: col.color }}
                  />
                  <h3 className="text-sm font-semibold text-gray-700">{col.label}</h3>
                </div>
                <span className="text-xs text-gray-400 bg-white px-2 py-0.5 rounded-full border">
                  {columnTasks.length}
                </span>
              </div>

              <div className="space-y-2">
                {columnTasks.map(task => (
                  <TaskCard
                    key={task.id}
                    task={task}
                    onDragStart={handleDragStart}
                    onClick={setSelectedTask}
                  />
                ))}
              </div>
            </div>
          )
        })}
      </div>

      {selectedTask && (
        <TaskDetailDialog
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
        />
      )}
    </>
  )
}

import { useState, useEffect } from 'react'
import { useAuth } from '../lib/auth'
import { api } from '../lib/api'
import RoleCard, { RoleCardSkeleton } from '../components/role/RoleCard'
import RoleFormDialog from '../components/role/RoleFormDialog'
import type { Role } from '../types'

export default function RoleManager() {
  const { user } = useAuth()
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingRole, setEditingRole] = useState<Role | null>(null)

  const fetchRoles = async () => {
    if (!user) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getRoles(user.id)
      setRoles(data || [])
    } catch (err: any) {
      setError(err.message || '加载角色列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchRoles()
  }, [user])

  const handleCreate = async (data: any) => {
    if (!user) return
    try {
      const newRole = await api.createRole(user.id, data)
      setRoles(prev => [newRole, ...prev])
    } catch (err: any) {
      throw new Error(err.message || '创建角色失败')
    }
  }

  const handleUpdate = async (data: any, roleId?: string) => {
    if (!user || !roleId) return
    const updated = await api.updateRole(roleId, user.id, data)
    setRoles(prev => prev.map(r => r.id === roleId ? updated : r))
  }

  const handleDelete = async (roleId: string) => {
    if (!user) return
    try {
      await api.deleteRole(roleId, user.id)
      setRoles(prev => prev.filter(r => r.id !== roleId))
    } catch (err: any) {
      alert(err.message || '删除失败')
    }
  }

  const openEdit = (role: Role) => {
    setEditingRole(role)
    setShowForm(true)
  }

  const openCreate = () => {
    setEditingRole(null)
    setShowForm(true)
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">角色管理</h1>
          <p className="text-sm text-gray-500 mt-1">创建和管理 AI 讨论角色</p>
        </div>
        <button
          onClick={openCreate}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg"
        >
          + 创建角色
        </button>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3, 4, 5, 6].map(i => <RoleCardSkeleton key={i} />)}
        </div>
      ) : error ? (
        <div className="text-center py-16">
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <button onClick={fetchRoles} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">重试</button>
        </div>
      ) : roles.length === 0 ? (
        <div className="text-center py-16">
          <p className="text-4xl mb-3">📋</p>
          <p className="text-sm text-gray-400 mb-3">暂无角色</p>
          <button onClick={openCreate} className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg">
            + 创建角色
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {roles.map(role => (
            <RoleCard
              key={role.id}
              role={role}
              onEdit={openEdit}
              onDelete={handleDelete}
            />
          ))}
        </div>
      )}

      <RoleFormDialog
        open={showForm}
        role={editingRole}
        onClose={() => { setShowForm(false); setEditingRole(null) }}
        onSubmit={editingRole ? handleUpdate : handleCreate}
      />
    </div>
  )
}

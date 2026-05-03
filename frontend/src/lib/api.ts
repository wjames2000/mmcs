/**
 * 双模式 API 层
 * 自动检测 Wails 桌面环境或浏览器 HTTP 环境
 */

const isWails = typeof window !== 'undefined' && !!(window as any).go?.main?.App
const API_BASE = 'http://localhost:8080/api/v1'

// ==================== 底层调用 ====================

async function wailsCall<T>(name: string, ...args: any[]): Promise<T> {
  const fn = (window as any).go.main.App[name]
  if (!fn) throw new Error(`Wails 方法未绑定: ${name}`)
  return await fn(...args)
}

function getToken(): string | null {
  try {
    return localStorage.getItem('token')
  } catch {
    return null
  }
}

function setToken(token: string | null) {
  try {
    if (token) localStorage.setItem('token', token)
    else localStorage.removeItem('token')
  } catch { /* noop */ }
}

function getStoredUser(): any | null {
  try {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

function setStoredUser(user: any | null) {
  try {
    if (user) localStorage.setItem('user', JSON.stringify(user))
    else localStorage.removeItem('user')
  } catch { /* noop */ }
}

async function httpCall<T>(method: string, path: string, body?: any): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  // 处理 401 自动登出
  if (res.status === 401) {
    setToken(null)
    setStoredUser(null)
    if (typeof window !== 'undefined') {
      window.location.href = '/login'
    }
    throw new Error('登录已过期，请重新登录')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || `HTTP ${res.status}`)
  }

  // 204 No Content
  if (res.status === 204) return undefined as T

  return res.json()
}

// ==================== 认证 Token 管理 ====================
export const auth = {
  getToken,
  setToken,
  getStoredUser,
  setStoredUser,
  isAuthenticated: (): boolean => !!getToken(),
}

// ==================== API 接口 ====================
export const api = {
  // ---- Auth ----
  login: async (email: string, password: string) => {
    if (isWails) return wailsCall<any>('Login', email, password)
    return httpCall<any>('POST', '/auth/login', { email, password })
  },

  register: async (name: string, email: string, password: string) => {
    if (isWails) return wailsCall<any>('Register', name, email, password)
    return httpCall<any>('POST', '/auth/register', { name, email, password })
  },

  refreshToken: async (token: string) => {
    if (isWails) return wailsCall<string>('RefreshToken', token)
    return httpCall<{ token: string }>('POST', '/auth/refresh', { token }).then(r => r.token)
  },

  getCurrentUser: async (userId: string) => {
    if (isWails) return wailsCall<any>('GetCurrentUser', userId)
    return httpCall<any>('GET', `/users/${userId}`)
  },

  // ---- Workspace ----
  getWorkspaces: async (userId: string) => {
    if (isWails) return wailsCall<any[]>('GetWorkspaces', userId)
    return httpCall<any[]>('GET', '/workspaces')
  },

  createWorkspace: async (creatorId: string, name: string, description: string, mode: string) => {
    if (isWails) return wailsCall<any>('CreateWorkspace', creatorId, name, description, mode)
    return httpCall<any>('POST', '/workspaces', { name, description, mode })
  },

  getWorkspaceDetail: async (id: string, userId: string) => {
    if (isWails) return wailsCall<any>('GetWorkspaceDetail', id, userId)
    return httpCall<any>('GET', `/workspaces/${id}`)
  },

  updateWorkspace: async (id: string, userId: string, name: string, description: string, mode: string) => {
    if (isWails) return wailsCall<any>('UpdateWorkspace', id, userId, name, description, mode)
    return httpCall<any>('PUT', `/workspaces/${id}`, { name, description, mode })
  },

  archiveWorkspace: async (id: string, userId: string) => {
    if (isWails) return wailsCall<void>('ArchiveWorkspace', id, userId)
    return httpCall<void>('POST', `/workspaces/${id}/archive`)
  },

  // ---- Session ----
  getSessions: async (workspaceId: string) => {
    if (isWails) return wailsCall<any[]>('GetSessions', workspaceId)
    return httpCall<any[]>('GET', `/workspaces/${workspaceId}/sessions`)
  },

  createSession: async (creatorId: string, workspaceId: string, title: string, paradigm: string, maxRounds: number, roleIds: string[]) => {
    if (isWails) return wailsCall<any>('CreateSession', creatorId, workspaceId, title, paradigm, maxRounds, roleIds)
    return httpCall<any>('POST', '/sessions', { workspace_id: workspaceId, title, paradigm, max_rounds: maxRounds, role_ids: roleIds })
  },

  getSessionDetail: async (id: string) => {
    if (isWails) return wailsCall<any>('GetSessionDetail', id)
    return httpCall<any>('GET', `/sessions/${id}`)
  },

  startSession: async (id: string) => {
    if (isWails) return wailsCall<void>('StartSession', id)
    return httpCall<void>('POST', `/sessions/${id}/start`)
  },

  pauseSession: async (id: string, nodeName?: string, message?: string) => {
    if (isWails) return wailsCall<void>('PauseSession', id, nodeName || '', message || '')
    return httpCall<void>('POST', `/sessions/${id}/pause`, { node_name: nodeName, message })
  },

  resumeSession: async (id: string, message?: string) => {
    if (isWails) return wailsCall<void>('ResumeSession', id, message || '')
    return httpCall<void>('POST', `/sessions/${id}/resume`, { message })
  },

  terminateSession: async (id: string) => {
    if (isWails) return wailsCall<void>('TerminateSession', id)
    return httpCall<void>('POST', `/sessions/${id}/terminate`)
  },

  // ---- Role ----
  getRoles: async (userId: string) => {
    if (isWails) return wailsCall<any[]>('GetRoles', userId)
    return httpCall<any[]>('GET', '/roles')
  },

  createRole: async (creatorId: string, data: any) => {
    if (isWails) return wailsCall<any>('CreateRole', creatorId, data)
    return httpCall<any>('POST', '/roles', data)
  },

  updateRole: async (id: string, userId: string, data: any) => {
    if (isWails) return wailsCall<any>('UpdateRole', id, userId, data)
    return httpCall<any>('PUT', `/roles/${id}`, data)
  },

  deleteRole: async (id: string, userId: string) => {
    if (isWails) return wailsCall<void>('DeleteRole', id, userId)
    return httpCall<void>('DELETE', `/roles/${id}`)
  },

  getSkills: async () => {
    if (isWails) return wailsCall<any[]>('GetSkills')
    return httpCall<any[]>('GET', '/roles/skills')
  },

  // ---- Task ----
  getTasks: async (workspaceId: string) => {
    if (isWails) return wailsCall<any[]>('GetTasks', workspaceId)
    return httpCall<any[]>('GET', `/workspaces/${workspaceId}/tasks`)
  },

  createTask: async (data: any) => {
    if (isWails) return wailsCall<any>('CreateTask', data)
    return httpCall<any>('POST', '/tasks', data)
  },

  updateTaskStatus: async (id: string, status: string) => {
    if (isWails) return wailsCall<void>('UpdateTaskStatus', id, status)
    return httpCall<void>('PATCH', `/tasks/${id}/status`, { status })
  },

  assignTask: async (taskId: string, agentId: string, assignedBy: string) => {
    if (isWails) return wailsCall<void>('AssignTask', taskId, agentId, assignedBy)
    return httpCall<void>('POST', `/tasks/${taskId}/assign`, { assigned_agent: agentId, assigned_by: assignedBy })
  },

  getTaskDetail: async (id: string) => {
    if (isWails) return wailsCall<any>('GetTaskDetail', id)
    return httpCall<any>('GET', `/tasks/${id}`)
  },

  // ---- Model ----
  getModels: async () => {
    if (isWails) return wailsCall<string[]>('GetModels')
    return httpCall<string[]>('GET', '/models')
  },
}

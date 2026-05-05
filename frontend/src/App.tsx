import { Component, type ReactNode } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './lib/auth'
import Layout from './components/layout/Layout'
import Login from './pages/Login'
import WorkspaceList from './pages/WorkspaceList'
import WorkspaceDetail from './pages/WorkspaceDetail'
import TaskBoard from './pages/TaskBoard'
import SessionDetail from './pages/SessionDetail'
import SessionMinutes from './pages/SessionMinutes'
import RoleManager from './pages/RoleManager'
import ModelConfig from './pages/ModelConfig'

class ErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; error?: Error }> {
  constructor(props: { children: ReactNode }) {
    super(props)
    this.state = { hasError: false }
  }
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error }
  }
  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center min-h-screen bg-gray-50">
          <div className="text-center p-8">
            <p className="text-5xl mb-4">💥</p>
            <h2 className="text-lg font-semibold text-gray-900 mb-2">页面出错了</h2>
            <p className="text-sm text-gray-500 mb-4 max-w-md">{this.state.error?.message}</p>
            <button
              onClick={() => { this.setState({ hasError: false }); window.location.reload() }}
              className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700"
            >
              重新加载
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ErrorBoundary>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<AuthGuard><Layout /></AuthGuard>}>
          <Route path="/" element={<WorkspaceList />} />
          <Route path="/workspaces/:id" element={<WorkspaceDetail />} />
          <Route path="/workspaces/:id/tasks" element={<TaskBoard />} />
          <Route path="/sessions/:id" element={<SessionDetail />} />
          <Route path="/sessions/:id/minutes" element={<SessionMinutes />} />
          <Route path="/roles" element={<RoleManager />} />
          <Route path="/models" element={<ModelConfig />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ErrorBoundary>
  )
}

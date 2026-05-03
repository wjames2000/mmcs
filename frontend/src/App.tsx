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

function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
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
  )
}

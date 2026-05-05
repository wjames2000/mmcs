import React, { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import { api, auth as authStore } from './api'
import type { User, LoginResponse, RegisterResponse } from '../types'

interface AuthContextType {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (name: string, email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(() => authStore.getStoredUser())
  const [token, setToken] = useState<string | null>(() => authStore.getToken())
  const [isLoading, setIsLoading] = useState(false)

  const isAuthenticated = !!token

  const login = useCallback(async (email: string, password: string) => {
    setIsLoading(true)
    try {
      const res = await api.login(email, password) as LoginResponse
      const accessToken = res.access_token || (res as any).token
      const userData = res.user
      setToken(accessToken)
      setUser(userData)
      authStore.setToken(accessToken)
      authStore.setStoredUser(userData)
    } finally {
      setIsLoading(false)
    }
  }, [])

  const register = useCallback(async (name: string, email: string, password: string) => {
    setIsLoading(true)
    try {
      const res = await api.register(name, email, password) as RegisterResponse
      // Register response may not auto-login; need separate login
      // If it returns token/user, use them; otherwise user will redirect to login
      const accessToken = (res as any).access_token || (res as any).token
      if (accessToken) {
        setToken(accessToken)
        authStore.setToken(accessToken)
      }
      if ((res as any).user) {
        setUser((res as any).user)
        authStore.setStoredUser((res as any).user)
      }
    } finally {
      setIsLoading(false)
    }
  }, [])

  const logout = useCallback(() => {
    setToken(null)
    setUser(null)
    authStore.setToken(null)
    authStore.setStoredUser(null)
  }, [])

  // 恢复登录状态
  useEffect(() => {
    const storedToken = authStore.getToken()
    const storedUser = authStore.getStoredUser()
    if (storedToken && storedUser) {
      setToken(storedToken)
      setUser(storedUser)
    }
  }, [])

  return (
    <AuthContext.Provider value={{ user, token, isAuthenticated, isLoading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextType {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

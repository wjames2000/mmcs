import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import MinutesView from '../components/minutes/MinutesView'
import { MinutesEmpty } from '../components/minutes/MinutesView'
import type { MeetingMinutes } from '../types'

export default function SessionMinutes() {
  const { id: sessionId } = useParams<{ id: string }>()
  const [minutes, setMinutes] = useState<MeetingMinutes | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Try to build minutes from session data and messages
  const fetchMinutes = async () => {
    if (!sessionId) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getSessionDetail(sessionId)
      const session = data.session || data

      // If there were a dedicated minutes endpoint, we'd call it.
      // For now, construct a basic minutes view from session data.
      const participants = (data.roles || []).map((r: any) => {
        // In Wails mode, roles may have a name property
        return r.name || r.role_name || r.role_id
      })

      const minutesData: MeetingMinutes = {
        session_id: session.id,
        title: session.title,
        paradigm: session.paradigm,
        participants,
        started_at: session.started_at || '',
        ended_at: session.ended_at || '',
        rounds: [],
        decisions: [],
        disagreements: [],
        conclusion: '',
      }

      setMinutes(minutesData)
    } catch (err: any) {
      setError(err.message || '加载会议纪要失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchMinutes()
  }, [sessionId])

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-400 mb-4">
        <Link to={`/sessions/${sessionId}`} className="hover:text-blue-600">会话</Link>
        <span>/</span>
        <span className="text-gray-700">会议纪要</span>
      </div>

      {error ? (
        <div className="text-center py-16">
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <button onClick={fetchMinutes} className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded-lg">重试</button>
        </div>
      ) : loading ? (
        <MinutesView minutes={null as any} loading />
      ) : minutes ? (
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <MinutesView minutes={minutes} />
        </div>
      ) : (
        <MinutesEmpty />
      )}
    </div>
  )
}

import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import MinutesView from '../components/minutes/MinutesView'
import MergedMinutesView from '../components/minutes/MergedMinutesView'
import { MinutesEmpty } from '../components/minutes/MinutesView'
import type { MeetingMinutes, MergedMinutes, Session } from '../types'

export default function SessionMinutes() {
  const { id: sessionId } = useParams<{ id: string }>()
  const [minutes, setMinutes] = useState<MeetingMinutes | null>(null)
  const [merged, setMerged] = useState<MergedMinutes | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchMinutes = async () => {
    if (!sessionId) return
    setLoading(true)
    setError('')
    try {
      // First, get the session to check if it has a parent_session_id
      let parentSessionId: string | undefined
      try {
        const sessionDetail = await api.getSessionDetail(sessionId)
        const sess: Session = sessionDetail.session || sessionDetail
        parentSessionId = sess.parent_session_id
      } catch {
        // Ignore - treat as normal session
      }

      if (parentSessionId) {
        // This is a restarted session - fetch merged minutes
        const mergedData = await api.getMergedMinutes(sessionId, parentSessionId)
        setMerged(mergedData)
      } else {
        // Normal session - fetch regular minutes
        const data = await api.getSessionMinutes(sessionId)
        setMinutes(data)
      }
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
      ) : merged ? (
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <MergedMinutesView merged={merged} />
        </div>
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

import { getRoleColor, getRoleBorderStyle, USER_COLOR } from '../../styles/colors'
import Markdown from '../shared/Markdown'

interface Props {
  roleName: string
  content: string
  timestamp: string
  isStreaming: boolean
}

export default function MessageBubble({ roleName, content, timestamp, isStreaming }: Props) {
  const isUser = roleName === 'user' || roleName === ''

  if (isUser) {
    return (
      <div className="flex justify-end mb-3">
        <div className="max-w-[70%]">
          <div
            className="rounded-2xl rounded-tr-sm px-4 py-2.5 text-sm"
            style={{ backgroundColor: USER_COLOR.bg }}
          >
            <p className="text-gray-800 whitespace-pre-wrap break-words">{content}</p>
          </div>
          {timestamp && (
            <p className="text-xs text-gray-400 mt-1 text-right">
              {new Date(timestamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
            </p>
          )}
        </div>
      </div>
    )
  }

  const color = getRoleColor(roleName)

  return (
    <div className="mb-3">
      <div
        className="rounded-lg px-4 py-3 max-w-[80%]"
        style={{
          ...getRoleBorderStyle(roleName),
          backgroundColor: color.bg,
        }}
      >
        <div className="flex items-center gap-2 mb-1.5">
          <span
            className="w-2 h-2 rounded-full shrink-0"
            style={{ backgroundColor: color.dot }}
          />
          <span className="text-xs font-semibold" style={{ color: color.border }}>
            {roleName}
          </span>
          <span className="text-xs text-gray-400">
            {new Date(timestamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
          </span>
        </div>
        <div className={`text-sm text-gray-800 ${isStreaming ? 'speaking-cursor' : ''}`}>
          <Markdown content={content} />
        </div>
      </div>
    </div>
  )
}

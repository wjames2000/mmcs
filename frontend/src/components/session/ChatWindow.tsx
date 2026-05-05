import { useRef, useEffect } from 'react'
import MessageBubble from './MessageBubble'
import TypingIndicator from './TypingIndicator'
import type { StreamMessage } from '../../types'

interface Props {
  messages: StreamMessage[]
  activeSpeaker?: string
  isStreaming: boolean
  fontSize?: number
}

export default function ChatWindow({ messages, activeSpeaker, isStreaming, fontSize }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="text-center text-gray-400">
          <p className="text-4xl mb-3">💬</p>
          <p className="text-sm">等待讨论开始...</p>
          <p className="text-xs mt-1">消息将会实时显示在此处</p>
        </div>
      </div>
    )
  }

  // Group round indicators
  const renderMessages = () => {
    const elements: React.ReactNode[] = []

    messages.forEach((msg, idx) => {
      if (msg.type === 'round.start' || msg.type === 'node_start') {
        // 从 node_name 提取轮次号，如 "round_3"
        const roundMatch = msg.node_name?.match(/round_(\d+)/)
        const roundNum = roundMatch ? parseInt(roundMatch[1]) : ''
        const label = roundNum
          ? `第 ${roundNum} 轮讨论开始`
          : (msg.content || '讨论阶段开始')
        elements.push(
          <div key={`round-${idx}`} className="flex items-center gap-3 my-4">
            <div className="flex-1 h-px bg-gray-200" />
            <span className="text-xs text-gray-400 font-medium px-2 whitespace-nowrap">
              {label}
            </span>
            <div className="flex-1 h-px bg-gray-200" />
          </div>
        )
        return
      }

      if (msg.type === 'round.eval' || msg.type === 'evaluation') {
        elements.push(
          <div key={`eval-${idx}`} className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 my-2 text-sm text-yellow-800">
            <span className="font-medium">📊 评估：</span>
            {msg.content}
          </div>
        )
        return
      }

      // Only render message/speak type events
      if (msg.type === 'message' || msg.type === 'role.speak' || msg.type === 'role.done' || msg.type === 'moderator_speech') {
        elements.push(
          <MessageBubble
            key={`msg-${idx}`}
            roleName={msg.role_name || ''}
            content={msg.content || ''}
            timestamp={msg.timestamp}
            isStreaming={msg.type === 'role.speak' && isStreaming && msg.role_name === activeSpeaker}
            fontSize={fontSize}
          />
        )
      }
    })

    return elements
  }

  return (
    <div className="flex-1 overflow-y-auto px-4 py-4 space-y-1">
      {renderMessages()}
      {isStreaming && activeSpeaker && (
        <TypingIndicator roleName={activeSpeaker} />
      )}
      <div ref={bottomRef} />
    </div>
  )
}

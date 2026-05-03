import { getRoleColor } from '../../styles/colors'

interface Props {
  roleName: string
}

export default function TypingIndicator({ roleName }: Props) {
  const color = getRoleColor(roleName)

  return (
    <div className="mb-3">
      <div
        className="rounded-lg px-4 py-3 max-w-[60%]"
        style={{
          borderLeft: `4px solid ${color.border}`,
          backgroundColor: color.bg,
        }}
      >
        <div className="flex items-center gap-2 mb-1">
          <span className="w-2 h-2 rounded-full" style={{ backgroundColor: color.dot }} />
          <span className="text-xs font-semibold" style={{ color: color.border }}>
            {roleName}
          </span>
          <span className="text-xs text-gray-400">正在输入...</span>
        </div>
        <div className="flex items-center gap-1">
          <span
            className="w-2 h-2 rounded-full animate-bounce"
            style={{ backgroundColor: color.border, animationDelay: '0ms' }}
          />
          <span
            className="w-2 h-2 rounded-full animate-bounce"
            style={{ backgroundColor: color.border, animationDelay: '150ms' }}
          />
          <span
            className="w-2 h-2 rounded-full animate-bounce"
            style={{ backgroundColor: color.border, animationDelay: '300ms' }}
          />
        </div>
      </div>
    </div>
  )
}

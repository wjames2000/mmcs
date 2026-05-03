import type { RoundRecord } from '../../types'
import { getRoleColor } from '../../styles/colors'

interface Props {
  rounds: RoundRecord[]
}

export default function ReasoningChain({ rounds }: Props) {
  if (rounds.length === 0) {
    return (
      <div className="bg-gray-50 rounded-xl p-5 text-center text-sm text-gray-400">
        暂无讨论记录
      </div>
    )
  }

  return (
    <div>
      <h3 className="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
        <span>🔗</span> 推理链
      </h3>

      <div className="relative">
        {/* Vertical line */}
        <div className="absolute left-4 top-2 bottom-2 w-0.5 bg-gray-200" />

        <div className="space-y-6">
          {rounds.map((round, idx) => (
            <div key={idx} className="relative pl-10">
              {/* Round number circle */}
              <div className="absolute left-2.5 top-1 w-3 h-3 rounded-full bg-blue-500 border-2 border-white shadow" />

              <div className="bg-white border border-gray-200 rounded-lg p-4">
                <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">
                  第 {round.round_number} 轮
                </h4>

                <div className="space-y-2">
                  {round.speeches.map((speech, sIdx) => {
                    const color = getRoleColor(speech.role_name)
                    return (
                      <div key={sIdx} className="flex gap-2">
                        <span
                          className="text-xs font-semibold shrink-0 mt-0.5"
                          style={{ color: color.border }}
                        >
                          {speech.role_name}:
                        </span>
                        <p className="text-sm text-gray-700">{speech.content}</p>
                      </div>
                    )
                  })}
                </div>

                {round.eval_result && (
                  <div className="mt-2 pt-2 border-t border-gray-100">
                    <p className="text-xs text-yellow-700">
                      <span className="font-medium">📊 评估：</span>
                      {round.eval_result}
                    </p>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

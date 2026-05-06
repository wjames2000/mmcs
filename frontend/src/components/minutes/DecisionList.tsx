import type { Decision, Disagreement } from '../../types'
import Markdown from '../shared/Markdown'

interface Props {
  decisions: Decision[]
  disagreements: Disagreement[]
}

export default function DecisionList({ decisions, disagreements }: Props) {
  if (decisions.length === 0 && disagreements.length === 0) {
    return (
      <div className="bg-gray-50 rounded-xl p-5 text-center text-sm text-gray-400">
        暂无决策记录
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold text-gray-700 flex items-center gap-2">
        <span>📌</span> 关键决策
      </h3>

      <div className="space-y-2">
        {decisions.map((d, i) => (
          <div
            key={i}
            className={`rounded-lg border p-4 ${
              d.accepted ? 'bg-green-50 border-green-200' : 'bg-yellow-50 border-yellow-200'
            }`}
          >
            <div className="flex items-start gap-2">
              <span className="text-lg mt-0.5">{d.accepted ? '✅' : '⚠️'}</span>
              <div>
                <h4 className="text-sm font-medium text-gray-900">{d.title}</h4>
                <div className="text-sm text-gray-600 mt-1">
                  <Markdown content={d.description} />
                </div>
                {d.rejected_by && d.rejected_by.length > 0 && (
                  <p className="text-xs text-gray-400 mt-1">
                    反对: {d.rejected_by.join(', ')}
                  </p>
                )}
              </div>
            </div>
          </div>
        ))}

        {disagreements.map((d, i) => (
          <div key={`dis-${i}`} className="bg-orange-50 border border-orange-200 rounded-lg p-4">
            <div className="flex items-start gap-2">
              <span className="text-lg mt-0.5">🔶</span>
              <div>
                <h4 className="text-sm font-medium text-gray-900">{d.topic}</h4>
                <p className="text-sm text-gray-600 mt-1">
                  观点: {d.positions.join(' vs ')}
                </p>
                <span className={`text-xs mt-1 inline-block px-2 py-0.5 rounded-full ${
                  d.resolved ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'
                }`}>
                  {d.resolved ? '已解决' : '未解决'}
                </span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

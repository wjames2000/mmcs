import type { ScoreMatrix as ScoreMatrixType } from '../../types'

interface Props {
  matrix: ScoreMatrixType
}

export default function ScoreMatrix({ matrix }: Props) {
  if (!matrix.options || matrix.options.length === 0) {
    return (
      <div className="bg-gray-50 rounded-xl p-5 text-center text-sm text-gray-400">
        暂无评分数据
      </div>
    )
  }

  // Calculate averages per option per criterion
  const calcAverage = (optionId: string, criterion: string) => {
    const entries = matrix.entries.filter(
      e => e.option_id === optionId && e.criterion_name === criterion
    )
    if (entries.length === 0) return null
    const sum = entries.reduce((acc, e) => acc + e.score, 0)
    return Math.round((sum / entries.length) * 10) / 10
  }

  return (
    <div>
      <h3 className="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
        <span>⚡</span> 评分矩阵
      </h3>

      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              <th className="text-left px-3 py-2 bg-gray-50 border border-gray-200 text-gray-600 font-medium">
                方案
              </th>
              {matrix.criteria.map(c => (
                <th
                  key={c}
                  className="px-3 py-2 bg-gray-50 border border-gray-200 text-gray-600 font-medium text-center"
                >
                  {c}
                </th>
              ))}
              <th className="px-3 py-2 bg-gray-50 border border-gray-200 text-gray-600 font-medium text-center">
                总分
              </th>
            </tr>
          </thead>
          <tbody>
            {matrix.options.map(opt => {
              // Calculate total
              let total = 0
              let count = 0
              matrix.criteria.forEach(c => {
                const avg = calcAverage(opt, c)
                if (avg !== null) { total += avg; count++ }
              })

              return (
                <tr key={opt}>
                  <td className="px-3 py-2 border border-gray-200 font-medium text-gray-800">
                    {matrix.entries.find(e => e.option_id === opt)?.option_name || opt}
                  </td>
                  {matrix.criteria.map(c => {
                    const avg = calcAverage(opt, c)
                    return (
                      <td
                        key={c}
                        className="px-3 py-2 border border-gray-200 text-center text-gray-700"
                      >
                        {avg !== null ? avg : '-'}
                      </td>
                    )
                  })}
                  <td className="px-3 py-2 border border-gray-200 text-center font-semibold text-blue-600">
                    {count > 0 ? Math.round(total / count) : '-'}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

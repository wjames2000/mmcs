import type { Role } from '../../types'
import { getRoleColor } from '../../styles/colors'

interface Props {
  role: Role
  onEdit: (role: Role) => void
  onDelete: (roleId: string) => void
}

export default function RoleCard({ role, onEdit, onDelete }: Props) {
  const color = getRoleColor(role.name)
  const traits = role.traits || {}

  const renderTraitBar = (label: string, value: number, leftLabel: string, rightLabel: string) => (
    <div className="mb-2">
      <div className="flex justify-between text-xs text-gray-500 mb-0.5">
        <span>{leftLabel}</span>
        <span>{rightLabel}</span>
      </div>
      <div className="relative h-2 bg-gray-100 rounded-full overflow-hidden">
        <div
          className="absolute top-0 left-0 h-full rounded-full transition-all"
          style={{
            width: `${(value / 10) * 100}%`,
            backgroundColor: color.border,
          }}
        />
      </div>
      <span className="text-xs text-gray-400 mt-0.5 block text-center">{value}/10</span>
    </div>
  )

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5 hover:shadow-md transition-all">
      {/* Header */}
      <div className="flex items-center gap-3 mb-3">
        <span
          className="w-3 h-3 rounded-full shrink-0"
          style={{ backgroundColor: color.dot }}
        />
        <div>
          <h3 className="text-base font-semibold text-gray-900">{role.name}</h3>
          <p className="text-xs text-gray-400">{role.title}</p>
        </div>
        {role.is_global && (
          <span className="ml-auto px-2 py-0.5 bg-purple-50 text-purple-600 text-xs rounded-full">
            系统
          </span>
        )}
      </div>

      {/* Traits */}
      <div className="mb-3">
        {renderTraitBar('激进-保守', traits.激进 !== undefined ? traits.激进 : 5, '激进', '保守')}
        {renderTraitBar('乐观-悲观', traits.乐观 !== undefined ? traits.乐观 : 5, '乐观', '悲观')}
        {renderTraitBar('创意-务实', traits.创意 !== undefined ? traits.创意 : 5, '创意', '务实')}
        {renderTraitBar('细节-宏观', traits.细节 !== undefined ? traits.细节 : 5, '细节', '宏观')}
      </div>

      {/* Skills */}
      {role.skills && role.skills.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-3">
          {role.skills.map(skill => (
            <span
              key={skill}
              className="px-2 py-0.5 bg-gray-100 text-gray-600 text-xs rounded-full"
            >
              {skill}
            </span>
          ))}
        </div>
      )}

      {/* Actions */}
      {!role.is_global && (
        <div className="flex justify-end gap-2 pt-2 border-t border-gray-100">
          <button
            onClick={() => onEdit(role)}
            className="px-3 py-1 text-xs text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
          >
            编辑
          </button>
          <button
            onClick={() => onDelete(role.id)}
            className="px-3 py-1 text-xs text-red-600 hover:bg-red-50 rounded-lg transition-colors"
          >
            删除
          </button>
        </div>
      )}
    </div>
  )
}

export function RoleCardSkeleton() {
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5">
      <div className="flex items-center gap-3 mb-3">
        <div className="skeleton w-3 h-3 rounded-full" />
        <div className="skeleton h-5 w-24" />
      </div>
      <div className="skeleton h-2 w-full mb-2" />
      <div className="skeleton h-2 w-full mb-2" />
      <div className="skeleton h-2 w-3/4" />
    </div>
  )
}

import { useState, useEffect } from 'react'
import type { Role } from '../../types'
import { api } from '../../lib/api'

interface Props {
  open: boolean
  role?: Role | null // null = create, Role = edit
  onClose: () => void
  onSubmit: (data: any, roleId?: string) => Promise<void>
}

const TRAIT_CONFIG = [
  { key: '激进', left: '激进', right: '保守' },
  { key: '乐观', left: '乐观', right: '悲观' },
  { key: '创意', left: '创意', right: '务实' },
  { key: '细节', left: '细节', right: '宏观' },
]

export default function RoleFormDialog({ open, role, onClose, onSubmit }: Props) {
  const [name, setName] = useState('')
  const [title, setTitle] = useState('')
  const [traits, setTraits] = useState<Record<string, number>>({})
  const [expertiseStr, setExpertiseStr] = useState('')
  const [speakingStyle, setSpeakingStyle] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [skills, setSkills] = useState<string[]>([])
  const [availableSkills, setAvailableSkills] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return

    // Load available skills
    api.getSkills().then(skills => {
      setAvailableSkills(skills.map((s: any) => s.name || s))
    }).catch(() => {})

    if (role) {
      setName(role.name || '')
      setTitle(role.title || '')
      setTraits(role.traits || {})
      setExpertiseStr((role.expertise || []).join(', '))
      setSpeakingStyle(role.speaking_style || '')
      setSystemPrompt(role.system_prompt || '')
      setSkills(role.skills || [])
    } else {
      setName('')
      setTitle('')
      setTraits({})
      setExpertiseStr('')
      setSpeakingStyle('')
      setSystemPrompt('')
      setSkills([])
    }
    setError('')
  }, [open, role])

  if (!open) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('角色名称不能为空'); return }
    if (!title.trim()) { setError('职位头衔不能为空'); return }

    setSubmitting(true)
    setError('')
    try {
      await onSubmit({
        name: name.trim(),
        title: title.trim(),
        traits,
        expertise: expertiseStr.split(',').map(s => s.trim()).filter(Boolean),
        speaking_style: speakingStyle,
        system_prompt: systemPrompt,
        skills,
      }, role?.id)
      onClose()
    } catch (err: any) {
      setError(err.message || '保存角色失败')
    } finally {
      setSubmitting(false)
    }
  }

  const setTraitValue = (key: string, value: number) => {
    setTraits(prev => ({ ...prev, [key]: value }))
  }

  const toggleSkill = (skill: string) => {
    setSkills(prev => prev.includes(skill) ? prev.filter(s => s !== skill) : [...prev, skill])
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-gray-900 mb-4">
          {role ? '编辑角色' : '创建角色'}
        </h2>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">角色名称 *</label>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="例如：安全审查员"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">职位头衔 *</label>
              <input
                type="text"
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder="例如：安全工程师"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
          </div>

          {/* Traits with sliders */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">核心特质</label>
            <div className="space-y-3">
              {TRAIT_CONFIG.map(tc => (
                <div key={tc.key}>
                  <div className="flex justify-between text-xs text-gray-500 mb-1">
                    <span>{tc.left}</span>
                    <span className="font-medium text-gray-700">{traits[tc.key] || 5}</span>
                    <span>{tc.right}</span>
                  </div>
                  <input
                    type="range"
                    min="1"
                    max="10"
                    value={traits[tc.key] || 5}
                    onChange={e => setTraitValue(tc.key, parseInt(e.target.value))}
                    className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-blue-600"
                  />
                </div>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">专业知识领域</label>
            <input
              type="text"
              value={expertiseStr}
              onChange={e => setExpertiseStr(e.target.value)}
              placeholder="用逗号分隔，例如：网络安全, 应用安全"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">说话风格</label>
            <input
              type="text"
              value={speakingStyle}
              onChange={e => setSpeakingStyle(e.target.value)}
              placeholder="例如：专业严谨、直截了当"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
            />
          </div>

          {/* Skills */}
          {availableSkills.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Skills</label>
              <div className="flex flex-wrap gap-1.5">
                {availableSkills.map(skill => (
                  <button
                    key={skill}
                    type="button"
                    onClick={() => toggleSkill(skill)}
                    className={`px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors ${
                      skills.includes(skill)
                        ? 'bg-blue-50 border-blue-300 text-blue-700'
                        : 'bg-white border-gray-200 text-gray-500 hover:border-gray-300'
                    }`}
                  >
                    {skill}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">系统提示词</label>
            <textarea
              value={systemPrompt}
              onChange={e => setSystemPrompt(e.target.value)}
              placeholder="定义角色的行为准则和思考方式..."
              rows={4}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none resize-none font-mono"
            />
          </div>

          {error && <p className="text-sm text-red-600">{error}</p>}

          <div className="flex justify-end gap-3 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-lg">
              取消
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
            >
              {submitting ? '保存中...' : role ? '更新' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

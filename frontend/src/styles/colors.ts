/**
 * 角色色值映射表
 * 根据 08 前端设计说明书.md 中的角色色彩池
 */
export const ROLE_COLORS: Record<string, { border: string; dot: string; bg: string; light: string }> = {
  '安全审查员': { border: '#dc2626', dot: '#dc2626', bg: '#fef2f2', light: '#fee2e2' },
  '性能分析员': { border: '#16a34a', dot: '#16a34a', bg: '#f0fdf4', light: '#dcfce7' },
  '可维护性评估员': { border: '#ca8a04', dot: '#ca8a04', bg: '#fefce8', light: '#fef9c3' },
  '产品经理': { border: '#7c3aed', dot: '#7c3aed', bg: '#faf5ff', light: '#f3e8ff' },
  '技术负责人': { border: '#2563eb', dot: '#2563eb', bg: '#eff6ff', light: '#dbeafe' },
  '质疑者': { border: '#ea580c', dot: '#ea580c', bg: '#fff7ed', light: '#ffedd5' },
}

// 默认角色色值（用于未在预置列表中的角色）
const DEFAULT_COLORS = [
  { border: '#0891b2', dot: '#0891b2', bg: '#ecfeff', light: '#cffafe' }, // cyan
  { border: '#d946ef', dot: '#d946ef', bg: '#fdf4ff', light: '#fae8ff' }, // fuchsia
  { border: '#059669', dot: '#059669', bg: '#ecfdf5', light: '#d1fae5' }, // emerald
  { border: '#d97706', dot: '#d97706', bg: '#fffbeb', light: '#fef3c7' }, // amber
  { border: '#6366f1', dot: '#6366f1', bg: '#eef2ff', light: '#e0e7ff' }, // indigo
  { border: '#ec4899', dot: '#ec4899', bg: '#fdf2f8', light: '#fce7f3' }, // pink
]

const colorIndexMap = new Map<string, number>()
let nextDefaultIndex = 0

/**
 * 获取角色的色彩配置
 * @param roleName 角色名称
 * @returns 角色色值对象
 */
export function getRoleColor(roleName: string): { border: string; dot: string; bg: string; light: string } {
  // 预置角色直接返回
  if (ROLE_COLORS[roleName]) {
    return ROLE_COLORS[roleName]
  }

  // 自定义角色从默认色池中分配（缓存分配结果）
  if (!colorIndexMap.has(roleName)) {
    colorIndexMap.set(roleName, nextDefaultIndex % DEFAULT_COLORS.length)
    nextDefaultIndex++
  }

  return DEFAULT_COLORS[colorIndexMap.get(roleName)!]
}

/**
 * 获取角色名称前的圆点样式
 */
export function getRoleDotStyle(roleName: string): React.CSSProperties {
  const color = getRoleColor(roleName)
  return {
    display: 'inline-block',
    width: 8,
    height: 8,
    borderRadius: '50%',
    backgroundColor: color.dot,
    marginRight: 6,
    flexShrink: 0,
  }
}

/**
 * 获取消息气泡的左侧边框样式
 */
export function getRoleBorderStyle(roleName: string): React.CSSProperties {
  const color = getRoleColor(roleName)
  return {
    borderLeft: `4px solid ${color.border}`,
  }
}

/**
 * 获取角色背景色
 */
export function getRoleBgStyle(roleName: string): React.CSSProperties {
  const color = getRoleColor(roleName)
  return {
    backgroundColor: color.bg,
  }
}

// 人类用户消息色值
export const USER_COLOR = {
  border: '#6b7280',
  dot: '#6b7280',
  bg: '#f3f4f6',
  light: '#e5e7eb',
}

// 语义色
export const STATUS_COLORS: Record<string, string> = {
  active: '#16a34a',
  archived: '#6b7280',
  draft: '#6b7280',
  running: '#16a34a',
  paused: '#ca8a04',
  ended: '#6b7280',
  failed: '#dc2626',
  pending: '#6b7280',
  in_progress: '#2563eb',
  reviewing: '#ca8a04',
  completed: '#16a34a',
  rejected: '#dc2626',
  low: '#6b7280',
  medium: '#ca8a04',
  high: '#ea580c',
  critical: '#dc2626',
}

// 范式名称中文映射
export const PARADIGM_LABELS: Record<string, string> = {
  round_robin: '轮询',
  court: '模拟法庭',
  evaluation: '评估',
  free_chat: '自由讨论',
}

export const PARADIGM_ICONS: Record<string, string> = {
  round_robin: '🔄',
  court: '⚖️',
  evaluation: '📊',
  free_chat: '💬',
}

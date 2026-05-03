// ==================== User ====================
export interface User {
  id: string
  name: string
  email: string
  avatar_url?: string
  status: string
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  user: User
  token: string
}

export interface RegisterResponse {
  user: User
  token: string
}

// ==================== Workspace ====================
export interface Workspace {
  id: string
  name: string
  description: string
  mode: 'standalone' | 'shared'
  status: 'active' | 'archived'
  members: string[]
  creator_id: string
  created_at: string
  updated_at: string
}

// ==================== Session ====================
export interface Session {
  id: string
  workspace_id: string
  title: string
  paradigm: 'round_robin' | 'court' | 'evaluation' | 'free_chat'
  status: 'draft' | 'running' | 'paused' | 'ended' | 'failed'
  max_rounds: number
  round_timeout: number
  config: any
  creator_id: string
  created_at: string
  updated_at: string
  started_at?: string
  ended_at?: string
}

export interface SessionRole {
  id: string
  session_id: string
  role_id: string
  model_override?: any
  sort_order: number
}

// ==================== Role ====================
export interface Role {
  id: string
  name: string
  title: string
  traits: Record<string, number>
  expertise: string[]
  speaking_style: string
  system_prompt: string
  skills: string[]
  default_model?: any
  is_global: boolean
  creator_id?: string
  created_at: string
  updated_at: string
}

export interface SkillDefinition {
  name: string
  description: string
}

// ==================== Task ====================
export type TaskStatus = 'pending' | 'in_progress' | 'reviewing' | 'completed' | 'rejected'
export type TaskPriority = 'low' | 'medium' | 'high' | 'critical'

export interface Task {
  id: string
  session_id: string
  workspace_id: string
  title: string
  description: string
  acceptance_criteria: string
  status: TaskStatus
  priority: TaskPriority
  assigned_agent?: string
  assigned_by: string
  source_round: number
  validation_result?: ValidationResult
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface ValidationResult {
  id: string
  task_id: string
  validator: string
  verdict: 'passed' | 'needs_revision' | 'rejected'
  reason: string
  detail: Record<string, any>
  created_at: string
}

// ==================== SSE Messages ====================
export interface StreamMessage {
  type: 'node_start' | 'node_end' | 'message' | 'evaluation' | 'error' | 'connected' | 'role.speak' | 'role.done' | 'round.start' | 'round.eval' | 'session.paused' | 'session.resumed' | 'session.ended'
  node_name?: string
  role_name?: string
  content?: string
  metadata?: any
  timestamp: string
  error?: string
}

// ==================== Minutes ====================
export interface MeetingMinutes {
  session_id: string
  title: string
  paradigm: string
  participants: string[]
  started_at: string
  ended_at: string
  rounds: RoundRecord[]
  decisions: Decision[]
  disagreements: Disagreement[]
  score_matrix?: ScoreMatrix
  conclusion: string
}

export interface RoundRecord {
  round_number: number
  speeches: SpeechRecord[]
  eval_result?: string
}

export interface SpeechRecord {
  role_name: string
  content: string
  tokens?: number
}

export interface Decision {
  title: string
  description: string
  accepted: boolean
  rejected_by?: string[]
}

export interface Disagreement {
  topic: string
  positions: string[]
  resolved: boolean
}

export interface ScoreMatrix {
  options: string[]
  criteria: string[]
  entries: ScoreEntry[]
}

export interface ScoreEntry {
  option_id: string
  option_name: string
  criterion_name: string
  score: number
  expert_name: string
  rationale?: string
}

// ==================== Model ====================
export interface ModelProvider {
  name: string
  enabled: boolean
  base_url: string
  default_model: string
}

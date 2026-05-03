// Package task 提供任务模块核心类型定义
// 包括任务状态、优先级、任务结构和验证结果
package task

import "time"

// Status 任务状态
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusReviewing  Status = "reviewing"
	StatusCompleted  Status = "completed"
	StatusRejected   Status = "rejected"
)

// Priority 任务优先级
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Task 任务实体
// 包含完整的信息，支持状态流转和 Agent 分配
type Task struct {
	ID                 string            `json:"id"`
	SessionID          string            `json:"session_id"`
	WorkspaceID        string            `json:"workspace_id"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	Status             Status            `json:"status"`
	Priority           Priority          `json:"priority"`
	AssignedAgent      string            `json:"assigned_agent,omitempty"`
	AssignedBy         string            `json:"assigned_by"`
	SourceRound        int               `json:"source_round"`
	ValidationResult   *ValidationResult `json:"validation_result,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	CompletedAt        *time.Time        `json:"completed_at,omitempty"`
}

// ValidationResult 验证结果
// 记录验证过程中的判定、理由和详情
type ValidationResult struct {
	ID        string         `json:"id"`
	TaskID    string         `json:"task_id"`
	Validator string         `json:"validator"`
	Verdict   string         `json:"verdict"` // passed / needs_revision / rejected
	Reason    string         `json:"reason"`
	Detail    map[string]any `json:"detail"`
	CreatedAt time.Time      `json:"created_at"`
}

// ValidateTransition 校验状态转换是否合法
// 返回非 nil error 表示转换不被允许
func ValidateTransition(from, to Status) error {
	transitions := map[Status]map[Status]bool{
		StatusPending: {
			StatusInProgress: true,
		},
		StatusInProgress: {
			StatusReviewing: true,
		},
		StatusReviewing: {
			StatusCompleted:  true,
			StatusRejected:   true,
			StatusInProgress: true, // 验证退回重试
		},
		StatusRejected: {
			StatusInProgress: true,
		},
		StatusCompleted: {}, // 终止状态，不可转换
	}

	if nextStates, ok := transitions[from]; ok {
		if nextStates[to] {
			return nil
		}
	}

	return &ErrInvalidTransition{From: from, To: to}
}

// ErrInvalidTransition 非法状态转换错误
type ErrInvalidTransition struct {
	From Status
	To   Status
}

func (e *ErrInvalidTransition) Error() string {
	return "非法状态转换: " + string(e.From) + " → " + string(e.To)
}

// IsValidTransition 检查状态转换是否合法（不返回 error）
func IsValidTransition(from, to Status) bool {
	return ValidateTransition(from, to) == nil
}

// TerminalStatus 判断是否为终止状态
func TerminalStatus(s Status) bool {
	return s == StatusCompleted
}

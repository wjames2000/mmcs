package session

import "fmt"

// 会话状态常量
const (
	StatusDraft    = "draft"
	StatusRunning  = "running"
	StatusPaused   = "paused"
	StatusEnded    = "ended"
	StatusFailed   = "failed"
	StatusArchived = "archived"
)

// validTransitions 定义状态机的合法转换
// map[当前状态]允许的下一个状态集合
var validTransitions = map[string]map[string]bool{
	StatusDraft: {
		StatusRunning: true,
		StatusEnded:   true,
	},
	StatusRunning: {
		StatusPaused: true,
		StatusEnded:  true,
		StatusFailed: true,
	},
	StatusPaused: {
		StatusRunning: true,
		StatusEnded:   true,
		StatusFailed:  true,
	},
	StatusEnded: {
		StatusArchived: true,
	},
	StatusFailed: {
		StatusArchived: true,
	},
}

// ValidStatuses 返回所有合法状态
func ValidStatuses() []string {
	return []string{StatusDraft, StatusRunning, StatusPaused, StatusEnded, StatusFailed}
}

// ValidateTransition 检查状态转换是否合法
// 返回 error 如果转换非法
func ValidateTransition(current, next string) error {
	if current == next {
		return nil // 允许设置相同状态（幂等）
	}

	allowed, ok := validTransitions[current]
	if !ok {
		return fmt.Errorf("未知的当前状态: %s", current)
	}

	if !allowed[next] {
		return fmt.Errorf("非法状态转换: %s → %s", current, next)
	}

	return nil
}

// IsTerminal 检查状态是否为终态
func IsTerminal(status string) bool {
	return status == StatusEnded || status == StatusFailed
}

// CanStart 检查会话是否可以启动（进入 running 状态）
func CanStart(status string) bool {
	return status == StatusDraft || status == StatusPaused
}

// CanModify 检查会话是否可以被修改
func CanModify(status string) bool {
	return status == StatusDraft || status == StatusPaused
}

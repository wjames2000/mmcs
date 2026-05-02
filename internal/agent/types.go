// Package agent 提供 Agent 接口定义和类型系统
// 支持 ChatModelAgent、SupervisorAgent 等多种 Agent 实现
package agent

import (
	"context"
	"encoding/json"
	"time"
)

// Asynq 任务类型常量
const (
	TypeAgentTask    = "agent:execute" // 异步执行 Agent 任务
	TypeValidateTask = "agent:validate"
)

// Agent 通用 Agent 接口
// 所有 Agent 实现必须满足此接口
type Agent interface {
	// ID 返回 Agent 唯一标识
	ID() string
	// Run 执行 Agent 逻辑，接收输入字符串返回执行结果
	Run(ctx context.Context, input string) (*Result, error)
}

// Result Agent 执行结果
type Result struct {
	AgentID  string        `json:"agent_id"`
	Output   string        `json:"output"`
	Tokens   int           `json:"tokens"`
	Duration time.Duration `json:"duration_ms"`
}

// TaskPayload Asynq 异步任务载荷
// 序列化为 JSON 存入 Asynq 队列
type TaskPayload struct {
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
	Input   string `json:"input"`
}

// MarshalPayload 序列化 TaskPayload 到 JSON
func MarshalPayload(p *TaskPayload) ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalPayload 从 JSON 反序列化 TaskPayload
func UnmarshalPayload(data []byte) (*TaskPayload, error) {
	var p TaskPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

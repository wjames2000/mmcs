package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/pkg/util"
)

// Executor Agent 执行器
// 管理 Agent 注册表，提供同步和异步执行能力
// 线程安全，支持并发注册和执行
type Executor struct {
	mu     sync.RWMutex
	agents map[string]Agent
	client *asynq.Client // Asynq 客户端，异步执行时使用
}

// NewExecutor 创建 Agent 执行器
// client: Asynq 客户端（允许为 nil，此时 ExecuteAsync 返回错误）
func NewExecutor(client *asynq.Client) *Executor {
	return &Executor{
		agents: make(map[string]Agent),
		client: client,
	}
}

// Register 注册 Agent
// 如果 Agent ID 已存在则返回错误
func (e *Executor) Register(agent Agent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := agent.ID()
	if _, exists := e.agents[id]; exists {
		return fmt.Errorf("Agent %s 已注册", id)
	}

	e.agents[id] = agent
	log.Info().Str("agent_id", id).Msg("Agent 注册成功")
	return nil
}

// Get 获取已注册的 Agent
func (e *Executor) Get(id string) (Agent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	agent, ok := e.agents[id]
	if !ok {
		return nil, fmt.Errorf("Agent %s 未注册", id)
	}
	return agent, nil
}

// List 返回所有已注册的 Agent ID
func (e *Executor) List() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.agents))
	for id := range e.agents {
		ids = append(ids, id)
	}
	return ids
}

// ExecuteSync 同步执行 Agent
// 直接找到 Agent 并调用 Run 方法，返回结果
func (e *Executor) ExecuteSync(ctx context.Context, agentID, input string) (*Result, error) {
	agent, err := e.Get(agentID)
	if err != nil {
		return nil, err
	}

	return agent.Run(ctx, input)
}

// ExecuteAsync 异步执行 Agent
// 将任务投递到 Asynq 队列，返回任务 ID
func (e *Executor) ExecuteAsync(ctx context.Context, agentID, input string) (taskID string, err error) {
	if e.client == nil {
		return "", fmt.Errorf("Asynq 客户端未配置，无法异步执行")
	}

	// 检查 Agent 是否存在
	if _, err := e.Get(agentID); err != nil {
		return "", err
	}

	taskID = util.NewID("task")

	payload := &TaskPayload{
		TaskID:  taskID,
		AgentID: agentID,
		Input:   input,
	}

	data, err := MarshalPayload(payload)
	if err != nil {
		return "", fmt.Errorf("序列化任务载荷失败: %w", err)
	}

	task := asynq.NewTask(TypeAgentTask, data)
	info, err := e.client.Enqueue(task)
	if err != nil {
		return "", fmt.Errorf("投递 Asynq 任务失败: %w", err)
	}

	log.Info().
		Str("task_id", taskID).
		Str("agent_id", agentID).
		Str("asynq_id", info.ID).
		Msg("异步任务已投递")

	return taskID, nil
}

// HandleAsynqTask Asynq 任务处理器
// 反序列化 TaskPayload → 查找 Agent → 执行 → 记录结果
func (e *Executor) HandleAsynqTask(ctx context.Context, task *asynq.Task) error {
	payload, err := UnmarshalPayload(task.Payload())
	if err != nil {
		log.Error().Err(err).Msg("反序列化任务载荷失败")
		return fmt.Errorf("反序列化任务载荷失败: %w", err)
	}

	log.Info().
		Str("task_id", payload.TaskID).
		Str("agent_id", payload.AgentID).
		Msg("开始处理异步任务")

	result, err := e.ExecuteSync(ctx, payload.AgentID, payload.Input)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", payload.TaskID).
			Str("agent_id", payload.AgentID).
			Msg("异步任务执行失败")
		return fmt.Errorf("执行异步任务失败: %w", err)
	}

	log.Info().
		Str("task_id", payload.TaskID).
		Str("agent_id", payload.AgentID).
		Int("tokens", result.Tokens).
		Dur("duration", result.Duration).
		Msg("异步任务执行成功")

	return nil
}

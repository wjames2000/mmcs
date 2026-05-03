package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/agent"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/task"
	"github.com/wjames2000/mmcs/pkg/util"
)

// TaskService 任务服务接口
// ValidationService 通过此接口与任务模块解耦
type TaskService interface {
	// Get 获取任务详情
	Get(ctx context.Context, id string) (*task.Task, error)
	// UpdateStatus 更新任务状态（含状态机校验）
	UpdateStatus(ctx context.Context, id string, status task.Status) error
	// Assign 分配 Agent 到任务
	Assign(ctx context.Context, taskID, agentID, assignedBy string) error
	// Update 更新任务内容（不包含状态转换）
	Update(ctx context.Context, id string, updates map[string]interface{}) (*task.Task, error)
}

// HubRegistry SSE Hub 注册表接口
// 用于获取 Session 对应的 Hub 并广播事件
type HubRegistry interface {
	// GetOrCreate 获取或创建 Session 的 Hub
	GetOrCreate(sessionID string) *stream.Hub
}

// Service 验证服务
// 协调验证官 Agent、任务服务和 SSE 广播，完成验证闭环
type Service struct {
	validatorAgent agent.Agent
	taskService    TaskService
	hubRegistry    HubRegistry
}

// NewService 创建验证服务
// validatorAgent: 实现 agent.Agent 接口的验证官 Agent
// taskService: 实现 TaskService 接口的任务服务
// hubRegistry: 用于广播 SSE 事件的 Hub 注册表
// 任一参数为 nil 会 panic
func NewService(validatorAgent agent.Agent, taskService TaskService, hubRegistry HubRegistry) *Service {
	if validatorAgent == nil {
		panic("validation: validatorAgent 不能为 nil")
	}
	if taskService == nil {
		panic("validation: taskService 不能为 nil")
	}
	if hubRegistry == nil {
		panic("validation: hubRegistry 不能为 nil")
	}
	return &Service{
		validatorAgent: validatorAgent,
		taskService:    taskService,
		hubRegistry:    hubRegistry,
	}
}

// ValidateTask 执行同步验证
// 1. 获取任务详情（含 acceptance_criteria）
// 2. 构造验证输入并调用 ValidatorAgent
// 3. 解析 AI 返回的判定结果
// 4. 创建 ValidationResult 并关联到 Task
// 5. 根据 verdict 更新 Task.Status
// 6. 通过 hub 广播 task.updated SSE 事件
// 7. 如果 verdict=needs_revision，触发退回重试
//
// 只有 StatusInProgress 和 StatusReviewing 状态的任务允许验证
func (s *Service) ValidateTask(ctx context.Context, taskID string) (*task.ValidationResult, error) {
	// 1. 获取任务详情
	t, err := s.taskService.Get(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 验证状态：只有 in_progress 或 reviewing 可以验证
	if t.Status != task.StatusInProgress && t.Status != task.StatusReviewing {
		return nil, fmt.Errorf("任务 %s 状态不正确（当前: %s），只有 in_progress 或 reviewing 状态的任务可以验证", taskID, t.Status)
	}

	// 2. 构造验证输入并调用 Agent
	executionResult := ""
	if t.ValidationResult != nil {
		executionResult = t.ValidationResult.Reason
	}
	input := BuildValidationInput(t.Title, t.Description, t.AcceptanceCriteria, executionResult)

	agentResult, err := s.validatorAgent.Run(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("验证 Agent 执行失败: %w", err)
	}

	// 3. 解析 AI 返回的判定结果
	verdict, reason, detail := parseVerdict(agentResult.Output)

	// 4. 创建 ValidationResult
	now := time.Now()
	validationResult := &task.ValidationResult{
		ID:        util.NewID("vr"),
		TaskID:    taskID,
		Validator: s.validatorAgent.ID(),
		Verdict:   verdict,
		Reason:    reason,
		Detail:    detail,
		CreatedAt: now,
	}

	// 5. 根据 verdict 更新 Task.Status
	switch verdict {
	case "passed":
		if err := s.taskService.UpdateStatus(ctx, taskID, task.StatusCompleted); err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("更新任务状态为 completed 失败")
		}
	case "needs_revision":
		if err := s.taskService.UpdateStatus(ctx, taskID, task.StatusReviewing); err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("更新任务状态为 reviewing 失败")
		}
	case "rejected":
		if err := s.taskService.UpdateStatus(ctx, taskID, task.StatusRejected); err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("更新任务状态为 rejected 失败")
		}
	}

	// 6. 广播 SSE 事件
	s.broadcastTaskUpdated(ctx, t.SessionID, taskID, validationResult)

	// 7. 如果 verdict=needs_revision，触发退回重试
	if verdict == "needs_revision" {
		// 重新获取任务（状态已更新）
		updatedTask, err := s.taskService.Get(ctx, taskID)
		if err == nil {
			if retryErr := s.HandleRetry(ctx, updatedTask, validationResult); retryErr != nil {
				log.Error().Err(retryErr).Str("task_id", taskID).Msg("退回重试失败")
			}
		}
	}

	log.Info().
		Str("task_id", taskID).
		Str("verdict", verdict).
		Str("validator", s.validatorAgent.ID()).
		Msg("任务验证完成")

	return validationResult, nil
}

// ValidateTaskAsync 异步执行验证
// 在 goroutine 中执行 ValidateTask，HTTP handler 可立即返回
func (s *Service) ValidateTaskAsync(ctx context.Context, taskID string) error {
	// 先校验任务是否存在
	_, err := s.taskService.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	go func() {
		asyncCtx := context.Background()
		if _, err := s.ValidateTask(asyncCtx, taskID); err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("异步验证失败")
		}
	}()

	return nil
}

// HandleRetry 处理退回重试
// 当验证 verdict = needs_revision 时：
// 1. 将 reason 附加到 task.Description（追加 "修订意见: ..."）
// 2. 调用 UpdateStatus 将状态设为 in_progress
// 3. 如果原分配的 Agent 仍然可用，重新分配给它；否则调用后续逻辑
//
// verdict = rejected 时不触发退回重试，返回错误提示
// 终止状态（completed）的任务不可退回重试
func (s *Service) HandleRetry(ctx context.Context, t *task.Task, result *task.ValidationResult) error {
	if result.Verdict == "rejected" {
		return fmt.Errorf("任务 %s 已被拒绝，不执行退回重试，等待人工干预", t.ID)
	}

	if result.Verdict != "needs_revision" {
		return fmt.Errorf("任务 %s verdict 为 %s，不执行退回重试", t.ID, result.Verdict)
	}

	if task.TerminalStatus(t.Status) {
		return fmt.Errorf("任务 %s 处于终止状态 %s，不可退回重试", t.ID, t.Status)
	}

	// 1. 将 reason 附加到 task.Description（使用 task.PrepareRetry）
	_, err := task.PrepareRetry(t, result)
	if err != nil {
		return fmt.Errorf("准备退回重试失败: %w", err)
	}
	_, err = s.taskService.Update(ctx, t.ID, map[string]interface{}{
		"description": t.Description,
	})
	if err != nil {
		return fmt.Errorf("追加修订意见失败: %w", err)
	}

	// 2. 将状态设为 in_progress
	if err := s.taskService.UpdateStatus(ctx, t.ID, task.StatusInProgress); err != nil {
		return fmt.Errorf("退回重试状态更新失败: %w", err)
	}

	// 3. 如果原分配的 Agent 仍然可用，重新分配给它
	if t.AssignedAgent != "" {
		if err := s.taskService.Assign(ctx, t.ID, t.AssignedAgent, "validation_retry"); err != nil {
			log.Warn().Err(err).Str("task_id", t.ID).Str("agent_id", t.AssignedAgent).Msg("退回重试分配原 Agent 失败，交由后续处理")
		}
	}

	log.Info().
		Str("task_id", t.ID).
		Str("agent_id", t.AssignedAgent).
		Str("reason", result.Reason).
		Msg("任务退回重试成功")

	return nil
}

// broadcastTaskUpdated 广播任务更新 SSE 事件
func (s *Service) broadcastTaskUpdated(ctx context.Context, sessionID, taskID string, result *task.ValidationResult) {
	if sessionID == "" {
		return
	}

	hub := s.hubRegistry.GetOrCreate(sessionID)
	hub.Broadcast(&stream.Event{
		Type: stream.EventTypeTaskUpdated,
		Data: map[string]interface{}{
			"task_id":           taskID,
			"validation_result": result,
		},
	})
}

// parsedVerdict AI 判定解析结果
type parsedVerdict struct {
	Verdict      string                   `json:"verdict"`
	Reason       string                   `json:"reason"`
	CheckedItems []map[string]interface{} `json:"checked_items"`
}

// parseVerdict 解析 AI 返回的 JSON 判定结果
// 解析失败时默认返回 "needs_revision" 和原始输出作为 reason
func parseVerdict(output string) (verdict, reason string, detail map[string]any) {
	detail = make(map[string]any)

	var parsed parsedVerdict
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		// 解析失败，返回原始输出
		return "needs_revision", output, detail
	}

	// 验证 verdict 合法性
	switch parsed.Verdict {
	case "passed", "needs_revision", "rejected":
		verdict = parsed.Verdict
	default:
		verdict = "needs_revision"
	}

	reason = parsed.Reason
	if reason == "" {
		reason = "无详细理由"
	}

	if parsed.CheckedItems != nil {
		detail["checked_items"] = parsed.CheckedItems
	}

	return
}

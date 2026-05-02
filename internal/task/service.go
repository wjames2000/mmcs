package task

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcs/pkg/util"
)

// Service 任务服务
// 提供任务 CRUD、状态流转、Agent 分配等业务逻辑
type Service struct {
	store Store
}

// NewService 创建任务服务
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateRequest 创建任务请求
type CreateRequest struct {
	SessionID          string   `json:"session_id"`
	WorkspaceID        string   `json:"workspace_id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptance_criteria"`
	Priority           Priority `json:"priority"`
	AssignedAgent      string   `json:"assigned_agent,omitempty"`
	AssignedBy         string   `json:"assigned_by"`
	SourceRound        int      `json:"source_round"`
}

// Create 创建任务
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*Task, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("任务标题不能为空")
	}
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("工作区 ID 不能为空")
	}
	if req.Priority == "" {
		req.Priority = PriorityMedium
	}

	now := time.Now()
	task := &Task{
		ID:                 util.NewID("t"),
		SessionID:          req.SessionID,
		WorkspaceID:        req.WorkspaceID,
		Title:              req.Title,
		Description:        req.Description,
		AcceptanceCriteria: req.AcceptanceCriteria,
		Status:             StatusPending,
		Priority:           req.Priority,
		AssignedAgent:      req.AssignedAgent,
		AssignedBy:         req.AssignedBy,
		SourceRound:        req.SourceRound,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.store.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	return task, nil
}

// Get 获取任务
func (s *Service) Get(ctx context.Context, id string) (*Task, error) {
	return s.store.Get(ctx, id)
}

// UpdateStatus 更新任务状态（含状态机校验）
// completed 为终止状态，不可转换
// 合法转换：
//
//	pending → in_progress
//	in_progress → reviewing
//	reviewing → completed | rejected
//	rejected → in_progress
func (s *Service) UpdateStatus(ctx context.Context, id string, newStatus Status) error {
	task, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(task.Status, newStatus); err != nil {
		return err
	}

	task.Status = newStatus
	task.UpdatedAt = time.Now()

	if newStatus == StatusCompleted {
		now := time.Now()
		task.CompletedAt = &now
	}

	return s.store.Update(ctx, task)
}

// Update 更新任务内容（不包含状态转换）
func (s *Service) Update(ctx context.Context, id string, updates map[string]interface{}) (*Task, error) {
	task, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// 仅允许更新部分字段
	if title, ok := updates["title"].(string); ok && title != "" {
		task.Title = title
	}
	if desc, ok := updates["description"].(string); ok {
		task.Description = desc
	}
	if criteria, ok := updates["acceptance_criteria"].(string); ok {
		task.AcceptanceCriteria = criteria
	}
	if priority, ok := updates["priority"].(string); ok {
		task.Priority = Priority(priority)
	}
	if agent, ok := updates["assigned_agent"].(string); ok {
		task.AssignedAgent = agent
	}

	task.UpdatedAt = time.Now()

	if err := s.store.Update(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// Assign 分配 Agent
func (s *Service) Assign(ctx context.Context, taskID, agentID, assignedBy string) error {
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	task.AssignedAgent = agentID
	task.AssignedBy = assignedBy
	task.UpdatedAt = time.Now()

	// 如果当前是 pending 状态，自动转入 in_progress
	if task.Status == StatusPending {
		if err := ValidateTransition(task.Status, StatusInProgress); err != nil {
			return err
		}
		task.Status = StatusInProgress
	}

	return s.store.Update(ctx, task)
}

// AddValidationResult 添加验证结果并自动更新状态
func (s *Service) AddValidationResult(ctx context.Context, taskID string, result *ValidationResult) error {
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	task.ValidationResult = result
	task.UpdatedAt = time.Now()

	// 根据验证结果自动转换状态
	switch result.Verdict {
	case "passed":
		if err := ValidateTransition(task.Status, StatusCompleted); err == nil {
			task.Status = StatusCompleted
			now := time.Now()
			task.CompletedAt = &now
		}
	case "needs_revision", "rejected":
		if err := ValidateTransition(task.Status, StatusRejected); err == nil {
			task.Status = StatusRejected
		}
	}

	return s.store.Update(ctx, task)
}

// ListByWorkspace 按工作区列出任务
func (s *Service) ListByWorkspace(ctx context.Context, workspaceID string) ([]*Task, error) {
	return s.store.ListByWorkspace(ctx, workspaceID)
}

// ListBySession 按会话列出任务
func (s *Service) ListBySession(ctx context.Context, sessionID string) ([]*Task, error) {
	return s.store.ListBySession(ctx, sessionID)
}

// ListByStatus 按状态列出任务
func (s *Service) ListByStatus(ctx context.Context, status Status) ([]*Task, error) {
	return s.store.ListByStatus(ctx, status)
}

// ListAll 列出所有任务
func (s *Service) ListAll(ctx context.Context) ([]*Task, error) {
	return s.store.ListAll(ctx)
}

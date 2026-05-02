package task

import (
	"context"
	"fmt"
	"sync"
)

// Store 任务存储接口
// Phase 4 使用内存实现，后续可替换为数据库实现
type Store interface {
	Create(ctx context.Context, task *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	Update(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id string) error
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*Task, error)
	ListBySession(ctx context.Context, sessionID string) ([]*Task, error)
	ListByStatus(ctx context.Context, status Status) ([]*Task, error)
	ListAll(ctx context.Context) ([]*Task, error)
}

// MemoryStore 内存任务存储
// 线程安全，使用 sync.RWMutex 保护并发访问
type MemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewMemoryStore 创建内存任务存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks: make(map[string]*Task),
	}
}

// Create 创建任务
func (s *MemoryStore) Create(ctx context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("任务 %s 已存在", task.ID)
	}

	// 存储副本，避免外部修改
	taskCopy := *task
	s.tasks[task.ID] = &taskCopy
	return nil
}

// Get 获取任务
func (s *MemoryStore) Get(ctx context.Context, id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("任务 %s 不存在", id)
	}

	// 返回副本
	taskCopy := *task
	return &taskCopy, nil
}

// Update 更新任务
func (s *MemoryStore) Update(ctx context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("任务 %s 不存在", task.ID)
	}

	taskCopy := *task
	s.tasks[task.ID] = &taskCopy
	return nil
}

// Delete 删除任务
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("任务 %s 不存在", id)
	}

	delete(s.tasks, id)
	return nil
}

// ListByWorkspace 按工作区列出任务
func (s *MemoryStore) ListByWorkspace(ctx context.Context, workspaceID string) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.WorkspaceID == workspaceID {
			taskCopy := *t
			result = append(result, &taskCopy)
		}
	}
	if result == nil {
		result = []*Task{}
	}
	return result, nil
}

// ListBySession 按会话列出任务
func (s *MemoryStore) ListBySession(ctx context.Context, sessionID string) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.SessionID == sessionID {
			taskCopy := *t
			result = append(result, &taskCopy)
		}
	}
	if result == nil {
		result = []*Task{}
	}
	return result, nil
}

// ListByStatus 按状态列出任务
func (s *MemoryStore) ListByStatus(ctx context.Context, status Status) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.Status == status {
			taskCopy := *t
			result = append(result, &taskCopy)
		}
	}
	if result == nil {
		result = []*Task{}
	}
	return result, nil
}

// ListAll 列出所有任务
func (s *MemoryStore) ListAll(ctx context.Context) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		taskCopy := *t
		result = append(result, &taskCopy)
	}
	return result, nil
}

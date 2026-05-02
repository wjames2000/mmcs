package session

import (
	"fmt"
	"sync"
)

// GraphInstance 抽象图实例
// 对应一次讨论会话的运行实例
type GraphInstance struct {
	SessionID string
	Cancel    func() // 用于取消运行的 context
}

// GraphPool 活跃会话 Graph 实例池
// 线程安全，管理所有正在运行中的会话实例
type GraphPool struct {
	mu   sync.RWMutex
	pool map[string]*GraphInstance
}

// NewGraphPool 创建图实例池
func NewGraphPool(capacity int) *GraphPool {
	return &GraphPool{
		pool: make(map[string]*GraphInstance, capacity),
	}
}

// Add 添加运行中的实例到池中
func (gp *GraphPool) Add(instance *GraphInstance) error {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	if _, exists := gp.pool[instance.SessionID]; exists {
		return fmt.Errorf("会话 %s 已在运行中", instance.SessionID)
	}

	gp.pool[instance.SessionID] = instance
	return nil
}

// Get 获取实例
func (gp *GraphPool) Get(sessionID string) (*GraphInstance, bool) {
	gp.mu.RLock()
	defer gp.mu.RUnlock()

	inst, ok := gp.pool[sessionID]
	return inst, ok
}

// Remove 移除并关闭实例
func (gp *GraphPool) Remove(sessionID string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	if inst, ok := gp.pool[sessionID]; ok {
		if inst.Cancel != nil {
			inst.Cancel()
		}
		delete(gp.pool, sessionID)
	}
}

// CancelAll 取消并清空所有运行中的实例
func (gp *GraphPool) CancelAll() {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	for _, inst := range gp.pool {
		if inst.Cancel != nil {
			inst.Cancel()
		}
	}
	gp.pool = make(map[string]*GraphInstance)
}

// Len 返回当前运行中的会话数
func (gp *GraphPool) Len() int {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	return len(gp.pool)
}

// ListActive 列出所有活跃的 session ID
func (gp *GraphPool) ListActive() []string {
	gp.mu.RLock()
	defer gp.mu.RUnlock()

	ids := make([]string, 0, len(gp.pool))
	for id := range gp.pool {
		ids = append(ids, id)
	}
	return ids
}

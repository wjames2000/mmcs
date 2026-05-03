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
//
// 内部使用 sync.Map 减少锁竞争，优化高并发下的 Add/Remove 性能。
// LoadOrStore 用于 Add 的原子性操作，Range 用于遍历删除。
type GraphPool struct {
	pool     sync.Map
	capacity int
}

// NewGraphPool 创建图实例池
func NewGraphPool(capacity int) *GraphPool {
	return &GraphPool{
		capacity: capacity,
	}
}

// Add 添加运行中的实例到池中
func (gp *GraphPool) Add(instance *GraphInstance) error {
	if _, loaded := gp.pool.LoadOrStore(instance.SessionID, instance); loaded {
		return fmt.Errorf("会话 %s 已在运行中", instance.SessionID)
	}
	return nil
}

// Get 获取实例
func (gp *GraphPool) Get(sessionID string) (*GraphInstance, bool) {
	v, ok := gp.pool.Load(sessionID)
	if !ok {
		return nil, false
	}
	return v.(*GraphInstance), true
}

// Remove 移除并关闭实例
func (gp *GraphPool) Remove(sessionID string) {
	if v, ok := gp.pool.Load(sessionID); ok {
		inst := v.(*GraphInstance)
		if inst.Cancel != nil {
			inst.Cancel()
		}
		gp.pool.Delete(sessionID)
	}
}

// CancelAll 取消并清空所有运行中的实例
func (gp *GraphPool) CancelAll() {
	gp.pool.Range(func(key, value any) bool {
		inst := value.(*GraphInstance)
		if inst.Cancel != nil {
			inst.Cancel()
		}
		gp.pool.Delete(key)
		return true
	})
}

// Len 返回当前运行中的会话数
func (gp *GraphPool) Len() int {
	count := 0
	gp.pool.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// ListActive 列出所有活跃的 session ID
func (gp *GraphPool) ListActive() []string {
	var ids []string
	gp.pool.Range(func(key, _ any) bool {
		ids = append(ids, key.(string))
		return true
	})
	return ids
}

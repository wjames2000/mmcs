// Package stream 提供 SSE 流式推送的 Hub 和 Graph 桥接
package stream

import (
	"fmt"
	"sync"
)

// SSE 事件类型常量
const (
	EventTypeConnected   = "connected"
	EventTypeMessage     = "message"
	EventTypeStatus      = "status"
	EventTypeError       = "error"
	EventTypeDone        = "done"
	EventTypeTaskCreated = "task.created"
	EventTypeTaskUpdated = "task.updated"
)

// Event SSE 事件
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Subscriber SSE 订阅者
type Subscriber struct {
	ID      string
	Events  chan *Event
	CloseCh chan struct{}
}

// Hub Session 级别的 SSE 广播器
// 每个 Session 对应一个 Hub，管理与多个 SSE 客户端的连接
type Hub struct {
	mu          sync.RWMutex
	sessionID   string
	subscribers map[string]*Subscriber
	doneCh      chan struct{}
}

// NewHub 创建一个 SSE Hub
func NewHub(sessionID string) *Hub {
	return &Hub{
		sessionID:   sessionID,
		subscribers: make(map[string]*Subscriber),
		doneCh:      make(chan struct{}),
	}
}

// Subscribe 添加 SSE 订阅者，返回 subscriber ID
func (h *Hub) Subscribe(sub *Subscriber) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[sub.ID] = sub
	return sub.ID
}

// Unsubscribe 移除 SSE 订阅者
func (h *Hub) Unsubscribe(subID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if sub, ok := h.subscribers[subID]; ok {
		close(sub.CloseCh)
		delete(h.subscribers, subID)
	}
}

// Broadcast 向所有订阅者广播事件
func (h *Hub) Broadcast(event *Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		select {
		case sub.Events <- event:
		default:
			// 订阅者 channel 已满，跳过（不做背压）
		}
	}
}

// BroadcastExcept 向除指定订阅者外的所有人广播
func (h *Hub) BroadcastExcept(event *Event, exceptID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for id, sub := range h.subscribers {
		if id == exceptID {
			continue
		}
		select {
		case sub.Events <- event:
		default:
		}
	}
}

// SubscriberCount 返回当前订阅者数量
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// Close 关闭 Hub，清理所有订阅者
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sub := range h.subscribers {
		close(sub.CloseCh)
		close(sub.Events)
	}
	h.subscribers = make(map[string]*Subscriber)
	close(h.doneCh)
}

// Done 返回 Hub 关闭信号
func (h *Hub) Done() <-chan struct{} {
	return h.doneCh
}

// SessionID 返回 Hub 关联的 Session ID
func (h *Hub) SessionID() string {
	return h.sessionID
}

// HubRegistry 全局 Hub 注册表
// 管理所有活跃 Session 的 Hub
type HubRegistry struct {
	mu   sync.RWMutex
	hubs map[string]*Hub
}

// NewHubRegistry 创建 Hub 注册表
func NewHubRegistry() *HubRegistry {
	return &HubRegistry{
		hubs: make(map[string]*Hub),
	}
}

// GetOrCreate 获取或创建 Session 的 Hub
func (r *HubRegistry) GetOrCreate(sessionID string) *Hub {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hub, ok := r.hubs[sessionID]; ok {
		return hub
	}

	hub := NewHub(sessionID)
	r.hubs[sessionID] = hub
	return hub
}

// Get 获取 Session 的 Hub
func (r *HubRegistry) Get(sessionID string) (*Hub, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hub, ok := r.hubs[sessionID]
	return hub, ok
}

// Remove 移除并关闭 Session 的 Hub
func (r *HubRegistry) Remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hub, ok := r.hubs[sessionID]; ok {
		hub.Close()
		delete(r.hubs, sessionID)
	}
}

// RemoveWithCheck 带确认回调的移除
// confirm 返回 true 时才移除
func (r *HubRegistry) RemoveWithCheck(sessionID string, confirm func(*Hub) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hub, ok := r.hubs[sessionID]; ok {
		if confirm(hub) {
			hub.Close()
			delete(r.hubs, sessionID)
		}
	}
}

// Cleanup 清理所有 Hub
func (r *HubRegistry) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, hub := range r.hubs {
		hub.Close()
		delete(r.hubs, id)
	}
}

// FindSubscriber 找到订阅者所属的 Session ID（用于断连处理）
func (r *HubRegistry) FindSubscriber(subID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for sessionID, hub := range r.hubs {
		hub.mu.RLock()
		_, ok := hub.subscribers[subID]
		hub.mu.RUnlock()
		if ok {
			return sessionID, true
		}
	}
	return "", false
}

// NewSubscriberID 生成唯一的订阅者 ID
func NewSubscriberID() string {
	return fmt.Sprintf("sub_%d", globalSubID.Add(1))
}

// globalSubID 全局自增订阅者 ID
var globalSubID = &atomicInt64{}

type atomicInt64 struct {
	mu sync.Mutex
	v  int64
}

func (a *atomicInt64) Add(n int64) int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.v += n
	return a.v
}

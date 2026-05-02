package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// GraphEvent Graph 运行时事件
type GraphEvent struct {
	Type      string      `json:"type"` // node_start / node_end / agent_speak / moderator_eval / error
	NodeName  string      `json:"node_name,omitempty"`
	RoleName  string      `json:"role_name,omitempty"`
	Content   string      `json:"content,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// Bridge Graph 事件流到 SSE 的连接桥
// 将 Graph 运行时产生的事件转发到 Hub
type Bridge struct {
	hub       *Hub
	eventCh   chan *GraphEvent
	done      chan struct{}
	closeOnce sync.Once
}

// NewBridge 创建 Graph → SSE 桥
func NewBridge(hub *Hub, bufferSize int) *Bridge {
	if bufferSize <= 0 {
		bufferSize = 128
	}
	return &Bridge{
		hub:     hub,
		eventCh: make(chan *GraphEvent, bufferSize),
		done:    make(chan struct{}),
	}
}

// Start 启动桥接器，开始转发事件
func (b *Bridge) Start(ctx context.Context) {
	go b.run(ctx)
}

// run 事件转发主循环
func (b *Bridge) run(ctx context.Context) {
	defer func() {
		b.closeOnce.Do(func() {
			close(b.done)
		})
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.done:
			return
		case graphEvent, ok := <-b.eventCh:
			if !ok {
				return
			}
			b.forward(graphEvent)
		}
	}
}

// forward 将 GraphEvent 转发为 SSE Event
func (b *Bridge) forward(ge *GraphEvent) {
	event := b.graphEventToSSE(ge)
	b.hub.Broadcast(event)
}

// Push 向桥推送 Graph 事件
func (b *Bridge) Push(event *GraphEvent) error {
	select {
	case b.eventCh <- event:
		return nil
	case <-time.After(time.Second):
		return fmt.Errorf("推送事件超时（channel 已满）")
	}
}

// Close 关闭桥
func (b *Bridge) Close() {
	b.closeOnce.Do(func() {
		close(b.done)
	})
}

// Done 返回桥关闭信号
func (b *Bridge) Done() <-chan struct{} {
	return b.done
}

// EventChannel 返回事件写入 channel（供 Graph 节点写入）
func (b *Bridge) EventChannel() chan<- *GraphEvent {
	return b.eventCh
}

// graphEventToSSE 将 Graph 内部事件转换为 SSE Event
func (b *Bridge) graphEventToSSE(ge *GraphEvent) *Event {
	data, _ := json.Marshal(map[string]interface{}{
		"node_name": ge.NodeName,
		"role_name": ge.RoleName,
		"content":   ge.Content,
		"metadata":  ge.Metadata,
		"timestamp": ge.Timestamp,
		"error":     ge.Error,
	})

	eventType := ge.Type
	switch ge.Type {
	case "node_start":
		eventType = "node_start"
	case "node_end":
		eventType = "node_end"
	case "agent_speak":
		eventType = "message"
	case "moderator_eval":
		eventType = "evaluation"
	case "error":
		eventType = "error"
	}

	return &Event{
		Type: eventType,
		Data: string(data),
	}
}

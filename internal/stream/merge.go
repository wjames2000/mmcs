package stream

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// StreamReader 流读取器
// 包装一个 channel 并提供读取方法
type StreamReader struct {
	ID        string
	Ch        <-chan *Message
	done      chan struct{}
	closeOnce sync.Once
}

// Message 流消息
type Message struct {
	Data      interface{} `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
	StreamID  string      `json:"stream_id"`
	Seq       int64       `json:"seq"`
}

// NewStreamReader 创建流读取器
func NewStreamReader(id string, ch <-chan *Message) *StreamReader {
	return &StreamReader{
		ID:   id,
		Ch:   ch,
		done: make(chan struct{}),
	}
}

// Close 关闭读取器
func (sr *StreamReader) Close() {
	sr.closeOnce.Do(func() {
		close(sr.done)
	})
}

// Done 返回关闭信号
func (sr *StreamReader) Done() <-chan struct{} {
	return sr.done
}

// MergedStream 合并流
// 从多个 StreamReader 中读取消息，按时间戳排序后输出到 output channel
type MergedStream struct {
	streams []*StreamReader
	output  chan *Message
	done    chan struct{}
	mu      sync.Mutex
}

// NewMergedStream 创建合并流
func NewMergedStream(streams ...*StreamReader) *MergedStream {
	return &MergedStream{
		streams: streams,
		output:  make(chan *Message, 128),
		done:    make(chan struct{}),
	}
}

// Start 启动合并流处理
// 从多个 StreamReader 中并发读取消息
// 使用 min-heap 按时间戳排序输出
func (m *MergedStream) Start(ctx context.Context) {
	go m.runMerge(ctx)
}

// Output 返回输出 channel
func (m *MergedStream) Output() <-chan *Message {
	return m.output
}

// Close 关闭合并流
func (m *MergedStream) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.done:
		return
	default:
		close(m.done)
	}
}

// Done 返回关闭信号
func (m *MergedStream) Done() <-chan struct{} {
	return m.done
}

// runMerge 合并主循环
// 使用 select 多路复用从所有 stream 中读取消息
// 先收集到 buffer，再按时间戳排序后输出
func (m *MergedStream) runMerge(ctx context.Context) {
	defer func() {
		for _, sr := range m.streams {
			sr.Close()
		}
		close(m.output)
	}()

	// 创建消息缓冲器
	buf := newMessageBuffer()
	var wg sync.WaitGroup

	// 从每个 stream 读取消息
	for _, sr := range m.streams {
		wg.Add(1)
		go func(reader *StreamReader) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-reader.Done():
					return
				case <-m.done:
					return
				case msg, ok := <-reader.Ch:
					if !ok {
						return
					}
					buf.add(msg)
				}
			}
		}(sr)
	}

	// 等待所有读取完成
	wg.Wait()

	// 按时间戳顺序输出剩余消息
	for buf.Len() > 0 {
		msg := buf.pop()
		select {
		case <-ctx.Done():
			return
		case m.output <- msg:
		}
	}
}

// messageBuffer 消息缓冲器（min-heap，按 CreatedAt 排序）
type messageBuffer struct {
	items []*Message
	mu    sync.Mutex
}

func newMessageBuffer() *messageBuffer {
	return &messageBuffer{
		items: make([]*Message, 0),
	}
}

func (b *messageBuffer) add(msg *Message) {
	b.mu.Lock()
	heap.Push(b, msg)
	b.mu.Unlock()
}

func (b *messageBuffer) pop() *Message {
	b.mu.Lock()
	if b.Len() == 0 {
		b.mu.Unlock()
		return nil
	}
	item := heap.Pop(b).(*Message)
	b.mu.Unlock()
	return item
}

// heap.Interface 实现
func (b *messageBuffer) Len() int { return len(b.items) }

func (b *messageBuffer) Less(i, j int) bool {
	return b.items[i].CreatedAt.Before(b.items[j].CreatedAt)
}

func (b *messageBuffer) Swap(i, j int) {
	b.items[i], b.items[j] = b.items[j], b.items[i]
}

func (b *messageBuffer) Push(x interface{}) {
	b.items = append(b.items, x.(*Message))
}

func (b *messageBuffer) Pop() interface{} {
	old := b.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	b.items = old[:n-1]
	return item
}

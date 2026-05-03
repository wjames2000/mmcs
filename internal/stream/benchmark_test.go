package stream

import (
	"fmt"
	"sync"
	"testing"
)

// BenchmarkHub_Broadcast 测试 SSE Hub 广播性能
// 100 个客户端同时接收广播
func BenchmarkHub_Broadcast(b *testing.B) {
	hub := NewHub("bench-session")
	b.ReportAllocs()

	// 注册 100 个订阅者
	const numSubs = 100
	subs := make([]*Subscriber, numSubs)
	for i := 0; i < numSubs; i++ {
		subs[i] = &Subscriber{
			ID:      fmt.Sprintf("sub_%d", i),
			Events:  make(chan *Event, 1024),
			CloseCh: make(chan struct{}),
		}
		hub.Subscribe(subs[i])
	}

	// 后台消费 events
	var drainWg sync.WaitGroup
	for _, sub := range subs {
		drainWg.Add(1)
		go func(s *Subscriber) {
			defer drainWg.Done()
			// drain 所有事件，不阻塞广播
			for {
				select {
				case <-s.Events:
				case <-s.CloseCh:
					return
				default:
					return
				}
			}
		}(sub)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hub.Broadcast(&Event{
			Type: "test",
			Data: "benchmark message",
		})
	}

	b.StopTimer()
	drainWg.Wait()

	// 清理
	for _, sub := range subs {
		hub.Unsubscribe(sub.ID)
	}
}

// BenchmarkHub_RegisterUnregister 测试 SSE Hub 注册/注销性能
// 1000 个客户端并发注册/注销
func BenchmarkHub_RegisterUnregister(b *testing.B) {
	hub := NewHub("bench-session")
	b.ReportAllocs()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		const numOps = 1000

		// 并发注册
		subs := make([]*Subscriber, numOps)
		for j := 0; j < numOps; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				sub := &Subscriber{
					ID:      fmt.Sprintf("sub_bench_%d_%d", i, idx),
					Events:  make(chan *Event, 1024),
					CloseCh: make(chan struct{}),
				}
				subs[idx] = sub
				hub.Subscribe(sub)
			}(j)
		}
		wg.Wait()

		// 并发注销
		for j := 0; j < numOps; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				hub.Unsubscribe(subs[idx].ID)
			}(j)
		}
		wg.Wait()
	}
}

// BenchmarkHub_ConcurrentBroadcast 测试高并发广播
// 多个 goroutine 同时广播消息
func BenchmarkHub_ConcurrentBroadcast(b *testing.B) {
	hub := NewHub("bench-session")
	b.ReportAllocs()

	// 注册 50 个订阅者
	const numSubs = 50
	subs := make([]*Subscriber, numSubs)
	for i := 0; i < numSubs; i++ {
		subs[i] = &Subscriber{
			ID:      fmt.Sprintf("sub_%d", i),
			Events:  make(chan *Event, 1024),
			CloseCh: make(chan struct{}),
		}
		hub.Subscribe(subs[i])
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		const numBroadcasters = 20

		// 消费 events 防止阻塞
		for _, sub := range subs {
			select {
			case <-sub.Events:
			default:
			}
		}

		for j := 0; j < numBroadcasters; j++ {
			wg.Add(1)
			go func(broadcasterID int) {
				defer wg.Done()
				hub.Broadcast(&Event{
					Type: "concurrent",
					Data: fmt.Sprintf("msg_%d", broadcasterID),
				})
			}(j)
		}
		wg.Wait()
	}

	b.StopTimer()

	// 清理
	for _, sub := range subs {
		hub.Unsubscribe(sub.ID)
	}
}

// BenchmarkHub_SubscriberCount 测试 SubscriberCount 性能
func BenchmarkHub_SubscriberCount(b *testing.B) {
	hub := NewHub("bench-session")
	b.ReportAllocs()

	// 注册 500 个订阅者
	for i := 0; i < 500; i++ {
		sub := &Subscriber{
			ID:      fmt.Sprintf("sub_cnt_%d", i),
			Events:  make(chan *Event, 16),
			CloseCh: make(chan struct{}),
		}
		hub.Subscribe(sub)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hub.SubscriberCount()
	}
}

// BenchmarkHub_BroadcastExcept 测试排除特定订阅者的广播
func BenchmarkHub_BroadcastExcept(b *testing.B) {
	hub := NewHub("bench-session")
	b.ReportAllocs()

	// 注册 100 个订阅者
	const numSubs = 100
	subs := make([]*Subscriber, numSubs)
	for i := 0; i < numSubs; i++ {
		subs[i] = &Subscriber{
			ID:      fmt.Sprintf("sub_except_%d", i),
			Events:  make(chan *Event, 1024),
			CloseCh: make(chan struct{}),
		}
		hub.Subscribe(subs[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.BroadcastExcept(&Event{Type: "except", Data: "test"}, subs[0].ID)
		// 消费 events 防止阻塞
		for _, sub := range subs {
			select {
			case <-sub.Events:
			default:
			}
		}
	}

	b.StopTimer()

	// 清理
	for _, sub := range subs {
		hub.Unsubscribe(sub.ID)
	}
}

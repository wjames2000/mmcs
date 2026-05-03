package stream

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== Hub 基础测试 =====

func TestNewHub(t *testing.T) {
	h := NewHub("test-session")
	assert.NotNil(t, h)
	assert.Equal(t, "test-session", h.SessionID())
	assert.Equal(t, 0, h.SubscriberCount())

	select {
	case <-h.Done():
		t.Fatal("新创建的 Hub 不应处于关闭状态")
	default:
	}
}

func TestSubscribe(t *testing.T) {
	h := NewHub("test-session")

	sub := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}

	id := h.Subscribe(sub)
	assert.Equal(t, "sub_1", id)
	assert.Equal(t, 1, h.SubscriberCount())
}

func TestSubscribe_Multiple(t *testing.T) {
	h := NewHub("test-session")

	for i := 0; i < 10; i++ {
		sub := &Subscriber{
			ID:      "sub_" + itoa(i),
			Events:  make(chan *Event, 10),
			CloseCh: make(chan struct{}),
		}
		h.Subscribe(sub)
	}

	assert.Equal(t, 10, h.SubscriberCount())
}

func TestUnsubscribe(t *testing.T) {
	h := NewHub("test-session")

	sub := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)
	h.Unsubscribe("sub_1")

	assert.Equal(t, 0, h.SubscriberCount())

	// CloseCh 应该已被关闭
	select {
	case _, ok := <-sub.CloseCh:
		assert.False(t, ok, "Unsubscribe 后 CloseCh 应已关闭")
	default:
		t.Fatal("Unsubscribe 后 CloseCh 应已关闭")
	}
}

func TestUnsubscribe_NonExistent(t *testing.T) {
	h := NewHub("test-session")
	// 不应 panic
	h.Unsubscribe("non_existent")
}

func TestBroadcast(t *testing.T) {
	h := NewHub("test-session")

	sub1 := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	sub2 := &Subscriber{
		ID:      "sub_2",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub1)
	h.Subscribe(sub2)

	event := &Event{Type: "test", Data: "hello"}
	h.Broadcast(event)

	// 两个订阅者都应收到
	select {
	case e := <-sub1.Events:
		assert.Equal(t, "test", e.Type)
		assert.Equal(t, "hello", e.Data)
	default:
		t.Fatal("sub1 未收到事件")
	}

	select {
	case e := <-sub2.Events:
		assert.Equal(t, "test", e.Type)
		assert.Equal(t, "hello", e.Data)
	default:
		t.Fatal("sub2 未收到事件")
	}
}

func TestBroadcastExcept(t *testing.T) {
	h := NewHub("test-session")

	sub1 := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	sub2 := &Subscriber{
		ID:      "sub_2",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	sub3 := &Subscriber{
		ID:      "sub_3",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub1)
	h.Subscribe(sub2)
	h.Subscribe(sub3)

	event := &Event{Type: "except", Data: "skip_sub1"}
	h.BroadcastExcept(event, "sub_1")

	// sub1 不应收到（被排除）
	select {
	case <-sub1.Events:
		t.Fatal("sub1 不应收到被排除的事件")
	default:
	}

	// sub2 和 sub3 应收到
	select {
	case e := <-sub2.Events:
		assert.Equal(t, "except", e.Type)
	default:
		t.Fatal("sub2 未收到事件")
	}

	select {
	case e := <-sub3.Events:
		assert.Equal(t, "except", e.Type)
	default:
		t.Fatal("sub3 未收到事件")
	}
}

func TestSubscriberCount(t *testing.T) {
	h := NewHub("test-session")
	assert.Equal(t, 0, h.SubscriberCount())

	sub := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)
	assert.Equal(t, 1, h.SubscriberCount())

	h.Unsubscribe("sub_1")
	assert.Equal(t, 0, h.SubscriberCount())
}

func TestClose(t *testing.T) {
	h := NewHub("test-session")

	// 添加订阅者
	sub := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)

	// 验证 Close 前 Done 未触发
	select {
	case <-h.Done():
		t.Fatal("Close 前 Done 不应触发")
	default:
	}

	h.Close()

	// Done 应触发
	select {
	case <-h.Done():
	default:
		t.Fatal("Close 后 Done 应触发")
	}

	// 订阅者 count 应为 0
	assert.Equal(t, 0, h.SubscriberCount())

	// CloseCh 应关闭
	select {
	case _, ok := <-sub.CloseCh:
		assert.False(t, ok)
	default:
		t.Fatal("Close 后 CloseCh 应已关闭")
	}

	// Events 应关闭
	select {
	case _, ok := <-sub.Events:
		assert.False(t, ok)
	default:
		t.Fatal("Close 后 Events 应已关闭")
	}
}

// TestBroadcastAfterClose 确保 Hub.Close 后 Broadcast 不会 panic
func TestBroadcastAfterClose(t *testing.T) {
	h := NewHub("test-session")

	sub := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)
	h.Close()

	// Broadcast after Close should not panic
	require.NotPanics(t, func() {
		h.Broadcast(&Event{Type: "after_close", Data: "test"})
	})
}

// TestBroadcastExceptAfterClose 确保 Hub.Close 后 BroadcastExcept 不会 panic
func TestBroadcastExceptAfterClose(t *testing.T) {
	h := NewHub("test-session")

	sub := &Subscriber{
		ID:      "sub_1",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)
	h.Close()

	require.NotPanics(t, func() {
		h.BroadcastExcept(&Event{Type: "after_close", Data: "test"}, "sub_1")
	})
}

// TestMultipleClose 验证多次 Close 不 panic
func TestMultipleClose(t *testing.T) {
	h := NewHub("test-session")
	h.Close()
	require.NotPanics(t, func() {
		h.Close()
	})
}

// TestBroadcastFullChannel 验证广播时 channel 已满不会阻塞
func TestBroadcastFullChannel(t *testing.T) {
	h := NewHub("test-session")

	// 创建 channel buffer 为 1 的订阅者
	sub := &Subscriber{
		ID:      "sub_buffer1",
		Events:  make(chan *Event, 1),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)

	// 填满 channel
	sub.Events <- &Event{Type: "full"}
	// 发送第二个事件，应被丢弃而非阻塞
	h.Broadcast(&Event{Type: "dropped", Data: "should be dropped"})

	// 验证 channel 中只有第一个事件
	assert.Equal(t, 1, len(sub.Events))
}

// ===== HubRegistry 基础测试 =====

func TestNewHubRegistry(t *testing.T) {
	r := NewHubRegistry()
	assert.NotNil(t, r)
}

func TestGetOrCreate(t *testing.T) {
	r := NewHubRegistry()

	// 第一次获取应创建新 Hub
	h1 := r.GetOrCreate("session_1")
	assert.NotNil(t, h1)
	assert.Equal(t, "session_1", h1.SessionID())

	// 再次获取应返回同一个 Hub
	h2 := r.GetOrCreate("session_1")
	assert.Equal(t, h1, h2, "GetOrCreate 应返回同一个 Hub 实例")
}

func TestGetOrCreate_MultipleSessions(t *testing.T) {
	r := NewHubRegistry()

	h1 := r.GetOrCreate("session_1")
	h2 := r.GetOrCreate("session_2")

	assert.NotEqual(t, h1, h2)
	assert.Equal(t, "session_1", h1.SessionID())
	assert.Equal(t, "session_2", h2.SessionID())
}

func TestGet(t *testing.T) {
	r := NewHubRegistry()

	// 不存在的 session
	_, ok := r.Get("non_existent")
	assert.False(t, ok)

	// 创建后获取
	r.GetOrCreate("session_1")
	h, ok := r.Get("session_1")
	assert.True(t, ok)
	assert.NotNil(t, h)
}

func TestRemove(t *testing.T) {
	r := NewHubRegistry()

	r.GetOrCreate("session_1")
	r.Remove("session_1")

	// Remove 后应无法再获取
	_, ok := r.Get("session_1")
	assert.False(t, ok)
}

func TestRemove_NonExistent(t *testing.T) {
	r := NewHubRegistry()
	// 不应 panic
	require.NotPanics(t, func() {
		r.Remove("non_existent")
	})
}

func TestRemove_ClosesHub(t *testing.T) {
	r := NewHubRegistry()

	h := r.GetOrCreate("session_1")
	r.Remove("session_1")

	select {
	case <-h.Done():
	default:
		t.Fatal("Remove 后 Hub 应被关闭")
	}
}

func TestRemoveWithCheck(t *testing.T) {
	r := NewHubRegistry()

	r.GetOrCreate("session_1")

	// confirm 返回 false，不应移除
	r.RemoveWithCheck("session_1", func(h *Hub) bool {
		return false
	})
	_, ok := r.Get("session_1")
	assert.True(t, ok, "confirm=false 时不应移除 Hub")

	// confirm 返回 true，应移除
	r.RemoveWithCheck("session_1", func(h *Hub) bool {
		return true
	})
	_, ok = r.Get("session_1")
	assert.False(t, ok, "confirm=true 时应有 Hub")
}

func TestRemoveWithCheck_NonExistent(t *testing.T) {
	r := NewHubRegistry()
	// 不应 panic
	require.NotPanics(t, func() {
		r.RemoveWithCheck("non_existent", func(h *Hub) bool {
			return true
		})
	})
}

func TestCleanup(t *testing.T) {
	r := NewHubRegistry()

	r.GetOrCreate("session_1")
	r.GetOrCreate("session_2")
	r.GetOrCreate("session_3")

	r.Cleanup()

	// 所有 Hub 都应被移除
	_, ok1 := r.Get("session_1")
	_, ok2 := r.Get("session_2")
	_, ok3 := r.Get("session_3")
	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.False(t, ok3)
}

func TestFindSubscriber(t *testing.T) {
	r := NewHubRegistry()

	h := r.GetOrCreate("session_1")
	sub := &Subscriber{
		ID:      "sub_find",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)

	sessionID, ok := r.FindSubscriber("sub_find")
	assert.True(t, ok)
	assert.Equal(t, "session_1", sessionID)
}

func TestFindSubscriber_NonExistent(t *testing.T) {
	r := NewHubRegistry()

	_, ok := r.FindSubscriber("non_existent")
	assert.False(t, ok)
}

// ===== 并发安全测试 =====

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	h := NewHub("concurrent-test")

	var wg sync.WaitGroup
	const numOps = 100

	// 并发注册/注销
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sub := &Subscriber{
				ID:      "sub_" + itoa(n),
				Events:  make(chan *Event, 10),
				CloseCh: make(chan struct{}),
			}
			h.Subscribe(sub)
			h.Unsubscribe(sub.ID)
		}(i)
	}
	wg.Wait()

	// 不应 panic，且最终 count 应为 0
	assert.Equal(t, 0, h.SubscriberCount())
}

func TestConcurrentBroadcast(t *testing.T) {
	h := NewHub("concurrent-broadcast")

	// 注册 10 个订阅者
	const numSubs = 10
	subs := make([]*Subscriber, numSubs)
	for i := 0; i < numSubs; i++ {
		subs[i] = &Subscriber{
			ID:      "sub_" + itoa(i),
			Events:  make(chan *Event, 100),
			CloseCh: make(chan struct{}),
		}
		h.Subscribe(subs[i])
	}

	// 并发广播
	var wg sync.WaitGroup
	const numBroadcasters = 50
	for i := 0; i < numBroadcasters; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h.Broadcast(&Event{
				Type: "concurrent",
				Data: n,
			})
		}(i)
	}
	wg.Wait()

	// 同时进行订阅/取消订阅操作
	for i := numBroadcasters; i < numBroadcasters+20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sub := &Subscriber{
				ID:      "sub_new_" + itoa(n),
				Events:  make(chan *Event, 10),
				CloseCh: make(chan struct{}),
			}
			h.Subscribe(sub)
			h.Unsubscribe(sub.ID)
		}(i)
	}
	wg.Wait()

	// 清理
	for _, sub := range subs {
		h.Unsubscribe(sub.ID)
	}
}

func TestConcurrentRegistryOperations(t *testing.T) {
	r := NewHubRegistry()

	var wg sync.WaitGroup
	const numOps = 100

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sessionID := "session_" + itoa(n%10)

			switch n % 5 {
			case 0:
				r.GetOrCreate(sessionID)
			case 1:
				r.Get(sessionID)
			case 2:
				r.Remove(sessionID)
			case 3:
				r.RemoveWithCheck(sessionID, func(h *Hub) bool {
					return true
				})
			case 4:
				r.FindSubscriber("sub_whatever")
			}
		}(i)
	}
	wg.Wait()
	// 不应 panic
}

func TestConcurrentCloseAndBroadcast(t *testing.T) {
	h := NewHub("close-broadcast-race")

	// 注册订阅者
	sub := &Subscriber{
		ID:      "sub_race",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)

	var wg sync.WaitGroup

	// 一个 goroutine 持续广播
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			h.Broadcast(&Event{Type: "race", Data: i})
		}
	}()

	// 一个 goroutine 同时进行 Close
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.Close()
	}()

	wg.Wait()
	// 不应 panic
}

func TestConcurrentSubscribeAndClose(t *testing.T) {
	h := NewHub("subscribe-close-race")

	var wg sync.WaitGroup

	// 持续订阅
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			sub := &Subscriber{
				ID:      "sub_" + itoa(i),
				Events:  make(chan *Event, 10),
				CloseCh: make(chan struct{}),
			}
			h.Subscribe(sub)
		}
	}()

	// 同时 Close
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.Close()
	}()

	wg.Wait()
	// 不应 panic
}

// ===== Hub Done channel 测试 =====

func TestDoneChannel(t *testing.T) {
	h := NewHub("done-test")

	select {
	case <-h.Done():
		t.Fatal("Close 前 Done 不应触发")
	case <-time.After(10 * time.Millisecond):
		// OK
	}

	h.Close()

	select {
	case <-h.Done():
		// OK
	default:
		t.Fatal("Close 后 Done 应触发")
	}
}

// ===== SessionID 测试 =====

func TestSessionID(t *testing.T) {
	h := NewHub("my-session-id")
	assert.Equal(t, "my-session-id", h.SessionID())
}

// ===== NewSubscriberID 测试 =====

func TestNewSubscriberID(t *testing.T) {
	id1 := NewSubscriberID()
	id2 := NewSubscriberID()
	assert.NotEqual(t, id1, id2, "每次生成的 ID 应不同")
	assert.Contains(t, id1, "sub_")
}

// ===== Hub 在已关闭的 subscriber channels 上操作 =====

func TestBroadcastAfterSubscriberRemoved(t *testing.T) {
	h := NewHub("removed-sub-test")

	sub := &Subscriber{
		ID:      "sub_removed",
		Events:  make(chan *Event, 10),
		CloseCh: make(chan struct{}),
	}
	h.Subscribe(sub)
	h.Unsubscribe(sub.ID)

	// 向已移除的订阅者广播不应 panic
	require.NotPanics(t, func() {
		h.Broadcast(&Event{Type: "after_remove", Data: "test"})
	})
}

// ===== 辅助函数 =====

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	return itoa(n/10) + string(digits[n%10])
}

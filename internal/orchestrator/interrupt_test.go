package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
)

func TestCheckInterrupt_NoSignal(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test")
	bridge := stream.NewBridge(hub, 10)
	ctx := context.Background()

	interrupted := CheckInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.True(t, interrupted, "无信号不应中断")
}

func TestCheckInterrupt_NilChannels(t *testing.T) {
	ctx := context.Background()
	interrupted := CheckInterrupt(ctx, nil, nil, nil)
	assert.True(t, interrupted, "nil channel 不应中断")
}

func TestCheckInterrupt_HasSignal(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test_signal")
	bridge := stream.NewBridge(hub, 10)
	bridge.Start(context.Background())

	// 订阅事件以验证 bridge 收到了 paused/resumed 事件
	sub := &stream.Subscriber{
		ID:      "test-sub",
		Events:  make(chan *stream.Event, 10),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	// 预填充中断和恢复信号
	interruptCh <- &session.InterruptSignal{NodeName: "expert_speak", Message: "请暂停"}
	resumeCh <- &session.ResumeSignal{Message: "继续"}

	ctx := context.Background()
	interrupted := CheckInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.True(t, interrupted, "收到恢复信号后应继续")

	// 验证 bridge 收到了暂停事件
	select {
	case event := <-sub.Events:
		assert.Equal(t, "session.paused", event.Type)
	case <-time.After(500 * time.Millisecond):
		t.Error("期望收到 session.paused 事件，但未收到")
	}

	// 验证 bridge 收到了恢复事件
	select {
	case event := <-sub.Events:
		assert.Equal(t, "session.resumed", event.Type)
	case <-time.After(500 * time.Millisecond):
		t.Error("期望收到 session.resumed 事件，但未收到")
	}

	hub.Unsubscribe("test-sub")
	bridge.Close()
}

func TestCheckInterrupt_ContextCancelled(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test_cancel")
	bridge := stream.NewBridge(hub, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	interrupted := CheckInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.False(t, interrupted, "context 取消后应返回 false")
}

func TestCheckInterrupt_CancelDuringInterrupt(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test_cancel_during")
	bridge := stream.NewBridge(hub, 10)
	bridge.Start(context.Background())

	// 只发送中断信号，不发送恢复信号
	interruptCh <- &session.InterruptSignal{NodeName: "test", Message: "暂停"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 在另一 goroutine 中取消 context
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// CheckInterrupt 会阻塞等待恢复信号，但 context 取消会触发
	interrupted := CheckInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.False(t, interrupted, "暂停中 context 取消后应返回 false")
}

func TestWaitForInterrupt_ResumeReceived(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test_wait_resume")
	bridge := stream.NewBridge(hub, 10)
	bridge.Start(context.Background())

	// 预填充信号
	interruptCh <- &session.InterruptSignal{NodeName: "test", Message: "请暂停"}
	resumeCh <- &session.ResumeSignal{Message: "继续"}

	ctx := context.Background()
	result := WaitForInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.True(t, result, "收到恢复信号后应返回 true")
}

func TestWaitForInterrupt_ContextCancelled(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test_wait_cancel")
	bridge := stream.NewBridge(hub, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	result := WaitForInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.False(t, result, "context 取消后应返回 false")
}

func TestWaitForInterrupt_NoSignalButCancel(t *testing.T) {
	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("test_wait_nosig")
	bridge := stream.NewBridge(hub, 10)

	ctx, cancel := context.WithCancel(context.Background())

	// 在另一 goroutine 中取消 context
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// WaitForInterrupt 会阻塞等待信号，但 context 取消会返回
	result := WaitForInterrupt(ctx, interruptCh, resumeCh, bridge)
	assert.False(t, result, "无信号且 context 取消后应返回 false")
}

func TestWaitForInterrupt_NilChannels(t *testing.T) {
	ctx := context.Background()
	result := WaitForInterrupt(ctx, nil, nil, nil)
	assert.True(t, result, "nil channel 应直接返回 true")
}

package session

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===== 状态机测试 =====

func TestValidStatuses(t *testing.T) {
	statuses := ValidStatuses()
	expected := []string{"draft", "running", "paused", "ended", "failed"}
	assert.ElementsMatch(t, expected, statuses)
}

func TestValidateTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
	}{
		{name: "draft→running", current: "draft", next: "running"},
		{name: "draft→ended", current: "draft", next: "ended"},
		{name: "running→paused", current: "running", next: "paused"},
		{name: "running→ended", current: "running", next: "ended"},
		{name: "running→failed", current: "running", next: "failed"},
		{name: "paused→running", current: "paused", next: "running"},
		{name: "paused→ended", current: "paused", next: "ended"},
		{name: "paused→failed", current: "paused", next: "failed"},
		{name: "failed→draft", current: "failed", next: "draft"},
		// 相同状态应允许（幂等）
		{name: "draft→draft", current: "draft", next: "draft"},
		{name: "running→running", current: "running", next: "running"},
		{name: "ended→ended", current: "ended", next: "ended"},
		{name: "paused→paused", current: "paused", next: "paused"},
		{name: "failed→failed", current: "failed", next: "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.current, tt.next)
			assert.NoError(t, err)
		})
	}
}

func TestValidateTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
	}{
		{name: "draft→failed", current: "draft", next: "failed"},
		{name: "draft→paused", current: "draft", next: "paused"},
		{name: "running→draft", current: "running", next: "draft"},
		{name: "paused→draft", current: "paused", next: "draft"},
		{name: "ended→draft", current: "ended", next: "draft"},
		{name: "ended→running", current: "ended", next: "running"},
		{name: "ended→paused", current: "ended", next: "paused"},
		{name: "ended→failed", current: "ended", next: "failed"},
		{name: "failed→running", current: "failed", next: "running"},
		{name: "failed→paused", current: "failed", next: "paused"},
		{name: "failed→ended", current: "failed", next: "ended"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.current, tt.next)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "非法状态转换")
		})
	}
}

func TestValidateTransition_UnknownStatus(t *testing.T) {
	err := ValidateTransition("unknown", "running")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知的当前状态")

	err = ValidateTransition("draft", "unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非法状态转换")
}

func TestIsTerminal(t *testing.T) {
	assert.True(t, IsTerminal("ended"))
	assert.True(t, IsTerminal("failed"))
	assert.False(t, IsTerminal("draft"))
	assert.False(t, IsTerminal("running"))
	assert.False(t, IsTerminal("paused"))
}

func TestCanStart(t *testing.T) {
	assert.True(t, CanStart("draft"))
	assert.True(t, CanStart("paused"))
	assert.False(t, CanStart("running"))
	assert.False(t, CanStart("ended"))
	assert.False(t, CanStart("failed"))
}

func TestCanModify(t *testing.T) {
	assert.True(t, CanModify("draft"))
	assert.True(t, CanModify("paused"))
	assert.False(t, CanModify("running"))
	assert.False(t, CanModify("ended"))
	assert.False(t, CanModify("failed"))
}

// ===== Graph Pool 测试 =====

func TestGraphPool_AddAndGet(t *testing.T) {
	pool := NewGraphPool(10)

	inst := &GraphInstance{SessionID: "s_test_001"}
	err := pool.Add(inst)
	assert.NoError(t, err)

	got, ok := pool.Get("s_test_001")
	assert.True(t, ok)
	assert.Equal(t, "s_test_001", got.SessionID)
}

func TestGraphPool_AddDuplicate(t *testing.T) {
	pool := NewGraphPool(10)

	_ = pool.Add(&GraphInstance{SessionID: "s_dup"})
	err := pool.Add(&GraphInstance{SessionID: "s_dup"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已在运行中")
}

func TestGraphPool_Remove(t *testing.T) {
	pool := NewGraphPool(10)

	cancelCalled := false
	inst := &GraphInstance{
		SessionID: "s_remove",
		Cancel:    func() { cancelCalled = true },
	}
	_ = pool.Add(inst)

	pool.Remove("s_remove")
	assert.True(t, cancelCalled)

	_, ok := pool.Get("s_remove")
	assert.False(t, ok)
}

func TestGraphPool_CancelAll(t *testing.T) {
	pool := NewGraphPool(10)

	var mu sync.Mutex
	cancelCount := 0

	for i := 0; i < 3; i++ {
		idx := i
		_ = pool.Add(&GraphInstance{
			SessionID: "s_cancel_all",
			Cancel:    func() { mu.Lock(); cancelCount++; mu.Unlock() },
		})
		_ = idx
	}

	_ = pool.Add(&GraphInstance{
		SessionID: "s_another",
		Cancel:    func() { mu.Lock(); cancelCount++; mu.Unlock() },
	})

	pool.CancelAll()
	assert.Equal(t, 0, pool.Len())
}

func TestGraphPool_ListActive(t *testing.T) {
	pool := NewGraphPool(10)

	ids := []string{"s_a", "s_b", "s_c"}
	for _, id := range ids {
		_ = pool.Add(&GraphInstance{SessionID: id})
	}

	active := pool.ListActive()
	assert.ElementsMatch(t, ids, active)
}

func TestGraphPool_Len(t *testing.T) {
	pool := NewGraphPool(10)
	assert.Equal(t, 0, pool.Len())

	_ = pool.Add(&GraphInstance{SessionID: "s_1"})
	assert.Equal(t, 1, pool.Len())

	_ = pool.Add(&GraphInstance{SessionID: "s_2"})
	assert.Equal(t, 2, pool.Len())

	pool.Remove("s_1")
	assert.Equal(t, 1, pool.Len())
}

func TestGraphPool_GetNonExistent(t *testing.T) {
	pool := NewGraphPool(10)

	_, ok := pool.Get("s_nonexistent")
	assert.False(t, ok)
}

func TestGraphPool_RemoveNonExistent(t *testing.T) {
	pool := NewGraphPool(10)

	pool.Remove("s_nonexistent")
	assert.Equal(t, 0, pool.Len())
}

func TestGraphPool_CancelOnRemove(t *testing.T) {
	pool := NewGraphPool(10)

	called := false
	_ = pool.Add(&GraphInstance{
		SessionID: "s_cancel",
		Cancel:    func() { called = true },
	})

	pool.Remove("s_cancel")
	assert.True(t, called)
}

func TestGraphPool_ConcurrentAccess(t *testing.T) {
	pool := NewGraphPool(100)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inst := &GraphInstance{SessionID: "s_concurrent"}
			_ = pool.Add(inst)
			_ = pool.Len()
			pool.ListActive()
			pool.Remove("s_concurrent")
		}(i)
	}
	wg.Wait()
	// 不 panic 即通过
}

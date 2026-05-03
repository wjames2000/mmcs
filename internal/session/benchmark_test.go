package session

import (
	"fmt"
	"sync"
	"testing"
)

// BenchmarkGraphPool_AddRemove 测试 Graph Pool 并发注册/注销性能
// 使用 1000 个 Session 并发操作
func BenchmarkGraphPool_AddRemove(b *testing.B) {
	pool := NewGraphPool(2000)
	b.ReportAllocs()

	// 预热：添加 1000 个 session
	const numSessions = 1000
	sessionIDs := make([]string, numSessions)
	for i := 0; i < numSessions; i++ {
		sessionIDs[i] = fmt.Sprintf("session_%d", i)
		_ = pool.Add(&GraphInstance{SessionID: sessionIDs[i]})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		// 并发添加和删除
		for j := 0; j < numSessions; j++ {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				pool.Remove(id)
			}(sessionIDs[j])
		}

		wg.Wait()

		// 重新添加
		for j := 0; j < numSessions; j++ {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				_ = pool.Add(&GraphInstance{SessionID: id})
			}(sessionIDs[j])
		}
		wg.Wait()
	}
}

// BenchmarkGraphPool_ConcurrentAccess 测试 Graph Pool 高并发读写
// 200 goroutine 同时进行 Get、Add、Remove 操作
func BenchmarkGraphPool_ConcurrentAccess(b *testing.B) {
	pool := NewGraphPool(2000)
	b.ReportAllocs()

	// 预热：添加 500 个 session
	const warmSessions = 500
	for i := 0; i < warmSessions; i++ {
		id := fmt.Sprintf("warm_%d", i)
		_ = pool.Add(&GraphInstance{SessionID: id})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		const numGoroutines = 200

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				// 混合操作：Get/Add/Remove/List/Len
				for op := 0; op < 10; op++ {
					id := fmt.Sprintf("op_%d_%d", goroutineID, op)

					switch op % 4 {
					case 0:
						// Get 已有 session
						_, _ = pool.Get(fmt.Sprintf("warm_%d", goroutineID%warmSessions))
					case 1:
						// Add 新 session
						_ = pool.Add(&GraphInstance{SessionID: id})
					case 2:
						// Remove session
						pool.Remove(id)
					case 3:
						// Len
						_ = pool.Len()
					}
				}
			}(g)
		}

		wg.Wait()
	}
}

// BenchmarkGraphPool_Len 测试 Len 操作的性能
func BenchmarkGraphPool_Len(b *testing.B) {
	pool := NewGraphPool(2000)
	b.ReportAllocs()

	// 预热
	for i := 0; i < 500; i++ {
		_ = pool.Add(&GraphInstance{SessionID: fmt.Sprintf("bench_%d", i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.Len()
	}
}

// BenchmarkGraphPool_ListActive 测试 ListActive 操作的性能
func BenchmarkGraphPool_ListActive(b *testing.B) {
	pool := NewGraphPool(2000)
	b.ReportAllocs()

	// 预热
	for i := 0; i < 500; i++ {
		_ = pool.Add(&GraphInstance{SessionID: fmt.Sprintf("bench_%d", i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.ListActive()
	}
}

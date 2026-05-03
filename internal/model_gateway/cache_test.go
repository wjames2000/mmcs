package model_gateway

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChatModel 模拟 ChatModel 用于测试
type mockChatModel struct {
	name string
}

func (m *mockChatModel) Generate(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{Content: "mock response"}, nil
}

func (m *mockChatModel) Stream(ctx context.Context, req *ChatRequest) (<-chan *StreamChunk, error) {
	ch := make(chan *StreamChunk, 1)
	ch <- &StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func TestNewChatModelCache(t *testing.T) {
	cache := NewChatModelCache(10)
	require.NotNil(t, cache)
	assert.Equal(t, 10, cache.maxSize)
	assert.Equal(t, 0, cache.Len())

	// 默认容量
	defaultCache := NewChatModelCache(0)
	assert.Equal(t, 1000, defaultCache.maxSize)
}

func TestChatModelCache_GetOrCreate(t *testing.T) {
	cache := NewChatModelCache(10)

	// 首次创建
	callCount := 0
	factory := func() (ChatModel, error) {
		callCount++
		return &mockChatModel{name: "model1"}, nil
	}

	model, err := cache.GetOrCreate("key1", factory)
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "mock response", func() string {
		resp, _ := model.Generate(nil, &ChatRequest{})
		return resp.Content
	}())
	assert.Equal(t, 1, callCount)

	// 第二次应使用缓存
	model2, err := cache.GetOrCreate("key1", factory)
	require.NoError(t, err)
	assert.Equal(t, model, model2) // 应该是同一个实例
	assert.Equal(t, 1, callCount)  // factory 只调用一次
}

func TestChatModelCache_GetOrCreate_FactoryError(t *testing.T) {
	cache := NewChatModelCache(10)
	expectedErr := errors.New("factory error")

	model, err := cache.GetOrCreate("error", func() (ChatModel, error) {
		return nil, expectedErr
	})
	require.Error(t, err)
	assert.Nil(t, model)
	assert.Contains(t, err.Error(), "factory error")
}

func TestChatModelCache_Get(t *testing.T) {
	cache := NewChatModelCache(10)

	// 获取不存在的 key
	model, ok := cache.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, model)

	// 先创建，再获取
	_, err := cache.GetOrCreate("exists", func() (ChatModel, error) {
		return &mockChatModel{name: "exists"}, nil
	})
	require.NoError(t, err)

	model, ok = cache.Get("exists")
	assert.True(t, ok)
	assert.NotNil(t, model)
}

func TestChatModelCache_Set(t *testing.T) {
	cache := NewChatModelCache(10)

	cache.Set("key1", &mockChatModel{name: "m1"})
	assert.Equal(t, 1, cache.Len())

	// 覆盖已有 key
	cache.Set("key1", &mockChatModel{name: "m2"})
	assert.Equal(t, 1, cache.Len())

	// 验证覆盖后的值
	model, ok := cache.Get("key1")
	assert.True(t, ok)
	resp, _ := model.Generate(nil, &ChatRequest{})
	assert.Equal(t, "mock response", resp.Content)
}

func TestChatModelCache_Delete(t *testing.T) {
	cache := NewChatModelCache(10)

	cache.Set("key1", &mockChatModel{})
	assert.Equal(t, 1, cache.Len())

	cache.Delete("key1")
	assert.Equal(t, 0, cache.Len())

	// 删除不存在的 key 不应 panic
	cache.Delete("nonexistent")
}

func TestChatModelCache_Clear(t *testing.T) {
	cache := NewChatModelCache(10)

	for i := 0; i < 10; i++ {
		cache.Set(string(rune('a'+i)), &mockChatModel{})
	}
	assert.Equal(t, 10, cache.Len())

	cache.Clear()
	assert.Equal(t, 0, cache.Len())
}

func TestChatModelCache_Eviction(t *testing.T) {
	cache := NewChatModelCache(3) // 最多 3 个条目

	// 添加 4 个条目，最旧的应被淘汰
	cache.Set("key1", &mockChatModel{})
	cache.Set("key2", &mockChatModel{})
	cache.Set("key3", &mockChatModel{})
	assert.Equal(t, 3, cache.Len())

	cache.Set("key4", &mockChatModel{})
	assert.Equal(t, 3, cache.Len())

	// key1 应被淘汰
	_, ok := cache.Get("key1")
	assert.False(t, ok)

	// key2, key3, key4 应在
	_, ok = cache.Get("key2")
	assert.True(t, ok)
	_, ok = cache.Get("key3")
	assert.True(t, ok)
	_, ok = cache.Get("key4")
	assert.True(t, ok)
}

func TestChatModelCache_LRU(t *testing.T) {
	cache := NewChatModelCache(3)

	cache.Set("key1", &mockChatModel{})
	cache.Set("key2", &mockChatModel{})
	cache.Set("key3", &mockChatModel{})

	// 访问 key1，使其成为最近使用
	_, ok := cache.Get("key1")
	assert.True(t, ok)

	// 添加 key4，应淘汰最久未使用的 key2
	cache.Set("key4", &mockChatModel{})

	_, ok = cache.Get("key2")
	assert.False(t, ok, "key2 should have been evicted (LRU)")

	// key1, key3, key4 应在
	_, ok = cache.Get("key1")
	assert.True(t, ok)
	_, ok = cache.Get("key3")
	assert.True(t, ok)
	_, ok = cache.Get("key4")
	assert.True(t, ok)
}

func TestChatModelCache_Keys(t *testing.T) {
	cache := NewChatModelCache(10)

	keys := cache.Keys()
	assert.Empty(t, keys)

	cache.Set("a", &mockChatModel{})
	cache.Set("b", &mockChatModel{})
	cache.Set("c", &mockChatModel{})

	keys = cache.Keys()
	assert.ElementsMatch(t, []string{"a", "b", "c"}, keys)
}

func TestChatModelCache_ConcurrentAccess(t *testing.T) {
	cache := NewChatModelCache(100)
	var wg sync.WaitGroup

	// 并发 GetOrCreate
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + id%26))
			_, _ = cache.GetOrCreate(key, func() (ChatModel, error) {
				return &mockChatModel{}, nil
			})
		}(i)
	}

	// 并发 Set
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('A' + id%26))
			cache.Set(key, &mockChatModel{})
		}(i)
	}

	// 并发 Get
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + id%26))
			_, _ = cache.Get(key)
		}(i)
	}

	wg.Wait()
	// 不应 panic 或 data race
}

func TestChatModelCache_HitMissCallbacks(t *testing.T) {
	cache := NewChatModelCache(10)

	var hits, misses int32
	cache.SetOnHit(func(key string) {
		atomic.AddInt32(&hits, 1)
	})
	cache.SetOnMiss(func(key string) {
		atomic.AddInt32(&misses, 1)
	})

	// 先创建
	_, err := cache.GetOrCreate("k", func() (ChatModel, error) {
		return &mockChatModel{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&misses))

	// 命中
	_, ok := cache.Get("k")
	assert.True(t, ok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	// Get 不会触发 missFunc（仅 GetOrCreate 触发）
	// 因此 misses 仍为 1
	_, ok = cache.Get("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&misses))
}

func TestChatModelCache_GetOrCreate_DoubleCheck(t *testing.T) {
	// 测试 GetOrCreate 的双重检查锁定机制
	cache := NewChatModelCache(10)
	var callCount int32
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.GetOrCreate("same_key", func() (ChatModel, error) {
				mu.Lock()
				callCount++
				mu.Unlock()
				return &mockChatModel{name: "test"}, nil
			})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	// factory 应该只被调用一次
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

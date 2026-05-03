// Package model_gateway 提供模型提供商工厂注册表
// 支持多种 AI 模型提供商，通过配置动态绑定
package model_gateway

import (
	"container/list"
	"fmt"
	"sync"
)

// cacheEntry 缓存条目
type cacheEntry struct {
	key   string
	value ChatModel
}

// ChatModelCache 提供 ChatModel 实例的 LRU 缓存
// 线程安全，支持最大容量限制和自动淘汰
// 用于缓存 ChatModel 实例，避免重复创建 Provider 连接
type ChatModelCache struct {
	mu       sync.RWMutex
	cache    map[string]*list.Element // key → list element
	lruList  *list.List               // LRU 链表（front=最近使用）
	maxSize  int
	hitFunc  func(key string)
	missFunc func(key string)
}

// NewChatModelCache 创建 ChatModel 缓存
// maxSize: 最大缓存条目数（<=0 表示不限制）
func NewChatModelCache(maxSize int) *ChatModelCache {
	if maxSize <= 0 {
		maxSize = 1000 // 默认 1000
	}
	return &ChatModelCache{
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
		maxSize: maxSize,
	}
}

// SetOnHit 设置缓存命中回调（用于记录指标）
func (c *ChatModelCache) SetOnHit(fn func(key string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hitFunc = fn
}

// SetOnMiss 设置缓存未命中回调（用于记录指标）
func (c *ChatModelCache) SetOnMiss(fn func(key string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.missFunc = fn
}

// GetOrCreate 获取或创建 ChatModel 实例
// key: 缓存键（通常为 "provider:model" 格式）
// factory: 当缓存未命中时调用，用于创建新实例
// 返回 ChatModel 实例。如果 factory 返回错误，则返回该错误。
func (c *ChatModelCache) GetOrCreate(key string, factory func() (ChatModel, error)) (ChatModel, error) {
	// 读路径：快速路径，不加写锁
	c.mu.RLock()
	if elem, ok := c.cache[key]; ok {
		c.mu.RUnlock()

		// 移到链表前端（表示最近使用）
		c.mu.Lock()
		c.lruList.MoveToFront(elem)
		c.mu.Unlock()

		if c.hitFunc != nil {
			c.hitFunc(key)
		}
		return elem.Value.(*cacheEntry).value, nil
	}
	c.mu.RUnlock()

	// 缓存未命中：加写锁处理
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if elem, ok := c.cache[key]; ok {
		c.lruList.MoveToFront(elem)
		if c.hitFunc != nil {
			c.hitFunc(key)
		}
		return elem.Value.(*cacheEntry).value, nil
	}

	if c.missFunc != nil {
		c.missFunc(key)
	}

	// 调用工厂创建实例
	model, err := factory()
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 实例失败: %w", err)
	}

	// 插入缓存
	entry := &cacheEntry{key: key, value: model}
	elem := c.lruList.PushFront(entry)
	c.cache[key] = elem

	// 淘汰最久未使用的条目
	c.evict()

	return model, nil
}

// Get 从缓存中获取 ChatModel 实例
// 如果 key 不存在，返回 nil, false
func (c *ChatModelCache) Get(key string) (ChatModel, bool) {
	c.mu.RLock()
	elem, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// 移到链表前端
	c.mu.Lock()
	c.lruList.MoveToFront(elem)
	c.mu.Unlock()

	if c.hitFunc != nil {
		c.hitFunc(key)
	}
	return elem.Value.(*cacheEntry).value, true
}

// Set 手动设置缓存条目
func (c *ChatModelCache) Set(key string, model ChatModel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新值并移到前端
	if elem, ok := c.cache[key]; ok {
		elem.Value.(*cacheEntry).value = model
		c.lruList.MoveToFront(elem)
		return
	}

	// 插入新条目
	entry := &cacheEntry{key: key, value: model}
	elem := c.lruList.PushFront(entry)
	c.cache[key] = elem

	// 淘汰最久未使用的条目
	c.evict()
}

// Delete 删除缓存中的条目
func (c *ChatModelCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.lruList.Remove(elem)
		delete(c.cache, key)
	}
}

// Len 返回当前缓存条目数
func (c *ChatModelCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Clear 清空缓存
func (c *ChatModelCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*list.Element)
	c.lruList = list.New()
}

// Keys 返回所有缓存键
func (c *ChatModelCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.cache))
	for k := range c.cache {
		keys = append(keys, k)
	}
	return keys
}

// evict 淘汰最久未使用的条目（调用者已持有写锁）
func (c *ChatModelCache) evict() {
	for c.maxSize > 0 && c.lruList.Len() > c.maxSize {
		elem := c.lruList.Back()
		if elem == nil {
			break
		}
		entry := elem.Value.(*cacheEntry)
		delete(c.cache, entry.key)
		c.lruList.Remove(elem)
	}
}

package context

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryItem 记忆条目
type MemoryItem struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	RoleID    string    `json:"role_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryStore 角色记忆存储
// 使用 map + RWMutex 实现线程安全的内存存储
// 后续可接入 pgvector 或本地 embedding 模型
type MemoryStore struct {
	mu       sync.RWMutex
	memories map[string][]*MemoryItem // key: sessionID+":"+roleID → memories
}

// NewMemoryStore 创建记忆存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		memories: make(map[string][]*MemoryItem),
	}
}

// memoryKey 生成记忆存储的 key
func memoryKey(sessionID, roleID string) string {
	return sessionID + ":" + roleID
}

// Add 添加记忆
// 简单实现：将 content 直接作为记忆存储，不使用真实 embedding
func (s *MemoryStore) Add(_ context.Context, sessionID, roleID, content string) error {
	if sessionID == "" || roleID == "" || content == "" {
		return nil // 忽略空值
	}

	item := &MemoryItem{
		ID:        generateMemoryID(),
		SessionID: sessionID,
		RoleID:    roleID,
		Content:   content,
		CreatedAt: time.Now(),
	}

	key := memoryKey(sessionID, roleID)
	s.mu.Lock()
	s.memories[key] = append(s.memories[key], item)
	s.mu.Unlock()

	return nil
}

// Retrieve 获取记忆
// 返回该 session+role 的最新 N 条记忆
// 按 CreatedAt 降序排列
func (s *MemoryStore) Retrieve(_ context.Context, sessionID, roleID string, limit int) ([]*MemoryItem, error) {
	if limit <= 0 {
		limit = 10 // 默认获取最近 10 条
	}

	key := memoryKey(sessionID, roleID)
	s.mu.RLock()
	items, ok := s.memories[key]
	s.mu.RUnlock()

	if !ok || len(items) == 0 {
		return []*MemoryItem{}, nil
	}

	// 按 CreatedAt 降序排列
	sorted := make([]*MemoryItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	// 取最新的 N 条
	if limit > len(sorted) {
		limit = len(sorted)
	}

	return sorted[:limit], nil
}

// RetrieveByRole 按角色 ID 召回记忆（跨 session）
// 返回所有 session 中该角色的最新 N 条记忆
// 按 CreatedAt 降序排列
func (s *MemoryStore) RetrieveByRole(_ context.Context, roleID string, limit int) ([]*MemoryItem, error) {
	if limit <= 0 {
		limit = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var allItems []*MemoryItem
	for _, items := range s.memories {
		for _, item := range items {
			if item.RoleID == roleID {
				allItems = append(allItems, item)
			}
		}
	}

	if len(allItems) == 0 {
		return []*MemoryItem{}, nil
	}

	// 按 CreatedAt 降序排列
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].CreatedAt.After(allItems[j].CreatedAt)
	})

	if limit > len(allItems) {
		limit = len(allItems)
	}

	return allItems[:limit], nil
}

// Clear 清除指定 session 的所有记忆
func (s *MemoryStore) Clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.memories {
		// key is sessionID:roleID, check prefix
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID && key[len(sessionID):][0] == ':' {
			delete(s.memories, key)
		}
	}
}

// ClearAll 清除所有记忆
func (s *MemoryStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memories = make(map[string][]*MemoryItem)
}

// Count 返回记忆总数
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, items := range s.memories {
		count += len(items)
	}
	return count
}

// memoryIDCounter 记忆 ID 自增计数器
var memoryIDCounter int64
var memoryIDMu sync.Mutex

func generateMemoryID() string {
	memoryIDMu.Lock()
	memoryIDCounter++
	id := memoryIDCounter
	memoryIDMu.Unlock()
	return formatMemoryID(id)
}

func formatMemoryID(id int64) string {
	return "mem_" + int64ToString(id)
}

// int64ToString 简易 int64 → string 转换
func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

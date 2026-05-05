// Package model_gateway 提供模型提供商工厂注册表
// 支持多种 AI 模型提供商，通过配置动态绑定
package model_gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// ModelProvider 模型提供商配置
type ModelProvider struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Provider     string    `json:"provider"`
	APIKey       string    `json:"api_key"`
	BaseURL      string    `json:"base_url"`
	DefaultModel string    `json:"default_model"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProviderStore 线程安全的模型提供商内存存储
type ProviderStore struct {
	mu    sync.RWMutex
	items map[string]*ModelProvider
}

// NewProviderStore 创建新的 ProviderStore
func NewProviderStore() *ProviderStore {
	return &ProviderStore{
		items: make(map[string]*ModelProvider),
	}
}

// List 返回所有模型提供商配置的副本
func (s *ProviderStore) List(_ context.Context) []*ModelProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ModelProvider, 0, len(s.items))
	for _, p := range s.items {
		// 返回副本防止外部修改
		cp := *p
		result = append(result, &cp)
	}
	return result
}

// Get 根据 ID 获取单个模型提供商配置
func (s *ProviderStore) Get(_ context.Context, id string) (*ModelProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("模型提供商配置不存在: %s", id)
	}
	cp := *p
	return &cp, nil
}

// Create 创建新的模型提供商配置
func (s *ProviderStore) Create(_ context.Context, p *ModelProvider) error {
	if p.Name == "" {
		return fmt.Errorf("提供商名称不能为空")
	}
	if p.Provider == "" {
		return fmt.Errorf("提供商类型不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查名称是否重复
	for _, existing := range s.items {
		if existing.Name == p.Name {
			return fmt.Errorf("提供商名称已存在: %s", p.Name)
		}
	}

	now := time.Now()
	entry := &ModelProvider{
		ID:           ulid.Make().String(),
		Name:         p.Name,
		Provider:     p.Provider,
		APIKey:       p.APIKey,
		BaseURL:      p.BaseURL,
		DefaultModel: p.DefaultModel,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.items[entry.ID] = entry
	return nil
}

// Update 更新已有的模型提供商配置
func (s *ProviderStore) Update(_ context.Context, p *ModelProvider) error {
	if p.ID == "" {
		return fmt.Errorf("提供商 ID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.items[p.ID]
	if !ok {
		return fmt.Errorf("模型提供商配置不存在: %s", p.ID)
	}

	// 检查名称是否与其他条目重复
	for id, other := range s.items {
		if id != p.ID && other.Name == p.Name {
			return fmt.Errorf("提供商名称已存在: %s", p.Name)
		}
	}

	if p.Name != "" {
		existing.Name = p.Name
	}
	if p.Provider != "" {
		existing.Provider = p.Provider
	}
	if p.APIKey != "" {
		existing.APIKey = p.APIKey
	}
	if p.BaseURL != "" {
		existing.BaseURL = p.BaseURL
	}
	if p.DefaultModel != "" {
		existing.DefaultModel = p.DefaultModel
	}
	existing.UpdatedAt = time.Now()

	return nil
}

// Delete 删除模型提供商配置
func (s *ProviderStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("模型提供商配置不存在: %s", id)
	}
	delete(s.items, id)
	return nil
}

// ToggleEnabled 切换模型提供商的启用/禁用状态
func (s *ProviderStore) ToggleEnabled(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.items[id]
	if !ok {
		return fmt.Errorf("模型提供商配置不存在: %s", id)
	}
	p.Enabled = !p.Enabled
	p.UpdatedAt = time.Now()
	return nil
}

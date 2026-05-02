// Package model_gateway 提供模型提供商工厂注册表
// 支持多种 AI 模型提供商，通过配置动态绑定
package model_gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/wjames2000/mmcs/config"
)

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content     string `json:"content"`
	TotalTokens int    `json:"total_tokens"`
	Model       string `json:"model"`
}

// StreamChunk 流式响应片段
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"-"`
}

// ChatModel 聊天模型接口
// 所有提供商必须实现此接口
type ChatModel interface {
	// Generate 同步生成回复
	Generate(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// Stream 流式生成回复，返回 channel 接收流式片段
	Stream(ctx context.Context, req *ChatRequest) (<-chan *StreamChunk, error)
}

// ProviderFactory 提供商工厂函数
type ProviderFactory func(cfg config.ProviderConfig) (ChatModel, error)

// Gateway 模型网关
// 管理所有注册的提供商，根据 binding 返回对应的 ChatModel
type Gateway struct {
	mu        sync.RWMutex
	factories map[string]ProviderFactory
	instances map[string]ChatModel
	configs   map[string]config.ProviderConfig
}

// NewGateway 创建模型网关
func NewGateway(cfg *config.ModelGatewayConfig) *Gateway {
	g := &Gateway{
		factories: make(map[string]ProviderFactory),
		instances: make(map[string]ChatModel),
		configs:   make(map[string]config.ProviderConfig),
	}

	// 存储配置
	for name, providerCfg := range cfg.Providers {
		if providerCfg.Enabled {
			g.configs[name] = providerCfg
		}
	}

	return g
}

// RegisterProvider 注册提供商工厂
func (g *Gateway) RegisterProvider(name string, factory ProviderFactory) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.factories[name] = factory
}

// GetChatModel 根据绑定名称获取 ChatModel
// binding 格式：provider_name 如 "openai"、"ollama"
func (g *Gateway) GetChatModel(binding string) (ChatModel, error) {
	g.mu.RLock()
	model, exists := g.instances[binding]
	g.mu.RUnlock()

	if exists {
		return model, nil
	}

	// 懒加载：首次使用时创建
	g.mu.Lock()
	defer g.mu.Unlock()

	// 双重检查
	if model, exists := g.instances[binding]; exists {
		return model, nil
	}

	cfg, ok := g.configs[binding]
	if !ok {
		return nil, fmt.Errorf("模型提供商 %s 未配置或未启用", binding)
	}

	factory, ok := g.factories[binding]
	if !ok {
		return nil, fmt.Errorf("模型提供商 %s 未注册", binding)
	}

	model, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("初始化模型提供商 %s 失败: %w", binding, err)
	}

	g.instances[binding] = model
	return model, nil
}

// ListProviders 列出所有已配置的提供商
func (g *Gateway) ListProviders() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.configs))
	for name := range g.configs {
		names = append(names, name)
	}
	return names
}

// Package context 提供上下文管理、摘要压缩和角色记忆功能
package context

import (
	"errors"
	"fmt"
	"sync"

	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// CompressionStrategy 压缩策略类型
type CompressionStrategy string

const (
	StrategySummarize     CompressionStrategy = "summarize"      // 摘要压缩
	StrategySlidingWindow CompressionStrategy = "sliding_window" // 滑动窗口
	StrategyDropOldest    CompressionStrategy = "drop_oldest"    // 丢弃最早
)

// Manager 上下文管理器
// 负责监控和压缩对话历史，确保不超过 token 限制
type Manager struct {
	maxTokens      int
	strategy       CompressionStrategy
	summarizeModel model_gateway.ChatModel // summarize 策略使用的模型
	windowSize     int                     // sliding_window 保留的轮次数
	mu             sync.RWMutex
}

// Option 上下文管理器配置选项
type Option func(*Manager)

// WithSummarizeModel 设置摘要模型
func WithSummarizeModel(model model_gateway.ChatModel) Option {
	return func(m *Manager) {
		m.summarizeModel = model
	}
}

// WithWindowSize 设置滑动窗口大小
func WithWindowSize(size int) Option {
	return func(m *Manager) {
		m.windowSize = size
	}
}

// NewManager 创建上下文管理器
// maxTokens: 最大 token 数限制
// strategy: 压缩策略（summarize / sliding_window / drop_oldest）
func NewManager(maxTokens int, strategy CompressionStrategy, opts ...Option) *Manager {
	m := &Manager{
		maxTokens:  maxTokens,
		strategy:   strategy,
		windowSize: 10, // 默认保留 10 轮
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Message 上下文消息
type Message struct {
	Role     string                 `json:"role"`               // system / user / assistant
	Content  string                 `json:"content"`            // 消息内容
	Round    int                    `json:"round"`              // 所属轮次（0 表示系统消息）
	Metadata map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// TokenCounter token 计数器接口
type TokenCounter interface {
	CountTokens(text string) int
}

// SimpleTokenCounter 简易 token 计数器（按字符数估算）
type SimpleTokenCounter struct{}

// CountTokens 估算 token 数（按 4 字符 = 1 token）
func (c *SimpleTokenCounter) CountTokens(text string) int {
	return (len([]rune(text)) + 3) / 4
}

// CountMessagesTokens 计算消息列表的总 token 数
func CountMessagesTokens(messages []*Message, counter TokenCounter) int {
	total := 0
	for _, msg := range messages {
		total += counter.CountTokens(msg.Content)
	}
	return total
}

// Compress 检查并压缩上下文
// 如果 tokenCount > maxTokens，根据 strategy 执行压缩
// 返回压缩后的消息列表和新的 token 数
func (m *Manager) Compress(history []*Message, tokenCount int) ([]*Message, int, error) {
	if tokenCount <= m.maxTokens {
		return history, tokenCount, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch m.strategy {
	case StrategySummarize:
		return m.compressBySummarize(history)
	case StrategySlidingWindow:
		return m.compressBySlidingWindow(history)
	case StrategyDropOldest:
		return m.compressByDropOldest(history)
	default:
		return nil, 0, fmt.Errorf("不支持的压缩策略: %s", m.strategy)
	}
}

// GetMaxTokens 返回最大 token 限制
func (m *Manager) GetMaxTokens() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxTokens
}

// SetMaxTokens 设置最大 token 限制
func (m *Manager) SetMaxTokens(maxTokens int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxTokens = maxTokens
}

// compressByDropOldest 丢弃最早消息策略
func (m *Manager) compressByDropOldest(history []*Message) ([]*Message, int, error) {
	if len(history) <= 1 {
		return history, 0, errors.New("历史消息太少，无法压缩")
	}

	counter := &SimpleTokenCounter{}

	// 保留最早的系统消息（如果有）
	var systemMsg []*Message
	var chatHistory []*Message
	for _, msg := range history {
		if msg.Role == "system" && len(systemMsg) == 0 {
			systemMsg = append(systemMsg, msg)
		} else {
			chatHistory = append(chatHistory, msg)
		}
	}

	// 从最早的消息开始丢弃，直到 token 数低于限制
	// 保留最新的一半
	keepCount := len(chatHistory) / 2
	if keepCount < 1 {
		keepCount = 1
	}

	kept := chatHistory[len(chatHistory)-keepCount:]

	result := append(systemMsg, kept...)
	newTokens := CountMessagesTokens(result, counter)

	return result, newTokens, nil
}

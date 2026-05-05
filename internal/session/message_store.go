// Package session 提供会话管理
package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/wjames2000/mmcs/pkg/util"
)

// ChatMessage 单条讨论消息
type ChatMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Round     int       `json:"round"`
	RoleName  string    `json:"role_name"`
	Content   string    `json:"content"`
	Tokens    int       `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
}

// MessageStoreInterface 消息存储接口（抽象，便于测试和多种实现）
type MessageStoreInterface interface {
	Add(sessionID string, round int, roleName, content string, tokens int) *ChatMessage
	ListBySession(sessionID string) ([]*ChatMessage, error)
	ClearBySession(sessionID string)
	GetByID(id string) (*ChatMessage, error)
}
type MessageStore struct {
	mu       sync.RWMutex
	messages map[string][]*ChatMessage // sessionID → messages
}

// NewMessageStore 创建消息存储
func NewMessageStore() *MessageStore {
	return &MessageStore{
		messages: make(map[string][]*ChatMessage),
	}
}

// Add 添加消息到会话
// 返回创建的 ChatMessage 指针。并发安全。
func (s *MessageStore) Add(sessionID string, round int, roleName, content string, tokens int) *ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := &ChatMessage{
		ID:        util.NewID("msg"),
		SessionID: sessionID,
		Round:     round,
		RoleName:  roleName,
		Content:   content,
		Tokens:    tokens,
		CreatedAt: time.Now(),
	}

	s.messages[sessionID] = append(s.messages[sessionID], msg)
	return msg
}

// ListBySession 获取会话的所有消息
// 返回的消息切片为拷贝，外部修改不影响内部状态。并发安全。
func (s *MessageStore) ListBySession(sessionID string) ([]*ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs, ok := s.messages[sessionID]
	if !ok {
		return []*ChatMessage{}, nil
	}

	result := make([]*ChatMessage, len(msgs))
	for i, m := range msgs {
		cp := *m
		result[i] = &cp
	}
	return result, nil
}

// ClearBySession 清除会话的所有消息
// 并发安全。
func (s *MessageStore) ClearBySession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, sessionID)
}

// GetByID 获取单条消息
// 返回消息拷贝。并发安全。
func (s *MessageStore) GetByID(id string) (*ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, msgs := range s.messages {
		for _, m := range msgs {
			if m.ID == id {
				cp := *m
				return &cp, nil
			}
		}
	}
	return nil, fmt.Errorf("消息不存在: %s", id)
}

package session

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PGMessageStore 数据库持久化的消息存储
type PGMessageStore struct {
	cache *MessageStore
	pool  *pgxpool.Pool
	mu    sync.RWMutex
}

// NewPGMessageStore 创建数据库持久化消息存储
func NewPGMessageStore(pool *pgxpool.Pool) *PGMessageStore {
	return &PGMessageStore{
		cache: NewMessageStore(),
		pool:  pool,
	}
}

// Add 添加消息（内存 + 数据库持久化）
func (s *PGMessageStore) Add(sessionID string, round int, roleName, content string, tokens int) *ChatMessage {
	msg := s.cache.Add(sessionID, round, roleName, content, tokens)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := s.pool.Exec(ctx,
			`INSERT INTO session_messages (id, session_id, round, role_name, content, tokens, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (id) DO NOTHING`,
			msg.ID, msg.SessionID, msg.Round, msg.RoleName, msg.Content, msg.Tokens, msg.CreatedAt,
		)
		if err != nil {
			return
		}
	}()

	return msg
}

// ListBySession 获取会话消息（优先缓存，回退到数据库）
func (s *PGMessageStore) ListBySession(sessionID string) ([]*ChatMessage, error) {
	msgs, err := s.cache.ListBySession(sessionID)
	if err != nil {
		return nil, err
	}
	if len(msgs) > 0 {
		return msgs, nil
	}

	return s.loadFromDB(sessionID)
}

// ClearBySession 清除缓存
func (s *PGMessageStore) ClearBySession(sessionID string) {
	s.cache.ClearBySession(sessionID)
}

// GetByID 根据 ID 获取消息
func (s *PGMessageStore) GetByID(id string) (*ChatMessage, error) {
	msg, err := s.cache.GetByID(id)
	if err == nil {
		return msg, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var m ChatMessage
	err = s.pool.QueryRow(ctx,
		`SELECT id, session_id, round, role_name, content, tokens, created_at
		 FROM session_messages WHERE id = $1`, id,
	).Scan(&m.ID, &m.SessionID, &m.Round, &m.RoleName, &m.Content, &m.Tokens, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// loadFromDB 从数据库加载会话消息，并写回缓存
func (s *PGMessageStore) loadFromDB(sessionID string) ([]*ChatMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, round, role_name, content, tokens, created_at
		 FROM session_messages WHERE session_id = $1 ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*ChatMessage
	for rows.Next() {
		m := &ChatMessage{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Round, &m.RoleName, &m.Content, &m.Tokens, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}

	if msgs == nil {
		msgs = []*ChatMessage{}
	}

	for _, m := range msgs {
		s.cache.Add(m.SessionID, m.Round, m.RoleName, m.Content, m.Tokens)
	}

	return msgs, nil
}

// RefreshFromDB 主动触发从数据库刷新某个会话的消息
func (s *PGMessageStore) RefreshFromDB(sessionID string) ([]*ChatMessage, error) {
	s.cache.ClearBySession(sessionID)
	return s.loadFromDB(sessionID)
}

// Package session 提供讨论会话的管理
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session 会话模型
type Session struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	Title           string          `json:"title"`
	Topic           *string         `json:"topic,omitempty"` // 讨论主题/背景描述
	Paradigm        string          `json:"paradigm"`
	Status          string          `json:"status"`
	MaxRounds       int             `json:"max_rounds"`
	RoundTimeout    int             `json:"round_timeout"`
	Config          json.RawMessage `json:"config"`
	CreatorID       string          `json:"creator_id"`
	ParentSessionID *string         `json:"parent_session_id,omitempty"` // 重启时指向原会话 ID
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	EndedAt         *time.Time      `json:"ended_at,omitempty"`
}

// SessionRole 会话角色绑定
type SessionRole struct {
	ID            string          `json:"id"`
	SessionID     string          `json:"session_id"`
	RoleID        string          `json:"role_id"`
	ModelOverride json.RawMessage `json:"model_override,omitempty"`
	SortOrder     int             `json:"sort_order"`
}

// Repository 会话仓储
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository 创建会话仓储
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建会话
func (r *Repository) Create(ctx context.Context, s *Session) error {
	query := `INSERT INTO sessions (id, workspace_id, title, topic, paradigm, status, max_rounds, round_timeout, config, creator_id, parent_session_id, created_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := r.pool.Exec(ctx, query,
		s.ID, s.WorkspaceID, s.Title, s.Topic, s.Paradigm, s.Status,
		s.MaxRounds, s.RoundTimeout, s.Config, s.CreatorID, s.ParentSessionID, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	return nil
}

// GetByID 根据 ID 获取会话
func (r *Repository) GetByID(ctx context.Context, id string) (*Session, error) {
	query := `SELECT id, workspace_id, title, topic, paradigm, status, max_rounds, round_timeout, config, creator_id, parent_session_id, created_at, updated_at, started_at, ended_at
	           FROM sessions WHERE id = $1`
	s := &Session{}
	var config []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.WorkspaceID, &s.Title, &s.Topic, &s.Paradigm, &s.Status,
		&s.MaxRounds, &s.RoundTimeout, &config, &s.CreatorID, &s.ParentSessionID,
		&s.CreatedAt, &s.UpdatedAt, &s.StartedAt, &s.EndedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("会话不存在")
		}
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}
	s.Config = config
	return s, nil
}

// UpdateStatus 更新会话状态
func (r *Repository) UpdateStatus(ctx context.Context, id, status string, extra ...time.Time) error {
	switch status {
	case "running":
		var startedAt time.Time
		if len(extra) > 0 {
			startedAt = extra[0]
		} else {
			startedAt = time.Now()
		}
		query := `UPDATE sessions SET status = $2, started_at = $3, updated_at = $3 WHERE id = $1`
		_, err := r.pool.Exec(ctx, query, id, status, startedAt)
		if err != nil {
			return fmt.Errorf("更新会话状态失败: %w", err)
		}
	case "ended", "failed":
		var endedAt time.Time
		if len(extra) > 0 {
			endedAt = extra[0]
		} else {
			endedAt = time.Now()
		}
		query := `UPDATE sessions SET status = $2, ended_at = $3, updated_at = $3 WHERE id = $1`
		_, err := r.pool.Exec(ctx, query, id, status, endedAt)
		if err != nil {
			return fmt.Errorf("更新会话状态失败: %w", err)
		}
	default:
		query := `UPDATE sessions SET status = $2, updated_at = NOW() WHERE id = $1`
		_, err := r.pool.Exec(ctx, query, id, status)
		if err != nil {
			return fmt.Errorf("更新会话状态失败: %w", err)
		}
	}
	return nil
}

// Update 更新会话信息
func (r *Repository) Update(ctx context.Context, s *Session) error {
	query := `UPDATE sessions SET title = $2, topic = $3, max_rounds = $4, config = $5, updated_at = NOW()
	           WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, s.ID, s.Title, s.Topic, s.MaxRounds, s.Config)
	if err != nil {
		return fmt.Errorf("更新会话失败: %w", err)
	}
	return nil
}

// ListByWorkspace 获取工作区下的会话列表
func (r *Repository) ListByWorkspace(ctx context.Context, workspaceID string) ([]*Session, error) {
	query := `SELECT id, workspace_id, title, topic, paradigm, status, max_rounds, round_timeout, config, creator_id, parent_session_id, created_at, updated_at, started_at, ended_at
	           FROM sessions WHERE workspace_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		var config []byte
		if err := rows.Scan(
			&s.ID, &s.WorkspaceID, &s.Title, &s.Topic, &s.Paradigm, &s.Status,
			&s.MaxRounds, &s.RoundTimeout, &config, &s.CreatorID, &s.ParentSessionID,
			&s.CreatedAt, &s.UpdatedAt, &s.StartedAt, &s.EndedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描会话行失败: %w", err)
		}
		s.Config = config
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []*Session{}
	}
	return sessions, nil
}

// ===== SessionRole 操作 =====

// AddSessionRole 添加会话角色绑定
func (r *Repository) AddSessionRole(ctx context.Context, sr *SessionRole) error {
	query := `INSERT INTO session_roles (id, session_id, role_id, model_override, sort_order)
	           VALUES ($1, $2, $3, $4, $5) ON CONFLICT (session_id, role_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, query, sr.ID, sr.SessionID, sr.RoleID, sr.ModelOverride, sr.SortOrder)
	if err != nil {
		return fmt.Errorf("添加会话角色失败: %w", err)
	}
	return nil
}

// GetSessionRoles 获取会话的角色绑定列表
func (r *Repository) GetSessionRoles(ctx context.Context, sessionID string) ([]*SessionRole, error) {
	query := `SELECT id, session_id, role_id, model_override, sort_order
	           FROM session_roles WHERE session_id = $1 ORDER BY sort_order`
	rows, err := r.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("查询会话角色失败: %w", err)
	}
	defer rows.Close()

	var roles []*SessionRole
	for rows.Next() {
		sr := &SessionRole{}
		if err := rows.Scan(&sr.ID, &sr.SessionID, &sr.RoleID, &sr.ModelOverride, &sr.SortOrder); err != nil {
			return nil, fmt.Errorf("扫描会话角色行失败: %w", err)
		}
		roles = append(roles, sr)
	}
	if roles == nil {
		roles = []*SessionRole{}
	}
	return roles, nil
}

// Delete 硬删除会话
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
	}
	return nil
}
func (r *Repository) RemoveSessionRole(ctx context.Context, sessionID, roleID string) error {
	query := `DELETE FROM session_roles WHERE session_id = $1 AND role_id = $2`
	_, err := r.pool.Exec(ctx, query, sessionID, roleID)
	if err != nil {
		return fmt.Errorf("移除会话角色失败: %w", err)
	}
	return nil
}

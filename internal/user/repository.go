// Package user 提供用户认证与管理的仓储和服务层
package user

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User 用户模型
type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // 不序列化
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Repository 用户仓储
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository 创建用户仓储
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建用户
func (r *Repository) Create(ctx context.Context, u *User) error {
	query := `INSERT INTO users (id, name, email, password_hash, avatar_url, status, created_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, query,
		u.ID, u.Name, u.Email, u.PasswordHash, u.AvatarURL, u.Status, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// GetByID 根据 ID 获取用户
func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, name, email, password_hash, COALESCE(avatar_url, ''), status, created_at, updated_at
	           FROM users WHERE id = $1`
	u := &User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("用户不存在: %w", err)
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return u, nil
}

// GetByEmail 根据邮箱获取用户（用于登录）
func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, name, email, password_hash, COALESCE(avatar_url, ''), status, created_at, updated_at
	           FROM users WHERE email = $1`
	u := &User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("用户不存在: %w", err)
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return u, nil
}

// Update 更新用户信息
func (r *Repository) Update(ctx context.Context, u *User) error {
	query := `UPDATE users SET name = $2, avatar_url = $3, status = $4, updated_at = $5
	           WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, u.ID, u.Name, u.AvatarURL, u.Status, time.Now())
	if err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}
	return nil
}

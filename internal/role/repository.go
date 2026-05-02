// Package role 提供角色管理与技能注册表
package role

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Role 角色模型
type Role struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Title         string          `json:"title"`
	Traits        json.RawMessage `json:"traits"`
	Expertise     []string        `json:"expertise"`
	SpeakingStyle string          `json:"speaking_style,omitempty"`
	SystemPrompt  string          `json:"system_prompt,omitempty"`
	Skills        []string        `json:"skills"`
	DefaultModel  json.RawMessage `json:"default_model,omitempty"`
	IsGlobal      bool            `json:"is_global"`
	CreatorID     string          `json:"creator_id,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Repository 角色仓储
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository 创建角色仓储
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建角色
func (r *Repository) Create(ctx context.Context, role *Role) error {
	query := `INSERT INTO roles (id, name, title, traits, expertise, speaking_style, system_prompt, skills, default_model, is_global, creator_id, created_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := r.pool.Exec(ctx, query,
		role.ID, role.Name, role.Title, role.Traits, role.Expertise,
		role.SpeakingStyle, role.SystemPrompt, role.Skills, role.DefaultModel,
		role.IsGlobal, role.CreatorID, role.CreatedAt, role.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建角色失败: %w", err)
	}
	return nil
}

// GetByID 根据 ID 获取角色
func (r *Repository) GetByID(ctx context.Context, id string) (*Role, error) {
	query := `SELECT id, name, title, traits, expertise, COALESCE(speaking_style, ''), COALESCE(system_prompt, ''), skills, default_model, is_global, COALESCE(creator_id, ''), created_at, updated_at
	           FROM roles WHERE id = $1`
	role := &Role{}
	var traits, defaultModel []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&role.ID, &role.Name, &role.Title, &traits, &role.Expertise,
		&role.SpeakingStyle, &role.SystemPrompt, &role.Skills, &defaultModel,
		&role.IsGlobal, &role.CreatorID, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("角色不存在")
		}
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}
	role.Traits = traits
	role.DefaultModel = defaultModel
	return role, nil
}

// List 获取角色列表，支持按 is_global 筛选
func (r *Repository) List(ctx context.Context, isGlobal *bool, creatorID string) ([]*Role, error) {
	var rows pgx.Rows
	var err error

	if isGlobal != nil && *isGlobal {
		// 只查全局角色
		query := `SELECT id, name, title, traits, expertise, COALESCE(speaking_style, ''), COALESCE(system_prompt, ''), skills, default_model, is_global, COALESCE(creator_id, ''), created_at, updated_at
		           FROM roles WHERE is_global = true ORDER BY name`
		rows, err = r.pool.Query(ctx, query)
	} else if creatorID != "" {
		// 查用户自己的角色 + 全局角色
		query := `SELECT id, name, title, traits, expertise, COALESCE(speaking_style, ''), COALESCE(system_prompt, ''), skills, default_model, is_global, COALESCE(creator_id, ''), created_at, updated_at
		           FROM roles WHERE is_global = true OR creator_id = $1 ORDER BY is_global DESC, name`
		rows, err = r.pool.Query(ctx, query, creatorID)
	} else {
		query := `SELECT id, name, title, traits, expertise, COALESCE(speaking_style, ''), COALESCE(system_prompt, ''), skills, default_model, is_global, COALESCE(creator_id, ''), created_at, updated_at
		           FROM roles ORDER BY is_global DESC, name`
		rows, err = r.pool.Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("查询角色列表失败: %w", err)
	}
	defer rows.Close()

	var roles []*Role
	for rows.Next() {
		role := &Role{}
		var traits, defaultModel []byte
		if err := rows.Scan(
			&role.ID, &role.Name, &role.Title, &traits, &role.Expertise,
			&role.SpeakingStyle, &role.SystemPrompt, &role.Skills, &defaultModel,
			&role.IsGlobal, &role.CreatorID, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描角色行失败: %w", err)
		}
		role.Traits = traits
		role.DefaultModel = defaultModel
		roles = append(roles, role)
	}
	if roles == nil {
		roles = []*Role{}
	}
	return roles, nil
}

// Update 更新角色
func (r *Repository) Update(ctx context.Context, role *Role) error {
	query := `UPDATE roles SET name = $2, title = $3, traits = $4, expertise = $5,
	           speaking_style = $6, system_prompt = $7, skills = $8, default_model = $9,
	           updated_at = $10
	           WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query,
		role.ID, role.Name, role.Title, role.Traits, role.Expertise,
		role.SpeakingStyle, role.SystemPrompt, role.Skills, role.DefaultModel, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("更新角色失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("角色不存在")
	}
	return nil
}

// Delete 删除角色（仅非全局角色可删除）
func (r *Repository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM roles WHERE id = $1 AND is_global = false AND creator_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("删除角色失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("角色不存在或无权限删除")
	}
	return nil
}

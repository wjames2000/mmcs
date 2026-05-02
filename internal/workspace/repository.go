// Package workspace 提供工作区的仓储和服务层
package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Workspace 工作区模型
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	Members     []string  `json:"members"`
	CreatorID   string    `json:"creator_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository 工作区仓储
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository 创建工作区仓储
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建工作区
func (r *Repository) Create(ctx context.Context, w *Workspace) error {
	query := `INSERT INTO workspaces (id, name, description, mode, status, members, creator_id, created_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.pool.Exec(ctx, query,
		w.ID, w.Name, w.Description, w.Mode, w.Status, w.Members, w.CreatorID, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建工作区失败: %w", err)
	}
	return nil
}

// GetByID 根据 ID 获取工作区
func (r *Repository) GetByID(ctx context.Context, id string) (*Workspace, error) {
	query := `SELECT id, name, COALESCE(description, ''), mode, status, members, creator_id, created_at, updated_at
	           FROM workspaces WHERE id = $1`
	w := &Workspace{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.Name, &w.Description, &w.Mode, &w.Status, &w.Members, &w.CreatorID, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("工作区不存在")
		}
		return nil, fmt.Errorf("查询工作区失败: %w", err)
	}
	return w, nil
}

// ListByUser 获取用户参与的工作区列表
// 用户是 creator 或 members 中的一员
func (r *Repository) ListByUser(ctx context.Context, userID string) ([]*Workspace, error) {
	query := `SELECT id, name, COALESCE(description, ''), mode, status, members, creator_id, created_at, updated_at
	           FROM workspaces
	           WHERE creator_id = $1 OR $1 = ANY(members)
	           ORDER BY updated_at DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询工作区列表失败: %w", err)
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		w := &Workspace{}
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Mode, &w.Status, &w.Members, &w.CreatorID, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描工作区行失败: %w", err)
		}
		workspaces = append(workspaces, w)
	}
	if workspaces == nil {
		workspaces = []*Workspace{}
	}
	return workspaces, nil
}

// Update 更新工作区
func (r *Repository) Update(ctx context.Context, w *Workspace) error {
	query := `UPDATE workspaces SET name = $2, description = $3, mode = $4, members = $6, updated_at = $7
	           WHERE id = $1 AND creator_id = $5`
	tag, err := r.pool.Exec(ctx, query, w.ID, w.Name, w.Description, w.Mode, w.CreatorID, w.Members, time.Now())
	if err != nil {
		return fmt.Errorf("更新工作区失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("工作区不存在或无权限更新")
	}
	return nil
}

// Archive 归档工作区
func (r *Repository) Archive(ctx context.Context, id, userID string) error {
	query := `UPDATE workspaces SET status = 'archived', updated_at = $3
	           WHERE id = $1 AND creator_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, userID, time.Now())
	if err != nil {
		return fmt.Errorf("归档工作区失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("工作区不存在或无权限归档")
	}
	return nil
}

package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/pkg/util"
)

// Service 工作区服务
type Service struct {
	repo *Repository
}

// NewService 创建工作区服务
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateRequest 创建工作区请求
type CreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Mode        string   `json:"mode"` // standalone / shared
	Members     []string `json:"members,omitempty"`
}

// UpdateRequest 更新工作区请求
type UpdateRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Members     []string `json:"members,omitempty"`
}

// Create 创建工作区
func (s *Service) Create(ctx context.Context, creatorID string, req *CreateRequest) (*Workspace, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("工作区名称不能为空")
	}
	if req.Mode == "" {
		req.Mode = "standalone"
	}
	if req.Mode != "standalone" && req.Mode != "shared" {
		return nil, fmt.Errorf("工作区模式无效: %s", req.Mode)
	}

	now := time.Now()
	w := &Workspace{
		ID:          util.NewID("ws"),
		Name:        req.Name,
		Description: req.Description,
		Mode:        req.Mode,
		Status:      "active",
		Members:     ensureStrings(req.Members),
		CreatorID:   creatorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("创建工作区失败: %w", err)
	}

	log.Info().Str("workspace_id", w.ID).Str("creator_id", creatorID).Msg("工作区创建成功")
	return w, nil
}

// ensureStrings 确保字符串切片不为 nil，避免数据库 NULL 问题
func ensureStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Get 获取工作区
func (s *Service) Get(ctx context.Context, id, userID string) (*Workspace, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 权限检查：只有创建者或成员才能查看
	if !isMemberOrCreator(w, userID) {
		return nil, fmt.Errorf("无权访问该工作区")
	}
	return w, nil
}

// List 获取用户的工作区列表
func (s *Service) List(ctx context.Context, userID string) ([]*Workspace, error) {
	return s.repo.ListByUser(ctx, userID)
}

// Update 更新工作区
func (s *Service) Update(ctx context.Context, id, userID string, req *UpdateRequest) (*Workspace, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w.CreatorID != userID {
		return nil, fmt.Errorf("只有创建者才能更新工作区")
	}

	if req.Name != "" {
		w.Name = req.Name
	}
	if req.Description != "" {
		w.Description = req.Description
	}
	if req.Mode != "" {
		w.Mode = req.Mode
	}
	if req.Members != nil {
		w.Members = req.Members
	}
	w.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("更新工作区失败: %w", err)
	}

	log.Info().Str("workspace_id", id).Str("user_id", userID).Msg("工作区更新成功")
	return w, nil
}

// Archive 归档工作区
func (s *Service) Archive(ctx context.Context, id, userID string) error {
	if err := s.repo.Archive(ctx, id, userID); err != nil {
		return err
	}
	log.Info().Str("workspace_id", id).Str("user_id", userID).Msg("工作区已归档")
	return nil
}

// isMemberOrCreator 检查用户是否为工作区成员或创建者
func isMemberOrCreator(w *Workspace, userID string) bool {
	if w.CreatorID == userID {
		return true
	}
	for _, m := range w.Members {
		if m == userID {
			return true
		}
	}
	return false
}

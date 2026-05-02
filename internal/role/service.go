package role

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmcs/pkg/util"
	"github.com/rs/zerolog/log"
)

// RoleRepository 角色仓储接口
type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	GetByID(ctx context.Context, id string) (*Role, error)
	List(ctx context.Context, isGlobal *bool, creatorID string) ([]*Role, error)
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id, userID string) error
}

// Service 角色服务
type Service struct {
	repo          RoleRepository
	skillRegistry *SkillRegistry
}

// NewService 创建角色服务
func NewService(repo RoleRepository, skillRegistry *SkillRegistry) *Service {
	return &Service{repo: repo, skillRegistry: skillRegistry}
}

// CreateRequest 创建角色请求
type CreateRequest struct {
	Name          string          `json:"name"`
	Title         string          `json:"title"`
	Traits        json.RawMessage `json:"traits,omitempty"`
	Expertise     []string        `json:"expertise,omitempty"`
	SpeakingStyle string          `json:"speaking_style,omitempty"`
	SystemPrompt  string          `json:"system_prompt,omitempty"`
	Skills        []string        `json:"skills,omitempty"`
	DefaultModel  json.RawMessage `json:"default_model,omitempty"`
}

// UpdateRequest 更新角色请求
type UpdateRequest struct {
	Name          string          `json:"name,omitempty"`
	Title         string          `json:"title,omitempty"`
	Traits        json.RawMessage `json:"traits,omitempty"`
	Expertise     []string        `json:"expertise,omitempty"`
	SpeakingStyle string          `json:"speaking_style,omitempty"`
	SystemPrompt  string          `json:"system_prompt,omitempty"`
	Skills        []string        `json:"skills,omitempty"`
	DefaultModel  json.RawMessage `json:"default_model,omitempty"`
}

// Create 创建角色
func (s *Service) Create(ctx context.Context, creatorID string, req *CreateRequest) (*Role, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("角色名称不能为空")
	}
	if req.Title == "" {
		return nil, fmt.Errorf("角色头衔不能为空")
	}

	// 验证 skills 是否存在
	for _, skillName := range req.Skills {
		if _, err := s.skillRegistry.Get(skillName); err != nil {
			return nil, fmt.Errorf("技能 %s 不存在", skillName)
		}
	}

	if req.Traits == nil {
		req.Traits = json.RawMessage(`{}`)
	}

	now := time.Now()
	role := &Role{
		ID:            util.NewID("r"),
		Name:          req.Name,
		Title:         req.Title,
		Traits:        req.Traits,
		Expertise:     req.Expertise,
		SpeakingStyle: req.SpeakingStyle,
		SystemPrompt:  req.SystemPrompt,
		Skills:        req.Skills,
		DefaultModel:  req.DefaultModel,
		IsGlobal:      false,
		CreatorID:     creatorID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("创建角色失败: %w", err)
	}

	log.Info().Str("role_id", role.ID).Str("creator_id", creatorID).Msg("角色创建成功")
	return role, nil
}

// Get 获取角色
func (s *Service) Get(ctx context.Context, id string) (*Role, error) {
	return s.repo.GetByID(ctx, id)
}

// List 获取角色列表
func (s *Service) List(ctx context.Context, isGlobal *bool, creatorID string) ([]*Role, error) {
	return s.repo.List(ctx, isGlobal, creatorID)
}

// Update 更新角色
func (s *Service) Update(ctx context.Context, id, userID string, req *UpdateRequest) (*Role, error) {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 全局角色只能由系统管理，用户不能修改
	if role.IsGlobal {
		return nil, fmt.Errorf("不能修改系统预置角色")
	}
	if role.CreatorID != userID {
		return nil, fmt.Errorf("只有创建者才能修改角色")
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Title != "" {
		role.Title = req.Title
	}
	if req.Traits != nil {
		role.Traits = req.Traits
	}
	if req.Expertise != nil {
		role.Expertise = req.Expertise
	}
	if req.SpeakingStyle != "" {
		role.SpeakingStyle = req.SpeakingStyle
	}
	if req.SystemPrompt != "" {
		role.SystemPrompt = req.SystemPrompt
	}
	if req.Skills != nil {
		// 验证 skills 是否存在
		for _, skillName := range req.Skills {
			if _, err := s.skillRegistry.Get(skillName); err != nil {
				return nil, fmt.Errorf("技能 %s 不存在", skillName)
			}
		}
		role.Skills = req.Skills
	}
	if req.DefaultModel != nil {
		role.DefaultModel = req.DefaultModel
	}

	if err := s.repo.Update(ctx, role); err != nil {
		return nil, fmt.Errorf("更新角色失败: %w", err)
	}

	log.Info().Str("role_id", id).Str("user_id", userID).Msg("角色更新成功")
	return role, nil
}

// Delete 删除角色
func (s *Service) Delete(ctx context.Context, id, userID string) error {
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return err
	}
	log.Info().Str("role_id", id).Str("user_id", userID).Msg("角色已删除")
	return nil
}

// ListSkills 列出所有可用技能
func (s *Service) ListSkills(ctx context.Context) []*SkillDefinition {
	return s.skillRegistry.GetAll()
}

package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmcs/internal/role"
	"github.com/mmcs/pkg/util"
	"github.com/rs/zerolog/log"
)

// Service 会话服务
type Service struct {
	repo      *Repository
	graphPool *GraphPool
	roleSvc   *role.Service
}

// NewService 创建会话服务
func NewService(repo *Repository, graphPool *GraphPool, roleSvc *role.Service) *Service {
	return &Service{repo: repo, graphPool: graphPool, roleSvc: roleSvc}
}

// CreateRequest 创建会话请求
type CreateRequest struct {
	WorkspaceID  string          `json:"workspace_id"`
	Title        string          `json:"title"`
	Paradigm     string          `json:"paradigm"`
	MaxRounds    int             `json:"max_rounds,omitempty"`
	RoundTimeout int             `json:"round_timeout,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
	RoleIDs      []string        `json:"role_ids"` // 绑定的角色 ID 列表
}

// CreateResponse 创建会话响应
type CreateResponse struct {
	Session      *Session       `json:"session"`
	SessionRoles []*SessionRole `json:"session_roles,omitempty"`
}

// Create 创建会话
func (s *Service) Create(ctx context.Context, creatorID string, req *CreateRequest) (*CreateResponse, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("会话标题不能为空")
	}
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("工作区 ID 不能为空")
	}
	if req.Paradigm == "" {
		req.Paradigm = "round_robin"
	}
	validParadigms := map[string]bool{"round_robin": true, "court": true, "evaluation": true, "free_chat": true}
	if !validParadigms[req.Paradigm] {
		return nil, fmt.Errorf("无效的讨论范式: %s", req.Paradigm)
	}
	if len(req.RoleIDs) == 0 {
		return nil, fmt.Errorf("至少需要一个角色")
	}

	if req.MaxRounds <= 0 {
		req.MaxRounds = 10
	}
	if req.RoundTimeout <= 0 {
		req.RoundTimeout = 300
	}
	if req.Config == nil {
		req.Config = json.RawMessage(`{}`)
	}

	now := time.Now()
	sess := &Session{
		ID:           util.NewID("s"),
		WorkspaceID:  req.WorkspaceID,
		Title:        req.Title,
		Paradigm:     req.Paradigm,
		Status:       StatusDraft,
		MaxRounds:    req.MaxRounds,
		RoundTimeout: req.RoundTimeout,
		Config:       req.Config,
		CreatorID:    creatorID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	// 绑定角色
	var sessionRoles []*SessionRole
	for i, roleID := range req.RoleIDs {
		// 验证角色存在
		if _, err := s.roleSvc.Get(ctx, roleID); err != nil {
			return nil, fmt.Errorf("角色 %s 不存在: %w", roleID, err)
		}

		sr := &SessionRole{
			ID:        util.NewID("sr"),
			SessionID: sess.ID,
			RoleID:    roleID,
			SortOrder: i,
		}
		if err := s.repo.AddSessionRole(ctx, sr); err != nil {
			return nil, fmt.Errorf("绑定角色失败: %w", err)
		}
		sessionRoles = append(sessionRoles, sr)
	}

	log.Info().Str("session_id", sess.ID).Int("roles", len(sessionRoles)).Msg("会话创建成功")
	return &CreateResponse{Session: sess, SessionRoles: sessionRoles}, nil
}

// Get 获取会话
func (s *Service) Get(ctx context.Context, id string) (*Session, error) {
	return s.repo.GetByID(ctx, id)
}

// GetWithRoles 获取会话及其绑定的角色
func (s *Service) GetWithRoles(ctx context.Context, id string) (*Session, []*SessionRole, error) {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	roles, err := s.repo.GetSessionRoles(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return sess, roles, nil
}

// ListByWorkspace 获取工作区下的会话列表
func (s *Service) ListByWorkspace(ctx context.Context, workspaceID string) ([]*Session, error) {
	return s.repo.ListByWorkspace(ctx, workspaceID)
}

// Start 启动会话（draft/paused → running）
func (s *Service) Start(ctx context.Context, id string) error {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(sess.Status, StatusRunning); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, StatusRunning); err != nil {
		return fmt.Errorf("启动会话失败: %w", err)
	}

	log.Info().Str("session_id", id).Msg("会话已启动")
	return nil
}

// Pause 暂停会话（running → paused）
func (s *Service) Pause(ctx context.Context, id string) error {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(sess.Status, StatusPaused); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, StatusPaused); err != nil {
		return fmt.Errorf("暂停会话失败: %w", err)
	}

	// 从池中移除实例
	s.graphPool.Remove(id)

	log.Info().Str("session_id", id).Msg("会话已暂停")
	return nil
}

// Resume 恢复会话（paused → running）
func (s *Service) Resume(ctx context.Context, id string) error {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(sess.Status, StatusRunning); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, StatusRunning); err != nil {
		return fmt.Errorf("恢复会话失败: %w", err)
	}

	log.Info().Str("session_id", id).Msg("会话已恢复")
	return nil
}

// Terminate 终止会话（any → ended）
func (s *Service) Terminate(ctx context.Context, id string) error {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(sess.Status, StatusEnded); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, StatusEnded); err != nil {
		return fmt.Errorf("终止会话失败: %w", err)
	}

	// 从池中移除实例
	s.graphPool.Remove(id)

	log.Info().Str("session_id", id).Msg("会话已终止")
	return nil
}

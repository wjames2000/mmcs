package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/pkg/util"
)

// Storage 会话存储接口（抽象 Repository，便于测试 mock）
type Storage interface {
	Create(ctx context.Context, s *Session) error
	GetByID(ctx context.Context, id string) (*Session, error)
	UpdateStatus(ctx context.Context, id, status string, extra ...time.Time) error
	Update(ctx context.Context, s *Session) error
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*Session, error)
	AddSessionRole(ctx context.Context, sr *SessionRole) error
	GetSessionRoles(ctx context.Context, sessionID string) ([]*SessionRole, error)
	RemoveSessionRole(ctx context.Context, sessionID, roleID string) error
}

// RoleProvider 角色提供者接口（抽象 role.Service）
type RoleProvider interface {
	Get(ctx context.Context, id string) (*role.Role, error)
}

// Service 会话服务
type Service struct {
	repo      Storage
	graphPool *GraphPool
	roleSvc   RoleProvider

	mu           sync.RWMutex
	runtimeChans map[string]*SessionChannels // sessionID → runtime channels
	hubRegistry  *stream.HubRegistry         // 用于广播 SSE 事件
}

// SetHubRegistry 设置 Hub 注册表（支持从外部注入）
func (s *Service) SetHubRegistry(hr *stream.HubRegistry) {
	s.hubRegistry = hr
}

// NewService 创建会话服务
func NewService(repo Storage, graphPool *GraphPool, roleSvc RoleProvider) *Service {
	return &Service{
		repo:         repo,
		graphPool:    graphPool,
		roleSvc:      roleSvc,
		runtimeChans: make(map[string]*SessionChannels),
	}
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

// InitChannels 初始化会话运行时 channel
// 如果已存在则返回现有实例
func (s *Service) InitChannels(sessionID string) *SessionChannels {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, ok := s.runtimeChans[sessionID]; ok {
		return ch
	}
	ch := NewSessionChannels()
	s.runtimeChans[sessionID] = ch
	return ch
}

// GetChannels 获取会话运行时 channel
func (s *Service) GetChannels(sessionID string) (*SessionChannels, bool) {
	s.mu.RLock()
	ch, ok := s.runtimeChans[sessionID]
	s.mu.RUnlock()
	return ch, ok
}

// RemoveChannels 移除会话运行时 channel
func (s *Service) RemoveChannels(sessionID string) {
	s.mu.Lock()
	delete(s.runtimeChans, sessionID)
	s.mu.Unlock()
}

// broadcastSessionEvent 广播会话级别 SSE 事件
func (s *Service) broadcastSessionEvent(sessionID, eventType string, data interface{}) {
	if s.hubRegistry == nil {
		return
	}
	hub := s.hubRegistry.GetOrCreate(sessionID)
	hub.Broadcast(&stream.Event{
		Type: eventType,
		Data: data,
	})
}

// Pause 暂停会话（running → paused）
// 向运行中的编排器发送中断信号
func (s *Service) Pause(ctx context.Context, id string, nodeName string, message string) error {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(sess.Status, StatusPaused); err != nil {
		return err
	}

	// 发送中断信号到运行中的编排器
	if ch, ok := s.GetChannels(id); ok {
		select {
		case ch.InterruptCh <- &InterruptSignal{
			NodeName: nodeName,
			Message:  message,
			UserID:   sess.CreatorID,
		}:
		case <-time.After(time.Second * 5):
			return fmt.Errorf("发送中断信号超时")
		}
	}

	if err := s.repo.UpdateStatus(ctx, id, StatusPaused); err != nil {
		return fmt.Errorf("暂停会话失败: %w", err)
	}

	// 广播 SSE 事件
	s.broadcastSessionEvent(id, "session.paused", map[string]interface{}{
		"session_id": id,
		"status":     "paused",
		"message":    message,
		"node_name":  nodeName,
	})

	log.Info().Str("session_id", id).Str("node_name", nodeName).Msg("会话已暂停")
	return nil
}

// Resume 恢复会话（paused → running）
// 向暂停中的编排器发送恢复信号
func (s *Service) Resume(ctx context.Context, id string, message string) error {
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

	// 发送恢复信号到暂停中的编排器
	if ch, ok := s.GetChannels(id); ok {
		select {
		case ch.ResumeCh <- &ResumeSignal{Message: message}:
		case <-time.After(time.Second * 5):
			return fmt.Errorf("发送恢复信号超时")
		}
	}

	// 广播 SSE 事件
	s.broadcastSessionEvent(id, "session.resumed", map[string]interface{}{
		"session_id": id,
		"status":     "running",
		"message":    message,
	})

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

	// 清理运行时 channel
	s.RemoveChannels(id)

	log.Info().Str("session_id", id).Msg("会话已终止")
	return nil
}

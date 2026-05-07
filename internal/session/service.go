package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/minutes"
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
	Delete(ctx context.Context, id string) error
}

// RoleProvider 角色提供者接口（抽象 role.Service）
type RoleProvider interface {
	Get(ctx context.Context, id string) (*role.Role, error)
}

// RoleBindingsProvider 角色绑定提供者（用于重启时复制角色绑定）
type RoleBindingsProvider interface {
	GetSessionRoles(ctx context.Context, sessionID string) ([]*SessionRole, error)
}

// Service 会话服务
type Service struct {
	repo      Storage
	graphPool *GraphPool
	roleSvc   RoleProvider

	mu            sync.RWMutex
	runtimeChans  map[string]*SessionChannels // sessionID → runtime channels
	hubRegistry   *stream.HubRegistry         // 用于广播 SSE 事件
	materialStore MaterialStoreInterface      // 会议材料存储
	messageStore  MessageStoreInterface       // 消息持久化存储
}

// SetHubRegistry 设置 Hub 注册表（支持从外部注入）
func (s *Service) SetHubRegistry(hr *stream.HubRegistry) {
	s.hubRegistry = hr
}

// SetMaterialStore 设置会议材料存储（支持从外部注入）
func (s *Service) SetMaterialStore(ms MaterialStoreInterface) {
	s.materialStore = ms
}

// SetMessageStore 设置消息存储（支持从外部注入）
func (s *Service) SetMessageStore(ms MessageStoreInterface) {
	s.messageStore = ms
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

// RoleBinding 角色模型绑定
type RoleBinding struct {
	RoleID        string          `json:"role_id"`
	ModelOverride json.RawMessage `json:"model_override,omitempty"` // {provider, model_name, parameters...}
}

// CreateRequest 创建会话请求
type CreateRequest struct {
	WorkspaceID  string          `json:"workspace_id"`
	Title        string          `json:"title"`
	Topic        string          `json:"topic,omitempty"` // 讨论主题/背景描述
	Paradigm     string          `json:"paradigm"`
	MaxRounds    int             `json:"max_rounds,omitempty"`
	RoundTimeout int             `json:"round_timeout,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
	RoleIDs      []string        `json:"role_ids"`                // 兼容旧版：仅角色 ID 列表
	RoleBindings []RoleBinding   `json:"role_bindings,omitempty"` // 新版：含模型绑定
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
	if len(req.RoleIDs) == 0 && len(req.RoleBindings) == 0 {
		return nil, fmt.Errorf("至少需要一个角色")
	}
	// 如果提供了 RoleBindings，自动填充 RoleIDs（便于后续兼容判断）
	if len(req.RoleBindings) > 0 && len(req.RoleIDs) == 0 {
		for _, rb := range req.RoleBindings {
			req.RoleIDs = append(req.RoleIDs, rb.RoleID)
		}
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
		Topic:        &req.Topic,
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

	// 绑定角色：优先使用 RoleBindings（含模型绑定），回退到 RoleIDs
	var sessionRoles []*SessionRole

	if len(req.RoleBindings) > 0 {
		// 新版：RoleBindings 含模型绑定
		for i, rb := range req.RoleBindings {
			if rb.RoleID == "" {
				continue
			}
			// 验证角色存在
			if _, err := s.roleSvc.Get(ctx, rb.RoleID); err != nil {
				return nil, fmt.Errorf("角色 %s 不存在: %w", rb.RoleID, err)
			}

			sr := &SessionRole{
				ID:            util.NewID("sr"),
				SessionID:     sess.ID,
				RoleID:        rb.RoleID,
				ModelOverride: rb.ModelOverride,
				SortOrder:     i,
			}
			if err := s.repo.AddSessionRole(ctx, sr); err != nil {
				return nil, fmt.Errorf("绑定角色失败: %w", err)
			}
			sessionRoles = append(sessionRoles, sr)
		}
	} else {
		// 兼容旧版：仅角色 ID 列表
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
	}

	log.Info().Str("session_id", sess.ID).Int("roles", len(sessionRoles)).Msg("会话创建成功")
	return &CreateResponse{Session: sess, SessionRoles: sessionRoles}, nil
}

// AddSessionRole 向草稿会话添加角色（支持模型绑定）
func (s *Service) AddSessionRole(ctx context.Context, sessionID, roleID string, modelOverride json.RawMessage) error {
	// 验证会话存在且为草稿状态
	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("会话不存在: %w", err)
	}
	if sess.Status != StatusDraft {
		return fmt.Errorf("只能在草稿状态下添加角色，当前状态: %s", sess.Status)
	}

	// 验证角色存在
	if _, err := s.roleSvc.Get(ctx, roleID); err != nil {
		return fmt.Errorf("角色 %s 不存在: %w", roleID, err)
	}

	// 获取当前排序
	roles, err := s.repo.GetSessionRoles(ctx, sessionID)
	if err != nil {
		return err
	}

	sr := &SessionRole{
		ID:            util.NewID("sr"),
		SessionID:     sessionID,
		RoleID:        roleID,
		ModelOverride: modelOverride,
		SortOrder:     len(roles),
	}
	if err := s.repo.AddSessionRole(ctx, sr); err != nil {
		return fmt.Errorf("添加会话角色失败: %w", err)
	}

	log.Info().Str("session_id", sessionID).Str("role_id", roleID).Msg("会话角色已添加")
	return nil
}

// RemoveSessionRole 从草稿会话移除角色
func (s *Service) RemoveSessionRole(ctx context.Context, sessionID, roleID string) error {
	// 验证会话存在且为草稿状态
	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("会话不存在: %w", err)
	}
	if sess.Status != StatusDraft {
		return fmt.Errorf("只能在草稿状态下移除角色，当前状态: %s", sess.Status)
	}

	if err := s.repo.RemoveSessionRole(ctx, sessionID, roleID); err != nil {
		return fmt.Errorf("移除会话角色失败: %w", err)
	}

	log.Info().Str("session_id", sessionID).Str("role_id", roleID).Msg("会话角色已移除")
	return nil
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

// GetMinutes 获取会话会议纪要
// 从已存储的会话信息和消息记录构建 MeetingMinutes
func (s *Service) GetMinutes(ctx context.Context, id string) (*minutes.MeetingMinutes, error) {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sessionRoles, err := s.repo.GetSessionRoles(ctx, id)
	if err != nil {
		return nil, err
	}

	// 获取参与者名称
	participants := make([]string, 0, len(sessionRoles))
	for _, sr := range sessionRoles {
		r, err := s.roleSvc.Get(ctx, sr.RoleID)
		if err == nil && r != nil {
			participants = append(participants, r.Name)
		} else {
			participants = append(participants, sr.RoleID)
		}
	}

	// 从消息存储构建轮次记录
	rounds := []minutes.RoundRecord{}
	decisions := []minutes.Decision{}
	var conclusion string

	if s.messageStore != nil {
		msgs, err := s.messageStore.ListBySession(id)
		if err == nil && len(msgs) > 0 {
			// 按轮次分组
			roundMap := make(map[int][]*ChatMessage)
			for _, msg := range msgs {
				roundMap[msg.Round] = append(roundMap[msg.Round], msg)
			}

			// 构建轮次记录
			roundNumbers := make([]int, 0, len(roundMap))
			for rn := range roundMap {
				roundNumbers = append(roundNumbers, rn)
			}
			sort.Ints(roundNumbers)

			for _, rn := range roundNumbers {
				recs := roundMap[rn]
				speeches := make([]minutes.SpeechRecord, 0, len(recs))
				for _, msg := range recs {
					speeches = append(speeches, minutes.SpeechRecord{
						RoleName: msg.RoleName,
						Content:  msg.Content,
						Tokens:   msg.Tokens,
					})
				}
				rounds = append(rounds, minutes.RoundRecord{
					RoundNumber: rn,
					Speeches:    speeches,
				})
			}

			// 从最后一轮消息中提取结论
			if len(msgs) > 0 {
				lastMsg := msgs[len(msgs)-1]
				conclusion = lastMsg.Content

				// 查找包含"共识""结论""决定""一致"等关键词的消息作为决策
				for _, msg := range msgs {
					content := msg.Content
					if strings.Contains(content, "共识") || strings.Contains(content, "结论") ||
						strings.Contains(content, "决定") || strings.Contains(content, "一致同意") {
						decisions = append(decisions, minutes.Decision{
							Title:       fmt.Sprintf("第%d轮讨论结论", msg.Round),
							Description: truncateContent(content, 5000),
							Accepted:    true,
						})
						if len(decisions) >= 5 {
							break
						}
					}
				}
			}
		}
	}

	if rounds == nil {
		rounds = []minutes.RoundRecord{}
	}
	if decisions == nil {
		decisions = []minutes.Decision{}
	}

	mm := &minutes.MeetingMinutes{
		SessionID:     sess.ID,
		Title:         sess.Title,
		Paradigm:      sess.Paradigm,
		Participants:  participants,
		Rounds:        rounds,
		Decisions:     decisions,
		Disagreements: []minutes.Disagreement{},
		Conclusion:    conclusion,
	}
	if sess.StartedAt != nil {
		mm.StartedAt = *sess.StartedAt
	}
	if sess.EndedAt != nil {
		mm.EndedAt = *sess.EndedAt
	}

	// 附加附件材料
	if s.materialStore != nil {
		materials := s.materialStore.ListBySession(id)
		if len(materials) > 0 {
			mm.Materials = make([]minutes.MaterialInfo, 0, len(materials))
			for _, m := range materials {
				mm.Materials = append(mm.Materials, minutes.MaterialInfo{
					ID:         m.ID,
					FileName:   m.FileName,
					FileSize:   m.FileSize,
					MimeType:   m.MimeType,
					Content:    m.Content,
					UploadedAt: m.UploadedAt,
				})
			}
		}
	}

	return mm, nil
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

// Archive 归档会话（ended/failed → archived）
func (s *Service) Archive(ctx context.Context, id string) error {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := ValidateTransition(sess.Status, StatusArchived); err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(ctx, id, StatusArchived); err != nil {
		return fmt.Errorf("归档会话失败: %w", err)
	}
	log.Info().Str("session_id", id).Msg("会话已归档")
	return nil
}

// Delete 硬删除会话
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
	}
	log.Info().Str("session_id", id).Msg("会话已删除")
	return nil
}

// Restart 重启已结束的会议
// 验证原会话状态为 ended，创建新会话并关联 parent_session_id，
// 复制角色绑定（使用传入的 roleIDs），并复制所有材料到新会话。
func (s *Service) Restart(ctx context.Context, originalSessionID, creatorID, newTitle, newTopic string, roleIDs []string, roleBindings []RoleBinding) (*CreateResponse, error) {
	// 1. 验证原会话存在且状态为 ended
	originalSession, err := s.repo.GetByID(ctx, originalSessionID)
	if err != nil {
		return nil, fmt.Errorf("原会话不存在: %w", err)
	}
	if originalSession.Status != StatusEnded {
		return nil, fmt.Errorf("只能重启已结束的会话，当前状态: %s", originalSession.Status)
	}

	// 2. 验证必要的参数
	if newTitle == "" {
		return nil, fmt.Errorf("新会话标题不能为空")
	}
	if len(roleIDs) == 0 && len(roleBindings) == 0 {
		return nil, fmt.Errorf("至少需要一个角色")
	}

	// 3. 创建新会话
	now := time.Now()
	topic := newTopic
	newSess := &Session{
		ID:              util.NewID("s"),
		WorkspaceID:     originalSession.WorkspaceID,
		Title:           newTitle,
		Topic:           &topic,
		Paradigm:        originalSession.Paradigm,
		Status:          StatusDraft,
		MaxRounds:       originalSession.MaxRounds,
		RoundTimeout:    originalSession.RoundTimeout,
		Config:          originalSession.Config,
		CreatorID:       creatorID,
		ParentSessionID: &originalSessionID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, newSess); err != nil {
		return nil, fmt.Errorf("创建新会话失败: %w", err)
	}

	// 4. 绑定角色
	var sessionRoles []*SessionRole
	if len(roleBindings) > 0 {
		for i, rb := range roleBindings {
			if rb.RoleID == "" {
				continue
			}
			if _, err := s.roleSvc.Get(ctx, rb.RoleID); err != nil {
				return nil, fmt.Errorf("角色 %s 不存在: %w", rb.RoleID, err)
			}
			sr := &SessionRole{
				ID:            util.NewID("sr"),
				SessionID:     newSess.ID,
				RoleID:        rb.RoleID,
				ModelOverride: rb.ModelOverride,
				SortOrder:     i,
			}
			if err := s.repo.AddSessionRole(ctx, sr); err != nil {
				return nil, fmt.Errorf("绑定角色失败: %w", err)
			}
			sessionRoles = append(sessionRoles, sr)
		}
	} else {
		for i, roleID := range roleIDs {
			if _, err := s.roleSvc.Get(ctx, roleID); err != nil {
				return nil, fmt.Errorf("角色 %s 不存在: %w", roleID, err)
			}
			sr := &SessionRole{
				ID:        util.NewID("sr"),
				SessionID: newSess.ID,
				RoleID:    roleID,
				SortOrder: i,
			}
			if err := s.repo.AddSessionRole(ctx, sr); err != nil {
				return nil, fmt.Errorf("绑定角色失败: %w", err)
			}
			sessionRoles = append(sessionRoles, sr)
		}
	}

	// 5. 复制材料到新会话
	if s.materialStore != nil {
		s.materialStore.CopyToSession(originalSessionID, newSess.ID)
	}

	log.Info().
		Str("original_session_id", originalSessionID).
		Str("new_session_id", newSess.ID).
		Int("roles", len(sessionRoles)).
		Msg("会话重启成功")
	return &CreateResponse{Session: newSess, SessionRoles: sessionRoles}, nil
}

// GetMergedMinutes 获取合并会议纪要
// 返回原会话和新会话的合并视图，包含合并的决策列表和结论
func (s *Service) GetMergedMinutes(ctx context.Context, newSessionID, originalSessionID string) (*MergedMinutes, error) {
	// 获取新会话
	newSession, err := s.repo.GetByID(ctx, newSessionID)
	if err != nil {
		return nil, fmt.Errorf("新会话不存在: %w", err)
	}

	// 获取原会话
	originalSession, err := s.repo.GetByID(ctx, originalSessionID)
	if err != nil {
		return nil, fmt.Errorf("原会话不存在: %w", err)
	}

	// 获取原会话的纪要
	originalMinutes, err := s.GetMinutes(ctx, originalSessionID)
	if err != nil {
		return nil, fmt.Errorf("获取原会话纪要失败: %w", err)
	}

	// 获取新会话的纪要
	newMinutes, err := s.GetMinutes(ctx, newSessionID)
	if err != nil {
		return nil, fmt.Errorf("获取新会话纪要失败: %w", err)
	}

	// 合并决策（去重，按 title 去重）
	seenDecisions := make(map[string]bool)
	var mergedDecisions []minutes.Decision

	for _, d := range originalMinutes.Decisions {
		key := d.Title
		if !seenDecisions[key] {
			seenDecisions[key] = true
			mergedDecisions = append(mergedDecisions, d)
		}
	}
	for _, d := range newMinutes.Decisions {
		key := d.Title
		if !seenDecisions[key] {
			seenDecisions[key] = true
			mergedDecisions = append(mergedDecisions, d)
		}
	}
	if mergedDecisions == nil {
		mergedDecisions = []minutes.Decision{}
	}

	// 合并结论（原 + 新）
	mergedConclusion := ""
	if originalMinutes.Conclusion != "" {
		mergedConclusion = "【原会话结论】\n" + originalMinutes.Conclusion
	}
	if newMinutes.Conclusion != "" {
		if mergedConclusion != "" {
			mergedConclusion += "\n\n"
		}
		mergedConclusion += "【新会话结论】\n" + newMinutes.Conclusion
	}

	// 合并材料列表（去重，按 file_name）
	seenMaterials := make(map[string]bool)
	var mergedMaterials []minutes.MaterialInfo
	for _, m := range originalMinutes.Materials {
		if !seenMaterials[m.FileName] {
			seenMaterials[m.FileName] = true
			mergedMaterials = append(mergedMaterials, m)
		}
	}
	for _, m := range newMinutes.Materials {
		if !seenMaterials[m.FileName] {
			seenMaterials[m.FileName] = true
			mergedMaterials = append(mergedMaterials, m)
		}
	}
	if mergedMaterials == nil {
		mergedMaterials = []minutes.MaterialInfo{}
	}

	result := &MergedMinutes{
		OriginalSessionID: originalSessionID,
		NewSessionID:      newSessionID,
		OriginalTitle:     originalSession.Title,
		NewTitle:          newSession.Title,
		OriginalMinutes:   originalMinutes,
		NewMinutes:        newMinutes,
		MergedDecisions:   mergedDecisions,
		MergedConclusion:  mergedConclusion,
		MergedMaterials:   mergedMaterials,
	}

	return result, nil
}

// truncateContent 截断内容到指定长度
func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "..."
}

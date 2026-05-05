package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/minutes"
	"github.com/wjames2000/mmcs/internal/orchestrator"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/user"
)

// SessionService 会话服务接口（抽象 session.Service，便于测试 mock）
type SessionService interface {
	Create(ctx context.Context, creatorID string, req *session.CreateRequest) (*session.CreateResponse, error)
	Get(ctx context.Context, id string) (*session.Session, error)
	GetWithRoles(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*session.Session, error)
	Start(ctx context.Context, id string) error
	Pause(ctx context.Context, id string, nodeName string, message string) error
	Resume(ctx context.Context, id string, message string) error
	Terminate(ctx context.Context, id string) error
	Archive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	GetMinutes(ctx context.Context, sessionID string) (*minutes.MeetingMinutes, error)
	GetMergedMinutes(ctx context.Context, newSessionID, originalSessionID string) (*session.MergedMinutes, error)
	Restart(ctx context.Context, originalSessionID, creatorID, newTitle, newTopic string, roleIDs []string, roleBindings []session.RoleBinding) (*session.CreateResponse, error)
	InitChannels(sessionID string) *session.SessionChannels
	RemoveChannels(sessionID string)
}

// SessionHandler 会话相关 HTTP handler
type SessionHandler struct {
	sessionService      SessionService
	orchestratorFactory *orchestrator.Factory
	hubRegistry         *stream.HubRegistry
	materialStore       *session.MaterialStore
}

// NewSessionHandler 创建会话 handler
func NewSessionHandler(
	sessionService SessionService,
	orchestratorFactory *orchestrator.Factory,
	hubRegistry *stream.HubRegistry,
	materialStore *session.MaterialStore,
) *SessionHandler {
	return &SessionHandler{
		sessionService:      sessionService,
		orchestratorFactory: orchestratorFactory,
		hubRegistry:         hubRegistry,
		materialStore:       materialStore,
	}
}

// List 获取工作区下的会话列表
// GET /api/v1/workspaces/{workspaceId}/sessions
func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if workspaceID == "" {
		middleware.WriteBadRequest(w, "缺少工作区 ID")
		return
	}

	sessions, err := h.sessionService.ListByWorkspace(r.Context(), workspaceID)
	if err != nil {
		middleware.WriteInternalError(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, sessions)
}

// Create 创建会话
// POST /api/v1/sessions
func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	var req session.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	resp, err := h.sessionService.Create(r.Context(), userID, &req)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteCreated(w, resp)
}

// Get 获取会话详情
// GET /api/v1/sessions/{id}
func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	sess, roles, err := h.sessionService.GetWithRoles(r.Context(), id)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]interface{}{
		"session": sess,
		"roles":   roles,
	})
}

// Start 启动会话
// POST /api/v1/sessions/{id}/start
func (h *SessionHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	// 先启动会话（draft → running）
	if err := h.sessionService.Start(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 启动编排（异步）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Str("session_id", id).Interface("panic", r).Msg("编排执行异常")
			}
		}()
		if err := h.startOrchestration(r.Context(), id); err != nil {
			log.Error().Err(err).Str("session_id", id).Msg("编排启动失败")
		}
	}()

	middleware.WriteSuccess(w, map[string]interface{}{
		"session_id": id,
		"status":     "running",
	})
}

// startOrchestration 启动异步编排
func (h *SessionHandler) startOrchestration(ctx context.Context, sessionID string) error {
	// 获取会话详情
	sess, sessionRoles, err := h.sessionService.GetWithRoles(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("获取会话失败: %w", err)
	}

	// 获取 Hub
	hub := h.hubRegistry.GetOrCreate(sessionID)
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(ctx)

	// 初始化中断/恢复 channel
	ch := h.sessionService.InitChannels(sessionID)
	defer h.sessionService.RemoveChannels(sessionID)

	// 获取编排器
	orch, err := h.orchestratorFactory.CreateOrchestrator(orchestrator.ParadigmType(sess.Paradigm))
	if err != nil {
		return fmt.Errorf("创建编排器失败: %w", err)
	}

	// 提取角色 ID 和主持人模型绑定
	roleIDs := make([]string, len(sessionRoles))
	var moderatorModelBinding string
	for i, sr := range sessionRoles {
		roleIDs[i] = sr.RoleID
		// 第一个角色是主持人，提取其模型绑定
		if i == 0 && sr.ModelOverride != nil {
			var override struct {
				Provider  string `json:"provider"`
				ModelName string `json:"model_name"`
			}
			if err := json.Unmarshal(sr.ModelOverride, &override); err == nil && override.Provider != "" {
				moderatorModelBinding = override.Provider
			}
		}
	}

	progressCh := make(chan string, 10)
	go func() {
		for msg := range progressCh {
			log.Debug().Str("session_id", sessionID).Str("progress", msg).Msg("讨论进度")
		}
	}()

	// 使用 topic 作为讨论主题描述（优先 topic，回退到 title）
	topic := ""
	if sess.Topic != nil {
		topic = *sess.Topic
	}
	if topic == "" {
		topic = sess.Title
	}

	// 根据范式类型执行
	var execErr error
	switch orch := orch.(type) {
	case *orchestrator.RoundRobinOrchestrator:
		config := &orchestrator.RoundRobinConfig{
			RoleIDs:               roleIDs,
			Topic:                 topic,
			MaxRounds:             sess.MaxRounds,
			ModeratorModelBinding: moderatorModelBinding,
			InterruptCh:           ch.InterruptCh,
			ResumeCh:              ch.ResumeCh,
		}
		_, execErr = orch.Execute(ctx, sessionID, config, bridge, progressCh)

	case *orchestrator.CourtOrchestrator:
		config := &orchestrator.CourtConfig{
			RoleIDs:     roleIDs,
			Topic:       topic,
			MaxRounds:   sess.MaxRounds,
			InterruptCh: ch.InterruptCh,
			ResumeCh:    ch.ResumeCh,
		}
		_, execErr = orch.Execute(ctx, sessionID, config, bridge, progressCh)

	case *orchestrator.EvaluationOrchestrator:
		config := &orchestrator.EvaluationConfig{
			RoleIDs:     roleIDs,
			Topic:       topic,
			InterruptCh: ch.InterruptCh,
			ResumeCh:    ch.ResumeCh,
		}
		_, execErr = orch.Execute(ctx, sessionID, config, bridge, progressCh)

	case *orchestrator.FreeChatOrchestrator:
		config := &orchestrator.FreeChatConfig{
			RoleIDs:     roleIDs,
			Topic:       topic,
			MaxRounds:   sess.MaxRounds,
			InterruptCh: ch.InterruptCh,
			ResumeCh:    ch.ResumeCh,
		}
		_, execErr = orch.Execute(ctx, sessionID, config, bridge, progressCh)

	default:
		execErr = fmt.Errorf("不支持的编排器类型: %T", orch)
	}

	// 关闭 progressCh（避免 goroutine 泄漏）
	close(progressCh)

	// 仅当执行出错时才终止会话（让用户手动终止正常完成的讨论）
	if execErr != nil {
		_ = h.sessionService.Terminate(ctx, sessionID)
		return fmt.Errorf("讨论执行失败: %w", execErr)
	}

	// 讨论正常结束，不自动终止，保持 running 状态
	// 用户可以通过 UI 手动点击"结束"来终止会话，期间可随时暂停/恢复
	log.Debug().Str("session_id", sessionID).Msg("讨论执行完成，等待用户手动终止")
	return nil
}

// PauseRequest 暂停请求体
type PauseRequest struct {
	NodeName string `json:"node_name"`
	Message  string `json:"message"`
}

// ResumeRequest 恢复请求体
type ResumeRequest struct {
	Message string `json:"message"`
}

// Pause 暂停会话
// POST /api/v1/sessions/{id}/pause
func (h *SessionHandler) Pause(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	var req PauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if err := h.sessionService.Pause(r.Context(), id, req.NodeName, req.Message); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]string{"status": "paused"})
}

// Resume 恢复会话
// POST /api/v1/sessions/{id}/resume
func (h *SessionHandler) Resume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	var req ResumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if err := h.sessionService.Resume(r.Context(), id, req.Message); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]string{"status": "running"})
}

// Terminate 终止会话
// POST /api/v1/sessions/{id}/terminate
func (h *SessionHandler) Terminate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	if err := h.sessionService.Terminate(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]string{"status": "ended"})
}

// GetMinutes 获取会话会议纪要
// GET /api/v1/sessions/{id}/minutes
func (h *SessionHandler) GetMinutes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	mm, err := h.sessionService.GetMinutes(r.Context(), id)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, mm)
}

// RestartRequest 重启会议请求体
type RestartRequest struct {
	OriginalSessionID string                `json:"original_session_id"`
	Title             string                `json:"title"`
	Topic             string                `json:"topic,omitempty"`
	RoleIDs           []string              `json:"role_ids"`
	RoleBindings      []session.RoleBinding `json:"role_bindings,omitempty"`
}

// Restart 重启已结束的会议
// POST /api/v1/sessions/restart
func (h *SessionHandler) Restart(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	var req RestartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}
	if req.OriginalSessionID == "" {
		middleware.WriteBadRequest(w, "缺少原会话 ID")
		return
	}
	if req.Title == "" {
		middleware.WriteBadRequest(w, "缺少新会话标题")
		return
	}

	resp, err := h.sessionService.Restart(r.Context(), req.OriginalSessionID, userID, req.Title, req.Topic, req.RoleIDs, req.RoleBindings)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteCreated(w, resp)
}

// GetMergedMinutes 获取合并会议纪要
// GET /api/v1/sessions/{sessionId}/merged-minutes?original={originalSessionId}
func (h *SessionHandler) GetMergedMinutes(w http.ResponseWriter, r *http.Request) {
	newSessionID := r.PathValue("sessionId")
	originalSessionID := r.URL.Query().Get("original")
	if newSessionID == "" || originalSessionID == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID 参数")
		return
	}

	mm, err := h.sessionService.GetMergedMinutes(r.Context(), newSessionID, originalSessionID)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, mm)
}

// UploadMaterialRequest 上传材料请求体
type UploadMaterialRequest struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64 编码的文件数据
}

// UploadMaterial 上传会议材料
// POST /api/v1/sessions/{sessionId}/materials
func (h *SessionHandler) UploadMaterial(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if sessionID == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	if h.materialStore == nil {
		middleware.WriteInternalError(w, "材料存储未初始化")
		return
	}

	var req UploadMaterialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}
	if req.FileName == "" {
		middleware.WriteBadRequest(w, "缺少文件名")
		return
	}

	// 解码 base64（支持 data URL 格式）
	data, err := decodeBase64(req.Data)
	if err != nil {
		middleware.WriteBadRequest(w, "base64 解码失败: "+err.Error())
		return
	}

	m := h.materialStore.Add(sessionID, req.FileName, req.MimeType, data)
	middleware.WriteCreated(w, m)
}

// ListMaterials 获取会话的材料列表
// GET /api/v1/sessions/{sessionId}/materials
func (h *SessionHandler) ListMaterials(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if sessionID == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	if h.materialStore == nil {
		middleware.WriteSuccess(w, []interface{}{})
		return
	}

	materials := h.materialStore.ListBySession(sessionID)
	middleware.WriteSuccess(w, materials)
}

// DeleteMaterial 删除会议材料
// DELETE /api/v1/materials/{id}
func (h *SessionHandler) DeleteMaterial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少材料 ID")
		return
	}

	if h.materialStore == nil {
		middleware.WriteInternalError(w, "材料存储未初始化")
		return
	}

	if err := h.materialStore.Delete(id); err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// decodeBase64 解码 base64 数据（支持 data URL 格式）
func decodeBase64(s string) ([]byte, error) {
	if idx := strings.Index(s, ";base64,"); idx >= 0 {
		s = s[idx+8:]
	} else if idx := strings.Index(s, "base64,"); idx >= 0 {
		s = s[idx+7:]
	}
	return base64.StdEncoding.DecodeString(s)
}

// Stream SSE 流式推送
// GET /api/v1/sessions/{id}/stream
func (h *SessionHandler) Stream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 获取或创建 Hub
	hub := h.hubRegistry.GetOrCreate(id)

	// 创建订阅者
	sub := &stream.Subscriber{
		ID:      stream.NewSubscriberID(),
		Events:  make(chan *stream.Event, 64),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	// 发送连接确认
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\",\"subscriber_id\":%q}\n\n", sub.ID)
	flusher.Flush()

	// 确保清理
	defer hub.Unsubscribe(sub.ID)

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-sub.CloseCh:
			return
		case event, ok := <-sub.Events:
			if !ok {
				return
			}
			h.writeSSEEvent(w, flusher, event)
		}
	}
}

// writeSSEEvent 写入 SSE 事件
func (h *SessionHandler) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event *stream.Event) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(data))
	flusher.Flush()
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mmcs/internal/api/middleware"
	"github.com/mmcs/internal/orchestrator"
	"github.com/mmcs/internal/session"
	"github.com/mmcs/internal/stream"
	"github.com/mmcs/internal/user"
	"github.com/rs/zerolog/log"
)

// SessionHandler 会话相关 HTTP handler
type SessionHandler struct {
	sessionService      *session.Service
	orchestratorFactory *orchestrator.Factory
	hubRegistry         *stream.HubRegistry
}

// NewSessionHandler 创建会话 handler
func NewSessionHandler(
	sessionService *session.Service,
	orchestratorFactory *orchestrator.Factory,
	hubRegistry *stream.HubRegistry,
) *SessionHandler {
	return &SessionHandler{
		sessionService:      sessionService,
		orchestratorFactory: orchestratorFactory,
		hubRegistry:         hubRegistry,
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

	// 获取编排器
	orch, err := h.orchestratorFactory.CreateOrchestrator(orchestrator.ParadigmType(sess.Paradigm))
	if err != nil {
		return fmt.Errorf("创建编排器失败: %w", err)
	}

	roundRobin, ok := orch.(*orchestrator.RoundRobinOrchestrator)
	if !ok {
		return fmt.Errorf("不支持的编排器类型")
	}

	// 提取角色 ID
	roleIDs := make([]string, len(sessionRoles))
	for i, sr := range sessionRoles {
		roleIDs[i] = sr.RoleID
	}

	// 执行讨论
	config := &orchestrator.RoundRobinConfig{
		RoleIDs:   roleIDs,
		Topic:     sess.Title,
		MaxRounds: sess.MaxRounds,
	}

	progressCh := make(chan string, 10)
	go func() {
		for msg := range progressCh {
			log.Debug().Str("session_id", sessionID).Str("progress", msg).Msg("讨论进度")
		}
	}()

	_, err = roundRobin.Execute(ctx, sessionID, config, bridge, progressCh)
	if err != nil {
		_ = h.sessionService.Terminate(ctx, sessionID)
		return fmt.Errorf("讨论执行失败: %w", err)
	}

	// 讨论正常结束，更新状态
	_ = h.sessionService.Terminate(ctx, sessionID)
	return nil
}

// Pause 暂停会话
// POST /api/v1/sessions/{id}/pause
func (h *SessionHandler) Pause(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	if err := h.sessionService.Pause(r.Context(), id); err != nil {
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

	if err := h.sessionService.Resume(r.Context(), id); err != nil {
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

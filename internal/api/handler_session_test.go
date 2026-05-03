package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/user"
)

// ===== Mock SessionService =====

type mockSessionService struct {
	mu             sync.RWMutex
	createFn       func(ctx context.Context, creatorID string, req *session.CreateRequest) (*session.CreateResponse, error)
	getFn          func(ctx context.Context, id string) (*session.Session, error)
	getWithRolesFn func(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error)
	listFn         func(ctx context.Context, workspaceID string) ([]*session.Session, error)
	startFn        func(ctx context.Context, id string) error
	pauseFn        func(ctx context.Context, id string, nodeName string, message string) error
	resumeFn       func(ctx context.Context, id string, message string) error
	terminateFn    func(ctx context.Context, id string) error
	initChFn       func(sessionID string) *session.SessionChannels
	removeChFn     func(sessionID string)
}

func (m *mockSessionService) Create(ctx context.Context, creatorID string, req *session.CreateRequest) (*session.CreateResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, creatorID, req)
	}
	return &session.CreateResponse{
		Session: &session.Session{
			ID:          "s_test_123",
			WorkspaceID: req.WorkspaceID,
			Title:       req.Title,
			Status:      session.StatusDraft,
			CreatorID:   creatorID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}, nil
}

func (m *mockSessionService) Get(ctx context.Context, id string) (*session.Session, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return &session.Session{ID: id, Status: session.StatusDraft}, nil
}

func (m *mockSessionService) GetWithRoles(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error) {
	if m.getWithRolesFn != nil {
		return m.getWithRolesFn(ctx, id)
	}
	return &session.Session{ID: id, Status: session.StatusDraft}, []*session.SessionRole{}, nil
}

func (m *mockSessionService) ListByWorkspace(ctx context.Context, workspaceID string) ([]*session.Session, error) {
	if m.listFn != nil {
		return m.listFn(ctx, workspaceID)
	}
	return []*session.Session{}, nil
}

func (m *mockSessionService) Start(ctx context.Context, id string) error {
	if m.startFn != nil {
		return m.startFn(ctx, id)
	}
	return nil
}

func (m *mockSessionService) Pause(ctx context.Context, id string, nodeName string, message string) error {
	if m.pauseFn != nil {
		return m.pauseFn(ctx, id, nodeName, message)
	}
	return nil
}

func (m *mockSessionService) Resume(ctx context.Context, id string, message string) error {
	if m.resumeFn != nil {
		return m.resumeFn(ctx, id, message)
	}
	return nil
}

func (m *mockSessionService) Terminate(ctx context.Context, id string) error {
	if m.terminateFn != nil {
		return m.terminateFn(ctx, id)
	}
	return nil
}

func (m *mockSessionService) InitChannels(sessionID string) *session.SessionChannels {
	if m.initChFn != nil {
		return m.initChFn(sessionID)
	}
	return session.NewSessionChannels()
}

func (m *mockSessionService) RemoveChannels(sessionID string) {
	if m.removeChFn != nil {
		m.removeChFn(sessionID)
	}
}

// ===== 辅助函数 =====

// newTestSessionHandler 创建测试用的 SessionHandler
func newTestSessionHandler(svc SessionService) *SessionHandler {
	return &SessionHandler{
		sessionService:      svc,
		orchestratorFactory: nil, // Start 测试中 goroutine 会调用 startOrchestration 但 GetWithRoles 会返回 error
		hubRegistry:         stream.NewHubRegistry(),
	}
}

// addUserToContext 在请求 context 中添加 user ID 绕过 JWT 认证
func addUserToContext(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), user.UserIDKey, user.UserIDKey)
	// 使用 user.UserIDKey (类型 contextKey) 来设置值，与 middleware.Authenticate 一致
	ctx = context.WithValue(ctx, user.UserIDKey, userID)
	return r.WithContext(ctx)
}

// setPathValues 为请求设置路由路径参数 (Go 1.22+)
func setPathValues(r *http.Request, values map[string]string) *http.Request {
	for k, v := range values {
		r.SetPathValue(k, v)
	}
	return r
}

// ===== Test: List =====
// GET /api/v1/workspaces/{workspaceId}/sessions

func TestSessionHandler_List_Success(t *testing.T) {
	mockSvc := &mockSessionService{
		listFn: func(ctx context.Context, workspaceID string) ([]*session.Session, error) {
			assert.Equal(t, "ws_1", workspaceID)
			return []*session.Session{
				{ID: "s_1", Title: "会话1", Status: session.StatusDraft},
				{ID: "s_2", Title: "会话2", Status: session.StatusRunning},
			}, nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("GET", "/api/v1/workspaces/ws_1/sessions", nil)
	r = setPathValues(r, map[string]string{"workspaceId": "ws_1"})
	w := httptest.NewRecorder()

	h.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp middleware.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, _ := json.Marshal(resp.Data)
	var sessions []*session.Session
	err = json.Unmarshal(data, &sessions)
	require.NoError(t, err)
	assert.Equal(t, 2, len(sessions))
}

func TestSessionHandler_List_EmptyWorkspaceID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("GET", "/api/v1/workspaces//sessions", nil)
	r = setPathValues(r, map[string]string{"workspaceId": ""})
	w := httptest.NewRecorder()

	h.List(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_List_Error(t *testing.T) {
	mockSvc := &mockSessionService{
		listFn: func(ctx context.Context, workspaceID string) ([]*session.Session, error) {
			return nil, errors.New("数据库错误")
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("GET", "/api/v1/workspaces/ws_1/sessions", nil)
	r = setPathValues(r, map[string]string{"workspaceId": "ws_1"})
	w := httptest.NewRecorder()

	h.List(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ===== Test: Create =====
// POST /api/v1/sessions

func TestSessionHandler_Create_Success(t *testing.T) {
	mockSvc := &mockSessionService{
		createFn: func(ctx context.Context, creatorID string, req *session.CreateRequest) (*session.CreateResponse, error) {
			assert.Equal(t, "test_user", creatorID)
			return &session.CreateResponse{
				Session: &session.Session{
					ID:          "s_new_123",
					WorkspaceID: req.WorkspaceID,
					Title:       req.Title,
					Status:      session.StatusDraft,
					CreatorID:   creatorID,
				},
				SessionRoles: []*session.SessionRole{
					{ID: "sr_1", SessionID: "s_new_123", RoleID: "role_1"},
				},
			}, nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	body := `{"workspace_id":"ws_1","title":"新建会话","role_ids":["role_1"]}`
	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = addUserToContext(r, "test_user")
	w := httptest.NewRecorder()

	h.Create(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp middleware.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestSessionHandler_Create_Unauthenticated(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	body := `{"workspace_id":"ws_1","title":"新建会话","role_ids":["role_1"]}`
	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	// 不添加 user ID 到 context
	w := httptest.NewRecorder()

	h.Create(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSessionHandler_Create_InvalidBody(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader("{invalid json"))
	r.Header.Set("Content-Type", "application/json")
	r = addUserToContext(r, "test_user")
	w := httptest.NewRecorder()

	h.Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Create_ServiceError(t *testing.T) {
	mockSvc := &mockSessionService{
		createFn: func(ctx context.Context, creatorID string, req *session.CreateRequest) (*session.CreateResponse, error) {
			return nil, errors.New("创建失败")
		},
	}
	h := newTestSessionHandler(mockSvc)

	body := `{"workspace_id":"ws_1","title":"测试","role_ids":["role_1"]}`
	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = addUserToContext(r, "test_user")
	w := httptest.NewRecorder()

	h.Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===== Test: Get =====
// GET /api/v1/sessions/{id}

func TestSessionHandler_Get_Success(t *testing.T) {
	mockSvc := &mockSessionService{
		getWithRolesFn: func(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error) {
			assert.Equal(t, "s_test_456", id)
			return &session.Session{
					ID:     "s_test_456",
					Title:  "测试会话",
					Status: session.StatusDraft,
				}, []*session.SessionRole{
					{ID: "sr_1", RoleID: "role_1", SessionID: "s_test_456"},
				}, nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("GET", "/api/v1/sessions/s_test_456", nil)
	r = setPathValues(r, map[string]string{"id": "s_test_456"})
	w := httptest.NewRecorder()

	h.Get(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp middleware.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestSessionHandler_Get_EmptyID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("GET", "/api/v1/sessions/", nil)
	r = setPathValues(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	h.Get(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Get_NotFound(t *testing.T) {
	mockSvc := &mockSessionService{
		getWithRolesFn: func(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error) {
			return nil, nil, errors.New("会话不存在")
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("GET", "/api/v1/sessions/non_existent", nil)
	r = setPathValues(r, map[string]string{"id": "non_existent"})
	w := httptest.NewRecorder()

	h.Get(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ===== Test: Start =====
// POST /api/v1/sessions/{id}/start

func TestSessionHandler_Start_Success(t *testing.T) {
	startCalled := false
	mockSvc := &mockSessionService{
		startFn: func(ctx context.Context, id string) error {
			startCalled = true
			assert.Equal(t, "s_start", id)
			return nil
		},
		// startOrchestration 的 goroutine 会调用 GetWithRoles
		// 返回 error 让 goroutine 提前退出，避免使用 nil orchestratorFactory
		getWithRolesFn: func(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error) {
			return nil, nil, errors.New("编排测试跳过")
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("POST", "/api/v1/sessions/s_start/start", nil)
	r = setPathValues(r, map[string]string{"id": "s_start"})
	// 给 goroutine 使用的 context 加上超时，用完即取消
	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
	defer cancel()
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Start(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, startCalled, "Start 方法应被调用")

	var respMiddleware middleware.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &respMiddleware)
	require.NoError(t, err)

	data, _ := json.Marshal(respMiddleware.Data)
	var result map[string]string
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "running", result["status"])
	assert.Equal(t, "s_start", result["session_id"])
}

func TestSessionHandler_Start_EmptyID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("POST", "/api/v1/sessions//start", nil)
	r = setPathValues(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	h.Start(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Start_ServiceError(t *testing.T) {
	mockSvc := &mockSessionService{
		startFn: func(ctx context.Context, id string) error {
			return errors.New("启动失败")
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("POST", "/api/v1/sessions/s_fail/start", nil)
	r = setPathValues(r, map[string]string{"id": "s_fail"})
	w := httptest.NewRecorder()

	h.Start(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===== Test: Pause =====
// POST /api/v1/sessions/{id}/pause

func TestSessionHandler_Pause_Success(t *testing.T) {
	var capturedNodeName, capturedMessage string
	mockSvc := &mockSessionService{
		pauseFn: func(ctx context.Context, id string, nodeName string, message string) error {
			capturedNodeName = nodeName
			capturedMessage = message
			assert.Equal(t, "s_pause", id)
			return nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	body := `{"node_name":"test_node","message":"暂停测试"}`
	r := httptest.NewRequest("POST", "/api/v1/sessions/s_pause/pause", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = setPathValues(r, map[string]string{"id": "s_pause"})
	w := httptest.NewRecorder()

	h.Pause(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test_node", capturedNodeName)
	assert.Equal(t, "暂停测试", capturedMessage)
}

func TestSessionHandler_Pause_EmptyID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("POST", "/api/v1/sessions//pause", nil)
	r = setPathValues(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	h.Pause(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Pause_InvalidBody(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("POST", "/api/v1/sessions/s_pause/pause", strings.NewReader("{invalid}"))
	r.Header.Set("Content-Type", "application/json")
	r = setPathValues(r, map[string]string{"id": "s_pause"})
	w := httptest.NewRecorder()

	h.Pause(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Pause_ServiceError(t *testing.T) {
	mockSvc := &mockSessionService{
		pauseFn: func(ctx context.Context, id string, nodeName string, message string) error {
			return errors.New("暂停失败")
		},
	}
	h := newTestSessionHandler(mockSvc)

	body := `{"node_name":"test","message":"error"}`
	r := httptest.NewRequest("POST", "/api/v1/sessions/s_err/pause", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = setPathValues(r, map[string]string{"id": "s_err"})
	w := httptest.NewRecorder()

	h.Pause(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===== Test: Resume =====
// POST /api/v1/sessions/{id}/resume

func TestSessionHandler_Resume_Success(t *testing.T) {
	var capturedMessage string
	mockSvc := &mockSessionService{
		resumeFn: func(ctx context.Context, id string, message string) error {
			capturedMessage = message
			assert.Equal(t, "s_resume", id)
			return nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	body := `{"message":"恢复测试"}`
	r := httptest.NewRequest("POST", "/api/v1/sessions/s_resume/resume", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = setPathValues(r, map[string]string{"id": "s_resume"})
	w := httptest.NewRecorder()

	h.Resume(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "恢复测试", capturedMessage)
}

func TestSessionHandler_Resume_EmptyID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("POST", "/api/v1/sessions//resume", nil)
	r = setPathValues(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	h.Resume(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Resume_ServiceError(t *testing.T) {
	mockSvc := &mockSessionService{
		resumeFn: func(ctx context.Context, id string, message string) error {
			return errors.New("恢复失败")
		},
	}
	h := newTestSessionHandler(mockSvc)

	body := `{"message":"error"}`
	r := httptest.NewRequest("POST", "/api/v1/sessions/s_err/resume", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = setPathValues(r, map[string]string{"id": "s_err"})
	w := httptest.NewRecorder()

	h.Resume(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===== Test: Terminate =====
// POST /api/v1/sessions/{id}/terminate

func TestSessionHandler_Terminate_Success(t *testing.T) {
	terminateCalled := false
	mockSvc := &mockSessionService{
		terminateFn: func(ctx context.Context, id string) error {
			terminateCalled = true
			assert.Equal(t, "s_term", id)
			return nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("POST", "/api/v1/sessions/s_term/terminate", nil)
	r = setPathValues(r, map[string]string{"id": "s_term"})
	w := httptest.NewRecorder()

	h.Terminate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, terminateCalled)

	var respMiddleware middleware.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &respMiddleware)
	require.NoError(t, err)

	data, _ := json.Marshal(respMiddleware.Data)
	var result map[string]string
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "ended", result["status"])
}

func TestSessionHandler_Terminate_EmptyID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("POST", "/api/v1/sessions//terminate", nil)
	r = setPathValues(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	h.Terminate(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Terminate_ServiceError(t *testing.T) {
	mockSvc := &mockSessionService{
		terminateFn: func(ctx context.Context, id string) error {
			return errors.New("终止失败")
		},
	}
	h := newTestSessionHandler(mockSvc)

	r := httptest.NewRequest("POST", "/api/v1/sessions/s_err/terminate", nil)
	r = setPathValues(r, map[string]string{"id": "s_err"})
	w := httptest.NewRecorder()

	h.Terminate(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===== Test: Stream =====
// GET /api/v1/sessions/{id}/stream
//
// 注：httptest.ResponseRecorder 在 Go 1.20+ 实现了 http.Flusher，
// 因此 "UnsupportedFlusher" 场景在标准测试中不可复现。
// Stream 测试需要 context 取消来让 handler 退出 select 循环。

func TestSessionHandler_Stream_EmptyID(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("GET", "/api/v1/sessions//stream", nil)
	r = setPathValues(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	h.Stream(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSessionHandler_Stream_ConnectionAndCancel(t *testing.T) {
	h := newTestSessionHandler(&mockSessionService{})

	r := httptest.NewRequest("GET", "/api/v1/sessions/s_stream/stream", nil)
	r = setPathValues(r, map[string]string{"id": "s_stream"})
	ctx, cancel := context.WithCancel(r.Context())
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	// 在 goroutine 中运行 handler（Stream 是阻塞的）
	var handlerErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				handlerErr = fmt.Errorf("panic: %v", r)
			}
		}()
		h.Stream(w, r)
	}()

	// 等待 connection 事件写入
	time.Sleep(100 * time.Millisecond)
	cancel() // 取消 context 让 handler 退出

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stream handler 未在 context 取消后 2 秒内退出")
	}

	// 检查连接事件是否写入
	if handlerErr != nil {
		t.Logf("Handler error (may be expected): %v", handlerErr)
	}

	body := w.Body.String()
	assert.Contains(t, body, "event: connected",
		"SSE 连接事件应在 context 取消前发送")
}

// ===== Test: 路由集成（通过真实 ServeMux）=====

func TestSessionHandler_WithRealMux(t *testing.T) {
	mockSvc := &mockSessionService{
		getWithRolesFn: func(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error) {
			return &session.Session{
				ID:     "route_test",
				Title:  "路由测试",
				Status: session.StatusRunning,
			}, []*session.SessionRole{}, nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/sessions/{id}", h.Get)

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/sessions/route_test")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respMiddleware middleware.APIResponse
	err = json.NewDecoder(resp.Body).Decode(&respMiddleware)
	require.NoError(t, err)
	assert.Equal(t, 0, respMiddleware.Code)
}

// ===== Test: Unauthorized Access via Mux =====

func TestSessionHandler_AuthMiddleware_BlocksUnauthenticated(t *testing.T) {
	mockSvc := &mockSessionService{}
	h := newTestSessionHandler(mockSvc)

	// 注意：此测试验证 handler 本身在无 user context 时的行为
	// 实际认证由 router 中的 middleware.Authenticate 处理
	// 这里 Create 会检查 UserIDFromContext，为空则返回 401
	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ===== Test: Concurrent Handler Calls =====

func TestSessionHandler_ConcurrentCalls(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	mockSvc := &mockSessionService{
		getWithRolesFn: func(ctx context.Context, id string) (*session.Session, []*session.SessionRole, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return &session.Session{ID: id, Title: "并发测试"}, []*session.SessionRole{}, nil
		},
	}
	h := newTestSessionHandler(mockSvc)

	var wg sync.WaitGroup
	const numCalls = 20

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sessions/s_%d", n), nil)
			r = setPathValues(r, map[string]string{"id": fmt.Sprintf("s_%d", n)})
			w := httptest.NewRecorder()
			h.Get(w, r)
			assert.Equal(t, http.StatusOK, w.Code)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, numCalls, callCount)
}

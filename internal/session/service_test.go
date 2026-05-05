package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wjames2000/mmcs/internal/role"
)

// ===== Mock Storage =====

type mockStorage struct {
	mu             sync.RWMutex
	sessions       map[string]*Session
	sessionRoles   map[string][]*SessionRole // sessionID → roles
	updateStatusFn func(ctx context.Context, id, status string, extra ...time.Time) error
	createFn       func(ctx context.Context, s *Session) error
	getByIDFn      func(ctx context.Context, id string) (*Session, error)
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		sessions:     make(map[string]*Session),
		sessionRoles: make(map[string][]*SessionRole),
	}
}

func (m *mockStorage) Create(ctx context.Context, s *Session) error {
	if m.createFn != nil {
		return m.createFn(ctx, s)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[s.ID]; exists {
		return fmt.Errorf("会话已存在")
	}
	m.sessions[s.ID] = s
	return nil
}

func (m *mockStorage) GetByID(ctx context.Context, id string) (*Session, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}
	return s, nil
}

func (m *mockStorage) UpdateStatus(ctx context.Context, id, status string, extra ...time.Time) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status, extra...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话不存在")
	}
	s.Status = status
	if len(extra) > 0 {
		if status == StatusRunning {
			s.StartedAt = &extra[0]
		} else if status == StatusEnded || status == StatusFailed {
			s.EndedAt = &extra[0]
		}
	}
	s.UpdatedAt = time.Now()
	return nil
}

func (m *mockStorage) Update(ctx context.Context, s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[s.ID]; !ok {
		return fmt.Errorf("会话不存在")
	}
	m.sessions[s.ID] = s
	return nil
}

func (m *mockStorage) ListByWorkspace(ctx context.Context, workspaceID string) ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Session
	for _, s := range m.sessions {
		if s.WorkspaceID == workspaceID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockStorage) AddSessionRole(ctx context.Context, sr *SessionRole) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionRoles[sr.SessionID] = append(m.sessionRoles[sr.SessionID], sr)
	return nil
}

func (m *mockStorage) GetSessionRoles(ctx context.Context, sessionID string) ([]*SessionRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionRoles[sessionID], nil
}

func (m *mockStorage) RemoveSessionRole(ctx context.Context, sessionID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	roles := m.sessionRoles[sessionID]
	for i, sr := range roles {
		if sr.RoleID == roleID {
			m.sessionRoles[sessionID] = append(roles[:i], roles[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockStorage) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

// ===== Mock RoleProvider =====

type mockRoleProvider struct {
	roles map[string]*role.Role
}

func newMockRoleProvider() *mockRoleProvider {
	return &mockRoleProvider{
		roles: make(map[string]*role.Role),
	}
}

func (m *mockRoleProvider) Get(ctx context.Context, id string) (*role.Role, error) {
	r, ok := m.roles[id]
	if !ok {
		return nil, fmt.Errorf("角色不存在")
	}
	return r, nil
}

func (m *mockRoleProvider) addRole(r *role.Role) {
	m.roles[r.ID] = r
}

// ===== Service 单元测试 =====

func setupTestService() (*Service, *mockStorage, *mockRoleProvider) {
	storage := newMockStorage()
	roleProvider := newMockRoleProvider()
	graphPool := NewGraphPool(10)
	svc := NewService(storage, graphPool, roleProvider)
	return svc, storage, roleProvider
}

func TestNewService(t *testing.T) {
	svc, _, _ := setupTestService()
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.runtimeChans)
	assert.Equal(t, 0, len(svc.runtimeChans))
}

func TestService_Create_Success(t *testing.T) {
	svc, storage, roleProvider := setupTestService()

	// 添加测试角色
	roleProvider.addRole(&role.Role{ID: "role_1"})
	roleProvider.addRole(&role.Role{ID: "role_2"})

	req := &CreateRequest{
		WorkspaceID: "ws_1",
		Title:       "测试会话",
		Paradigm:    "round_robin",
		RoleIDs:     []string{"role_1", "role_2"},
	}

	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Session.ID)
	assert.Equal(t, "draft", resp.Session.Status)
	assert.Equal(t, "ws_1", resp.Session.WorkspaceID)
	assert.Equal(t, 2, len(resp.SessionRoles))

	// 验证存储中有数据
	sess, err := storage.GetByID(context.Background(), resp.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, "测试会话", sess.Title)
}

func TestService_Create_ValidationErrors(t *testing.T) {
	svc, _, _ := setupTestService()
	ctx := context.Background()

	tests := []struct {
		name string
		req  *CreateRequest
	}{
		{name: "空标题", req: &CreateRequest{WorkspaceID: "ws_1", RoleIDs: []string{"r1"}}},
		{name: "空工作区", req: &CreateRequest{Title: "test", RoleIDs: []string{"r1"}}},
		{name: "空角色列表", req: &CreateRequest{WorkspaceID: "ws_1", Title: "test", RoleIDs: []string{}}},
		{name: "非法范式", req: &CreateRequest{WorkspaceID: "ws_1", Title: "test", Paradigm: "invalid", RoleIDs: []string{"r1"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, "user_1", tt.req)
			assert.Error(t, err)
		})
	}
}

func TestService_Create_RoleNotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	req := &CreateRequest{
		WorkspaceID: "ws_1",
		Title:       "测试",
		RoleIDs:     []string{"non_existent_role"},
	}

	_, err := svc.Create(context.Background(), "user_1", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestService_Create_DefaultValues(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{
		WorkspaceID: "ws_1",
		Title:       "默认值测试",
		RoleIDs:     []string{"role_1"},
	}

	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	assert.Equal(t, "round_robin", resp.Session.Paradigm)
	assert.Equal(t, 10, resp.Session.MaxRounds)
	assert.Equal(t, 300, resp.Session.RoundTimeout)
	assert.Equal(t, json.RawMessage(`{}`), resp.Session.Config)
}

func TestService_Get_Success(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	// 先创建一个会话
	req := &CreateRequest{
		WorkspaceID: "ws_1",
		Title:       "测试获取",
		RoleIDs:     []string{"role_1"},
	}
	createResp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// 通过 Get 获取
	sess, err := svc.Get(context.Background(), createResp.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, createResp.Session.ID, sess.ID)
}

func TestService_Get_NotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Get(context.Background(), "non_existent")
	assert.Error(t, err)
}

func TestService_GetWithRoles_Success(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})
	roleProvider.addRole(&role.Role{ID: "role_2"})

	req := &CreateRequest{
		WorkspaceID: "ws_1",
		Title:       "测试获取带角色",
		RoleIDs:     []string{"role_1", "role_2"},
	}
	createResp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	sess, roles, err := svc.GetWithRoles(context.Background(), createResp.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, createResp.Session.ID, sess.ID)
	assert.Equal(t, 2, len(roles))
}

func TestService_GetWithRoles_NotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	_, _, err := svc.GetWithRoles(context.Background(), "non_existent")
	assert.Error(t, err)
}

func TestService_ListByWorkspace(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	// 创建 2 个会话在 ws_1，1 个在 ws_2
	req1 := &CreateRequest{WorkspaceID: "ws_1", Title: "s1", RoleIDs: []string{"role_1"}}
	req2 := &CreateRequest{WorkspaceID: "ws_1", Title: "s2", RoleIDs: []string{"role_1"}}
	req3 := &CreateRequest{WorkspaceID: "ws_2", Title: "s3", RoleIDs: []string{"role_1"}}

	_, _ = svc.Create(context.Background(), "user_1", req1)
	_, _ = svc.Create(context.Background(), "user_1", req2)
	_, _ = svc.Create(context.Background(), "user_1", req3)

	sessions, err := svc.ListByWorkspace(context.Background(), "ws_1")
	require.NoError(t, err)
	assert.Equal(t, 2, len(sessions))

	sessions, err = svc.ListByWorkspace(context.Background(), "ws_3")
	require.NoError(t, err)
	assert.Equal(t, 0, len(sessions))
}

func TestService_Start_Success(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "启动测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	err = svc.Start(context.Background(), resp.Session.ID)
	assert.NoError(t, err)

	sess, _ := svc.Get(context.Background(), resp.Session.ID)
	assert.Equal(t, StatusRunning, sess.Status)
}

func TestService_Start_InvalidTransition(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// draft → ended → 尝试从 ended 启动应该失败
	_ = svc.Terminate(context.Background(), resp.Session.ID)
	err = svc.Start(context.Background(), resp.Session.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非法状态转换")
}

func TestService_Start_NotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	err := svc.Start(context.Background(), "non_existent")
	assert.Error(t, err)
}

func TestService_Pause_Success(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "暂停测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// draft → running → paused
	_ = svc.Start(context.Background(), resp.Session.ID)
	err = svc.Pause(context.Background(), resp.Session.ID, "test_node", "暂停消息")
	assert.NoError(t, err)

	sess, _ := svc.Get(context.Background(), resp.Session.ID)
	assert.Equal(t, StatusPaused, sess.Status)
}

func TestService_Pause_FromDraft(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// 从 draft 直接 pause 应失败
	err = svc.Pause(context.Background(), resp.Session.ID, "test", "msg")
	assert.Error(t, err)
}

func TestService_Resume_Success(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "恢复测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// draft → running → paused → running
	_ = svc.Start(context.Background(), resp.Session.ID)
	_ = svc.Pause(context.Background(), resp.Session.ID, "test", "暂停")
	err = svc.Resume(context.Background(), resp.Session.ID, "恢复消息")
	assert.NoError(t, err)

	sess, _ := svc.Get(context.Background(), resp.Session.ID)
	assert.Equal(t, StatusRunning, sess.Status)
}

func TestService_Resume_FromEnded(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// 先终止，然后从 ended 恢复应失败
	_ = svc.Terminate(context.Background(), resp.Session.ID)
	err = svc.Resume(context.Background(), resp.Session.ID, "msg")
	assert.Error(t, err)
}

func TestService_Terminate_Success(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "终止测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// draft → terminated
	err = svc.Terminate(context.Background(), resp.Session.ID)
	assert.NoError(t, err)

	sess, _ := svc.Get(context.Background(), resp.Session.ID)
	assert.Equal(t, StatusEnded, sess.Status)
}

func TestService_Terminate_Running(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// Kick off session and then terminate
	_ = svc.Start(context.Background(), resp.Session.ID)
	err = svc.Terminate(context.Background(), resp.Session.ID)
	assert.NoError(t, err)

	// Verify the running session was terminated
	sess, _ := svc.Get(context.Background(), resp.Session.ID)
	assert.Equal(t, StatusEnded, sess.Status)
}

func TestService_Terminate_NotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	err := svc.Terminate(context.Background(), "non_existent")
	assert.Error(t, err)
}

func TestService_InitAndRemoveChannels(t *testing.T) {
	svc, _, _ := setupTestService()

	// InitChannels
	ch := svc.InitChannels("session_chan_test")
	require.NotNil(t, ch)
	assert.NotNil(t, ch.InterruptCh)
	assert.NotNil(t, ch.ResumeCh)

	// GetChannels
	gotCh, ok := svc.GetChannels("session_chan_test")
	assert.True(t, ok)
	assert.Equal(t, ch, gotCh)

	// RemoveChannels
	svc.RemoveChannels("session_chan_test")
	_, ok = svc.GetChannels("session_chan_test")
	assert.False(t, ok)
}

func TestService_InitChannels_Twice(t *testing.T) {
	svc, _, _ := setupTestService()

	ch1 := svc.InitChannels("session_double")
	ch2 := svc.InitChannels("session_double")
	assert.Equal(t, ch1, ch2, "重复 Init 应返回同一个实例")
}

// TestService_Pause_WithChannels 测试 Pause 时通过 channel 发送中断信号
func TestService_Pause_WithChannels(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "channel 暂停测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// 先 InitChannels
	ch := svc.InitChannels(resp.Session.ID)

	// 启动并暂停
	_ = svc.Start(context.Background(), resp.Session.ID)

	// 异步 goroutine 接收中断信号
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case sig := <-ch.InterruptCh:
			assert.Equal(t, "test_node", sig.NodeName)
			assert.Equal(t, "暂停消息", sig.Message)
		case <-time.After(time.Second):
		}
	}()

	err = svc.Pause(context.Background(), resp.Session.ID, "test_node", "暂停消息")
	assert.NoError(t, err)
	wg.Wait()
}

// TestService_Resume_WithChannels 测试 Resume 时通过 channel 发送恢复信号
func TestService_Resume_WithChannels(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "channel 恢复测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	ch := svc.InitChannels(resp.Session.ID)

	_ = svc.Start(context.Background(), resp.Session.ID)
	_ = svc.Pause(context.Background(), resp.Session.ID, "test", "暂停")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case sig := <-ch.ResumeCh:
			assert.Equal(t, "恢复消息", sig.Message)
		case <-time.After(time.Second):
		}
	}()

	err = svc.Resume(context.Background(), resp.Session.ID, "恢复消息")
	assert.NoError(t, err)
	wg.Wait()
}

// TestService_ConcurrentChannels 并发 Init/Get/Remove channels
func TestService_ConcurrentChannels(t *testing.T) {
	svc, _, _ := setupTestService()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sessionID := "session_concurrent"
			_ = svc.InitChannels(sessionID)
			_, _ = svc.GetChannels(sessionID)
			svc.RemoveChannels(sessionID)
		}(i)
	}
	wg.Wait()
	// 不应 panic
}

// TestService_StorageError 测试存储层返回错误时的处理
func TestService_Create_StorageError(t *testing.T) {
	svc, storage, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	// 让存储层返回错误
	storage.createFn = func(ctx context.Context, s *Session) error {
		return errors.New("数据库错误")
	}

	req := &CreateRequest{
		WorkspaceID: "ws_1",
		Title:       "存储错误测试",
		RoleIDs:     []string{"role_1"},
	}
	_, err := svc.Create(context.Background(), "user_1", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "创建会话失败")
}

// TestService_Terminate_ClearsGraphPool 验证 Terminate 会清理 GraphPool
func TestService_Terminate_ClearsGraphPool(t *testing.T) {
	svc, _, roleProvider := setupTestService()
	roleProvider.addRole(&role.Role{ID: "role_1"})

	req := &CreateRequest{WorkspaceID: "ws_1", Title: "清理测试", RoleIDs: []string{"role_1"}}
	resp, err := svc.Create(context.Background(), "user_1", req)
	require.NoError(t, err)

	// 在 GraphPool 中添加实例
	svc.graphPool.Add(&GraphInstance{SessionID: resp.Session.ID})
	assert.Equal(t, 1, svc.graphPool.Len())

	_ = svc.Terminate(context.Background(), resp.Session.ID)

	// GraphPool 应已被清理
	_, ok := svc.graphPool.Get(resp.Session.ID)
	assert.False(t, ok)
}

func TestService_GetByID_StorageError(t *testing.T) {
	svc, storage, _ := setupTestService()
	storage.getByIDFn = func(ctx context.Context, id string) (*Session, error) {
		return nil, errors.New("存储错误")
	}

	_, err := svc.Get(context.Background(), "any_id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "存储错误")
}

package validation

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wjames2000/mmcs/internal/agent"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/task"
)

// ===== Mocks =====

// mockChatModel 实现 model_gateway.ChatModel 接口
type mockChatModel struct {
	response    string
	tokens      int
	generateErr error
}

func (m *mockChatModel) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	tokens := m.tokens
	if tokens == 0 {
		tokens = 42
	}
	return &model_gateway.ChatResponse{
		Content:     m.response,
		TotalTokens: tokens,
		Model:       "mock-model",
	}, nil
}

func (m *mockChatModel) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	ch := make(chan *model_gateway.StreamChunk, 1)
	ch <- &model_gateway.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}

// mockTaskService 实现 TaskService 接口
type mockTaskService struct {
	mu           sync.Mutex
	getFn        func(ctx context.Context, id string) (*task.Task, error)
	updateStatus func(ctx context.Context, id string, status task.Status) error
	assignFn     func(ctx context.Context, taskID, agentID, assignedBy string) error
	updateFn     func(ctx context.Context, id string, updates map[string]interface{}) (*task.Task, error)
}

func (m *mockTaskService) Get(ctx context.Context, id string) (*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getFn(ctx, id)
}

func (m *mockTaskService) UpdateStatus(ctx context.Context, id string, status task.Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateStatus(ctx, id, status)
}

func (m *mockTaskService) Assign(ctx context.Context, taskID, agentID, assignedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assignFn(ctx, taskID, agentID, assignedBy)
}

func (m *mockTaskService) Update(ctx context.Context, id string, updates map[string]interface{}) (*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateFn != nil {
		return m.updateFn(ctx, id, updates)
	}
	return nil, nil
}

// capturedEvent 用于验证 SSE 事件的捕获
type capturedEvent struct {
	Type string
	Data interface{}
}

// subscribeHub 订阅 Hub 并返回事件 channel
func subscribeHub(hub *stream.Hub) chan *stream.Event {
	sub := &stream.Subscriber{
		ID:      stream.NewSubscriberID(),
		Events:  make(chan *stream.Event, 10),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)
	return sub.Events
}

// collectEvents 收集订阅的 SSE 事件（带超时）
func collectEvents(eventCh chan *stream.Event, timeout time.Duration) []*stream.Event {
	var events []*stream.Event
	timer := time.After(timeout)
	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				return events
			}
			events = append(events, evt)
		case <-timer:
			return events
		}
	}
}

// ===== Helpers =====

func makeVerdictJSON(verdict, reason string, items []string) string {
	type checkedItem struct {
		Item   string `json:"item"`
		Passed bool   `json:"passed"`
		Note   string `json:"note"`
	}
	checked := make([]checkedItem, len(items))
	for i, item := range items {
		checked[i] = checkedItem{
			Item:   item,
			Passed: verdict == "passed",
			Note:   "验证通过",
		}
	}
	data, _ := json.Marshal(map[string]interface{}{
		"verdict":       verdict,
		"reason":        reason,
		"checked_items": checked,
	})
	return string(data)
}

func newTestTask(id, sessionID, title, description, criteria, agentID string, status task.Status) *task.Task {
	return &task.Task{
		ID:                 id,
		SessionID:          sessionID,
		WorkspaceID:        "w_test",
		Title:              title,
		Description:        description,
		AcceptanceCriteria: criteria,
		Status:             status,
		Priority:           task.PriorityMedium,
		AssignedAgent:      agentID,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
}

// ===== Tests =====

func TestNewValidatorAgent(t *testing.T) {
	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("validator_test", model)
	assert.NotNil(t, va)
	assert.Equal(t, "validator_test", va.ID())
}

func TestValidatorAgent_ImplementsAgentInterface(t *testing.T) {
	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("validator_test", model)
	// 验证实现了 agent.Agent 接口
	var _ agent.Agent = va
	assert.NotNil(t, va)
}

func TestValidatorAgent_Run_Passed(t *testing.T) {
	model := &mockChatModel{
		response: makeVerdictJSON("passed", "所有验收标准已满足", []string{"登录功能", "权限检查"}),
		tokens:   50,
	}
	va := NewValidatorAgent("validator_passed", model)

	input := "任务: 实现登录\n描述: JWT 登录\n验收标准: 登录成功返回token\n执行结果: 已完成"
	result, err := va.Run(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "validator_passed", result.AgentID)
	assert.Contains(t, result.Output, `"passed"`)
	assert.Equal(t, 50, result.Tokens)
	assert.True(t, result.Duration > 0)
}

func TestValidatorAgent_Run_NeedsRevision(t *testing.T) {
	model := &mockChatModel{
		response: makeVerdictJSON("needs_revision", "验收标准第2条未通过", []string{"登录功能", "权限检查"}),
	}
	va := NewValidatorAgent("validator_revision", model)

	result, err := va.Run(context.Background(), "test input")
	require.NoError(t, err)
	assert.Contains(t, result.Output, `"needs_revision"`)
	assert.Contains(t, result.Output, "验收标准第2条未通过")
}

func TestValidatorAgent_Run_Rejected(t *testing.T) {
	model := &mockChatModel{
		response: makeVerdictJSON("rejected", "执行结果完全不满足验收标准", []string{"登录功能", "权限检查"}),
	}
	va := NewValidatorAgent("validator_rejected", model)

	result, err := va.Run(context.Background(), "test input")
	require.NoError(t, err)
	assert.Contains(t, result.Output, `"rejected"`)
}

func TestValidatorAgent_Run_ModelError(t *testing.T) {
	model := &mockChatModel{generateErr: errors.New("api timeout")}
	va := NewValidatorAgent("validator_err", model)

	_, err := va.Run(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api timeout")
}

func TestValidatorAgent_Run_InvalidJSON(t *testing.T) {
	model := &mockChatModel{response: "这不是合法 JSON"}
	va := NewValidatorAgent("validator_badjson", model)

	result, err := va.Run(context.Background(), "test")
	require.NoError(t, err)
	// 即使返回非 JSON，也应该正常返回，由调用方解析
	assert.Equal(t, "这不是合法 JSON", result.Output)
}

func TestNewService(t *testing.T) {
	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("validator", model)
	ts := &mockTaskService{}
	hubReg := stream.NewHubRegistry()

	svc := NewService(va, ts, hubReg)
	assert.NotNil(t, svc)
}

func TestNewService_NilDeps(t *testing.T) {
	assert.Panics(t, func() {
		NewService(nil, nil, nil)
	})
	assert.Panics(t, func() {
		model := &mockChatModel{response: "{}"}
		va := NewValidatorAgent("v", model)
		NewService(va, nil, nil)
	})
}

func TestValidateTask_Passed(t *testing.T) {
	// 准备
	model := &mockChatModel{
		response: makeVerdictJSON("passed", "全部通过", []string{"登录功能"}),
	}
	va := NewValidatorAgent("val_pass", model)

	taskID := "t_pass123"
	sessionID := "s_test"
	taskObj := newTestTask(taskID, sessionID, "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusInProgress)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			taskObj.Status = status
			now := time.Now()
			taskObj.CompletedAt = &now
			return nil
		},
		assignFn: func(ctx context.Context, taskID, agentID, assignedBy string) error {
			return nil
		},
	}

	hubReg := stream.NewHubRegistry()
	hub := hubReg.GetOrCreate(sessionID)
	eventCh := subscribeHub(hub)

	svc := NewService(va, ts, hubReg)

	// 执行
	result, err := svc.ValidateTask(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "passed", result.Verdict)
	assert.Equal(t, taskID, result.TaskID)
	assert.Equal(t, "val_pass", result.Validator)
	assert.NotZero(t, result.CreatedAt)

	// 验证任务状态
	assert.Equal(t, task.StatusCompleted, taskObj.Status)
	assert.NotNil(t, taskObj.CompletedAt)

	// 验证 SSE 事件
	events := collectEvents(eventCh, 200*time.Millisecond)
	require.Len(t, events, 1)
	assert.Equal(t, stream.EventTypeTaskUpdated, events[0].Type)
}

func TestValidateTask_NeedsRevision(t *testing.T) {
	// 准备
	model := &mockChatModel{
		response: makeVerdictJSON("needs_revision", "权限检查逻辑不完整", []string{"登录功能", "权限检查"}),
	}
	va := NewValidatorAgent("val_rev", model)

	taskID := "t_rev123"
	sessionID := "s_test"
	taskObj := newTestTask(taskID, sessionID, "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusInProgress)

	updateStatusCalls := make([]task.Status, 0)
	assignCalled := false

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			updateStatusCalls = append(updateStatusCalls, status)
			taskObj.Status = status
			return nil
		},
		assignFn: func(ctx context.Context, taskID, agentID, assignedBy string) error {
			assignCalled = true
			assert.Equal(t, "agent_1", agentID)
			return nil
		},
		updateFn: func(ctx context.Context, id string, updates map[string]interface{}) (*task.Task, error) {
			if desc, ok := updates["description"].(string); ok {
				taskObj.Description = desc
			}
			return taskObj, nil
		},
	}

	hubReg := stream.NewHubRegistry()
	hub := hubReg.GetOrCreate(sessionID)
	eventCh := subscribeHub(hub)

	svc := NewService(va, ts, hubReg)

	// 执行
	result, err := svc.ValidateTask(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "needs_revision", result.Verdict)
	assert.Equal(t, taskID, result.TaskID)

	// 验证退回重试被触发：状态回到 in_progress
	assert.Equal(t, task.StatusInProgress, taskObj.Status)
	assert.Contains(t, updateStatusCalls, task.StatusReviewing)
	assert.Contains(t, updateStatusCalls, task.StatusInProgress)
	assert.True(t, assignCalled)

	// 验证 description 追加了修订意见
	assert.Contains(t, taskObj.Description, "修订意见")

	// 验证 SSE 事件
	events := collectEvents(eventCh, 200*time.Millisecond)
	require.Len(t, events, 1)
	assert.Equal(t, stream.EventTypeTaskUpdated, events[0].Type)
}

func TestValidateTask_Rejected(t *testing.T) {
	// 准备
	model := &mockChatModel{
		response: makeVerdictJSON("rejected", "执行结果严重偏离需求", []string{"登录功能", "权限检查"}),
	}
	va := NewValidatorAgent("val_rej", model)

	taskID := "t_rej123"
	sessionID := "s_test"
	taskObj := newTestTask(taskID, sessionID, "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusInProgress)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			assert.Equal(t, task.StatusRejected, status)
			taskObj.Status = status
			return nil
		},
		assignFn: func(ctx context.Context, taskID, agentID, assignedBy string) error {
			return nil
		},
	}

	hubReg := stream.NewHubRegistry()
	hub := hubReg.GetOrCreate(sessionID)
	eventCh := subscribeHub(hub)

	svc := NewService(va, ts, hubReg)

	// 执行
	result, err := svc.ValidateTask(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "rejected", result.Verdict)
	assert.Equal(t, taskID, result.TaskID)

	// 验证任务状态
	assert.Equal(t, task.StatusRejected, taskObj.Status)

	// 验证 SSE 事件
	events := collectEvents(eventCh, 200*time.Millisecond)
	require.Len(t, events, 1)
	assert.Equal(t, stream.EventTypeTaskUpdated, events[0].Type)
}

func TestValidateNonExistentTask(t *testing.T) {
	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("val_nf", model)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return nil, errors.New("任务不存在")
		},
	}

	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	_, err := svc.ValidateTask(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestValidateTask_AgentError(t *testing.T) {
	model := &mockChatModel{generateErr: errors.New("model unavailable")}
	va := NewValidatorAgent("val_err", model)

	taskID := "t_err123"
	taskObj := newTestTask(taskID, "s_test", "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusInProgress)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
	}

	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	_, err := svc.ValidateTask(context.Background(), taskID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model unavailable")
}

func TestValidateTask_StatusInProgress(t *testing.T) {
	// 只有 in_progress 和 reviewing 状态的任务才能被验证
	model := &mockChatModel{
		response: makeVerdictJSON("passed", "ok", []string{"test"}),
	}
	va := NewValidatorAgent("val_sip", model)

	taskID := "t_sip123"
	taskObj := newTestTask(taskID, "s_test", "测试", "desc", "criteria", "agent_1", task.StatusPending)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
	}

	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	_, err := svc.ValidateTask(context.Background(), taskID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "状态不正确")
}

func TestHandleRetry(t *testing.T) {
	// 准备一个 needs_revision 场景
	taskID := "t_retry123"
	taskObj := newTestTask(taskID, "s_test", "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusReviewing)

	assignCalled := false

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			taskObj.Status = status
			return nil
		},
		assignFn: func(ctx context.Context, taskID, agentID, assignedBy string) error {
			assignCalled = true
			assert.Equal(t, "agent_1", agentID)
			return nil
		},
		updateFn: func(ctx context.Context, id string, updates map[string]interface{}) (*task.Task, error) {
			if desc, ok := updates["description"].(string); ok {
				taskObj.Description = desc
			}
			return taskObj, nil
		},
	}

	result := &task.ValidationResult{
		ID:        "vr_retry",
		TaskID:    taskID,
		Validator: "val_retry",
		Verdict:   "needs_revision",
		Reason:    "权限检查逻辑不完整",
		Detail: map[string]any{
			"checked_items": []map[string]any{
				{"item": "登录功能", "passed": true, "note": "通过"},
				{"item": "权限检查", "passed": false, "note": "缺少管理员权限校验"},
			},
		},
		CreatedAt: time.Now(),
	}

	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("val_retry", model)
	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	err := svc.HandleRetry(context.Background(), taskObj, result)
	require.NoError(t, err)

	// 验证状态变为 in_progress
	assert.Equal(t, task.StatusInProgress, taskObj.Status)
	assert.True(t, assignCalled)

	// 验证 description 追加了修订意见
	assert.Contains(t, taskObj.Description, "修订意见")
	assert.Contains(t, taskObj.Description, "权限检查逻辑不完整")
}

func TestHandleRetry_RejectedTask(t *testing.T) {
	// rejected 不应触发退回重试
	taskID := "t_rejected_no_retry"
	taskObj := newTestTask(taskID, "s_test", "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusRejected)

	updateStatusCalled := false
	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			updateStatusCalled = true
			return nil
		},
	}

	result := &task.ValidationResult{
		ID:        "vr_rej",
		TaskID:    taskID,
		Validator: "val_rej",
		Verdict:   "rejected",
		Reason:    "完全不通过",
		CreatedAt: time.Now(),
	}

	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("val_rej", model)
	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	err := svc.HandleRetry(context.Background(), taskObj, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已被拒绝")
	assert.False(t, updateStatusCalled)
}

func TestHandleRetry_CompletedTask(t *testing.T) {
	taskID := "t_completed"
	taskObj := newTestTask(taskID, "s_test", "实现登录", "JWT登录", "登录成功返回token", "agent_1", task.StatusCompleted)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
	}
	result := &task.ValidationResult{
		ID:      "vr_completed",
		TaskID:  taskID,
		Verdict: "needs_revision",
	}

	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("val", model)
	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	err := svc.HandleRetry(context.Background(), taskObj, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "终止状态")
}

func TestAsyncValidation(t *testing.T) {
	model := &mockChatModel{
		response: makeVerdictJSON("passed", "全部通过", []string{"功能测试"}),
	}
	va := NewValidatorAgent("val_async", model)

	taskID := "t_async123"
	sessionID := "s_async"
	taskObj := newTestTask(taskID, sessionID, "异步测试", "测试异步验证", "功能正常运行", "agent_1", task.StatusInProgress)

	var mu sync.Mutex
	done := make(chan struct{})

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			mu.Lock()
			taskObj.Status = status
			if status == task.StatusCompleted {
				now := time.Now()
				taskObj.CompletedAt = &now
				close(done)
			}
			mu.Unlock()
			return nil
		},
		assignFn: func(ctx context.Context, taskID, agentID, assignedBy string) error {
			return nil
		},
	}

	hubReg := stream.NewHubRegistry()
	hub := hubReg.GetOrCreate(sessionID)
	eventCh := subscribeHub(hub)

	svc := NewService(va, ts, hubReg)

	// 异步触发验证
	err := svc.ValidateTaskAsync(context.Background(), taskID)
	require.NoError(t, err)

	// 等待异步完成（或超时）
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("异步验证超时")
	}

	// 验证任务最终状态
	mu.Lock()
	assert.Equal(t, task.StatusCompleted, taskObj.Status)
	assert.NotNil(t, taskObj.CompletedAt)
	mu.Unlock()

	// 验证 SSE 事件
	events := collectEvents(eventCh, 200*time.Millisecond)
	require.Len(t, events, 1)
	assert.Equal(t, stream.EventTypeTaskUpdated, events[0].Type)
}

func TestValidatorAgentID(t *testing.T) {
	model := &mockChatModel{response: "{}"}
	va := NewValidatorAgent("custom_id", model)
	assert.Equal(t, "custom_id", va.ID())

	va2 := NewValidatorAgent("", model)
	assert.Equal(t, "validator_default", va2.ID())
}

func TestBuildValidationInput(t *testing.T) {
	input := BuildValidationInput("测试任务", "任务描述", "验收标准1", "执行完成")
	assert.Contains(t, input, "测试任务")
	assert.Contains(t, input, "任务描述")
	assert.Contains(t, input, "验收标准1")
	assert.Contains(t, input, "执行完成")
	assert.Contains(t, input, "你是一个任务验证官")
}

func TestParseVerdict_ValidJSON(t *testing.T) {
	verdict, reason, detail := parseVerdict(`{"verdict":"passed","reason":"全部通过","checked_items":[{"item":"test","passed":true}]}`)
	assert.Equal(t, "passed", verdict)
	assert.Equal(t, "全部通过", reason)
	assert.NotNil(t, detail["checked_items"])
}

func TestParseVerdict_InvalidJSON(t *testing.T) {
	verdict, reason, detail := parseVerdict("这不是 JSON")
	assert.Equal(t, "needs_revision", verdict)
	assert.Equal(t, "这不是 JSON", reason)
	assert.Empty(t, detail)
}

func TestParseVerdict_EmptyReason(t *testing.T) {
	verdict, reason, _ := parseVerdict(`{"verdict":"passed"}`)
	assert.Equal(t, "passed", verdict)
	assert.Equal(t, "无详细理由", reason)
}

func TestParseVerdict_InvalidVerdict(t *testing.T) {
	verdict, _, _ := parseVerdict(`{"verdict":"unknown"}`)
	assert.Equal(t, "needs_revision", verdict)
}

func TestPrepareRetry(t *testing.T) {
	taskObj := newTestTask("t_prep", "s_test", "测试", "原始描述", "标准", "agent_1", task.StatusReviewing)
	result := &task.ValidationResult{
		Verdict: "needs_revision",
		Reason:  "需要修改登录逻辑",
	}

	updatedDesc, err := task.PrepareRetry(taskObj, result)
	require.NoError(t, err)
	assert.Contains(t, updatedDesc, "原始描述")
	assert.Contains(t, updatedDesc, "修订意见")
	assert.Contains(t, updatedDesc, "需要修改登录逻辑")
	assert.Equal(t, updatedDesc, taskObj.Description)
}

func TestPrepareRetry_RejectedVerdict(t *testing.T) {
	taskObj := newTestTask("t_prep2", "s_test", "测试", "描述", "标准", "agent_1", task.StatusRejected)
	result := &task.ValidationResult{
		Verdict: "rejected",
		Reason:  "完全不行",
	}

	_, err := task.PrepareRetry(taskObj, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "只有 needs_revision")
}

func TestIsRetryable(t *testing.T) {
	assert.True(t, task.IsRetryable(newTestTask("t1", "s", "t", "d", "c", "a", task.StatusInProgress)))
	assert.True(t, task.IsRetryable(newTestTask("t2", "s", "t", "d", "c", "a", task.StatusReviewing)))
	assert.False(t, task.IsRetryable(newTestTask("t3", "s", "t", "d", "c", "a", task.StatusRejected)))
	assert.False(t, task.IsRetryable(newTestTask("t4", "s", "t", "d", "c", "a", task.StatusCompleted)))
	assert.False(t, task.IsRetryable(nil))
}

func TestValidateTask_SessionIDEmpty(t *testing.T) {
	// 即使没有 sessionID（不广播 SSE），验证仍应正常进行
	model := &mockChatModel{
		response: makeVerdictJSON("passed", "ok", []string{"test"}),
	}
	va := NewValidatorAgent("val_no_sse", model)

	taskID := "t_no_sse"
	taskObj := newTestTask(taskID, "", "测试", "desc", "criteria", "agent_1", task.StatusInProgress)

	ts := &mockTaskService{
		getFn: func(ctx context.Context, id string) (*task.Task, error) {
			return taskObj, nil
		},
		updateStatus: func(ctx context.Context, id string, status task.Status) error {
			taskObj.Status = status
			return nil
		},
	}

	hubReg := stream.NewHubRegistry()
	svc := NewService(va, ts, hubReg)

	result, err := svc.ValidateTask(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, "passed", result.Verdict)
}

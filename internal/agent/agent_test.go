package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// ===== Mock 依赖 =====

// mockChatModel 实现 model_gateway.ChatModel 接口
type mockChatModel struct {
	response     string
	tokens       int
	generateErr  error
	generateHook func(req *model_gateway.ChatRequest)
}

func (m *mockChatModel) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	if m.generateHook != nil {
		m.generateHook(req)
	}
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

// mockAgent 实现 Agent 接口
type mockAgent struct {
	id       string
	runError error
}

func (m *mockAgent) ID() string { return m.id }

func (m *mockAgent) Run(ctx context.Context, input string) (*Result, error) {
	if m.runError != nil {
		return nil, m.runError
	}
	return &Result{
		AgentID:  m.id,
		Output:   "mock output for: " + input,
		Tokens:   10,
		Duration: time.Millisecond,
	}, nil
}

// mockTaskCreator 实现 TaskCreator 接口
type mockTaskCreator struct {
	createErr error
}

func (m *mockTaskCreator) CreateTask(ctx context.Context, title, description, assignee string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "task_mock123", nil
}

// mockQueryExecutor 实现 QueryExecutor 接口
type mockQueryExecutor struct {
	queryErr error
	results  []map[string]any
}

func (m *mockQueryExecutor) Query(ctx context.Context, sql string) ([]map[string]any, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.results == nil {
		m.results = []map[string]any{
			{"id": 1, "name": "test"},
		}
	}
	return m.results, nil
}

// ===== 测试用例 =====

func TestNewChatModelAgent(t *testing.T) {
	model := &mockChatModel{response: "hello"}
	agent := NewChatModelAgent("agent_test", model)
	assert.NotNil(t, agent)
	assert.Equal(t, "agent_test", agent.ID())
}

func TestChatModelAgent_Run(t *testing.T) {
	model := &mockChatModel{response: "Hello, I am an AI assistant.", tokens: 50}
	agent := NewChatModelAgent("agent_chat", model)

	result, err := agent.Run(context.Background(), "Who are you?")
	assert.NoError(t, err)
	assert.Equal(t, "agent_chat", result.AgentID)
	assert.Contains(t, result.Output, "Hello")
	assert.Equal(t, 50, result.Tokens)
	assert.True(t, result.Duration > 0)
}

func TestChatModelAgent_Run_Error(t *testing.T) {
	model := &mockChatModel{generateErr: errors.New("model error")}
	agent := NewChatModelAgent("agent_err", model)

	_, err := agent.Run(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model error")
}

func TestChatModelAgent_Run_VerifyMessages(t *testing.T) {
	var capturedReq *model_gateway.ChatRequest
	model := &mockChatModel{
		response: "ok",
		generateHook: func(req *model_gateway.ChatRequest) {
			capturedReq = req
		},
	}
	agent := NewChatModelAgent("agent_verify", model)

	_, err := agent.Run(context.Background(), "test input")
	assert.NoError(t, err)
	assert.NotNil(t, capturedReq)
	assert.Equal(t, 1, len(capturedReq.Messages))
	assert.Equal(t, "user", capturedReq.Messages[0].Role)
	assert.Equal(t, "test input", capturedReq.Messages[0].Content)
}

func TestNewSupervisorAgent(t *testing.T) {
	model := &mockChatModel{response: "agent_sub1"}
	sub1 := &mockAgent{id: "agent_sub1"}
	sub2 := &mockAgent{id: "agent_sub2"}

	sup := NewSupervisorAgent("supervisor_test", model, []Agent{sub1, sub2})
	assert.NotNil(t, sup)
	assert.Equal(t, "supervisor_test", sup.ID())
}

func TestSupervisorAgent_Run(t *testing.T) {
	model := &mockChatModel{response: "agent_sub1", tokens: 5}
	sub1 := &mockAgent{id: "agent_sub1"}
	sub2 := &mockAgent{id: "agent_sub2"}

	sup := NewSupervisorAgent("supervisor_run", model, []Agent{sub1, sub2})

	result, err := sup.Run(context.Background(), "execute task")
	assert.NoError(t, err)
	assert.Equal(t, "supervisor_run", result.AgentID)
	assert.Contains(t, result.Output, "mock output for: execute task")
}

func TestSupervisorAgent_NoSubAgents(t *testing.T) {
	model := &mockChatModel{response: "anything"}
	sup := NewSupervisorAgent("supervisor_empty", model, nil)

	_, err := sup.Run(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有注册任何子 Agent")
}

func TestSupervisorAgent_SelectedAgentNotFound(t *testing.T) {
	model := &mockChatModel{response: "nonexistent_agent"}
	sub := &mockAgent{id: "agent_real"}

	sup := NewSupervisorAgent("supervisor_bad", model, []Agent{sub})

	_, err := sup.Run(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未注册")
}

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor(nil)
	assert.NotNil(t, exec)
	assert.Empty(t, exec.List())
}

func TestExecutor_RegisterAndGet(t *testing.T) {
	exec := NewExecutor(nil)
	sub := &mockAgent{id: "agent_reg"}

	err := exec.Register(sub)
	assert.NoError(t, err)

	got, err := exec.Get("agent_reg")
	assert.NoError(t, err)
	assert.Equal(t, "agent_reg", got.ID())
}

func TestExecutor_RegisterDuplicate(t *testing.T) {
	exec := NewExecutor(nil)
	sub := &mockAgent{id: "agent_dup"}

	assert.NoError(t, exec.Register(sub))
	err := exec.Register(sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已注册")
}

func TestExecutor_GetNotFound(t *testing.T) {
	exec := NewExecutor(nil)
	_, err := exec.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未注册")
}

func TestExecutor_List(t *testing.T) {
	exec := NewExecutor(nil)
	exec.Register(&mockAgent{id: "a1"})
	exec.Register(&mockAgent{id: "a2"})
	exec.Register(&mockAgent{id: "a3"})

	ids := exec.List()
	assert.Equal(t, 3, len(ids))
	assert.Contains(t, ids, "a1")
	assert.Contains(t, ids, "a2")
	assert.Contains(t, ids, "a3")
}

func TestExecutor_Sync(t *testing.T) {
	exec := NewExecutor(nil)
	model := &mockChatModel{response: "sync result", tokens: 30}
	exec.Register(NewChatModelAgent("agent_sync", model))

	result, err := exec.ExecuteSync(context.Background(), "agent_sync", "sync test")
	assert.NoError(t, err)
	assert.Equal(t, "agent_sync", result.AgentID)
	assert.Contains(t, result.Output, "sync result")
	assert.Equal(t, 30, result.Tokens)
}

func TestExecutor_Sync_AgentNotFound(t *testing.T) {
	exec := NewExecutor(nil)
	_, err := exec.ExecuteSync(context.Background(), "nonexistent", "test")
	assert.Error(t, err)
}

func TestExecutor_Async_NoClient(t *testing.T) {
	exec := NewExecutor(nil)
	exec.Register(&mockAgent{id: "agent_async"})

	_, err := exec.ExecuteAsync(context.Background(), "agent_async", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未配置")
}

func TestExecutor_Async_AgentNotFound(t *testing.T) {
	exec := NewExecutor(nil)
	_, err := exec.ExecuteAsync(context.Background(), "nonexistent", "test")
	assert.Error(t, err)
}

func TestNewToolRegistry(t *testing.T) {
	reg := NewToolRegistry()
	assert.NotNil(t, reg)
	assert.Empty(t, reg.List())
}

func TestToolRegistry_RegisterAndGet(t *testing.T) {
	reg := NewToolRegistry()
	tool := NewCreateTaskTool(&mockTaskCreator{})

	err := reg.Register(tool)
	assert.NoError(t, err)

	got, err := reg.Get("create_task")
	assert.NoError(t, err)
	assert.Equal(t, "create_task", got.Name())
	assert.NotEmpty(t, got.Description())
}

func TestToolRegistry_RegisterDuplicate(t *testing.T) {
	reg := NewToolRegistry()
	tool := NewCreateTaskTool(&mockTaskCreator{})

	assert.NoError(t, reg.Register(tool))
	err := reg.Register(tool)
	assert.Error(t, err)
}

func TestToolRegistry_GetNotFound(t *testing.T) {
	reg := NewToolRegistry()
	_, err := reg.Get("nonexistent")
	assert.Error(t, err)
}

func TestToolRegistry_List(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(NewCreateTaskTool(&mockTaskCreator{}))
	reg.Register(NewQueryDatabaseTool(&mockQueryExecutor{}))
	reg.Register(NewExecuteCodeTool(5 * time.Second))

	names := reg.List()
	assert.Equal(t, 3, len(names))
}

func TestCreateTaskTool_Execute(t *testing.T) {
	tool := NewCreateTaskTool(&mockTaskCreator{})
	params := map[string]any{
		"title":       "测试任务",
		"description": "这是一条测试任务",
		"assignee":    "user_001",
	}

	result, err := tool.Execute(context.Background(), params)
	assert.NoError(t, err)

	resMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "created", resMap["status"])
	assert.Equal(t, "task_mock123", resMap["task_id"])
}

func TestCreateTaskTool_MissingTitle(t *testing.T) {
	tool := NewCreateTaskTool(&mockTaskCreator{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"description": "desc",
		"assignee":    "user",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "标题不能为空")
}

func TestCreateTaskTool_MissingAssignee(t *testing.T) {
	tool := NewCreateTaskTool(&mockTaskCreator{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"title":       "title",
		"description": "desc",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "负责人不能为空")
}

func TestCreateTaskTool_CreatorError(t *testing.T) {
	tool := NewCreateTaskTool(&mockTaskCreator{createErr: errors.New("db error")})
	_, err := tool.Execute(context.Background(), map[string]any{
		"title":    "title",
		"assignee": "user",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestQueryDatabaseTool_Execute(t *testing.T) {
	tool := NewQueryDatabaseTool(&mockQueryExecutor{})
	params := map[string]any{
		"sql": "SELECT * FROM users LIMIT 1",
	}

	result, err := tool.Execute(context.Background(), params)
	assert.NoError(t, err)

	resMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, resMap["count"])
}

func TestQueryDatabaseTool_EmptySQL(t *testing.T) {
	tool := NewQueryDatabaseTool(&mockQueryExecutor{})
	_, err := tool.Execute(context.Background(), map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SQL 语句不能为空")
}

func TestExecuteCodeTool_Execute(t *testing.T) {
	tool := NewExecuteCodeTool(5 * time.Second)
	params := map[string]any{
		"code":     "fmt.Println(\"hello\")",
		"language": "go",
	}

	result, err := tool.Execute(context.Background(), params)
	assert.NoError(t, err)

	resMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 0, resMap["exit_code"])
	assert.Contains(t, resMap["stdout"], "模拟输出")
}

func TestExecuteCodeTool_EmptyCode(t *testing.T) {
	tool := NewExecuteCodeTool(5 * time.Second)
	_, err := tool.Execute(context.Background(), map[string]any{
		"language": "go",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "代码内容不能为空")
}

func TestExecuteCodeTool_ContextCancel(t *testing.T) {
	tool := NewExecuteCodeTool(5 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tool.Execute(ctx, map[string]any{
		"code":     "test",
		"language": "go",
	})
	assert.Error(t, err)
}

func TestExecutor_ConcurrentSafety(t *testing.T) {
	exec := NewExecutor(nil)
	model := &mockChatModel{response: "concurrent", tokens: 10}

	// 注册一个 Agent
	agent := NewChatModelAgent("agent_conc", model)
	assert.NoError(t, exec.Register(agent))

	// 并发执行
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := exec.ExecuteSync(context.Background(), "agent_conc", "concurrent test")
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NoError(t, err)
	}
}

func TestExecutor_ConcurrentRegister(t *testing.T) {
	exec := NewExecutor(nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = exec.Register(&mockAgent{id: "agent_conc_reg"})
		}(i)
	}
	wg.Wait()

	// 应该至少注册成功一个
	_, err := exec.Get("agent_conc_reg")
	assert.NoError(t, err)
}

func TestMarshalUnmarshalPayload(t *testing.T) {
	original := &TaskPayload{
		TaskID:  "task_123",
		AgentID: "agent_test",
		Input:   "hello",
	}

	data, err := MarshalPayload(original)
	assert.NoError(t, err)

	restored, err := UnmarshalPayload(data)
	assert.NoError(t, err)
	assert.Equal(t, original.TaskID, restored.TaskID)
	assert.Equal(t, original.AgentID, restored.AgentID)
	assert.Equal(t, original.Input, restored.Input)
}

func TestToolRegistry_ThreadSafety(t *testing.T) {
	reg := NewToolRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_ = reg.Register(NewCreateTaskTool(&mockTaskCreator{}))
			} else {
				_, _ = reg.Get("create_task")
			}
		}(i)
	}
	wg.Wait()
}

func TestChatModelAgent_WithSystemPrompt(t *testing.T) {
	var capturedReq *model_gateway.ChatRequest
	model := &mockChatModel{
		response: "response with context",
		generateHook: func(req *model_gateway.ChatRequest) {
			capturedReq = req
		},
	}

	agent := NewChatModelAgent("agent_system", model)
	_, err := agent.Run(context.Background(), "user question")
	assert.NoError(t, err)
	assert.NotNil(t, capturedReq)
	// ChatModelAgent 只传 user 消息
	assert.Equal(t, "user", capturedReq.Messages[0].Role)
}

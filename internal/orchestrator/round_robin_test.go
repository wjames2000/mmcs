package orchestrator

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/pkg/util"
)

// ===== Mock 依赖 =====

// mockRoleService 实现 RoleServiceInterface
type mockRoleService struct {
	roles map[string]*role.Role
}

func newMockRoleService() *mockRoleService {
	return &mockRoleService{
		roles: make(map[string]*role.Role),
	}
}

func (m *mockRoleService) Get(ctx context.Context, id string) (*role.Role, error) {
	r, ok := m.roles[id]
	if !ok {
		return nil, assert.AnError
	}
	return r, nil
}

func (m *mockRoleService) addRole(r *role.Role) {
	m.roles[r.ID] = r
}

// mockGateway 实现 ModelGatewayInterface
type mockGateway struct {
	models map[string]model_gateway.ChatModel
}

func newMockGateway() *mockGateway {
	return &mockGateway{
		models: make(map[string]model_gateway.ChatModel),
	}
}

func (m *mockGateway) GetChatModel(binding string) (model_gateway.ChatModel, error) {
	model, ok := m.models[binding]
	if !ok {
		return nil, assert.AnError
	}
	return model, nil
}

// mockChatModel 实现 model_gateway.ChatModel 接口
type mockChatModel struct {
	response string
}

func (m *mockChatModel) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	return &model_gateway.ChatResponse{
		Content:     m.response,
		TotalTokens: 42,
		Model:       "mock-model",
	}, nil
}

func (m *mockChatModel) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	ch := make(chan *model_gateway.StreamChunk, 1)
	ch <- &model_gateway.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}

// ===== 测试 =====

func TestContextInitNode_InitRoleContexts(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 创建测试角色
	testRole := &role.Role{
		ID:            util.NewID("r"),
		Name:          "安全审查员",
		Title:         "Security Auditor",
		Traits:        json.RawMessage(`{"detail": 9}`),
		Expertise:     []string{"安全审计", "代码审查"},
		SpeakingStyle: "严谨专业",
		SystemPrompt:  "请从安全角度审查代码。",
		Skills:        []string{"security-audit"},
		IsGlobal:      false,
	}
	svc.addRole(testRole)

	// 注册模拟模型
	mockModel := &mockChatModel{response: "这是一段安全审查意见。"}
	gw.models["openai"] = mockModel

	node := NewContextInitNode(svc, skillRegistry, gw)
	roleContexts, err := node.InitRoleContexts(context.Background(), []string{testRole.ID}, "自定义补充指令")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(roleContexts))
	assert.Equal(t, "安全审查员", roleContexts[0].Role.Name)
	assert.Contains(t, roleContexts[0].Prompt, "安全审查员")
	assert.Contains(t, roleContexts[0].Prompt, "安全审计")
	assert.Contains(t, roleContexts[0].Prompt, "自定义补充指令")
}

func TestExpertSpeakNode_Execute(t *testing.T) {
	mockModel := &mockChatModel{response: "专家发言测试内容"}

	roleContexts := []*RoleContext{
		{
			Role: &role.Role{
				ID:    util.NewID("r"),
				Name:  "专家A",
				Title: "Expert A",
			},
			ChatModel: mockModel,
			Prompt:    "你是专家A。",
		},
		{
			Role: &role.Role{
				ID:    util.NewID("r"),
				Name:  "专家B",
				Title: "Expert B",
			},
			ChatModel: mockModel,
			Prompt:    "你是专家B。",
		},
	}

	state := NewDiscussionState("s_test_expert", 10, nil)
	state.Roles = roleContexts

	node := NewExpertSpeakNode()
	results := node.Execute(context.Background(), roleContexts, "测试话题", state)

	assert.Equal(t, 2, len(results))
	for _, res := range results {
		assert.NoError(t, res.Error)
		assert.NotEmpty(t, res.Content)
		assert.Equal(t, 42, res.Tokens)
	}
}

func TestExpertSpeakNode_ParallelExecution(t *testing.T) {
	roleContexts := make([]*RoleContext, 5)
	for i := 0; i < 5; i++ {
		roleContexts[i] = &RoleContext{
			Role: &role.Role{
				ID:   util.NewID("r"),
				Name: "专家",
			},
			ChatModel: &mockChatModel{response: "response"},
			Prompt:    "你是专家。",
		}
	}

	hub := stream.NewHub("s_test_parallel")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	state := NewDiscussionState("s_test_parallel", 10, bridge)
	state.Roles = roleContexts

	node := NewExpertSpeakNode()
	results := node.Execute(context.Background(), roleContexts, "并行测试", state)

	assert.Equal(t, 5, len(results))
	for _, res := range results {
		assert.NoError(t, res.Error)
	}
}

func TestModeratorEvalNode_ShouldContinue(t *testing.T) {
	state := NewDiscussionState("s_eval_continue", 5, nil)

	node := NewModeratorEvalNode()
	result := node.Evaluate(state)

	assert.True(t, result.ShouldContinue)
	assert.Contains(t, result.Reason, "继续下一轮")
}

func TestModeratorEvalNode_MaxRoundsReached(t *testing.T) {
	state := NewDiscussionState("s_eval_max", 2, nil)
	state.CurrentRound = 2 // 已达到最大轮次

	node := NewModeratorEvalNode()
	result := node.Evaluate(state)

	assert.False(t, result.ShouldContinue)
	assert.Contains(t, result.Reason, "已达到最大讨论轮次")
}

func TestModeratorEvalNode_WithBridgeEvents(t *testing.T) {
	hub := stream.NewHub("s_eval_bridge")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	state := NewDiscussionState("s_eval_bridge", 5, bridge)
	state.CurrentRound = 1

	// 订阅事件
	sub := &stream.Subscriber{
		ID:      "test-sub",
		Events:  make(chan *stream.Event, 10),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	node := NewModeratorEvalNode()
	result := node.Evaluate(state)
	assert.True(t, result.ShouldContinue)

	// 等待所有事件推送完成
	time.Sleep(100 * time.Millisecond)

	// 从 subscriber 收集所有已发送的事件
	var receivedEvents []*stream.Event
	for {
		select {
		case ev := <-sub.Events:
			receivedEvents = append(receivedEvents, ev)
		default:
			goto checkEvents
		}
	}
checkEvents:

	// 验证事件已推送 - 第一个事件应是 round.start
	if len(receivedEvents) > 0 {
		assert.Equal(t, "round.start", receivedEvents[0].Type)
	} else {
		t.Error("期望收到 round.start 事件，但未收到")
	}
	// 验证有 round.eval 事件
	hasRoundEval := false
	for _, ev := range receivedEvents {
		if ev.Type == "round.eval" {
			hasRoundEval = true
			break
		}
	}
	assert.True(t, hasRoundEval, "期望收到 round.eval 事件")

	hub.Unsubscribe("test-sub")
}

func TestSummarizeNode_GenerateSummary(t *testing.T) {
	state := NewDiscussionState("s_summary", 5, nil)
	state.Roles = []*RoleContext{
		{Role: &role.Role{Name: "专家A"}},
		{Role: &role.Role{Name: "专家B"}},
	}

	// 添加一些历史消息
	state.AddHistory(&model_gateway.ChatMessage{Role: "user", Content: "话题"})
	state.AddHistory(&model_gateway.ChatMessage{Role: "assistant", Content: "回复1"})
	state.AddHistory(&model_gateway.ChatMessage{Role: "assistant", Content: "回复2"})

	evalResult := &EvalResult{
		ShouldContinue: false,
		Reason:         "讨论完成",
		Summary:        "本次讨论共 1 轮，2 位专家参与",
	}

	node := NewSummarizeNode()
	minutes := node.GenerateSummary(state, evalResult)

	assert.Equal(t, "s_summary", minutes.SessionID)
	assert.Equal(t, 3, minutes.TotalMessages)
	assert.Equal(t, "本次讨论共 1 轮，2 位专家参与", minutes.Summary)
	assert.False(t, minutes.CompletedAt.IsZero())
}

func TestDiscussionState_ConcurrentAccess(t *testing.T) {
	state := NewDiscussionState("s_concurrent", 10, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.AddHistory(&model_gateway.ChatMessage{Role: "assistant", Content: "msg"})
			state.IncrementRound()
		}()
	}
	wg.Wait()

	assert.Equal(t, 50, len(state.GetHistory()))
	assert.Equal(t, 50, state.GetCurrentRound())
}

func TestRoleChatTemplate_Build(t *testing.T) {
	skillRegistry := role.NewSkillRegistry()

	testRole := &role.Role{
		Name:          "代码审查员",
		Title:         "Code Reviewer",
		SystemPrompt:  "请审查以下代码。",
		Expertise:     []string{"Go", "安全"},
		SpeakingStyle: "专业严谨",
		Skills:        []string{"chinese-code-review", "performance-analysis"},
	}

	prompt, err := role.BuildRoleChatTemplate(testRole, skillRegistry, "请使用中文回复。")
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "代码审查员")
	assert.Contains(t, prompt, "Code Reviewer")
	assert.Contains(t, prompt, "chinese-code-review")
	assert.Contains(t, prompt, "performance-analysis")
	assert.Contains(t, prompt, "专业严谨")
	assert.Contains(t, prompt, "请使用中文回复")
}

func TestFactory_CreateRoundRobin(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)

	orch, err := factory.CreateOrchestrator(ParadigmRoundRobin)
	assert.NoError(t, err)
	assert.NotNil(t, orch)

	_, ok := orch.(*RoundRobinOrchestrator)
	assert.True(t, ok, "期望得到 RoundRobinOrchestrator")
}

func TestFactory_UnsupportedParadigm(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)

	_, err := factory.CreateOrchestrator("unknown_paradigm")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知的讨论范式")
}

func TestFactory_FreeChatNowSupported(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)

	orch, err := factory.CreateOrchestrator(ParadigmFreeChat)
	assert.NoError(t, err)
	assert.NotNil(t, orch)

	_, ok := orch.(*FreeChatOrchestrator)
	assert.True(t, ok, "期望得到 FreeChatOrchestrator")
}

func TestSkillRegistry_DefaultSkills(t *testing.T) {
	reg := role.NewSkillRegistry()
	skills := reg.GetAll()
	assert.Equal(t, 6, len(skills)) // 3 original + 3 hotel/corporate

	names := reg.ListNames()
	assert.Contains(t, names, "security-audit")
	assert.Contains(t, names, "chinese-code-review")
	assert.Contains(t, names, "performance-analysis")
}

func TestSkillRegistry_GetSkill(t *testing.T) {
	reg := role.NewSkillRegistry()

	skill, err := reg.Get("security-audit")
	assert.NoError(t, err)
	assert.Equal(t, "security-audit", skill.Name)
	assert.Contains(t, skill.Prompt, "安全审计")

	_, err = reg.Get("nonexistent-skill")
	assert.Error(t, err)
}

func TestSkillRegistry_RegisterDuplicate(t *testing.T) {
	reg := role.NewSkillRegistry()

	err := reg.Register(role.SkillDefinition{
		Name:        "security-audit",
		Description: "duplicate",
		Prompt:      "duplicate",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestSkillRegistry_ThreadSafety(t *testing.T) {
	reg := role.NewSkillRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.GetAll()
			reg.ListNames()
		}()
	}
	wg.Wait()
}

func TestNewDiscussionState(t *testing.T) {
	state := NewDiscussionState("s_test", 5, nil)
	assert.Equal(t, "s_test", state.SessionID)
	assert.Equal(t, 5, state.MaxRounds)
	assert.Equal(t, 0, state.CurrentRound)
	assert.Empty(t, state.GetHistory())
}

func TestDiscussionState_RoundIncrement(t *testing.T) {
	state := NewDiscussionState("s_rounds", 10, nil)
	assert.Equal(t, 0, state.GetCurrentRound())

	r1 := state.IncrementRound()
	assert.Equal(t, 1, r1)
	assert.Equal(t, 1, state.GetCurrentRound())

	r2 := state.IncrementRound()
	assert.Equal(t, 2, r2)
	assert.Equal(t, 2, state.GetCurrentRound())
}

func TestRoundRobinConfig_Defaults(t *testing.T) {
	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_1", "r_2"},
		Topic:     "测试话题",
		MaxRounds: 5,
	}

	assert.Equal(t, 2, len(config.RoleIDs))
	assert.Equal(t, "测试话题", config.Topic)
	assert.Equal(t, 5, config.MaxRounds)
}

func TestStreamBridge_EventPush(t *testing.T) {
	hub := stream.NewHub("s_bridge_test")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	sub := &stream.Subscriber{
		ID:      "bridge-test-sub",
		Events:  make(chan *stream.Event, 5),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	// 推送事件
	err := bridge.Push(&stream.GraphEvent{
		Type:      "agent_speak",
		NodeName:  "expert_speak",
		RoleName:  "测试专家",
		Content:   "测试内容",
		Timestamp: time.Now(),
	})
	assert.NoError(t, err)

	// 验证收到事件
	select {
	case ev := <-sub.Events:
		assert.Equal(t, "role.speak", ev.Type)
		assert.Contains(t, ev.Data.(string), "测试内容")
	case <-time.After(time.Second):
		t.Error("等待事件超时")
	}

	hub.Unsubscribe("bridge-test-sub")
	bridge.Close()
}

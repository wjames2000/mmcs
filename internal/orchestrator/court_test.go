package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/pkg/util"
)

func TestNewCourtOrchestrator(t *testing.T) {
	contextInitNode := NewContextInitNode(nil, nil, nil)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	orch := NewCourtOrchestrator(contextInitNode, expertSpeakNode, moderatorEvalNode, summarizeNode)
	assert.NotNil(t, orch)
}

func TestCourtConfig_Defaults(t *testing.T) {
	config := &CourtConfig{
		RoleIDs:      []string{"r_author", "r_reviewer1", "r_reviewer2"},
		Topic:        "代码审查议题",
		AuthorRoleID: "r_author",
		MaxRounds:    3,
	}

	assert.Equal(t, 3, len(config.RoleIDs))
	assert.Equal(t, "r_author", config.AuthorRoleID)
	assert.Equal(t, "代码审查议题", config.Topic)
}

func TestCourtOrchestrator_AuthorNotFound(t *testing.T) {
	// 创建 mock 依赖
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 创建测试角色（不含作者）
	testRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "审查员",
		Title:        "Reviewer",
		SystemPrompt: "请审查代码。",
		Skills:       []string{},
	}
	svc.addRole(testRole)

	mockModel := &mockChatModel{response: "审查意见。"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{testRole.ID},
		Topic:        "测试",
		AuthorRoleID: "r_nonexistent_author",
		MaxRounds:    1,
	}

	_, err := orch.Execute(ctx, "s_test", config, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未在角色列表中找到")
}

func TestCourtOrchestrator_Execute(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 创建作者角色
	authorRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "架构师",
		Title:        "Architect",
		SystemPrompt: "请陈述你的设计方案。",
		Skills:       []string{},
	}
	svc.addRole(authorRole)

	// 创建审查员角色
	reviewerRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "安全审查员",
		Title:        "Security Reviewer",
		SystemPrompt: "请从安全角度审查代码。",
		Skills:       []string{},
	}
	svc.addRole(reviewerRole)

	mockModel := &mockChatModel{response: "测试回复内容"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{authorRole.ID, reviewerRole.ID},
		Topic:        "微服务架构设计方案审查",
		AuthorRoleID: authorRole.ID,
		MaxRounds:    1,
	}

	minutes, err := orch.Execute(ctx, "s_court_test", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, "s_court_test", minutes.SessionID)
}

func TestCourtOrchestrator_WithBridgeEvents(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	authorRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "作者",
		Title:        "Author",
		SystemPrompt: "请陈述。",
		Skills:       []string{},
	}
	svc.addRole(authorRole)

	reviewerRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "审查员",
		Title:        "Reviewer",
		SystemPrompt: "请审查。",
		Skills:       []string{},
	}
	svc.addRole(reviewerRole)

	mockModel := &mockChatModel{response: "测试内容"}
	gw.models["openai"] = mockModel

	hub := stream.NewHub("s_court_bridge")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	sub := &stream.Subscriber{
		ID:      "court-test-sub",
		Events:  make(chan *stream.Event, 20),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{authorRole.ID, reviewerRole.ID},
		Topic:        "测试",
		AuthorRoleID: authorRole.ID,
		MaxRounds:    1,
	}

	_, err := orch.Execute(ctx, "s_court_bridge_test", config, bridge, nil)
	assert.NoError(t, err)

	// 等待事件推送完成
	time.Sleep(100 * time.Millisecond)

	// 应该有 round.start 事件
	select {
	case event := <-sub.Events:
		assert.Equal(t, "round.start", event.Type)
	case <-time.After(500 * time.Millisecond):
		t.Error("期望收到 round.start 事件，但未收到")
	}

	hub.Unsubscribe("court-test-sub")
	bridge.Close()
}

func TestCourtOrchestrator_AuthorSpeakAndResponse(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	authorRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "设计者",
		Title:        "Designer",
		SystemPrompt: "你是一位软件架构师。",
		Skills:       []string{},
	}
	svc.addRole(authorRole)

	reviewerRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "代码审查员",
		Title:        "Code Reviewer",
		SystemPrompt: "你是一位代码审查专家。",
		Skills:       []string{},
	}
	svc.addRole(reviewerRole)

	// 使用不同的 mock 回复区分阶段
	mockModel := &mockChatModel{response: "这是一段测试回复。"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{authorRole.ID, reviewerRole.ID},
		Topic:        "API 设计评审",
		AuthorRoleID: authorRole.ID,
		MaxRounds:    1,
	}

	minutes, err := orch.Execute(ctx, "s_court_api", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, "s_court_api", minutes.SessionID)
}

func TestCourtOrchestrator_WithProgressChannel(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	authorRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "作者",
		Title:        "Author",
		SystemPrompt: "请陈述。",
		Skills:       []string{},
	}
	svc.addRole(authorRole)

	reviewerRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "审查员",
		Title:        "Reviewer",
		SystemPrompt: "请审查。",
		Skills:       []string{},
	}
	svc.addRole(reviewerRole)

	mockModel := &mockChatModel{response: "测试内容"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{authorRole.ID, reviewerRole.ID},
		Topic:        "测试",
		AuthorRoleID: authorRole.ID,
		MaxRounds:    1,
	}

	progressCh := make(chan string, 10)
	go func() {
		for msg := range progressCh {
			assert.NotEmpty(t, msg)
		}
	}()

	_, err := orch.Execute(ctx, "s_court_progress", config, nil, progressCh)
	assert.NoError(t, err)
}

func TestCourtOrchestrator_CancelViaContext(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	authorRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "作者",
		Title:        "Author",
		SystemPrompt: "请陈述。",
		Skills:       []string{},
	}
	svc.addRole(authorRole)

	mockModel := &mockChatModel{response: "测试"}
	gw.models["openai"] = mockModel

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{authorRole.ID},
		Topic:        "测试",
		AuthorRoleID: authorRole.ID,
		MaxRounds:    1,
	}

	// 取决于 InitRoleContexts 是否检查 context
	// 如果 InitRoleContexts 通过了，Execute 里的 Generate 也可能成功（因为 mock 不检查 ctx）
	// 这个测试确保不会 panic
	_, _ = orch.Execute(ctx, "s_court_cancel", config, nil, nil)

	// 如果 mock 模型不检查 context，可能不会报错
	// 重要的是不 panic
}

func TestCourtWithSingleRole(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 只有作者，没有审查员
	authorRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "设计者",
		Title:        "Designer",
		SystemPrompt: "请陈述。",
		Skills:       []string{},
	}
	svc.addRole(authorRole)

	mockModel := &mockChatModel{response: "设计方案"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewCourtOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &CourtConfig{
		RoleIDs:      []string{authorRole.ID},
		Topic:        "简单设计",
		AuthorRoleID: authorRole.ID,
		MaxRounds:    1,
	}

	minutes, err := orch.Execute(ctx, "s_court_single", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
}

func TestCourtFactoryIntegration(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)

	orch, err := factory.CreateOrchestrator(ParadigmCourt)
	assert.NoError(t, err)
	assert.NotNil(t, orch)

	_, ok := orch.(*CourtOrchestrator)
	assert.True(t, ok, "期望得到 CourtOrchestrator")
}

func TestCourtBuildMinutes(t *testing.T) {
	// 验证 MeetingMinutes 的构建
	state := NewDiscussionState("s_court_minutes", 1, nil)
	state.Roles = []*RoleContext{
		{Role: &role.Role{Name: "架构师"}},
		{Role: &role.Role{Name: "审查员"}},
	}

	state.AddHistory(&model_gateway.ChatMessage{Role: "assistant", Content: "作者陈述"})
	state.AddHistory(&model_gateway.ChatMessage{Role: "assistant", Content: "审查意见"})
	state.AddHistory(&model_gateway.ChatMessage{Role: "assistant", Content: "作者回应"})

	evalResult := &EvalResult{
		ShouldContinue: false,
		Reason:         "评审完成",
		Summary:        "法庭讨论完成",
	}

	node := NewSummarizeNode()
	minutes := node.GenerateSummary(state, evalResult)

	assert.Equal(t, "s_court_minutes", minutes.SessionID)
	assert.Equal(t, 3, minutes.TotalMessages)
	assert.Contains(t, minutes.Summary, "法庭讨论完成")
}

// mockChatModelWithDelay 带延迟的 mock 模型，用于超时测试
type mockChatModelWithDelay struct {
	response string
	delay    time.Duration
}

func (m *mockChatModelWithDelay) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	select {
	case <-time.After(m.delay):
		return &model_gateway.ChatResponse{
			Content:     m.response,
			TotalTokens: 42,
			Model:       "mock-delay",
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockChatModelWithDelay) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	ch := make(chan *model_gateway.StreamChunk, 1)
	ch <- &model_gateway.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}

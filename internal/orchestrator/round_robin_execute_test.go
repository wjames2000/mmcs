package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
)

// setupRoundRobinTest 创建 RoundRobin 测试的公共依赖
// 返回 orchestrator、mock 依赖和清理函数
func setupRoundRobinTest(t *testing.T) (*RoundRobinOrchestrator, *mockRoleService, *mockGateway, func()) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 创建测试角色
	roleA := &role.Role{
		ID:            "r_exec_a",
		Name:          "专家A",
		Title:         "Expert A",
		SystemPrompt:  "你是一个专家。",
		SpeakingStyle: "专业",
	}
	roleB := &role.Role{
		ID:            "r_exec_b",
		Name:          "专家B",
		Title:         "Expert B",
		SystemPrompt:  "你是一个专家。",
		SpeakingStyle: "专业",
	}
	svc.addRole(roleA)
	svc.addRole(roleB)

	// 注册 mock 模型
	mockModel := &mockChatModel{response: "mock response content"}
	gw.models["openai"] = mockModel

	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	orch := NewRoundRobinOrchestrator(
		contextInitNode,
		expertSpeakNode,
		moderatorEvalNode,
		summarizeNode,
		gw,
	)

	cleanup := func() {}

	return orch, svc, gw, cleanup
}

func TestRoundRobinExecute_BasicFlow(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_exec_a", "r_exec_b"},
		Topic:     "测试话题",
		MaxRounds: 1,
	}

	progressCh := make(chan string, 10)
	ctx := context.Background()

	minutes, err := orch.Execute(ctx, "s_test_basic", config, nil, progressCh)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, "s_test_basic", minutes.SessionID)
	assert.Equal(t, 1, minutes.TotalRounds)
	assert.True(t, minutes.TotalMessages > 0, "应有历史消息")
	assert.False(t, minutes.CompletedAt.IsZero(), "完成时间不应为空")

	// 验证 progress 消息
	var progressMsgs []string
	for msg := range progressCh {
		progressMsgs = append(progressMsgs, msg)
	}
	assert.NotEmpty(t, progressMsgs, "应有进度消息")
	assert.Contains(t, progressMsgs[0], "初始化角色上下文")
}

func TestRoundRobinExecute_MultipleRounds(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_exec_a", "r_exec_b"},
		Topic:     "测试话题",
		MaxRounds: 3,
	}

	progressCh := make(chan string, 20)
	ctx := context.Background()

	minutes, err := orch.Execute(ctx, "s_test_multi", config, nil, progressCh)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, 3, minutes.TotalRounds, "应完成 3 轮")
	assert.True(t, minutes.TotalMessages >= 6, "每轮每角色发言 1 条 + 话题消息")

	// 验证每轮进度消息（Execute 会在返回前关闭 progressCh）
	var progressMsgsMulti []string
	for msg := range progressCh {
		progressMsgsMulti = append(progressMsgsMulti, msg)
	}
	assert.True(t, len(progressMsgsMulti) > 1, "应有多个进度消息")
}

func TestRoundRobinExecute_SingleRole(t *testing.T) {
	orch, svc, gw, _ := setupRoundRobinTest(t)

	// 添加第三个角色，但只用一个角色测试
	_ = svc
	_ = gw

	// 使用只有 1 个角色的配置
	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_exec_a"},
		Topic:     "单人测试",
		MaxRounds: 2,
	}

	progressCh := make(chan string, 10)
	ctx := context.Background()

	minutes, err := orch.Execute(ctx, "s_test_single", config, nil, progressCh)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, 2, minutes.TotalRounds)
}

func TestRoundRobinExecute_ContextCancel(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_exec_a", "r_exec_b"},
		Topic:     "测试话题",
		MaxRounds: 10, // 多轮确保有机会被取消
	}

	progressCh := make(chan string, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	minutes, err := orch.Execute(ctx, "s_test_cancel", config, nil, progressCh)
	assert.Error(t, err, "context 取消应返回错误")
	assert.Nil(t, minutes, "取消后应返回 nil")
}

func TestRoundRobinExecute_InterruptResume(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	interruptCh := make(chan *session.InterruptSignal, 1)
	resumeCh := make(chan *session.ResumeSignal, 1)
	hub := stream.NewHub("s_test_interrupt")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	sub := &stream.Subscriber{
		ID:      "interrupt-test-sub",
		Events:  make(chan *stream.Event, 10),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	// 预填充中断和恢复信号，确保第一次 CheckInterrupt 能触发中断/恢复流程
	interruptCh <- &session.InterruptSignal{
		NodeName: "expert_speak",
		Message:  "请暂停讨论",
	}
	resumeCh <- &session.ResumeSignal{Message: "继续讨论"}

	config := &RoundRobinConfig{
		RoleIDs:     []string{"r_exec_a", "r_exec_b"},
		Topic:       "测试话题",
		MaxRounds:   2,
		InterruptCh: interruptCh,
		ResumeCh:    resumeCh,
	}

	ctx := context.Background()

	minutes, err := orch.Execute(ctx, "s_test_interrupt", config, bridge, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, 2, minutes.TotalRounds, "中断恢复后应完成所有轮次")

	// 验证收到了暂停事件（需要遍历事件，因为有主持人开场白等前置事件）
	var foundPaused bool
	for i := 0; i < 10; i++ {
		select {
		case event := <-sub.Events:
			if event.Type == "session.paused" {
				foundPaused = true
			}
		case <-time.After(500 * time.Millisecond):
			break
		}
	}
	assert.True(t, foundPaused, "应收到暂停事件")

	hub.Unsubscribe("interrupt-test-sub")
	bridge.Close()
}

func TestRoundRobinExecute_WithBridge(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	hub := stream.NewHub("s_test_bridge")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	sub := &stream.Subscriber{
		ID:      "bridge-sub",
		Events:  make(chan *stream.Event, 20),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_exec_a", "r_exec_b"},
		Topic:     "话题",
		MaxRounds: 1,
	}

	ctx := context.Background()

	minutes, err := orch.Execute(ctx, "s_test_bridge", config, bridge, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)

	// 应收到 round_start 和 done 事件
	var foundRoundStart bool
	for i := 0; i < 10; i++ {
		select {
		case event := <-sub.Events:
			switch event.Type {
			case "round.start":
				foundRoundStart = true
			}
		case <-time.After(200 * time.Millisecond):
			break
		}
	}
	assert.True(t, foundRoundStart, "应收到 round.start 事件")

	hub.Unsubscribe("bridge-sub")
	bridge.Close()
}

func TestRoundRobinExecute_NilProgressCh(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	config := &RoundRobinConfig{
		RoleIDs:   []string{"r_exec_a", "r_exec_b"},
		Topic:     "测试",
		MaxRounds: 1,
	}

	// progressCh 为 nil，不应 panic
	ctx := context.Background()
	minutes, err := orch.Execute(ctx, "s_test_nil_progress", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
}

func TestRoundRobinExecute_EmptyRoleIDs(t *testing.T) {
	orch, _, _, _ := setupRoundRobinTest(t)

	config := &RoundRobinConfig{
		RoleIDs:   []string{},
		Topic:     "测试",
		MaxRounds: 1,
	}

	ctx := context.Background()
	minutes, err := orch.Execute(ctx, "s_test_empty_roles", config, nil, nil)
	assert.NoError(t, err) // 角色为空时，InitRoleContexts 返回空 slice，后续执行正常
	assert.NotNil(t, minutes)
	assert.Equal(t, 1, minutes.TotalRounds)
}

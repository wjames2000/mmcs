package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/pkg/util"
)

func TestNewFreeChatOrchestrator(t *testing.T) {
	contextInitNode := NewContextInitNode(nil, nil, nil)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	orch := NewFreeChatOrchestrator(contextInitNode, expertSpeakNode, moderatorEvalNode, summarizeNode)
	assert.NotNil(t, orch)
}

func TestFreeChatConfig_Defaults(t *testing.T) {
	config := &FreeChatConfig{
		RoleIDs:   []string{"r_expert1", "r_expert2"},
		Topic:     "自由讨论话题",
		MaxRounds: 5,
	}

	assert.Equal(t, 2, len(config.RoleIDs))
	assert.Equal(t, "自由讨论话题", config.Topic)
	assert.Equal(t, 5, config.MaxRounds)
}

func TestBuildFreeChatOrchestrator(t *testing.T) {
	// 用已有的 Factory 测试 FreeChat 创建
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)
	orch, err := factory.CreateOrchestrator(ParadigmFreeChat)
	assert.NoError(t, err)
	assert.NotNil(t, orch)

	freeChat, ok := orch.(*FreeChatOrchestrator)
	assert.True(t, ok)
	assert.NotNil(t, freeChat)
}

func TestFreeChatFullFlow(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 创建两个角色
	role1 := &role.Role{
		ID:           util.NewID("r"),
		Name:         "专家A",
		Title:        "Expert A",
		SystemPrompt: "你是专家A，请发表专业意见。",
		Skills:       []string{},
	}
	svc.addRole(role1)

	role2 := &role.Role{
		ID:           util.NewID("r"),
		Name:         "专家B",
		Title:        "Expert B",
		SystemPrompt: "你是专家B，请从不同角度分析。",
		Skills:       []string{},
	}
	svc.addRole(role2)

	// mockModel 同时作为主持人模型和角色发言模型返回角色 ID
	// selectNextSpeaker → 返回 role1.ID → 选中 role1 发言
	// role1 发言 → 返回 role1.ID（作为发言内容）
	// moderatorEvalNode.Evaluate → 判断是否超过 MaxRounds
	mockModel := &mockChatModel{response: role1.ID}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewFreeChatOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &FreeChatConfig{
		RoleIDs:   []string{role1.ID, role2.ID},
		Topic:     "测试自由群聊",
		MaxRounds: 2,
	}

	progressCh := make(chan string, 10)
	go func() {
		for range progressCh {
			// 消费进度消息
		}
	}()

	minutes, err := orch.Execute(ctx, "s_test_free", config, nil, progressCh)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
	assert.Equal(t, "s_test_free", minutes.SessionID)
	assert.True(t, minutes.TotalRounds >= 1)
	assert.True(t, minutes.TotalMessages >= 1)
}

func TestFreeChatWithBridge(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	role1 := &role.Role{
		ID:           util.NewID("r"),
		Name:         "发言者",
		Title:        "Speaker",
		SystemPrompt: "请发言。",
		Skills:       []string{},
	}
	svc.addRole(role1)

	// 主持人模型返回角色 ID 来"选择"发言者
	mockModel := &mockChatModel{response: role1.ID}
	gw.models["openai"] = mockModel

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewFreeChatOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	// 创建 Hub 和 Bridge
	hub := stream.NewHub("s_test_bridge")
	bridge := stream.NewBridge(hub, 64)
	bridge.Start(ctx)
	defer bridge.Close()

	config := &FreeChatConfig{
		RoleIDs:   []string{role1.ID},
		Topic:     "带桥接的测试",
		MaxRounds: 1,
	}

	minutes, err := orch.Execute(ctx, "s_test_bridge", config, bridge, nil)
	assert.NoError(t, err)
	assert.NotNil(t, minutes)
}

func TestFreeChatCancel(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	role1 := &role.Role{
		ID:           util.NewID("r"),
		Name:         "专家",
		Title:        "Expert",
		SystemPrompt: "请发表意见。",
		Skills:       []string{},
	}
	svc.addRole(role1)

	mockModel := &mockChatModel{response: role1.ID}
	gw.models["openai"] = mockModel

	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewFreeChatOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	// 使用已取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := &FreeChatConfig{
		RoleIDs:   []string{role1.ID},
		Topic:     "取消测试",
		MaxRounds: 10,
	}

	_, err := orch.Execute(ctx, "s_test_cancel", config, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "canceled") // context.Canceled
}

func TestFreeChatNoRoles(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewFreeChatOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &FreeChatConfig{
		RoleIDs:   []string{},
		Topic:     "空角色列表",
		MaxRounds: 1,
	}

	_, err := orch.Execute(context.Background(), "s_test_empty", config, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有可用的角色")
}

func TestFreeChatFactoryRegistration(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)

	// 验证所有 ParadigmType 均可创建
	paradigms := []ParadigmType{
		ParadigmRoundRobin,
		ParadigmCourt,
		ParadigmEvaluation,
		ParadigmFreeChat,
	}

	for _, p := range paradigms {
		orch, err := factory.CreateOrchestrator(p)
		assert.NoError(t, err, "范式 %s 创建失败", p)
		assert.NotNil(t, orch, "范式 %s 返回 nil", p)
	}

	// 未知范式返回错误
	_, err := factory.CreateOrchestrator("unknown")
	assert.Error(t, err)
}

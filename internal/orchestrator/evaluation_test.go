package orchestrator

import (
	"context"
	"testing"

	"github.com/mmcs/internal/role"
	"github.com/mmcs/internal/stream"
	"github.com/mmcs/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestNewEvaluationOrchestrator(t *testing.T) {
	contextInitNode := NewContextInitNode(nil, nil, nil)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	orch := NewEvaluationOrchestrator(contextInitNode, expertSpeakNode, moderatorEvalNode, summarizeNode)
	assert.NotNil(t, orch)
}

func TestEvaluationConfig_Defaults(t *testing.T) {
	config := &EvaluationConfig{
		RoleIDs: []string{"r_expert1", "r_expert2"},
		Topic:   "选择最佳架构方案",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "微服务架构", Description: "基于微服务的分布式架构"},
			{ID: "opt_2", Name: "单体架构", Description: "传统的单体应用架构"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "可扩展性", Weight: 0.4},
			{Name: "可维护性", Weight: 0.3},
			{Name: "性能", Weight: 0.3},
		},
	}

	assert.Equal(t, 2, len(config.Options))
	assert.Equal(t, 3, len(config.Criteria))
	assert.Equal(t, "选择最佳架构方案", config.Topic)
}

func TestBuildScoringPrompt(t *testing.T) {
	config := &EvaluationConfig{
		Topic: "评估方案",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A", Description: "方案A的详细描述"},
			{ID: "opt_2", Name: "方案B", Description: "方案B的详细描述"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "质量", Weight: 0.5},
			{Name: "成本", Weight: 0.5},
		},
	}

	prompt := buildScoringPrompt(config)
	assert.Contains(t, prompt, "评估方案")
	assert.Contains(t, prompt, "方案A")
	assert.Contains(t, prompt, "方案B")
	assert.Contains(t, prompt, "质量")
	assert.Contains(t, prompt, "成本")
	assert.Contains(t, prompt, "0.50")
}

func TestFormatOptions(t *testing.T) {
	options := []EvaluationOption{
		{ID: "opt_1", Name: "方案A", Description: "描述A"},
		{ID: "opt_2", Name: "方案B", Description: "描述B"},
	}

	result := formatOptions(options)
	assert.Contains(t, result, "方案A")
	assert.Contains(t, result, "方案B")
	assert.Contains(t, result, "描述A")
	assert.Contains(t, result, "描述B")
}

func TestFormatCriteria(t *testing.T) {
	criteria := []EvaluationCriterion{
		{Name: "标准1", Weight: 0.6},
		{Name: "标准2", Weight: 0.4},
	}

	result := formatCriteria(criteria)
	assert.Contains(t, result, "标准1")
	assert.Contains(t, result, "标准2")
	assert.Contains(t, result, "0.60")
	assert.Contains(t, result, "0.40")
}

func TestParseScoresFromResponse(t *testing.T) {
	config := &EvaluationConfig{
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A"},
			{ID: "opt_2", Name: "方案B"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "质量", Weight: 0.5},
			{Name: "成本", Weight: 0.5},
		},
	}

	scores := parseScoresFromResponse("这是一个包含评分的回复", config)
	assert.Equal(t, 4, len(scores)) // 2 options * 2 criteria

	for _, s := range scores {
		assert.NotEmpty(t, s.OptionID)
		assert.NotEmpty(t, s.CriterionName)
		assert.True(t, s.Score > 0)
	}
}

func TestEvaluationOrchestrator_Execute(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	// 创建评估角色
	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "架构评估专家",
		Title:        "Architecture Evaluator",
		SystemPrompt: "请根据标准评估方案。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	mockModel := &mockChatModel{response: "方案A在可扩展性方面得分85，可维护性方面得分80，性能方面得分90。"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs: []string{expertRole.ID},
		Topic:   "评估架构方案",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "微服务架构", Description: "分布式微服务方案"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "可扩展性", Weight: 0.4},
			{Name: "可维护性", Weight: 0.3},
			{Name: "性能", Weight: 0.3},
		},
	}

	output, err := orch.Execute(ctx, "s_eval_test", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, len(output.Matrix))
	assert.NotEmpty(t, output.Consensus)
	assert.NotEmpty(t, output.FinalRanking)
}

func TestEvaluationOrchestrator_WithMultipleOptions(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "评估专家",
		Title:        "Evaluator",
		SystemPrompt: "请评估。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	mockModel := &mockChatModel{response: "评分结果"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs: []string{expertRole.ID},
		Topic:   "多方案评估",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A", Description: "方案A描述"},
			{ID: "opt_2", Name: "方案B", Description: "方案B描述"},
			{ID: "opt_3", Name: "方案C", Description: "方案C描述"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "质量", Weight: 0.5},
			{Name: "成本", Weight: 0.5},
		},
	}

	output, err := orch.Execute(ctx, "s_eval_multi", config, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(output.Matrix))
	assert.Equal(t, 3, len(output.FinalRanking))
}

func TestEvaluationOrchestrator_WithCritic(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "评估专家",
		Title:        "Evaluator",
		SystemPrompt: "请评估方案。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	criticRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "批评者",
		Title:        "Devil's Advocate",
		SystemPrompt: "请质疑评分假设。",
		Skills:       []string{},
	}
	svc.addRole(criticRole)

	mockModel := &mockChatModel{response: "评估内容"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs:      []string{expertRole.ID, criticRole.ID},
		Topic:        "带批评者的评估",
		CriticRoleID: criticRole.ID,
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A", Description: "方案A"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "质量", Weight: 1.0},
		},
	}

	output, err := orch.Execute(ctx, "s_eval_critic", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.NotEmpty(t, output.CriticFeedbacks)
}

func TestEvaluationOrchestrator_WithBridgeEvents(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "评估专家",
		Title:        "Evaluator",
		SystemPrompt: "请评估。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	mockModel := &mockChatModel{response: "评估内容"}
	gw.models["openai"] = mockModel

	hub := stream.NewHub("s_eval_bridge")
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(context.Background())

	sub := &stream.Subscriber{
		ID:      "eval-test-sub",
		Events:  make(chan *stream.Event, 20),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(sub)

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs: []string{expertRole.ID},
		Topic:   "评估测试",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A", Description: "方案A"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "质量", Weight: 1.0},
		},
	}

	output, err := orch.Execute(ctx, "s_eval_bridge_test", config, bridge, nil)
	assert.NoError(t, err)
	assert.NotNil(t, output)

	hub.Unsubscribe("eval-test-sub")
	bridge.Close()
}

func TestEvaluationOrchestrator_WithProgressChannel(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "评估专家",
		Title:        "Evaluator",
		SystemPrompt: "请评估。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	mockModel := &mockChatModel{response: "评估内容"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs: []string{expertRole.ID},
		Topic:   "评估测试",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A", Description: "方案A"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "质量", Weight: 1.0},
		},
	}

	progressCh := make(chan string, 10)
	go func() {
		for msg := range progressCh {
			assert.NotEmpty(t, msg)
		}
	}()

	output, err := orch.Execute(ctx, "s_eval_progress", config, nil, progressCh)
	assert.NoError(t, err)
	assert.NotNil(t, output)
}

func TestEvaluationFactoryIntegration(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	factory := NewFactory(svc, skillRegistry, gw)

	orch, err := factory.CreateOrchestrator(ParadigmEvaluation)
	assert.NoError(t, err)
	assert.NotNil(t, orch)

	_, ok := orch.(*EvaluationOrchestrator)
	assert.True(t, ok, "期望得到 EvaluationOrchestrator")
}

func TestEvaluationMatrixCalculation(t *testing.T) {
	// 测试矩阵计算的正确性
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "评估专家",
		Title:        "Evaluator",
		SystemPrompt: "请评估。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	mockModel := &mockChatModelWithDelay{response: "评分", delay: 0}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs: []string{expertRole.ID},
		Topic:   "方案评估",
		Options: []EvaluationOption{
			{ID: "opt_1", Name: "方案A", Description: "方案A"},
		},
		Criteria: []EvaluationCriterion{
			{Name: "可扩展性", Weight: 0.5},
			{Name: "可维护性", Weight: 0.5},
		},
	}

	output, err := orch.Execute(ctx, "s_eval_matrix", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, output)

	// 验证矩阵有条目
	if len(output.Matrix) > 0 {
		entry := output.Matrix[0]
		assert.NotEmpty(t, entry.OptionID)
		assert.NotEmpty(t, entry.OptionName)
		assert.True(t, entry.TotalScore >= 0)
	}

	// 验证评分结果
	assert.NotEmpty(t, output.ScoringResults)
}

func TestEvaluationEmptyOptions(t *testing.T) {
	svc := newMockRoleService()
	skillRegistry := role.NewSkillRegistry()
	gw := newMockGateway()

	expertRole := &role.Role{
		ID:           util.NewID("r"),
		Name:         "评估专家",
		Title:        "Evaluator",
		SystemPrompt: "请评估。",
		Skills:       []string{},
	}
	svc.addRole(expertRole)

	mockModel := &mockChatModel{response: "无方案可评估"}
	gw.models["openai"] = mockModel

	ctx := context.Background()
	contextInitNode := NewContextInitNode(svc, skillRegistry, gw)
	orch := NewEvaluationOrchestrator(contextInitNode, NewExpertSpeakNode(), NewModeratorEvalNode(), NewSummarizeNode())

	config := &EvaluationConfig{
		RoleIDs:  []string{expertRole.ID},
		Topic:    "空评估",
		Options:  []EvaluationOption{},
		Criteria: []EvaluationCriterion{},
	}

	output, err := orch.Execute(ctx, "s_eval_empty", config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Empty(t, output.Matrix)
}

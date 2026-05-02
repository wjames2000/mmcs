package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// SupervisorAgent 协调多个子 Agent 的主持人 Agent
// 分析任务 → 选择最合适的子 Agent → 委派执行 → 汇总结果
type SupervisorAgent struct {
	id    string
	model model_gateway.ChatModel
	subs  []Agent // 已注册的子 Agent 列表
}

// NewSupervisorAgent 创建 SupervisorAgent
// id: Agent 唯一标识
// model: 用于分析和决策的 ChatModel
// agents: 子 Agent 列表（至少一个）
func NewSupervisorAgent(id string, model model_gateway.ChatModel, agents []Agent) *SupervisorAgent {
	return &SupervisorAgent{
		id:    id,
		model: model,
		subs:  agents,
	}
}

// ID 返回 SupervisorAgent 标识
func (s *SupervisorAgent) ID() string {
	return s.id
}

// Run 执行 Supervisor 流程
// 1. 构建提示词描述可用子 Agent
// 2. 调用 ChatModel 分析并选择最合适的子 Agent
// 3. 委派任务给选中的子 Agent 执行
// 4. 返回子 Agent 的执行结果
func (s *SupervisorAgent) Run(ctx context.Context, input string) (*Result, error) {
	start := time.Now()

	if len(s.subs) == 0 {
		return nil, fmt.Errorf("SupervisorAgent %s 没有注册任何子 Agent", s.id)
	}

	// 1. 构建提示词
	prompt := s.buildSelectionPrompt(input)

	// 2. 调用模型选择子 Agent
	selectedID, err := s.selectAgent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("SupervisorAgent %s 选择子 Agent 失败: %w", s.id, err)
	}

	log.Debug().Str("supervisor", s.id).Str("selected", selectedID).Msg("Supervisor 选择了子 Agent")

	// 3. 查找并委派子 Agent
	var target Agent
	for _, sub := range s.subs {
		if sub.ID() == selectedID {
			target = sub
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("SupervisorAgent %s 选择的子 Agent %s 未注册", s.id, selectedID)
	}

	result, err := target.Run(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("SupervisorAgent %s 委派给 %s 执行失败: %w", s.id, selectedID, err)
	}

	// 4. 汇总结果（直接返回子 Agent 的结果）
	result.AgentID = s.id
	return &Result{
		AgentID:  s.id,
		Output:   result.Output,
		Tokens:   result.Tokens,
		Duration: time.Since(start),
	}, nil
}

// buildSelectionPrompt 构建子 Agent 选择提示词
func (s *SupervisorAgent) buildSelectionPrompt(input string) string {
	var b strings.Builder
	b.WriteString("你是一个任务调度分析器。请分析以下任务，并从可用的 Agent 中选择最合适的一个来执行。\n\n")
	b.WriteString("可用 Agent 列表：\n")

	for _, sub := range s.subs {
		b.WriteString(fmt.Sprintf("- %s\n", sub.ID()))
	}

	b.WriteString("\n用户任务：\n")
	b.WriteString(input)
	b.WriteString("\n\n请仅返回你选择执行的 Agent ID（不加任何其他内容）。")

	return b.String()
}

// selectAgent 调用 ChatModel 选择子 Agent
func (s *SupervisorAgent) selectAgent(ctx context.Context, prompt string) (string, error) {
	messages := []model_gateway.ChatMessage{
		{Role: "system", Content: "你是任务调度专家。严格按要求格式输出 Agent ID。"},
		{Role: "user", Content: prompt},
	}

	resp, err := s.model.Generate(ctx, &model_gateway.ChatRequest{
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("模型决策失败: %w", err)
	}

	// 清理返回的 Agent ID（去除空白、引号等）
	selectedID := strings.TrimSpace(resp.Content)
	selectedID = strings.Trim(selectedID, "\"'`")

	if selectedID == "" {
		return "", fmt.Errorf("模型返回了空的 Agent ID")
	}

	return selectedID, nil
}

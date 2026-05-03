// Package validation 提供 AI 驱动的任务验证闭环
// 包含验证官 Agent、验证服务和退回重试机制
package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/wjames2000/mmcs/internal/agent"
	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// validatorPromptTemplate 验证官提示词模板
const validatorPromptTemplate = `你是一个任务验证官，请根据验收标准验证以下任务是否完成。

任务: %s
描述: %s
验收标准: %s
执行结果: %s

请逐项检查验收标准，输出 JSON:
{"verdict": "passed|needs_revision|rejected", "reason": "...", "checked_items": [...]}`

// ValidatorAgent 验证官 Agent
// 使用 AI 模型对任务执行结果进行验证，返回结构化判定结果
// 实现 agent.Agent 接口
type ValidatorAgent struct {
	id    string
	model model_gateway.ChatModel
}

// NewValidatorAgent 创建验证官 Agent
// id: Agent 唯一标识，为空时使用默认值 "validator_default"
// model: 用于执行验证判定的 AI 模型
func NewValidatorAgent(id string, model model_gateway.ChatModel) *ValidatorAgent {
	if id == "" {
		id = "validator_default"
	}
	return &ValidatorAgent{
		id:    id,
		model: model,
	}
}

// ID 返回验证官 Agent 唯一标识
// 实现 agent.Agent 接口
func (a *ValidatorAgent) ID() string {
	return a.id
}

// Run 执行验证判定
// 接收包含任务标题、描述、验收标准和执行结果的输入字符串，
// 调用 AI 模型进行判定，返回包含原始 AI 输出的执行结果
// 实现 agent.Agent 接口
func (a *ValidatorAgent) Run(ctx context.Context, input string) (*agent.Result, error) {
	start := time.Now()

	messages := []model_gateway.ChatMessage{
		{Role: "user", Content: input},
	}

	resp, err := a.model.Generate(ctx, &model_gateway.ChatRequest{
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("ValidatorAgent %s 执行失败: %w", a.id, err)
	}

	return &agent.Result{
		AgentID:  a.id,
		Output:   resp.Content,
		Tokens:   resp.TotalTokens,
		Duration: time.Since(start),
	}, nil
}

// BuildValidationInput 构造验证输入字符串
// 包含任务标题、描述、验收标准和执行结果，用于 ValidatorAgent.Run
func BuildValidationInput(title, description, acceptanceCriteria, executionResult string) string {
	return fmt.Sprintf(validatorPromptTemplate,
		title,
		description,
		acceptanceCriteria,
		executionResult,
	)
}

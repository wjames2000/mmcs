package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// ChatModelAgent 封装 ChatModel 实现 Agent 接口
// 将输入字符串作为用户消息传入 ChatModel 并返回生成结果
type ChatModelAgent struct {
	id    string
	model model_gateway.ChatModel
}

// NewChatModelAgent 创建 ChatModelAgent
// id: Agent 唯一标识
// model: 底层的 ChatModel 实例
func NewChatModelAgent(id string, model model_gateway.ChatModel) *ChatModelAgent {
	return &ChatModelAgent{
		id:    id,
		model: model,
	}
}

// ID 返回 Agent 标识
func (a *ChatModelAgent) ID() string {
	return a.id
}

// Run 执行 ChatModel 生成
// 将 input 作为 user 消息发送给 ChatModel，返回生成结果
func (a *ChatModelAgent) Run(ctx context.Context, input string) (*Result, error) {
	start := time.Now()

	messages := []model_gateway.ChatMessage{
		{Role: "user", Content: input},
	}

	resp, err := a.model.Generate(ctx, &model_gateway.ChatRequest{
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("ChatModelAgent %s 执行失败: %w", a.id, err)
	}

	return &Result{
		AgentID:  a.id,
		Output:   resp.Content,
		Tokens:   resp.TotalTokens,
		Duration: time.Since(start),
	}, nil
}

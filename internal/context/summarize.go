package context

import (
	"errors"
	"fmt"

	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// SummaryMessage 摘要消息的 role 标识
const SummaryRole = "system"

// compressBySummarize 摘要压缩策略
// 将最早 N 轮（~50% 的消息）发送给 summarizeModel 生成摘要
// 用一条 SummaryMessage 替换被压缩的轮次
// 需要设置 summarizeModel
func (m *Manager) compressBySummarize(history []*Message) ([]*Message, int, error) {
	if m.summarizeModel == nil {
		return m.compressByDropOldest(history)
	}

	if len(history) <= 2 {
		return history, 0, errors.New("历史消息太少，无法摘要压缩")
	}

	counter := &SimpleTokenCounter{}

	// 分离系统消息和聊天历史
	var systemMsg []*Message
	var chatHistory []*Message
	for _, msg := range history {
		if msg.Role == "system" {
			systemMsg = append(systemMsg, msg)
		} else {
			chatHistory = append(chatHistory, msg)
		}
	}

	if len(chatHistory) < 2 {
		return history, CountMessagesTokens(history, counter), nil
	}

	// 对前半部分做摘要（最早 ~50% 的消息）
	compressCount := len(chatHistory) / 2
	toCompress := chatHistory[:compressCount]
	keep := chatHistory[compressCount:]

	// 构建摘要提示
	summaryPrompt := "请对以下对话内容生成简洁的摘要，保留关键讨论要点、结论和分歧点：\n\n"
	for i, msg := range toCompress {
		summaryPrompt += fmt.Sprintf("[消息 %d] %s: %s\n\n", i+1, msg.Role, msg.Content)
	}
	summaryPrompt += "\n请输出一段连贯的中文摘要，不要使用列表格式。"

	// 调用模型生成摘要
	resp, err := m.summarizeModel.Generate(nil, &model_gateway.ChatRequest{
		Messages: []model_gateway.ChatMessage{
			{Role: "system", Content: "你是一个对话摘要生成助手。请将用户的对话内容精炼为一段连贯的摘要。"},
			{Role: "user", Content: summaryPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   1024,
	})
	if err != nil {
		// 摘要生成失败，回退到丢弃策略
		return m.compressByDropOldest(history)
	}

	summaryContent := resp.Content
	summaryTokens := resp.TotalTokens
	if summaryTokens <= 0 {
		summaryTokens = counter.CountTokens(summaryContent)
	}

	// 计算被压缩消息的原始 token 数
	originalTokens := CountMessagesTokens(toCompress, counter)

	// 构建摘要消息
	summaryMsg := &Message{
		Role:    SummaryRole,
		Content: fmt.Sprintf("【历史摘要】以下是对之前讨论的摘要：\n%s", summaryContent),
		Metadata: map[string]interface{}{
			"compressed":      true,
			"original_tokens": originalTokens,
			"summary_tokens":  summaryTokens,
		},
	}

	// 构建结果：系统消息 + 摘要消息 + 保留的最新消息
	result := make([]*Message, 0, len(systemMsg)+1+len(keep))
	result = append(result, systemMsg...)
	result = append(result, summaryMsg)
	result = append(result, keep...)

	newTokens := CountMessagesTokens(result, counter)
	return result, newTokens, nil
}

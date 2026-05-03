package context

import "fmt"

// compressBySlidingWindow 滑动窗口策略
// 保留最近 windowSize 轮的消息
// 丢弃更早的所有消息
// 在开头插入一条 SystemMessage 说明截断
func (m *Manager) compressBySlidingWindow(history []*Message) ([]*Message, int, error) {
	if m.windowSize <= 0 {
		m.windowSize = 10
	}

	counter := &SimpleTokenCounter{}

	// 分离系统消息和聊天历史
	var systemMsg []*Message
	var chatHistory []*Message
	for _, msg := range history {
		if msg.Role == "system" && !m.isSummaryMessage(msg) {
			systemMsg = append(systemMsg, msg)
		} else {
			chatHistory = append(chatHistory, msg)
		}
	}

	// 如果聊天历史少于窗口大小，无需压缩
	if len(chatHistory) <= m.windowSize {
		return history, CountMessagesTokens(history, counter), nil
	}

	// 按轮次分组
	type roundGroup struct {
		Round    int
		Messages []*Message
	}
	rounds := make([]*roundGroup, 0)
	var currentRound *roundGroup
	for _, msg := range chatHistory {
		if currentRound == nil || msg.Round != currentRound.Round {
			currentRound = &roundGroup{Round: msg.Round}
			rounds = append(rounds, currentRound)
		}
		currentRound.Messages = append(currentRound.Messages, msg)
	}

	// 保留最近的 windowSize 轮
	var keepRounds []*roundGroup
	if len(rounds) > m.windowSize {
		keepRounds = rounds[len(rounds)-m.windowSize:]
	} else {
		keepRounds = rounds
	}

	// 构建截断通知消息
	truncatedNotice := &Message{
		Role:    "system",
		Content: fmt.Sprintf("【注意】之前的讨论已被截断，当前为最近 %d 轮的讨论内容。", len(keepRounds)),
		Metadata: map[string]interface{}{
			"truncated":    true,
			"kept_rounds":  len(keepRounds),
			"total_rounds": len(rounds),
		},
	}

	// 构建结果
	result := make([]*Message, 0, len(systemMsg)+1+len(chatHistory))
	result = append(result, systemMsg...)
	result = append(result, truncatedNotice)
	for _, rg := range keepRounds {
		result = append(result, rg.Messages...)
	}

	newTokens := CountMessagesTokens(result, counter)
	return result, newTokens, nil
}

// isSummaryMessage 检查是否是摘要消息
func (m *Manager) isSummaryMessage(msg *Message) bool {
	if msg.Metadata == nil {
		return false
	}
	compressed, ok := msg.Metadata["compressed"]
	return ok && compressed == true
}

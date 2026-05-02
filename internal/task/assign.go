package task

import (
	"strings"
)

// AgentInfo Agent 信息
// 用于任务自动分配时的匹配评估
type AgentInfo struct {
	ID          string   `json:"id"`
	Tags        []string `json:"tags"`
	CurrentLoad int      `json:"current_load"`
}

// AutoAssign 基于标签匹配自动分配 Agent
// 遍历 availableAgents，计算 task.Title+Description 与 agent.Tags 的匹配分数
// 返回匹配度最高的 Agent ID（分数相同时选择负载最低的）
// 没有匹配时返回空字符串
func AutoAssign(task *Task, availableAgents []AgentInfo) string {
	if task == nil || len(availableAgents) == 0 {
		return ""
	}

	content := strings.ToLower(task.Title + " " + task.Description)
	contentWords := tokenize(content)

	var bestAgent string
	var bestScore int
	bestLoad := -1

	for _, agent := range availableAgents {
		score := calculateMatchScore(contentWords, agent.Tags)
		if score == 0 {
			continue
		}

		if score > bestScore || (score == bestScore && (bestLoad == -1 || agent.CurrentLoad < bestLoad)) {
			bestAgent = agent.ID
			bestScore = score
			bestLoad = agent.CurrentLoad
		}
	}

	return bestAgent
}

// calculateMatchScore 计算内容与标签的匹配分数
// 每个匹配的标签贡献 1 分，标签完全匹配加分更多
func calculateMatchScore(contentWords map[string]int, tags []string) int {
	score := 0
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		// 完全匹配：tag 作为完整单词出现在 content 中
		if _, ok := contentWords[tagLower]; ok {
			score += 3
			continue
		}
		// 部分匹配：tag 是 content 中某个单词的子串或反之
		for word := range contentWords {
			if strings.Contains(word, tagLower) || strings.Contains(tagLower, word) {
				score += 1
				break
			}
		}
	}
	return score
}

// tokenize 将文本分词为单词 -> 出现次数的映射
func tokenize(text string) map[string]int {
	words := make(map[string]int)
	current := strings.Builder{}

	for _, ch := range text {
		if isWordChar(ch) {
			current.WriteRune(ch)
		} else {
			if current.Len() > 0 {
				word := strings.TrimSpace(current.String())
				if word != "" {
					words[word]++
				}
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		word := strings.TrimSpace(current.String())
		if word != "" {
			words[word]++
		}
	}

	return words
}

// isWordChar 判断字符是否属于单词字符（字母、数字、下划线、中文字符）
func isWordChar(ch rune) bool {
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
		return true
	}
	// 中文字符范围
	if ch >= 0x4E00 && ch <= 0x9FFF {
		return true
	}
	if ch >= 0x3400 && ch <= 0x4DBF {
		return true
	}
	return false
}

package context

import (
	"fmt"
	"strings"
)

// BuildRetrieverContext 将记忆格式化为提示词上下文
// 格式：
// ## 你的记忆
// - 第 X 轮讨论中，你提到过...
// - 第 Y 轮讨论中，你的观点是...
func BuildRetrieverContext(memories []*MemoryItem) string {
	if len(memories) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## 你的记忆\n")
	builder.WriteString("以下是你之前讨论中表达过的观点和记忆，请参考它们保持一致性：\n\n")

	for i, mem := range memories {
		if mem.Content == "" {
			continue
		}
		// 截取内容前 200 字符
		content := mem.Content
		if len([]rune(content)) > 200 {
			content = string([]rune(content)[:200]) + "..."
		}
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
	}

	return builder.String()
}

// BuildRetrieverContextWithTime 带时间标记的记忆上下文
func BuildRetrieverContextWithTime(memories []*MemoryItem) string {
	if len(memories) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## 你的记忆\n")
	builder.WriteString("以下是你之前讨论中表达过的观点和记忆：\n\n")

	for i, mem := range memories {
		if mem.Content == "" {
			continue
		}
		content := mem.Content
		if len([]rune(content)) > 200 {
			content = string([]rune(content)[:200]) + "..."
		}
		timeStr := mem.CreatedAt.Format("01-02 15:04")
		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, timeStr, content))
	}

	return builder.String()
}

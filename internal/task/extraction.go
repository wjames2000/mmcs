package task

import (
	"fmt"
	"strings"

	"github.com/wjames2000/mmcs/internal/minutes"
)

// ExtractFromMinutes 从 MeetingMinutes 中提取任务列表
// 解析 Decisions 和 Conclusion，每个 Decision 生成一个 Task
// 如果没有 Decision，从 Conclusion 中提取关键行动项
func ExtractFromMinutes(mm *minutes.MeetingMinutes) ([]*Task, error) {
	if mm == nil {
		return nil, fmt.Errorf("MeetingMinutes 不能为空")
	}

	var tasks []*Task

	// 从 Decisions 中提取任务
	if len(mm.Decisions) > 0 {
		for _, d := range mm.Decisions {
			if !d.Accepted {
				continue
			}

			title := d.Title
			if title == "" {
				title = "讨论结论"
			}

			desc := d.Description
			if desc == "" {
				desc = d.Title
			}

			task := &Task{
				// ID, SessionID, WorkspaceID 由上层调用者填充
				Title:              title,
				Description:        desc,
				AcceptanceCriteria: d.Description,
				Status:             StatusPending,
				Priority:           PriorityMedium, // 现有 Decision 无 confidence 字段，默认 medium
			}

			tasks = append(tasks, task)
		}
	}

	// 从 Conclusion 中提取关键行动项（当没有 Decisions 时）
	if len(tasks) == 0 && mm.Conclusion != "" {
		items := ExtractActionItems(mm.Conclusion)
		for _, item := range items {
			task := &Task{
				Title:              item,
				Description:        mm.Conclusion,
				AcceptanceCriteria: "完成 " + item,
				Status:             StatusPending,
				Priority:           PriorityMedium,
			}
			tasks = append(tasks, task)
		}
	}

	// 如果既没有 Decisions 也没有 Conclusion，返回空列表
	if tasks == nil {
		tasks = []*Task{}
	}

	return tasks, nil
}

// ExtractActionItems 从结论文本中提取关键行动项
// 基于简单的启发式规则：按句号/换行分割，过滤包含行动关键词的句子
func ExtractActionItems(text string) []string {
	if text == "" {
		return nil
	}

	// 按换行分割
	lines := strings.Split(text, "\n")

	// 也按句号分割
	if len(lines) <= 1 {
		lines = strings.Split(text, "。")
	}

	var items []string
	actionKeywords := []string{
		"需要", "应该", "必须", "建议", "推荐",
		"需", "应", "要", "请",
		"implement", "create", "build", "add", "fix", "update",
		"refactor", "write", "开发", "实现", "完成", "处理",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		hasKeyword := false
		lower := strings.ToLower(line)
		for _, kw := range actionKeywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				hasKeyword = true
				break
			}
		}

		if hasKeyword {
			items = append(items, line)
		}
	}

	// 如果没有任何匹配关键词的句子，将整个文本作为一个行动项
	if len(items) == 0 {
		items = append(items, text)
	}

	return items
}

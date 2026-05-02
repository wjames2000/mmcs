package role

import (
	"fmt"
	"strings"
)

// BuildRoleChatTemplate 构建角色聊天模板
// 将角色配置 + 技能注入为 system_prompt
// 格式：
// [角色基础设定]
// {system_prompt or 默认 prompt}
//
// [技能注入]
// ### {skill_name}
// {skill_prompt}
//
// [用户自定义补充]
// {custom_prompt}
func BuildRoleChatTemplate(role *Role, skillRegistry *SkillRegistry, customPrompt string) (string, error) {
	var builder strings.Builder

	// 基础设定
	builder.WriteString("## 角色设定\n")
	builder.WriteString(fmt.Sprintf("你是一名 **%s**，你的身份是 **%s**。\n", role.Name, role.Title))

	if role.SystemPrompt != "" {
		builder.WriteString("\n")
		builder.WriteString(role.SystemPrompt)
	}

	if len(role.Expertise) > 0 {
		builder.WriteString("\n\n## 专业领域\n")
		for _, e := range role.Expertise {
			builder.WriteString(fmt.Sprintf("- %s\n", e))
		}
	}

	if role.SpeakingStyle != "" {
		builder.WriteString("\n\n## 说话风格\n")
		builder.WriteString(role.SpeakingStyle)
		builder.WriteString("\n")
	}

	// 技能注入
	if len(role.Skills) > 0 {
		builder.WriteString("\n## 技能\n")
		for _, skillName := range role.Skills {
			skill, err := skillRegistry.Get(skillName)
			if err != nil {
				return "", fmt.Errorf("获取技能 %s 失败: %w", skillName, err)
			}
			builder.WriteString(fmt.Sprintf("\n### %s\n", skill.Name))
			builder.WriteString(skill.Prompt)
			builder.WriteString("\n")
		}
	}

	// 用户自定义补充
	if customPrompt != "" {
		builder.WriteString("\n## 额外指令\n")
		builder.WriteString(customPrompt)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

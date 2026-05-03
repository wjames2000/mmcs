package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
)

// FreeChatConfig 自由群聊范式配置
type FreeChatConfig struct {
	RoleIDs      []string // 参与讨论的角色 ID
	Topic        string   // 讨论话题
	CustomPrompt string   // 用户自定义额外提示
	MaxRounds    int      // 最大讨论轮次

	// InterruptCh 和 ResumeCh 用于支持人类介入（Pause/Resume）
	InterruptCh chan *session.InterruptSignal `json:"-"`
	ResumeCh    chan *session.ResumeSignal    `json:"-"`
}

// FreeChatOrchestrator 自由群聊范式编排器
// 使用 SupervisorAgent 作为主持人，动态调度各角色发言：
// 1. 将各角色包装为 ChatModelAgent
// 2. 创建 SupervisorAgent 作为主持人
// 3. 主持人分析当前讨论 → 选择下一个发言人 → 该角色发言 → 主持人评估 → 继续或结束
type FreeChatOrchestrator struct {
	contextInitNode   *ContextInitNode
	expertSpeakNode   *ExpertSpeakNode
	moderatorEvalNode *ModeratorEvalNode
	summarizeNode     *SummarizeNode
}

// NewFreeChatOrchestrator 创建自由群聊编排器
func NewFreeChatOrchestrator(
	contextInitNode *ContextInitNode,
	expertSpeakNode *ExpertSpeakNode,
	moderatorEvalNode *ModeratorEvalNode,
	summarizeNode *SummarizeNode,
) *FreeChatOrchestrator {
	return &FreeChatOrchestrator{
		contextInitNode:   contextInitNode,
		expertSpeakNode:   expertSpeakNode,
		moderatorEvalNode: moderatorEvalNode,
		summarizeNode:     summarizeNode,
	}
}

// Execute 执行一次完整的自由群聊讨论
// sessionID: 会话 ID
// config: 自由群聊配置
// bridge: SSE 事件桥（可为 nil）
// progressCh: 进度通知 channel（可为 nil）
func (f *FreeChatOrchestrator) Execute(
	ctx context.Context,
	sessionID string,
	config *FreeChatConfig,
	bridge *stream.Bridge,
	progressCh chan<- string,
) (*MeetingMinutes, error) {
	log.Info().Str("session_id", sessionID).Int("roles", len(config.RoleIDs)).
		Str("topic", config.Topic).Msg("自由群聊讨论开始")

	// 1. 初始化角色上下文
	if progressCh != nil {
		progressCh <- "正在初始化角色上下文..."
	}

	roleContexts, err := f.contextInitNode.InitRoleContexts(ctx, config.RoleIDs, config.CustomPrompt)
	if err != nil {
		return nil, fmt.Errorf("初始化角色上下文失败: %w", err)
	}

	// 创建讨论状态
	state := NewDiscussionState(sessionID, config.MaxRounds, bridge)
	state.Roles = roleContexts
	if config.InterruptCh != nil && config.ResumeCh != nil {
		state.SetInterruptChannels(config.InterruptCh, config.ResumeCh)
	}

	// 推送开始事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "free_chat",
			Content:   config.Topic,
			Timestamp: time.Now(),
		})
	}

	// 2. 构建子 Agent（每个角色一个 ChatModelAgent）
	//    选择第一个角色的 ChatModel 作为主持人模型
	if len(roleContexts) == 0 {
		return nil, fmt.Errorf("没有可用的角色")
	}

	supervisorModel := roleContexts[0].ChatModel

	// 3. 自由群聊主循环
	for round := 1; round <= config.MaxRounds; round++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 检查中断/恢复信号（人类介入）
		if !CheckInterrupt(ctx, state.InterruptCh, state.ResumeCh, bridge) {
			log.Info().Str("session_id", sessionID).Int("round", round).Msg("自由群聊在中断后取消")
			if progressCh != nil {
				close(progressCh)
			}
			return nil, ctx.Err()
		}

		if progressCh != nil {
			progressCh <- fmt.Sprintf("第 %d 轮讨论开始...", round)
		}

		state.IncrementRound()

		// 4a. 构建当前讨论上下文
		topic := f.buildDiscussionContext(config.Topic, state, round)

		// 4b. 主持人分析并选择发言人
		selectedRoleID, err := f.selectNextSpeaker(ctx, supervisorModel, roleContexts, topic, state)
		if err != nil {
			log.Warn().Err(err).Str("session_id", sessionID).Int("round", round).Msg("选择发言人失败，跳过本轮")
			continue
		}

		log.Debug().Str("session_id", sessionID).Int("round", round).Str("speaker", selectedRoleID).Msg("选定发言人")

		// 查找选中的角色
		var selectedRC *RoleContext
		for _, rc := range roleContexts {
			if rc.Role.ID == selectedRoleID {
				selectedRC = rc
				break
			}
		}
		if selectedRC == nil {
			log.Warn().Str("session_id", sessionID).Str("role_id", selectedRoleID).Msg("选中的发言人 ID 无效，跳过本轮")
			continue
		}

		// 推送选择发言人事件
		if bridge != nil {
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "node_start",
				NodeName:  "free_chat_speak",
				RoleName:  selectedRC.Role.Name,
				Timestamp: time.Now(),
			})
		}

		// 4c. 选中的角色发言
		speakResult, err := selectedRC.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
			Messages: f.buildRoleMessages(selectedRC, topic, state),
		})
		if err != nil {
			log.Error().Err(err).Str("role", selectedRC.Role.Name).Msg("发言人发言失败")
			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "error",
					NodeName:  "free_chat_speak",
					RoleName:  selectedRC.Role.Name,
					Error:     err.Error(),
					Timestamp: time.Now(),
				})
			}
			continue
		}

		// 添加到历史
		state.AddHistory(&model_gateway.ChatMessage{
			Role:    "assistant",
			Content: speakResult.Content,
		})

		// 推送发言事件
		if bridge != nil {
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "agent_speak",
				NodeName:  "free_chat_speak",
				RoleName:  selectedRC.Role.Name,
				Content:   speakResult.Content,
				Timestamp: time.Now(),
			})
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "node_end",
				NodeName:  "free_chat_speak",
				RoleName:  selectedRC.Role.Name,
				Timestamp: time.Now(),
			})
		}

		log.Debug().Str("role", selectedRC.Role.Name).Int("round", round).
			Int("tokens", speakResult.TotalTokens).Msg("发言完成")

		// 4d. 主持人评估是否继续
		evalResult := f.moderatorEvalNode.Evaluate(state)
		if !evalResult.ShouldContinue {
			log.Info().Str("session_id", sessionID).
				Int("round", round).Msg("主持人决定结束讨论")

			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "moderator_eval",
					NodeName:  "free_chat_moderator",
					Content:   evalResult.Summary,
					Timestamp: time.Now(),
				})
			}

			break
		}
	}

	// 5. 生成总结
	if progressCh != nil {
		progressCh <- "生成讨论总结..."
	}
	minutes := f.summarizeNode.GenerateSummary(state, f.moderatorEvalNode.Evaluate(state))

	// 推送结束事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "done",
			NodeName:  "free_chat",
			Content:   "自由群聊讨论结束",
			Metadata:  minutes,
			Timestamp: time.Now(),
		})
	}

	log.Info().Str("session_id", sessionID).
		Int("rounds", state.GetCurrentRound()).
		Int("messages", len(state.GetHistory())).
		Msg("自由群聊讨论结束")

	if progressCh != nil {
		close(progressCh)
	}
	return minutes, nil
}

// buildDiscussionContext 构建当前讨论上下文描述
func (f *FreeChatOrchestrator) buildDiscussionContext(topic string, state *DiscussionState, round int) string {
	return fmt.Sprintf("第 %d 轮讨论\n话题：%s\n已有 %d 条讨论消息", round, topic, len(state.GetHistory()))
}

// selectNextSpeaker 调用模型选择下一个发言人
func (f *FreeChatOrchestrator) selectNextSpeaker(
	ctx context.Context,
	model model_gateway.ChatModel,
	roleContexts []*RoleContext,
	topic string,
	state *DiscussionState,
) (string, error) {
	// 构建所有可用角色列表
	rolesDesc := ""
	for i, rc := range roleContexts {
		rolesDesc += fmt.Sprintf("%d. %s (ID: %s) - %s\n", i+1, rc.Role.Name, rc.Role.ID, rc.Role.Title)
	}

	prompt := fmt.Sprintf(`你是一个自由群聊的主持人。请根据当前讨论进展，从以下角色中选择最合适的下一位发言人。

可用角色：
%s

讨论话题：%s
当前轮次：%d
已发言消息数：%d

请仅返回你选择发言的角色 ID（不加任何其他内容）。`,
		rolesDesc, topic, state.GetCurrentRound(), len(state.GetHistory()))

	messages := []model_gateway.ChatMessage{
		{Role: "system", Content: "你是群聊主持人。严格按要求格式输出角色 ID。"},
		{Role: "user", Content: prompt},
	}

	resp, err := model.Generate(ctx, &model_gateway.ChatRequest{
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("选择发言人失败: %w", err)
	}

	selectedID := resp.Content
	// 清理返回
	for len(selectedID) > 0 && (selectedID[0] == ' ' || selectedID[0] == '\t' || selectedID[0] == '\n' || selectedID[0] == '\r' || selectedID[0] == '"' || selectedID[0] == '\'' || selectedID[0] == '`') {
		selectedID = selectedID[1:]
	}
	for len(selectedID) > 0 && (selectedID[len(selectedID)-1] == ' ' || selectedID[len(selectedID)-1] == '\t' || selectedID[len(selectedID)-1] == '\n' || selectedID[len(selectedID)-1] == '\r' || selectedID[len(selectedID)-1] == '"' || selectedID[len(selectedID)-1] == '\'' || selectedID[len(selectedID)-1] == '`') {
		selectedID = selectedID[:len(selectedID)-1]
	}

	if selectedID == "" {
		// 默认选择第一个角色
		return roleContexts[0].Role.ID, nil
	}
	return selectedID, nil
}

// buildRoleMessages 构建角色的发言消息
func (f *FreeChatOrchestrator) buildRoleMessages(rc *RoleContext, topic string, state *DiscussionState) []model_gateway.ChatMessage {
	messages := []model_gateway.ChatMessage{
		{Role: "system", Content: rc.Prompt},
	}

	history := state.GetHistory()
	for _, msg := range history {
		messages = append(messages, *msg)
	}

	messages = append(messages, model_gateway.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("请针对以下话题发表你的专业意见：\n\n%s", topic),
	})

	return messages
}

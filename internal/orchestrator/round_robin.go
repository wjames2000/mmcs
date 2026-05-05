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

// RoundRobinConfig 轮询发言范式配置
type RoundRobinConfig struct {
	RoleIDs      []string // 参与讨论的角色 ID
	Topic        string   // 讨论话题
	CustomPrompt string   // 用户自定义额外提示
	MaxRounds    int

	// ModeratorModelBinding 主持人模型绑定（可选）
	// 格式如 "openai" 或 "openai:gpt-4"
	// 为空时使用默认模型
	ModeratorModelBinding string

	// InterruptCh 和 ResumeCh 用于支持人类介入（Pause/Resume）
	InterruptCh chan *session.InterruptSignal `json:"-"`
	ResumeCh    chan *session.ResumeSignal    `json:"-"`

	// MsgStore 消息持久化存储（可选）
	MsgStore *session.MessageStore `json:"-"`
}

// RoundRobinOrchestrator 轮询发言范式编排器
// 实现标准的 round_robin 讨论模式：
// 1. 初始化角色上下文
// 2. 每轮所有角色依次发言
// 3. 主持人评估是否继续
// 4. 达到结束条件时生成总结
type RoundRobinOrchestrator struct {
	contextInitNode   *ContextInitNode
	expertSpeakNode   *ExpertSpeakNode
	moderatorEvalNode *ModeratorEvalNode
	summarizeNode     *SummarizeNode
	gateway           ModelGatewayInterface // 模型网关，用于主持人模型绑定
}

// NewRoundRobinOrchestrator 创建轮询编排器
func NewRoundRobinOrchestrator(
	contextInitNode *ContextInitNode,
	expertSpeakNode *ExpertSpeakNode,
	moderatorEvalNode *ModeratorEvalNode,
	summarizeNode *SummarizeNode,
	gateway ModelGatewayInterface,
) *RoundRobinOrchestrator {
	return &RoundRobinOrchestrator{
		contextInitNode:   contextInitNode,
		expertSpeakNode:   expertSpeakNode,
		moderatorEvalNode: moderatorEvalNode,
		summarizeNode:     summarizeNode,
		gateway:           gateway,
	}
}

// Execute 执行一次完整的轮询讨论
// sessionID: 会话 ID
// bridge: SSE 事件桥（可为 nil）
// progressCh: 进度通知 channel（可为 nil）
func (r *RoundRobinOrchestrator) Execute(
	ctx context.Context,
	sessionID string,
	config *RoundRobinConfig,
	bridge *stream.Bridge,
	progressCh chan<- string,
) (*MeetingMinutes, error) {
	log.Info().Str("session_id", sessionID).Int("roles", len(config.RoleIDs)).Msg("轮询讨论开始")

	// 1. 初始化角色上下文
	if progressCh != nil {
		progressCh <- "正在初始化角色上下文..."
	}

	roleContexts, err := r.contextInitNode.InitRoleContexts(ctx, config.RoleIDs, config.CustomPrompt)
	if err != nil {
		return nil, fmt.Errorf("初始化角色上下文失败: %w", err)
	}

	// 应用主持人模型绑定
	if config.ModeratorModelBinding != "" && len(roleContexts) > 0 && r.gateway != nil {
		modModel, modErr := r.gateway.GetChatModel(config.ModeratorModelBinding)
		if modErr != nil {
			log.Warn().Err(modErr).Str("binding", config.ModeratorModelBinding).Msg("获取主持人模型失败，使用默认模型")
		} else {
			roleContexts[0].ChatModel = modModel
			log.Info().Str("binding", config.ModeratorModelBinding).Str("role", roleContexts[0].Role.Name).Msg("主持人模型绑定成功")
		}
	}

	// 2. 创建讨论状态
	state := NewDiscussionState(sessionID, config.MaxRounds, bridge)
	state.Roles = roleContexts
	if config.InterruptCh != nil && config.ResumeCh != nil {
		state.SetInterruptChannels(config.InterruptCh, config.ResumeCh)
	}
	if config.MsgStore != nil {
		state.SetMessageStore(config.MsgStore)
	}

	// 3. 主持人开场白
	if len(roleContexts) > 0 {
		moderatorRC := roleContexts[0] // 第一个角色作为主持人
		if progressCh != nil {
			progressCh <- fmt.Sprintf("主持人 %s 开场发言...", moderatorRC.Role.Name)
		}

		// 生成开场白（包含附件/主题背景信息）
		var openingStatement string
		if moderatorRC.ChatModel == nil {
			openingStatement = generateSimulatedOpinion(moderatorRC.Role, config.Topic, 1, nil)
		} else {
			openingPrompt := fmt.Sprintf(`作为本次会议的主持人，请为关于「%s」的讨论做一段开场白。

请包含以下内容：
1. 欢迎各位专家参与讨论
2. 复述本次讨论的议题和背景
3. 列出本次需要解决的关键问题
4. 引导各位专家依次发言`, config.Topic)

			modMessages := []model_gateway.ChatMessage{
				{Role: "system", Content: moderatorRC.Prompt},
				{Role: "user", Content: openingPrompt},
			}
			resp, err := moderatorRC.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
				Messages: modMessages,
			})
			if err != nil {
				log.Warn().Err(err).Str("role", moderatorRC.Role.Name).Msg("主持人开场失败，使用模拟")
				openingStatement = generateSimulatedOpinion(moderatorRC.Role, config.Topic, 1, nil)
			} else {
				openingStatement = resp.Content
			}
		}

		// 添加开场白到历史
		state.AddHistory(&model_gateway.ChatMessage{
			Role:    "assistant",
			Content: openingStatement,
		})

		// 持久化开场白消息
		if state.MsgStore != nil {
			state.MsgStore.Add(sessionID, 0, moderatorRC.Role.Name, openingStatement, len(openingStatement)/4)
		}

		// 推送事件
		if bridge != nil {
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "moderator_speech",
				NodeName:  "opening",
				RoleName:  moderatorRC.Role.Name,
				Content:   openingStatement,
				Timestamp: time.Now(),
			})
		}

		log.Info().Str("session_id", sessionID).Str("moderator", moderatorRC.Role.Name).Msg("主持人开场发言完成")
	}

	// 4. 主循环：每轮发言 → 评估
	for round := 1; round <= config.MaxRounds; round++ {
		// 检查上下文取消
		select {
		case <-ctx.Done():
			log.Info().Str("session_id", sessionID).Int("round", round).Msg("讨论被取消")
			return nil, ctx.Err()
		default:
		}

		// 检查中断/恢复信号（人类介入）
		if !CheckInterrupt(ctx, state.InterruptCh, state.ResumeCh, bridge) {
			log.Info().Str("session_id", sessionID).Int("round", round).Msg("讨论在中断后取消")
			return nil, ctx.Err()
		}

		state.IncrementRound()
		currentRound := state.GetCurrentRound()

		if progressCh != nil {
			progressCh <- fmt.Sprintf("第 %d 轮讨论开始...", currentRound)
		}

		// 推送轮次开始事件
		if bridge != nil {
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "round_start",
				NodeName:  fmt.Sprintf("round_%d", currentRound),
				Content:   config.Topic,
				Timestamp: time.Now(),
			})
		}

		// 专家发言
		results := r.expertSpeakNode.Execute(ctx, roleContexts, config.Topic, state)

		// 检查是否有错误
		for _, res := range results {
			if res.Error != nil {
				log.Error().Err(res.Error).Str("role", res.RoleName).Msg("发言出错")
				// 继续讨论，不因单个错误终止
			}
		}

		// 生成主持人总结（含所有角色本轮核心观点，供下一轮精简上下文使用）
		if len(roleContexts) > 0 {
			state.LastRoundSummary = generateRoundSummary(config.Topic, currentRound, results)
		}

		// 主持人评估
		evalResult := r.moderatorEvalNode.Evaluate(state)

		// 推送主持人评估消息到聊天窗口
		if bridge != nil {
			evalContent := fmt.Sprintf("【主持人评估】第 %d 轮\n%s", currentRound, evalResult.Reason)
			if evalResult.Solved {
				evalContent += "\n\n✅ 问题已解决，讨论结束"
			} else if !evalResult.ShouldContinue {
				evalContent += "\n\n📋 讨论结束"
			} else {
				evalContent += "\n\n🔄 继续下一轮讨论"
			}
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "moderator_speech",
				NodeName:  fmt.Sprintf("round_%d_eval", currentRound),
				RoleName:  roleContexts[0].Role.Name,
				Content:   evalContent,
				Timestamp: time.Now(),
			})
		}

		if progressCh != nil {
			progressCh <- fmt.Sprintf("第 %d 轮完成: %s", currentRound, evalResult.Reason)
		}

		// 轮次之间添加短暂延迟，让讨论有真实感（模拟模式下尤其重要）
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		if !evalResult.ShouldContinue {
			// 生成总结
			minutes := r.summarizeNode.GenerateSummary(state, evalResult)

			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "done",
					Content:   "讨论结束",
					Metadata:  minutes,
					Timestamp: time.Now(),
				})
			}

			log.Info().Str("session_id", sessionID).
				Int("rounds", currentRound).
				Int("messages", len(state.GetHistory())).
				Msg("讨论结束")

			if progressCh != nil {
				close(progressCh)
			}
			return minutes, nil
		}
	}

	// 防止编译错误：正常情况下循环内已 return，此处仅防御性保留
	state.CurrentRound = config.MaxRounds
	evalResult := r.moderatorEvalNode.Evaluate(state)
	minutes := r.summarizeNode.GenerateSummary(state, evalResult)

	log.Warn().Str("session_id", sessionID).
		Int("rounds", config.MaxRounds).
		Msg("讨论异常结束（循环外兜底）")

	if progressCh != nil {
		close(progressCh)
	}
	return minutes, nil
}

package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcs/internal/stream"
	"github.com/rs/zerolog/log"
)

// RoundRobinConfig 轮询发言范式配置
type RoundRobinConfig struct {
	RoleIDs      []string // 参与讨论的角色 ID
	Topic        string   // 讨论话题
	CustomPrompt string   // 用户自定义额外提示
	MaxRounds    int
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
}

// NewRoundRobinOrchestrator 创建轮询编排器
func NewRoundRobinOrchestrator(
	contextInitNode *ContextInitNode,
	expertSpeakNode *ExpertSpeakNode,
	moderatorEvalNode *ModeratorEvalNode,
	summarizeNode *SummarizeNode,
) *RoundRobinOrchestrator {
	return &RoundRobinOrchestrator{
		contextInitNode:   contextInitNode,
		expertSpeakNode:   expertSpeakNode,
		moderatorEvalNode: moderatorEvalNode,
		summarizeNode:     summarizeNode,
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

	// 2. 创建讨论状态
	state := NewDiscussionState(sessionID, config.MaxRounds, bridge)
	state.Roles = roleContexts

	// 3. 主循环：每轮发言 → 评估
	for round := 1; round <= config.MaxRounds; round++ {
		select {
		case <-ctx.Done():
			log.Info().Str("session_id", sessionID).Int("round", round).Msg("讨论被取消")
			return nil, ctx.Err()
		default:
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

		// 主持人评估
		evalResult := r.moderatorEvalNode.Evaluate(state)

		if progressCh != nil {
			progressCh <- fmt.Sprintf("第 %d 轮完成: %s", currentRound, evalResult.Reason)
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

	// 达到最大轮次
	state.CurrentRound = config.MaxRounds
	evalResult := r.moderatorEvalNode.Evaluate(state)
	minutes := r.summarizeNode.GenerateSummary(state, evalResult)

	log.Info().Str("session_id", sessionID).
		Int("rounds", config.MaxRounds).
		Int("messages", len(state.GetHistory())).
		Msg("讨论达到最大轮次")

	if progressCh != nil {
		close(progressCh)
	}
	return minutes, nil
}

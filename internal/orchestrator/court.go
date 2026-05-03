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

// CourtConfig 法庭范式配置
type CourtConfig struct {
	RoleIDs      []string // 参与讨论的角色 ID
	Topic        string   // 讨论话题
	CustomPrompt string   // 用户自定义额外提示
	AuthorRoleID string   // 代码作者/方案提出者角色 ID
	MaxRounds    int      // 最大评审-回应轮次

	// InterruptCh 和 ResumeCh 用于支持人类介入（Pause/Resume）
	InterruptCh chan *session.InterruptSignal `json:"-"`
	ResumeCh    chan *session.ResumeSignal    `json:"-"`
}

// CourtOrchestrator 法庭范式编排器
// 实现模拟法庭的讨论模式：
// 1. author_statement — 代码作者/方案提出者陈述设计意图
// 2. review_phase — 审查员并行审查
// 3. author_response — 作者逐条回应审查意见
// 4. final_summary — 主持 AI 汇总总结
type CourtOrchestrator struct {
	contextInitNode   *ContextInitNode
	expertSpeakNode   *ExpertSpeakNode
	moderatorEvalNode *ModeratorEvalNode
	summarizeNode     *SummarizeNode
}

// NewCourtOrchestrator 创建法庭范式编排器
func NewCourtOrchestrator(
	contextInitNode *ContextInitNode,
	expertSpeakNode *ExpertSpeakNode,
	moderatorEvalNode *ModeratorEvalNode,
	summarizeNode *SummarizeNode,
) *CourtOrchestrator {
	return &CourtOrchestrator{
		contextInitNode:   contextInitNode,
		expertSpeakNode:   expertSpeakNode,
		moderatorEvalNode: moderatorEvalNode,
		summarizeNode:     summarizeNode,
	}
}

// Execute 执行一次完整的法庭讨论
// sessionID: 会话 ID
// config: 法庭范式配置
// bridge: SSE 事件桥（可为 nil）
// progressCh: 进度通知 channel（可为 nil）
func (c *CourtOrchestrator) Execute(
	ctx context.Context,
	sessionID string,
	config *CourtConfig,
	bridge *stream.Bridge,
	progressCh chan<- string,
) (*MeetingMinutes, error) {
	log.Info().Str("session_id", sessionID).Int("roles", len(config.RoleIDs)).
		Str("author", config.AuthorRoleID).Msg("法庭讨论开始")

	// 1. 初始化角色上下文
	if progressCh != nil {
		progressCh <- "正在初始化角色上下文..."
	}

	roleContexts, err := c.contextInitNode.InitRoleContexts(ctx, config.RoleIDs, config.CustomPrompt)
	if err != nil {
		return nil, fmt.Errorf("初始化角色上下文失败: %w", err)
	}

	// 创建讨论状态
	state := NewDiscussionState(sessionID, config.MaxRounds, bridge)
	state.Roles = roleContexts
	if config.InterruptCh != nil && config.ResumeCh != nil {
		state.SetInterruptChannels(config.InterruptCh, config.ResumeCh)
	}

	// 分离作者角色和审查员角色
	var authorRC *RoleContext
	reviewers := make([]*RoleContext, 0, len(roleContexts)-1)
	for _, rc := range roleContexts {
		if rc.Role.ID == config.AuthorRoleID {
			authorRC = rc
		} else {
			reviewers = append(reviewers, rc)
		}
	}
	if authorRC == nil {
		return nil, fmt.Errorf("作者角色 %s 未在角色列表中找到", config.AuthorRoleID)
	}

	// 推送法庭讨论开始事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "court",
			Content:   config.Topic,
			Timestamp: time.Now(),
		})
	}

	// ===== 阶段 1: 作者陈述 =====
	if progressCh != nil {
		progressCh <- "作者陈述中..."
	}
	// 检查中断（人类可在阶段之间介入）
	if !CheckInterrupt(ctx, state.InterruptCh, state.ResumeCh, bridge) {
		if progressCh != nil {
			close(progressCh)
		}
		return nil, ctx.Err()
	}
	minutes, err := c.executeAuthorStatement(ctx, authorRC, config.Topic, state, bridge)
	if err != nil {
		return nil, err
	}

	// ===== 阶段 2: 审查阶段（并行） =====
	if len(reviewers) > 0 {
		if progressCh != nil {
			progressCh <- fmt.Sprintf("%d 位审查员并行审查中...", len(reviewers))
		}
		if !CheckInterrupt(ctx, state.InterruptCh, state.ResumeCh, bridge) {
			if progressCh != nil {
				close(progressCh)
			}
			return nil, ctx.Err()
		}
		c.executeReviewPhase(ctx, reviewers, config.Topic, state, bridge)
	} else {
		log.Warn().Str("session_id", sessionID).Msg("没有审查员角色，跳过审查阶段")
	}

	// ===== 阶段 3: 作者回应 =====
	if progressCh != nil {
		progressCh <- "作者回应审查意见..."
	}
	if !CheckInterrupt(ctx, state.InterruptCh, state.ResumeCh, bridge) {
		if progressCh != nil {
			close(progressCh)
		}
		return nil, ctx.Err()
	}
	if err := c.executeAuthorResponse(ctx, authorRC, state, bridge); err != nil {
		return nil, err
	}

	// ===== 阶段 4: 最终总结 =====
	if progressCh != nil {
		progressCh <- "生成讨论总结..."
	}
	// 最终阶段前也检查中断
	CheckInterrupt(ctx, state.InterruptCh, state.ResumeCh, bridge)
	summary := c.executeFinalSummary(state, bridge)

	// 推送讨论结束事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "done",
			NodeName:  "court",
			Content:   "法庭讨论结束",
			Metadata:  minutes,
			Timestamp: time.Now(),
		})
	}

	log.Info().Str("session_id", sessionID).
		Int("messages", len(state.GetHistory())).
		Msg("法庭讨论结束")

	if progressCh != nil {
		close(progressCh)
	}
	return summary, nil
}

// executeAuthorStatement 执行作者陈述阶段
func (c *CourtOrchestrator) executeAuthorStatement(
	ctx context.Context,
	authorRC *RoleContext,
	topic string,
	state *DiscussionState,
	bridge *stream.Bridge,
) (*MeetingMinutes, error) {
	state.IncrementRound()

	// 推送作者陈述事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "author_statement",
			RoleName:  authorRC.Role.Name,
			Content:   topic,
			Timestamp: time.Now(),
		})
	}

	// 构建发言消息
	messages := []model_gateway.ChatMessage{
		{Role: "system", Content: authorRC.Prompt},
		{Role: "user", Content: fmt.Sprintf("请详细陈述你的设计方案或代码实现意图。\n\n主题：%s\n\n请从以下方面阐述：\n1. 设计思路和架构决策\n2. 关键实现细节\n3. 预期效果和收益\n4. 潜在的改进空间", topic)},
	}

	resp, err := authorRC.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
		Messages:    messages,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("作者陈述失败: %w", err)
	}

	// 添加到历史
	state.AddHistory(&model_gateway.ChatMessage{
		Role:    "assistant",
		Content: resp.Content,
	})

	// 推送事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "agent_speak",
			NodeName:  "author_statement",
			RoleName:  authorRC.Role.Name,
			Content:   resp.Content,
			Timestamp: time.Now(),
		})
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "author_statement",
			RoleName:  authorRC.Role.Name,
			Timestamp: time.Now(),
		})
	}

	log.Debug().Str("role", authorRC.Role.Name).Int("tokens", resp.TotalTokens).Msg("作者陈述完成")
	return nil, nil
}

// executeReviewPhase 执行审查阶段
func (c *CourtOrchestrator) executeReviewPhase(
	ctx context.Context,
	reviewers []*RoleContext,
	topic string,
	state *DiscussionState,
	bridge *stream.Bridge,
) {
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "review_phase",
			Timestamp: time.Now(),
		})
	}

	// 使用 ExpertSpeakNode 并行执行审查
	results := c.expertSpeakNode.Execute(ctx, reviewers, topic, state)

	for _, res := range results {
		if res.Error != nil {
			log.Error().Err(res.Error).Str("role", res.RoleName).Msg("审查发言出错")
		}
	}

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "review_phase",
			Timestamp: time.Now(),
		})
	}

	log.Debug().Int("reviewers", len(reviewers)).Msg("审查阶段完成")
}

// executeAuthorResponse 执行作者回应阶段
func (c *CourtOrchestrator) executeAuthorResponse(
	ctx context.Context,
	authorRC *RoleContext,
	state *DiscussionState,
	bridge *stream.Bridge,
) error {
	// 推送作者回应事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "author_response",
			RoleName:  authorRC.Role.Name,
			Timestamp: time.Now(),
		})
	}

	// 构建回应消息（包含审查意见在内的历史）
	messages := []model_gateway.ChatMessage{
		{Role: "system", Content: authorRC.Prompt},
	}

	// 添加历史消息（包含作者的陈述和审查意见）
	for _, msg := range state.GetHistory() {
		messages = append(messages, *msg)
	}

	// 添加回应指令
	messages = append(messages, model_gateway.ChatMessage{
		Role: "user",
		Content: `请逐条回应审查意见，对于每条意见请说明：
1. 是否接受该意见
2. 如果接受，说明修改方案
3. 如果不接受，说明理由
4. 如果有争议点，提出你的论据

请以结构化方式回应，每条意见对应一个条目。`,
	})

	resp, err := authorRC.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
		Messages:    messages,
		Temperature: 0.7,
	})
	if err != nil {
		return fmt.Errorf("作者回应失败: %w", err)
	}

	// 添加到历史
	state.AddHistory(&model_gateway.ChatMessage{
		Role:    "assistant",
		Content: resp.Content,
	})

	// 推送事件
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "agent_speak",
			NodeName:  "author_response",
			RoleName:  authorRC.Role.Name,
			Content:   resp.Content,
			Timestamp: time.Now(),
		})
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "author_response",
			RoleName:  authorRC.Role.Name,
			Timestamp: time.Now(),
		})
	}

	log.Debug().Str("role", authorRC.Role.Name).Int("tokens", resp.TotalTokens).Msg("作者回应完成")
	return nil
}

// executeFinalSummary 执行最终总结
func (c *CourtOrchestrator) executeFinalSummary(state *DiscussionState, bridge *stream.Bridge) *MeetingMinutes {
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "final_summary",
			Timestamp: time.Now(),
		})
	}

	evalResult := c.moderatorEvalNode.Evaluate(state)
	minutes := c.summarizeNode.GenerateSummary(state, evalResult)

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "final_summary",
			Content:   evalResult.Summary,
			Metadata:  minutes,
			Timestamp: time.Now(),
		})
	}

	return minutes
}

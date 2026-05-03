// Package orchestrator 提供讨论编排的核心 Graph 节点
package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
)

// RoleContext 角色运行时上下文
type RoleContext struct {
	Role      *role.Role
	ChatModel model_gateway.ChatModel
	Prompt    string // 通过 BuildRoleChatTemplate 构建
}

// DiscussionState 讨论运行时状态
type DiscussionState struct {
	mu           sync.RWMutex
	SessionID    string
	Roles        []*RoleContext
	History      []*model_gateway.ChatMessage // 历史消息
	CurrentRound int
	MaxRounds    int
	Bridge       *stream.Bridge

	// InterruptCh 和 ResumeCh 用于支持人类介入（Pause/Resume）
	InterruptCh <-chan *session.InterruptSignal
	ResumeCh    <-chan *session.ResumeSignal
}

// NewDiscussionState 创建讨论状态
func NewDiscussionState(sessionID string, maxRounds int, bridge *stream.Bridge) *DiscussionState {
	return &DiscussionState{
		SessionID:    sessionID,
		MaxRounds:    maxRounds,
		CurrentRound: 0,
		Bridge:       bridge,
	}
}

// SetInterruptChannels 设置中断/恢复 channel
func (s *DiscussionState) SetInterruptChannels(interruptCh <-chan *session.InterruptSignal, resumeCh <-chan *session.ResumeSignal) {
	s.InterruptCh = interruptCh
	s.ResumeCh = resumeCh
}

// CheckInterrupt 检查是否有中断信号
// 如果有中断信号，暂停执行并等待恢复信号
// ctx: 外层上下文
// bridge: SSE 事件桥（可为 nil）
// returns: true 表示需要继续执行，false 表示 ctx 已取消
func CheckInterrupt(ctx context.Context, interruptCh <-chan *session.InterruptSignal, resumeCh <-chan *session.ResumeSignal, bridge *stream.Bridge) bool {
	if interruptCh == nil {
		return true
	}

	select {
	case sig := <-interruptCh:
		// 广播暂停事件
		if bridge != nil {
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "session.paused",
				NodeName:  sig.NodeName,
				Content:   sig.Message,
				Timestamp: time.Now(),
			})
		}

		// 等待恢复信号
		select {
		case <-resumeCh:
			// 广播恢复事件
			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "session.resumed",
					Timestamp: time.Now(),
				})
			}
			return true
		case <-ctx.Done():
			return false
		}
	case <-ctx.Done():
		return false
	default:
		return true
	}
}

// WaitForInterrupt 阻塞等待中断信号（用于暂停点）
func WaitForInterrupt(ctx context.Context, interruptCh <-chan *session.InterruptSignal, resumeCh <-chan *session.ResumeSignal, bridge *stream.Bridge) bool {
	if interruptCh == nil {
		return true
	}

	select {
	case sig := <-interruptCh:
		if bridge != nil {
			_ = bridge.Push(&stream.GraphEvent{
				Type:      "session.paused",
				NodeName:  sig.NodeName,
				Content:   sig.Message,
				Timestamp: time.Now(),
			})
		}

		select {
		case <-resumeCh:
			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "session.resumed",
					Timestamp: time.Now(),
				})
			}
			return true
		case <-ctx.Done():
			return false
		}
	case <-ctx.Done():
		return false
	}
}

// AddHistory 添加历史消息（线程安全）
func (s *DiscussionState) AddHistory(msg *model_gateway.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, msg)
}

// GetHistory 获取历史消息副本（线程安全）
func (s *DiscussionState) GetHistory() []*model_gateway.ChatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*model_gateway.ChatMessage, len(s.History))
	copy(result, s.History)
	return result
}

// IncrementRound 增加轮次（线程安全）
func (s *DiscussionState) IncrementRound() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentRound++
	return s.CurrentRound
}

// GetCurrentRound 获取当前轮次（线程安全）
func (s *DiscussionState) GetCurrentRound() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentRound
}

// RoleServiceInterface 角色服务接口
type RoleServiceInterface interface {
	Get(ctx context.Context, id string) (*role.Role, error)
}

// ModelGatewayInterface 模型网关接口
type ModelGatewayInterface interface {
	GetChatModel(binding string) (model_gateway.ChatModel, error)
}

// ContextInitNode 上下文初始化节点
// 加载角色配置，构建 RoleContext 列表
type ContextInitNode struct {
	roleService   RoleServiceInterface
	skillRegistry *role.SkillRegistry
	gateway       ModelGatewayInterface
}

// NewContextInitNode 创建上下文初始化节点
func NewContextInitNode(roleService RoleServiceInterface, skillRegistry *role.SkillRegistry, gateway ModelGatewayInterface) *ContextInitNode {
	return &ContextInitNode{
		roleService:   roleService,
		skillRegistry: skillRegistry,
		gateway:       gateway,
	}
}

// InitRoleContexts 初始化角色上下文
// 将角色 ID 列表转换为 RoleContext 列表
func (n *ContextInitNode) InitRoleContexts(ctx context.Context, roleIDs []string, customPrompt string) ([]*RoleContext, error) {
	roleContexts := make([]*RoleContext, 0, len(roleIDs))

	for _, roleID := range roleIDs {
		r, err := n.roleService.Get(ctx, roleID)
		if err != nil {
			return nil, fmt.Errorf("获取角色 %s 失败: %w", roleID, err)
		}

		// 构建角色聊天模板
		prompt, err := role.BuildRoleChatTemplate(r, n.skillRegistry, customPrompt)
		if err != nil {
			return nil, fmt.Errorf("构建角色提示词失败: %w", err)
		}

		// 获取 ChatModel
		binding := "openai" // 默认使用 openai，后续可扩展
		if r.DefaultModel != nil {
			// 尝试从 default_model 中提取 provider
			// 目前简单处理
		}
		chatModel, err := n.gateway.GetChatModel(binding)
		if err != nil {
			return nil, fmt.Errorf("获取角色 %s 的模型失败: %w", roleID, err)
		}

		roleContexts = append(roleContexts, &RoleContext{
			Role:      r,
			ChatModel: chatModel,
			Prompt:    prompt,
		})
	}

	return roleContexts, nil
}

// ExpertSpeakNode 专家发言节点
// 并行调用所有角色的 ChatModel
type ExpertSpeakNode struct{}

// NewExpertSpeakNode 创建专家发言节点
func NewExpertSpeakNode() *ExpertSpeakNode {
	return &ExpertSpeakNode{}
}

// ExpertSpeakResult 专家发言结果
type ExpertSpeakResult struct {
	RoleName string
	Content  string
	Tokens   int
	Error    error
}

// Execute 执行并行专家发言
// roles: 角色上下文列表
// topic: 当前讨论话题
// history: 历史消息
func (n *ExpertSpeakNode) Execute(ctx context.Context, roles []*RoleContext, topic string, state *DiscussionState) []*ExpertSpeakResult {
	results := make([]*ExpertSpeakResult, len(roles))
	var wg sync.WaitGroup

	for i, rc := range roles {
		wg.Add(1)
		go func(idx int, rctx *RoleContext) {
			defer wg.Done()

			// 构建消息
			messages := []model_gateway.ChatMessage{
				{Role: "system", Content: rctx.Prompt},
			}

			// 添加历史消息
			history := state.GetHistory()
			for _, msg := range history {
				messages = append(messages, *msg)
			}

			// 添加当前话题
			if topic != "" {
				messages = append(messages, model_gateway.ChatMessage{Role: "user", Content: topic})
			}

			// 推送节点开始事件
			if state.Bridge != nil {
				_ = state.Bridge.Push(&stream.GraphEvent{
					Type:      "node_start",
					NodeName:  "expert_speak",
					RoleName:  rctx.Role.Name,
					Timestamp: time.Now(),
				})
			}

			// 调用模型
			resp, err := rctx.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
				Messages: messages,
			})

			if err != nil {
				log.Error().Err(err).Str("role", rctx.Role.Name).Msg("专家发言失败")
				results[idx] = &ExpertSpeakResult{
					RoleName: rctx.Role.Name,
					Error:    fmt.Errorf("角色 %s 发言失败: %w", rctx.Role.Name, err),
				}

				if state.Bridge != nil {
					_ = state.Bridge.Push(&stream.GraphEvent{
						Type:      "error",
						NodeName:  "expert_speak",
						RoleName:  rctx.Role.Name,
						Error:     err.Error(),
						Timestamp: time.Now(),
					})
				}
				return
			}

			results[idx] = &ExpertSpeakResult{
				RoleName: rctx.Role.Name,
				Content:  resp.Content,
				Tokens:   resp.TotalTokens,
			}

			// 添加到历史
			state.AddHistory(&model_gateway.ChatMessage{
				Role:    "assistant",
				Content: resp.Content,
			})

			// 推送发言事件
			if state.Bridge != nil {
				_ = state.Bridge.Push(&stream.GraphEvent{
					Type:      "agent_speak",
					NodeName:  "expert_speak",
					RoleName:  rctx.Role.Name,
					Content:   resp.Content,
					Timestamp: time.Now(),
				})
				_ = state.Bridge.Push(&stream.GraphEvent{
					Type:      "node_end",
					NodeName:  "expert_speak",
					RoleName:  rctx.Role.Name,
					Timestamp: time.Now(),
				})
			}

			log.Debug().Str("role", rctx.Role.Name).Int("tokens", resp.TotalTokens).Msg("专家发言完成")
		}(i, rc)
	}

	wg.Wait()
	return results
}

// ModeratorEvalNode 主持人评估节点
type ModeratorEvalNode struct{}

// NewModeratorEvalNode 创建主持人评估节点
func NewModeratorEvalNode() *ModeratorEvalNode {
	return &ModeratorEvalNode{}
}

// EvalResult 评估结果
type EvalResult struct {
	ShouldContinue bool   // 是否继续讨论
	Reason         string // 评估理由
	Summary        string // 讨论总结（最终轮）
}

// Evaluate 执行评估
// 根据历史判断是否达到结束条件
func (n *ModeratorEvalNode) Evaluate(state *DiscussionState) *EvalResult {
	currentRound := state.GetCurrentRound()
	history := state.GetHistory()

	// 推送评估开始事件
	if state.Bridge != nil {
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "moderator_eval",
			Timestamp: time.Now(),
		})
	}

	// 评估逻辑：
	// 1. 达到最大轮次则结束
	// 2. 历史消息数达到上限
	if currentRound >= state.MaxRounds {
		result := &EvalResult{
			ShouldContinue: false,
			Reason:         fmt.Sprintf("已达到最大讨论轮次 (%d/%d)", currentRound, state.MaxRounds),
			Summary:        fmt.Sprintf("讨论共进行 %d 轮，%d 位专家参与", currentRound, len(state.Roles)),
		}

		if state.Bridge != nil {
			_ = state.Bridge.Push(&stream.GraphEvent{
				Type:      "node_end",
				NodeName:  "moderator_eval",
				Metadata:  result,
				Timestamp: time.Now(),
			})
		}

		return result
	}

	// 默认继续
	result := &EvalResult{
		ShouldContinue: true,
		Reason:         fmt.Sprintf("第 %d 轮讨论完成，继续下一轮", currentRound),
		Summary:        fmt.Sprintf("已进行 %d/%d 轮", currentRound, state.MaxRounds),
	}

	if len(history) > 100 {
		result.ShouldContinue = false
		result.Reason = "历史消息过多，自动终止"
	}

	if state.Bridge != nil {
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "moderator_eval",
			NodeName:  "moderator_eval",
			Metadata:  result,
			Timestamp: time.Now(),
		})
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "moderator_eval",
			Timestamp: time.Now(),
		})
	}

	return result
}

// SummarizeNode 总结节点
type SummarizeNode struct{}

// NewSummarizeNode 创建总结节点
func NewSummarizeNode() *SummarizeNode {
	return &SummarizeNode{}
}

// MeetingMinutes 会议纪要
type MeetingMinutes struct {
	SessionID     string    `json:"session_id"`
	TotalRounds   int       `json:"total_rounds"`
	TotalMessages int       `json:"total_messages"`
	Summary       string    `json:"summary"`
	CompletedAt   time.Time `json:"completed_at"`
}

// GenerateSummary 生成讨论总结
func (n *SummarizeNode) GenerateSummary(state *DiscussionState, evalResult *EvalResult) *MeetingMinutes {
	history := state.GetHistory()

	minutes := &MeetingMinutes{
		SessionID:     state.SessionID,
		TotalRounds:   state.GetCurrentRound(),
		TotalMessages: len(history),
		Summary:       evalResult.Summary,
		CompletedAt:   time.Now(),
	}

	if state.Bridge != nil {
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "done",
			NodeName:  "summarize",
			Content:   evalResult.Summary,
			Metadata:  minutes,
			Timestamp: time.Now(),
		})
	}

	return minutes
}

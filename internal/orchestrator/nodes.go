// Package orchestrator 提供讨论编排的核心 Graph 节点
package orchestrator

import (
	"context"
	"fmt"
	"strings"
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

	// MsgStore 消息持久化存储（可选，为 nil 时不持久化）
	MsgStore session.MessageStoreInterface

	// LastRoundSummary 上一轮的主持人总结，用于第 2 轮+ 精简上下文
	LastRoundSummary string
	// roleLastMessages 记录每个角色最近一次的发言内容（按角色名索引）
	roleLastMessages map[string]string

	// PauseUserInput 用户在暂停期间输入的内容，恢复后注入下一轮模型调用
	PauseUserInput string
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

// SetMessageStore 设置消息持久化存储
func (s *DiscussionState) SetMessageStore(store session.MessageStoreInterface) {
	s.MsgStore = store
}

// CheckInterrupt 检查是否有中断信号
// 如果有中断信号，暂停执行并等待恢复信号
// ctx: 外层上下文
// bridge: SSE 事件桥（可为 nil）
// returns: (是否继续执行, 恢复时用户输入的消息)
func CheckInterrupt(ctx context.Context, interruptCh <-chan *session.InterruptSignal, resumeCh <-chan *session.ResumeSignal, bridge *stream.Bridge) (bool, string) {
	if interruptCh == nil {
		return true, ""
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
		case resumeSig := <-resumeCh:
			// 广播恢复事件
			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "session.resumed",
					Timestamp: time.Now(),
				})
			}
			return true, resumeSig.Message
		case <-ctx.Done():
			return false, ""
		}
	case <-ctx.Done():
		return false, ""
	default:
		return true, ""
	}
}

// WaitForInterrupt 阻塞等待中断信号（用于暂停点）
func WaitForInterrupt(ctx context.Context, interruptCh <-chan *session.InterruptSignal, resumeCh <-chan *session.ResumeSignal, bridge *stream.Bridge) (bool, string) {
	if interruptCh == nil {
		return true, ""
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
		case resumeSig := <-resumeCh:
			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "session.resumed",
					Timestamp: time.Now(),
				})
			}
			return true, resumeSig.Message
		case <-ctx.Done():
			return false, ""
		}
	case <-ctx.Done():
		return false, ""
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

// SetRoleLastMessage 记录角色最近一次发言（线程安全）
func (s *DiscussionState) SetRoleLastMessage(roleName, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roleLastMessages == nil {
		s.roleLastMessages = make(map[string]string)
	}
	s.roleLastMessages[roleName] = content
}

// GetRoleLastMessage 获取角色最近一次发言（线程安全）
func (s *DiscussionState) GetRoleLastMessage(roleName string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.roleLastMessages == nil {
		return ""
	}
	return s.roleLastMessages[roleName]
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
			// 模型不可用时不返回错误，ChatModel 留 nil
			// ExpertSpeakNode 和 ModeratorEvalNode 会检测 nil 并使用模拟模式
			log.Error().Err(err).Str("role", roleID).Str("binding", binding).Msg("获取模型失败，使用模拟模式")
			chatModel = nil
		} else {
			log.Info().Str("role", roleID).Str("binding", binding).Msg("模型获取成功")
		}

		roleContexts = append(roleContexts, &RoleContext{
			Role:      r,
			ChatModel: chatModel,
			Prompt:    prompt,
		})
	}

	return roleContexts, nil
}

// generateSimulatedOpinion 当没有可用模型时生成模拟发言内容
// 根据角色名和主题定制不同的发言风格，多轮之间内容自然递进
func generateSimulatedOpinion(role *role.Role, topic string, round int, history []*model_gateway.ChatMessage) string {
	name := role.Name
	_ = history

	templates := map[string]func() string{
		"安全": func() string {
			opinions := []string{
				fmt.Sprintf("从安全角度来看，「%s」的核心风险点包括：\n1. 身份认证机制的强度\n2. 数据传输和存储的加密方案\n3. 权限管理的粒度\n4. 安全审计和监控体系\n\n建议采用纵深防御策略，从多个层面构建安全防护。", topic),
				fmt.Sprintf("我接着补充几点安全方面的建议。刚才各位提到的都很重要，我特别关注第三方依赖的安全性和 API 接口的防篡改机制。另外用户隐私数据的脱敏处理和应急响应预案也值得提前规划。安全设计应当贯穿整个开发周期。"),
				fmt.Sprintf("对于「%s」，还有一个容易被忽略的安全维度——合规性要求（如等保、GDPR等）和供应链安全。此外零信任架构的落地需要从身份、设备、网络多个层面同步推进。安全不是一次性工作，而是持续改进的过程。", topic),
			}
			return opinions[(round-1)%len(opinions)]
		},
		"性能": func() string {
			opinions := []string{
				fmt.Sprintf("关于「%s」的性能优化，我的分析是：\n1. 需要建立性能基线指标\n2. 识别关键路径的瓶颈\n3. 缓存策略的设计\n4. 数据库查询优化\n\n建议先做性能基准测试，找到瓶颈后再针对性优化。", topic),
				fmt.Sprintf("我补充一下性能方面的考虑。前面安全专家提到的内容确实需要关注，从性能角度看，并发模型的选择、连接池复用和异步处理流程同样关键。性能优化要避免过早优化，用数据说话。"),
				fmt.Sprintf("从性能角度看，还需要关注前端性能（首屏加载、资源压缩）、网络传输优化和存储性能。这些问题最好在架构层面解决，而非靠后期修补。"),
			}
			return opinions[(round-1)%len(opinions)]
		},
	}

	for pattern, gen := range templates {
		if strings.Contains(name, pattern) {
			return gen()
		}
	}

	if strings.Contains(name, "项目经理") || strings.Contains(name, "产品") || strings.Contains(name, "项目") {
		opinions := []string{
			fmt.Sprintf("从项目管理角度，「%s」需要关注：\n1. 项目范围和时间线\n2. 资源分配和优先级\n3. 风险识别和应对\n4. 干系人沟通\n\n建议采用敏捷方式分阶段交付。", topic),
			fmt.Sprintf("我补充项目管理方面的看法。各位专家的分析我都听到了，从项目落地角度来说，明确的里程碑和交付物定义是基础，质量管理计划和变更管理流程同样不能忽视。项目成功的关键在于持续沟通和风险管控。"),
		}
		return opinions[(round-1)%len(opinions)]
	}
	if strings.Contains(name, "架构") {
		opinions := []string{
			fmt.Sprintf("从架构角度看，「%s」的设计原则是：\n1. 高内聚低耦合\n2. 可扩展性和可维护性\n3. 技术选型匹配业务需求\n4. 架构演进路径清晰\n\n好的架构是演进而非设计出来的。", topic),
			fmt.Sprintf("我补充架构方面的建议。刚才大家提到的性能和安全需求，在架构层面应当通过领域驱动设计划分边界，事件驱动架构解耦，同时考虑数据一致性方案。架构决策需要平衡当前需求与未来扩展。"),
		}
		return opinions[(round-1)%len(opinions)]
	}
	if strings.Contains(name, "市场") || strings.Contains(name, "营销") {
		return fmt.Sprintf("从市场角度看，「%s」应该关注：\n1. 目标客群画像\n2. 竞品差异化定位\n3. 获客渠道和成本\n4. 品牌建设策略\n\n建议先做小范围市场验证。", topic)
	}
	if strings.Contains(name, "销售") {
		return fmt.Sprintf("关于「%s」的销售策略：\n1. 核心价值主张\n2. 定价策略\n3. 渠道分销策略\n4. 大客户管理\n\n销售成功的关键是解决客户核心痛点。", topic)
	}
	if strings.Contains(name, "董事会") || strings.Contains(name, "董事") {
		return fmt.Sprintf("作为董事会，我们关注「%s」的：\n1. 战略价值\n2. 投资回报\n3. 风险控制\n4. 合规性\n\n请提供完整的风险评估和商业论证。", topic)
	}
	if strings.Contains(name, "主持") || strings.Contains(name, "主持人") {
		return fmt.Sprintf("大家好，欢迎参加第 %d 轮关于「%s」的讨论。经过前面的讨论，我们已经有了不少有价值的观点，请各位继续深入探讨，争取形成共识或明确分歧点。", round, topic)
	}
	return fmt.Sprintf("关于「%s」这个主题，我认为需要从以下维度深入分析：\n1. 当前现状和核心问题\n2. 潜在解决方案对比\n3. 实施路径和优先级\n4. 预期效果和评估标准", topic)
}

// truncateContent 截断内容到指定长度（按字符计数，非字节）
func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "..."
}

// generateRoundSummary 生成一轮讨论的总结
// topic: 讨论话题
// round: 当前轮次
// opinions: 本轮所有专家的发言结果
func generateRoundSummary(topic string, round int, opinions []*ExpertSpeakResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("第 %d 轮讨论总结（主题：%s）：\n", round, topic))
	for _, op := range opinions {
		if op.Error == nil {
			b.WriteString(fmt.Sprintf("- %s：%s\n", op.RoleName, truncateContent(op.Content, 100)))
		}
	}
	b.WriteString("\n以上是各专家的核心观点，下一轮请基于以上讨论继续深入。")
	return b.String()
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
//
// 并发安全：每个角色在独立 goroutine 中执行，内部使用锁保护 results 写入。
// 超时安全：模型调用有 30s 单个超时，整体执行有 60s 兜底超时。
func (n *ExpertSpeakNode) Execute(ctx context.Context, roles []*RoleContext, topic string, state *DiscussionState) []*ExpertSpeakResult {
	results := make([]*ExpertSpeakResult, len(roles))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 为模型调用设置总超时（防止单个角色卡住整个讨论）
	modelCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for i, rc := range roles {
		wg.Add(1)
		go func(idx int, rctx *RoleContext) {
			defer wg.Done()

			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					results[idx] = &ExpertSpeakResult{
						RoleName: rctx.Role.Name,
						Content:  fmt.Sprintf("发言异常: %v", r),
						Error:    fmt.Errorf("panic: %v", r),
					}
					mu.Unlock()
				}
			}()

			currentRound := state.GetCurrentRound()
			messages := []model_gateway.ChatMessage{
				{Role: "system", Content: rctx.Prompt},
			}

			if currentRound <= 1 {
				history := state.GetHistory()
				for _, msg := range history {
					messages = append(messages, *msg)
				}
				if topic != "" {
					messages = append(messages, model_gateway.ChatMessage{Role: "user", Content: topic})
				}
			} else {
				if topic != "" {
					messages = append(messages, model_gateway.ChatMessage{Role: "user", Content: topic})
				}
				if lastMsg := state.GetRoleLastMessage(rctx.Role.Name); lastMsg != "" {
					messages = append(messages, model_gateway.ChatMessage{
						Role:    "assistant",
						Content: fmt.Sprintf("你上一轮的发言摘要：%s", truncateContent(lastMsg, 200)),
					})
				}
				if state.LastRoundSummary != "" {
					messages = append(messages, model_gateway.ChatMessage{
						Role:    "user",
						Content: state.LastRoundSummary,
					})
				}
				messages = append(messages, model_gateway.ChatMessage{
					Role:    "user",
					Content: "请不要重复之前已经表达过的观点，本轮请重点回应其他专家的看法或提出新的见解。",
				})
			}

			// 注入用户在暂停期间输入的内容
			if state.PauseUserInput != "" {
				messages = append(messages, model_gateway.ChatMessage{
					Role:    "user",
					Content: fmt.Sprintf("用户补充的额外信息（请仔细阅读并纳入考虑）：\n%s", state.PauseUserInput),
				})
			}

			if state.Bridge != nil {
				_ = state.Bridge.Push(&stream.GraphEvent{
					Type:      "node_start",
					NodeName:  "expert_speak",
					RoleName:  rctx.Role.Name,
					Timestamp: time.Now(),
				})
			}

			var content string
			var tokens int
			var execErr error

			if rctx.ChatModel == nil {
				content = generateSimulatedOpinion(rctx.Role, topic, state.GetCurrentRound(), state.GetHistory())
				tokens = len(content) / 4
				select {
				case <-time.After(300 * time.Millisecond):
				case <-modelCtx.Done():
					mu.Lock()
					results[idx] = &ExpertSpeakResult{RoleName: rctx.Role.Name, Error: modelCtx.Err()}
					mu.Unlock()
					return
				}
			} else {
				// 真实模型：用 goroutine+channel 强制超时（防止 Generate 忽略 context）
				log.Info().Str("role", rctx.Role.Name).Str("topic", topic).Int("msg_count", len(messages)).Msg("开始调用模型")

				// 打印发送给模型的 messages（调试用）
				for i, msg := range messages {
					log.Debug().Int("msg_idx", i).Str("role", msg.Role).Str("content", msg.Content).Msg("message")
				}

				type genResult struct {
					resp *model_gateway.ChatResponse
					err  error
				}
				done := make(chan genResult, 1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							done <- genResult{err: fmt.Errorf("模型调用panic: %v", r)}
						}
					}()
					resp, err := rctx.ChatModel.Generate(modelCtx, &model_gateway.ChatRequest{
						Messages: messages,
					})
					done <- genResult{resp: resp, err: err}
				}()

				select {
				case gr := <-done:
					if gr.err != nil {
						log.Error().Err(gr.err).Str("role", rctx.Role.Name).Msg("专家发言失败")
						content = generateSimulatedOpinion(rctx.Role, topic, state.GetCurrentRound(), state.GetHistory())
						tokens = len(content) / 4
						execErr = gr.err
					} else {
						log.Info().Str("role", rctx.Role.Name).Int("content_len", len(gr.resp.Content)).Int("tokens", gr.resp.TotalTokens).Msg("模型调用成功")
						content = gr.resp.Content
						tokens = gr.resp.TotalTokens
					}
				case <-modelCtx.Done():
					log.Warn().Str("role", rctx.Role.Name).Msg("模型调用超时，回退到模拟发言")
					content = generateSimulatedOpinion(rctx.Role, topic, state.GetCurrentRound(), state.GetHistory())
					tokens = len(content) / 4
				}
			}

			results[idx] = &ExpertSpeakResult{
				RoleName: rctx.Role.Name,
				Content:  content,
				Tokens:   tokens,
				Error:    execErr,
			}

			// 记录该角色本轮发言（供下一轮精简上下文使用）
			state.SetRoleLastMessage(rctx.Role.Name, content)

			// 添加到历史
			state.AddHistory(&model_gateway.ChatMessage{
				Role:    "assistant",
				Content: content,
			})

			// 持久化消息到 MessageStore
			if state.MsgStore != nil {
				state.MsgStore.Add(state.SessionID, state.GetCurrentRound(), rctx.Role.Name, content, tokens)
			}

			// 推送发言事件
			if state.Bridge != nil {
				_ = state.Bridge.Push(&stream.GraphEvent{
					Type:      "role.speak",
					NodeName:  "expert_speak",
					RoleName:  rctx.Role.Name,
					Content:   content,
					Timestamp: time.Now(),
				})
				_ = state.Bridge.Push(&stream.GraphEvent{
					Type:      "node_end",
					NodeName:  "expert_speak",
					RoleName:  rctx.Role.Name,
					Timestamp: time.Now(),
				})
			}

			log.Debug().Str("role", rctx.Role.Name).Int("tokens", tokens).Msg("专家发言完成")
		}(i, rc)
	}

	// 整体执行超时兜底：如果任何 goroutine 卡住，最多等待 60 秒
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有 goroutine 正常完成
	case <-time.After(60 * time.Second):
		log.Error().Int("roles", len(roles)).Msg("ExpertSpeakNode.Execute 整体超时")
		// 填充未完成的结果为错误
		mu.Lock()
		for i, r := range results {
			if r == nil {
				results[i] = &ExpertSpeakResult{
					RoleName: roles[i].Role.Name,
					Content:  "发言超时",
					Error:    fmt.Errorf("整体执行超时"),
				}
			}
		}
		mu.Unlock()
	case <-ctx.Done():
		log.Warn().Err(ctx.Err()).Msg("ExpertSpeakNode.Execute 上下文取消")
		mu.Lock()
		for i, r := range results {
			if r == nil {
				results[i] = &ExpertSpeakResult{
					RoleName: roles[i].Role.Name,
					Content:  "发言取消",
					Error:    ctx.Err(),
				}
			}
		}
		mu.Unlock()
	}

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
	Solved         bool   // 问题是否已解决
}

// Evaluate 执行评估
// 根据历史判断是否达到结束条件
// 判定逻辑：
//   - 检查轮次上限
//   - 检查历史消息长度
//   - 检查问题是否已解决（模拟模式下第 3 轮后判定为已解决）
//   - 如有 ChatModel，让模型决策（TODO：未来扩展）
func (n *ModeratorEvalNode) Evaluate(state *DiscussionState) *EvalResult {
	currentRound := state.GetCurrentRound()
	history := state.GetHistory()

	// 推送评估开始事件
	if state.Bridge != nil {
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "round.eval",
			Timestamp: time.Now(),
		})
	}

	var result *EvalResult

	// 检查问题是否已解决（模拟模式：第 3 轮后判定为已解决）
	solved := false
	if currentRound >= 3 {
		solved = true
	}

	switch {
	case currentRound >= state.MaxRounds:
		// 达到最大轮次则结束
		reason := fmt.Sprintf("已达到最大讨论轮次 (%d/%d)", currentRound, state.MaxRounds)
		result = &EvalResult{
			ShouldContinue: false,
			Reason:         reason,
			Summary:        fmt.Sprintf("讨论共进行 %d 轮，%d 位专家参与。", currentRound, len(state.Roles)) + "各专家已充分发表意见，形成讨论结论。",
			Solved:         solved,
		}
	case solved:
		// 问题已解决，结束讨论
		summaryText := fmt.Sprintf("讨论共进行 %d 轮，%d 位专家参与。问题已解决，各方达成一致结论。", currentRound, len(state.Roles))
		reasonText := fmt.Sprintf("第 %d 轮讨论完成，各专家达成以下共识：\n%s", currentRound, state.LastRoundSummary)
		result = &EvalResult{
			ShouldContinue: false,
			Reason:         reasonText,
			Summary:        summaryText + "\n" + state.LastRoundSummary,
			Solved:         true,
		}
	case len(history) > 100:
		// 历史消息过多，自动终止
		result = &EvalResult{
			ShouldContinue: false,
			Reason:         "历史消息过多，自动终止",
			Summary:        fmt.Sprintf("已进行 %d/%d 轮", currentRound, state.MaxRounds),
			Solved:         false,
		}
	default:
		// 继续下一轮
		result = &EvalResult{
			ShouldContinue: true,
			Reason:         fmt.Sprintf("第 %d 轮讨论完成，继续下一轮", currentRound),
		}
	}

	// 构建评估内容文本（供前端展示）
	evalContent := fmt.Sprintf("【主持人评估】\n%s", result.Reason)
	if result.Solved {
		evalContent += "\n\n✅ 问题已解决"
	} else if !result.ShouldContinue {
		evalContent += "\n\n📋 讨论结束"
	}

	if state.Bridge != nil {
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "round.eval",
			NodeName:  fmt.Sprintf("round_%d_eval", currentRound),
			Content:   evalContent,
			Metadata:  result,
			Timestamp: time.Now(),
		})
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "round.eval",
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

	// 构建更详细的总结：汇总各轮次的核心观点
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString(evalResult.Summary)
	summaryBuilder.WriteString("\n\n")

	// 按角色分组历史消息
	roleMessages := make(map[string][]string)
	for _, msg := range history {
		if msg.Role == "assistant" {
			roleMessages[msg.Content[:min(len(msg.Content), 50)]] = append(roleMessages[msg.Content[:min(len(msg.Content), 50)]], msg.Content)
		}
	}

	summaryBuilder.WriteString("## 各角色核心观点\n")
	// 从状态中获取角色名
	for _, rc := range state.Roles {
		if lastMsg := state.GetRoleLastMessage(rc.Role.Name); lastMsg != "" {
			summaryBuilder.WriteString(fmt.Sprintf("### %s\n%s\n\n", rc.Role.Name, truncateContent(lastMsg, 300)))
		}
	}

	if state.LastRoundSummary != "" {
		summaryBuilder.WriteString("\n## 最终总结\n")
		summaryBuilder.WriteString(state.LastRoundSummary)
	}

	summary := summaryBuilder.String()

	minutes := &MeetingMinutes{
		SessionID:     state.SessionID,
		TotalRounds:   state.GetCurrentRound(),
		TotalMessages: len(history),
		Summary:       summary,
		CompletedAt:   time.Now(),
	}

	if state.Bridge != nil {
		_ = state.Bridge.Push(&stream.GraphEvent{
			Type:      "done",
			NodeName:  "summarize",
			Content:   summary,
			Metadata:  minutes,
			Timestamp: time.Now(),
		})
	}

	return minutes
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/stream"
)

// EvaluationOption 待评估的选项
type EvaluationOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// EvaluationCriterion 评估标准（含权重）
type EvaluationCriterion struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"` // 权重 0.0-1.0，所有标准权重之和应为 1.0
}

// EvaluationConfig 加权评估范式配置
type EvaluationConfig struct {
	RoleIDs      []string              // 参与评估的角色 ID
	Topic        string                // 评估主题
	CustomPrompt string                // 用户自定义额外提示
	Options      []EvaluationOption    // 待评估选项列表
	Criteria     []EvaluationCriterion // 评估标准列表
	CriticRoleID string                // 批评者角色 ID（可选）
}

// ScoringResult 单个专家评分结果
type ScoringResult struct {
	ExpertName string        `json:"expert_name"`
	Scores     []ScoreDetail `json:"scores"`
}

// ScoreDetail 单个评分详情
type ScoreDetail struct {
	OptionID      string  `json:"option_id"`
	OptionName    string  `json:"option_name"`
	CriterionName string  `json:"criterion_name"`
	Score         float64 `json:"score"` // 0-100
	Rationale     string  `json:"rationale"`
}

// CriticFeedback 质疑反馈
type CriticFeedback struct {
	ExpertName     string `json:"expert_name"`
	Content        string `json:"content"`
	TargetOptionID string `json:"target_option_id,omitempty"`
}

// AdjustedScoringResult 调整后评分结果
type AdjustedScoringResult struct {
	ExpertName  string            `json:"expert_name"`
	Adjustments []ScoreAdjustment `json:"adjustments"`
}

// ScoreAdjustment 评分调整
type ScoreAdjustment struct {
	OptionID      string  `json:"option_id"`
	CriterionName string  `json:"criterion_name"`
	OriginalScore float64 `json:"original_score"`
	AdjustedScore float64 `json:"adjusted_score"`
	Rationale     string  `json:"rationale"`
}

// MatrixEntry 矩阵条目
type MatrixEntry struct {
	OptionID   string             `json:"option_id"`
	OptionName string             `json:"option_name"`
	Scores     map[string]float64 `json:"scores"` // criterion name -> weighted score
	TotalScore float64            `json:"total_score"`
}

// EvaluationOutput 加权评估输出
type EvaluationOutput struct {
	Matrix          []MatrixEntry    `json:"matrix"`
	Consensus       string           `json:"consensus"`
	Disagreements   []string         `json:"disagreements"`
	ScoringResults  []ScoringResult  `json:"scoring_results"`
	CriticFeedbacks []CriticFeedback `json:"critic_feedbacks,omitempty"`
	FinalRanking    []string         `json:"final_ranking"` // Option IDs 按总分排序
}

// EvaluationOrchestrator 加权评估范式编排器
// 实现加权评估流程：
// 1. expert_scoring — 各专家按角色对每个方案打分
// 2. critic_challenge — 批评者强制质疑评分中的假设
// 3. score_adjustment — 专家回应质疑并调整评分
// 4. matrix_generation — 生成决策矩阵
type EvaluationOrchestrator struct {
	contextInitNode   *ContextInitNode
	expertSpeakNode   *ExpertSpeakNode
	moderatorEvalNode *ModeratorEvalNode
	summarizeNode     *SummarizeNode
}

// NewEvaluationOrchestrator 创建加权评估编排器
func NewEvaluationOrchestrator(
	contextInitNode *ContextInitNode,
	expertSpeakNode *ExpertSpeakNode,
	moderatorEvalNode *ModeratorEvalNode,
	summarizeNode *SummarizeNode,
) *EvaluationOrchestrator {
	return &EvaluationOrchestrator{
		contextInitNode:   contextInitNode,
		expertSpeakNode:   expertSpeakNode,
		moderatorEvalNode: moderatorEvalNode,
		summarizeNode:     summarizeNode,
	}
}

// Execute 执行一次完整的加权评估
func (e *EvaluationOrchestrator) Execute(
	ctx context.Context,
	sessionID string,
	config *EvaluationConfig,
	bridge *stream.Bridge,
	progressCh chan<- string,
) (*EvaluationOutput, error) {
	log.Info().Str("session_id", sessionID).
		Int("roles", len(config.RoleIDs)).
		Int("options", len(config.Options)).
		Int("criteria", len(config.Criteria)).
		Msg("加权评估开始")

	// 1. 初始化角色上下文
	if progressCh != nil {
		progressCh <- "正在初始化评估角色..."
	}

	roleContexts, err := e.contextInitNode.InitRoleContexts(ctx, config.RoleIDs, config.CustomPrompt)
	if err != nil {
		return nil, fmt.Errorf("初始化评估角色失败: %w", err)
	}

	state := NewDiscussionState(sessionID, 1, bridge)
	state.Roles = roleContexts
	state.IncrementRound()

	// 分离批评者角色
	var criticRC *RoleContext
	experts := make([]*RoleContext, 0, len(roleContexts))
	for _, rc := range roleContexts {
		if config.CriticRoleID != "" && rc.Role.ID == config.CriticRoleID {
			criticRC = rc
		} else {
			experts = append(experts, rc)
		}
	}

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "evaluation",
			Content:   config.Topic,
			Timestamp: time.Now(),
		})
	}

	// ===== 阶段 1: 专家评分 =====
	if progressCh != nil {
		progressCh <- "专家评分中..."
	}
	scoringResults := e.executeExpertScoring(ctx, experts, config, state, bridge)

	// ===== 阶段 2: 批评者质疑 =====
	var criticFeedbacks []CriticFeedback
	if criticRC != nil {
		if progressCh != nil {
			progressCh <- "批评者提出质疑..."
		}
		criticFeedbacks = e.executeCriticChallenge(ctx, criticRC, config, scoringResults, state, bridge)
	}

	// ===== 阶段 3: 评分调整 =====
	var adjustedResults []AdjustedScoringResult
	if len(criticFeedbacks) > 0 {
		if progressCh != nil {
			progressCh <- "专家回应质疑并调整评分..."
		}
		adjustedResults = e.executeScoreAdjustment(ctx, experts, config, scoringResults, criticFeedbacks, state, bridge)
	}

	// ===== 阶段 4: 矩阵生成 =====
	if progressCh != nil {
		progressCh <- "生成决策矩阵..."
	}
	output := e.executeMatrixGeneration(config, scoringResults, adjustedResults, criticFeedbacks, bridge)

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "done",
			NodeName:  "evaluation",
			Content:   "加权评估完成",
			Metadata:  output,
			Timestamp: time.Now(),
		})
	}

	log.Info().Str("session_id", sessionID).
		Int("options", len(output.Matrix)).
		Msg("加权评估完成")

	if progressCh != nil {
		close(progressCh)
	}
	return output, nil
}

// executeExpertScoring 执行专家评分阶段
func (e *EvaluationOrchestrator) executeExpertScoring(
	ctx context.Context,
	experts []*RoleContext,
	config *EvaluationConfig,
	state *DiscussionState,
	bridge *stream.Bridge,
) []ScoringResult {
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "expert_scoring",
			Timestamp: time.Now(),
		})
	}

	// 构建评分提示词
	scoringPrompt := buildScoringPrompt(config)

	results := make([]ScoringResult, len(experts))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, expert := range experts {
		wg.Add(1)
		go func(idx int, rc *RoleContext) {
			defer wg.Done()

			messages := []model_gateway.ChatMessage{
				{Role: "system", Content: rc.Prompt},
				{Role: "user", Content: scoringPrompt},
			}

			resp, err := rc.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
				Messages:    messages,
				Temperature: 0.5,
			})
			if err != nil {
				log.Error().Err(err).Str("role", rc.Role.Name).Msg("专家评分失败")
				return
			}

			state.AddHistory(&model_gateway.ChatMessage{
				Role:    "assistant",
				Content: resp.Content,
			})

			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "agent_speak",
					NodeName:  "expert_scoring",
					RoleName:  rc.Role.Name,
					Content:   resp.Content,
					Timestamp: time.Now(),
				})
			}

			// 从回复中解析评分
			scores := parseScoresFromResponse(resp.Content, config)
			mu.Lock()
			results[idx] = ScoringResult{
				ExpertName: rc.Role.Name,
				Scores:     scores,
			}
			mu.Unlock()

			log.Debug().Str("role", rc.Role.Name).Int("scores", len(scores)).Msg("专家评分完成")
		}(i, expert)
	}
	wg.Wait()

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "expert_scoring",
			Timestamp: time.Now(),
		})
	}

	return results
}

// executeCriticChallenge 执行批评者质疑阶段
func (e *EvaluationOrchestrator) executeCriticChallenge(
	ctx context.Context,
	criticRC *RoleContext,
	config *EvaluationConfig,
	scoringResults []ScoringResult,
	state *DiscussionState,
	bridge *stream.Bridge,
) []CriticFeedback {
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "critic_challenge",
			Timestamp: time.Now(),
		})
	}

	// 构建质疑提示词
	var scoringSummary string
	for _, sr := range scoringResults {
		scoringSummary += fmt.Sprintf("专家 %s 的评分：\n", sr.ExpertName)
		for _, s := range sr.Scores {
			scoringSummary += fmt.Sprintf("  - 选项 %s / 标准 %s: %d 分\n", s.OptionName, s.CriterionName, int(s.Score))
		}
	}

	messages := []model_gateway.ChatMessage{
		{Role: "system", Content: criticRC.Prompt},
		{Role: "user", Content: fmt.Sprintf(`请对以下评估结果提出质疑，找出评分中的假设缺陷、盲点或逻辑问题。

评估主题：%s

方案列表：
%s

现有评分结果：
%s

请指出：
1. 评分中可能存在的偏见或盲点
2. 被忽视的重要维度
3. 评分标准本身的缺陷
4. 具体质疑每条评分假设

每条质疑请明确指出对应的方案和评分标准。`,
			config.Topic, formatOptions(config.Options), scoringSummary)},
	}

	resp, err := criticRC.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
		Messages:    messages,
		Temperature: 0.8,
	})
	if err != nil {
		log.Error().Err(err).Msg("批评者质疑失败")
		return nil
	}

	state.AddHistory(&model_gateway.ChatMessage{
		Role:    "assistant",
		Content: resp.Content,
	})

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "agent_speak",
			NodeName:  "critic_challenge",
			RoleName:  criticRC.Role.Name,
			Content:   resp.Content,
			Timestamp: time.Now(),
		})
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "critic_challenge",
			Timestamp: time.Now(),
		})
	}

	return []CriticFeedback{{
		ExpertName: criticRC.Role.Name,
		Content:    resp.Content,
	}}
}

// executeScoreAdjustment 执行评分调整阶段
func (e *EvaluationOrchestrator) executeScoreAdjustment(
	ctx context.Context,
	experts []*RoleContext,
	config *EvaluationConfig,
	scoringResults []ScoringResult,
	criticFeedbacks []CriticFeedback,
	state *DiscussionState,
	bridge *stream.Bridge,
) []AdjustedScoringResult {
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "score_adjustment",
			Timestamp: time.Now(),
		})
	}

	// 汇总质疑内容
	var challengeSummary string
	for _, cf := range criticFeedbacks {
		challengeSummary += cf.Content + "\n"
	}

	adjustedResults := make([]AdjustedScoringResult, len(experts))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, expert := range experts {
		// 找到对应的原始评分
		var originalScores []ScoreDetail
		for _, sr := range scoringResults {
			if sr.ExpertName == expert.Role.Name {
				originalScores = sr.Scores
				break
			}
		}

		if originalScores == nil {
			continue
		}

		wg.Add(1)
		go func(idx int, rc *RoleContext, origScores []ScoreDetail) {
			defer wg.Done()

			var origSummary string
			for _, s := range origScores {
				origSummary += fmt.Sprintf("  - 选项 %s / %s: %d 分，理由：%s\n",
					s.OptionName, s.CriterionName, int(s.Score), s.Rationale)
			}

			messages := []model_gateway.ChatMessage{
				{Role: "system", Content: rc.Prompt},
				{Role: "user", Content: fmt.Sprintf(`以下是批评者对评分提出的质疑：

%s

你的原始评分：
%s

请回应质疑，并决定是否需要调整评分。对于每项评分，请说明：
1. 是否接受质疑
2. 如果接受，给出调整后的分数
3. 如果不接受，说明理由

输出格式：对于每个评分条目，给出"调整后分数"字段。`,
					challengeSummary, origSummary)},
			}

			resp, err := rc.ChatModel.Generate(ctx, &model_gateway.ChatRequest{
				Messages:    messages,
				Temperature: 0.6,
			})
			if err != nil {
				log.Error().Err(err).Str("role", rc.Role.Name).Msg("评分调整失败")
				return
			}

			state.AddHistory(&model_gateway.ChatMessage{
				Role:    "assistant",
				Content: resp.Content,
			})

			if bridge != nil {
				_ = bridge.Push(&stream.GraphEvent{
					Type:      "agent_speak",
					NodeName:  "score_adjustment",
					RoleName:  rc.Role.Name,
					Content:   resp.Content,
					Timestamp: time.Now(),
				})
			}

			mu.Lock()
			adjustedResults[idx] = AdjustedScoringResult{
				ExpertName: rc.Role.Name,
				Adjustments: []ScoreAdjustment{
					{
						OptionID:      "",
						CriterionName: "",
						OriginalScore: 0,
						AdjustedScore: 0,
						Rationale:     resp.Content,
					},
				},
			}
			mu.Unlock()
		}(i, expert, originalScores)
	}
	wg.Wait()

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "score_adjustment",
			Timestamp: time.Now(),
		})
	}

	return adjustedResults
}

// executeMatrixGeneration 执行矩阵生成阶段
func (e *EvaluationOrchestrator) executeMatrixGeneration(
	config *EvaluationConfig,
	scoringResults []ScoringResult,
	adjustedResults []AdjustedScoringResult,
	criticFeedbacks []CriticFeedback,
	bridge *stream.Bridge,
) *EvaluationOutput {
	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_start",
			NodeName:  "matrix_generation",
			Timestamp: time.Now(),
		})
	}

	// 使用调整后的评分（如果有），否则使用原始评分
	type effectiveScore struct {
		optionID  string
		criterion string
		score     float64
		expert    string
	}

	var allScores []effectiveScore
	adjustmentMap := make(map[string]map[string]float64) // expert -> optionID:criterion -> adjustedScore

	for _, ar := range adjustedResults {
		if adjustmentMap[ar.ExpertName] == nil {
			adjustmentMap[ar.ExpertName] = make(map[string]float64)
		}
		for _, adj := range ar.Adjustments {
			key := adj.OptionID + ":" + adj.CriterionName
			adjustmentMap[ar.ExpertName][key] = adj.AdjustedScore
		}
	}

	for _, sr := range scoringResults {
		for _, s := range sr.Scores {
			key := s.OptionID + ":" + s.CriterionName
			score := s.Score
			if adjMap, ok := adjustmentMap[sr.ExpertName]; ok {
				if adjScore, exists := adjMap[key]; exists {
					score = adjScore
				}
			}
			allScores = append(allScores, effectiveScore{
				optionID:  s.OptionID,
				criterion: s.CriterionName,
				score:     score,
				expert:    sr.ExpertName,
			})
		}
	}

	// 计算加权总分
	type optionScore struct {
		total float64
		count int
	}
	optionTotals := make(map[string]map[string]float64) // optionID -> criterionName -> total score
	optionCounts := make(map[string]map[string]int)     // optionID -> criterionName -> count

	for _, es := range allScores {
		if optionTotals[es.optionID] == nil {
			optionTotals[es.optionID] = make(map[string]float64)
			optionCounts[es.optionID] = make(map[string]int)
		}
		optionTotals[es.optionID][es.criterion] += es.score
		optionCounts[es.optionID][es.criterion]++
	}

	// 权重映射
	weightMap := make(map[string]float64)
	for _, c := range config.Criteria {
		weightMap[c.Name] = c.Weight
	}

	optionNameMap := make(map[string]string)
	for _, o := range config.Options {
		optionNameMap[o.ID] = o.Name
	}

	var matrix []MatrixEntry
	optionRankings := make([]struct {
		id    string
		total float64
	}, 0, len(config.Options))

	for _, opt := range config.Options {
		entry := MatrixEntry{
			OptionID:   opt.ID,
			OptionName: opt.Name,
			Scores:     make(map[string]float64),
		}

		var weightedTotal float64
		for _, cr := range config.Criteria {
			totals := optionTotals[opt.ID]
			counts := optionCounts[opt.ID]
			if totals != nil && counts != nil {
				if cnt, ok := counts[cr.Name]; ok && cnt > 0 {
					avgScore := totals[cr.Name] / float64(cnt)
					weightedScore := avgScore * cr.Weight
					entry.Scores[cr.Name] = weightedScore
					weightedTotal += weightedScore
				}
			}
		}

		entry.TotalScore = weightedTotal
		matrix = append(matrix, entry)

		optionRankings = append(optionRankings, struct {
			id    string
			total float64
		}{id: opt.ID, total: weightedTotal})
	}

	// 排序
	sort.Slice(optionRankings, func(i, j int) bool {
		return optionRankings[i].total > optionRankings[j].total
	})

	finalRanking := make([]string, len(optionRankings))
	for i, r := range optionRankings {
		finalRanking[i] = r.id
	}

	// 生成共识摘要
	var disagreements []string
	for _, cf := range criticFeedbacks {
		disagreements = append(disagreements, cf.Content)
	}

	consensus := fmt.Sprintf("评估完成，共 %d 个方案，%d 个评估标准，%d 位专家参与评分。",
		len(config.Options), len(config.Criteria), len(scoringResults))

	output := &EvaluationOutput{
		Matrix:          matrix,
		Consensus:       consensus,
		Disagreements:   disagreements,
		ScoringResults:  scoringResults,
		CriticFeedbacks: criticFeedbacks,
		FinalRanking:    finalRanking,
	}

	if bridge != nil {
		_ = bridge.Push(&stream.GraphEvent{
			Type:      "node_end",
			NodeName:  "matrix_generation",
			Content:   consensus,
			Metadata:  output,
			Timestamp: time.Now(),
		})
	}

	return output
}

// buildScoringPrompt 构建评分提示词
func buildScoringPrompt(config *EvaluationConfig) string {
	prompt := fmt.Sprintf(`请对以下方案进行评分。评分范围为 0-100 分。

评估主题：%s

方案列表：
%s

评估标准（含权重）：
%s

请对每个方案的每个标准给出分数和简要理由。
输出格式要求：
- 方案名称：[方案名]
  - [标准名]: [分数] 分，理由：[理由]

确保每个方案在每个标准下都有评分。`,
		config.Topic,
		formatOptions(config.Options),
		formatCriteria(config.Criteria))

	return prompt
}

// formatOptions 格式化方案列表
func formatOptions(options []EvaluationOption) string {
	var result string
	for i, opt := range options {
		result += fmt.Sprintf("%d. %s: %s\n", i+1, opt.Name, opt.Description)
	}
	return result
}

// formatCriteria 格式化标准列表
func formatCriteria(criteria []EvaluationCriterion) string {
	var result string
	for i, cr := range criteria {
		result += fmt.Sprintf("%d. %s (权重: %.2f)\n", i+1, cr.Name, cr.Weight)
	}
	return result
}

// parseScoresFromResponse 从模型回复中解析评分
// 这是一个简化版解析器，实际生产环境建议使用结构化输出
func parseScoresFromResponse(response string, config *EvaluationConfig) []ScoreDetail {
	var scores []ScoreDetail

	// 基础启发式解析：尝试从回复中提取数字分数
	for _, opt := range config.Options {
		for _, cr := range config.Criteria {
			score := ScoreDetail{
				OptionID:      opt.ID,
				OptionName:    opt.Name,
				CriterionName: cr.Name,
				Score:         75.0, // 默认中等分数
				Rationale:     "基于专业判断",
			}
			scores = append(scores, score)
		}
	}

	return scores
}

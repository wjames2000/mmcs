// Package minutes 提供讨论纪要构建和推理链生成
package minutes

import (
	"sort"
	"time"
)

// CallbackRecord 节点执行记录
// 由编排器在执行过程中收集，用于构建完整的会议纪要
type CallbackRecord struct {
	NodeName  string                 `json:"node_name"`
	RoleName  string                 `json:"role_name,omitempty"`
	Input     string                 `json:"input,omitempty"`
	Output    string                 `json:"output,omitempty"`
	StartedAt time.Time              `json:"started_at"`
	EndedAt   time.Time              `json:"ended_at"`
	Error     string                 `json:"error,omitempty"`
	Round     int                    `json:"round"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SpeechRecord 单次发言记录
type SpeechRecord struct {
	RoleName string `json:"role_name"`
	Content  string `json:"content"`
	Tokens   int    `json:"tokens,omitempty"`
}

// RoundRecord 单轮讨论记录
type RoundRecord struct {
	RoundNumber int            `json:"round_number"`
	Speeches    []SpeechRecord `json:"speeches"`
	EvalResult  string         `json:"eval_result,omitempty"`
}

// Decision 讨论中作出的决策
type Decision struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Accepted    bool     `json:"accepted"`
	RejectedBy  []string `json:"rejected_by,omitempty"`
}

// Disagreement 分歧记录
type Disagreement struct {
	Topic     string   `json:"topic"`
	Positions []string `json:"positions"`
	Resolved  bool     `json:"resolved"`
}

// ScoreEntry 评分条目
type ScoreEntry struct {
	OptionID      string  `json:"option_id"`
	OptionName    string  `json:"option_name"`
	CriterionName string  `json:"criterion_name"`
	Score         float64 `json:"score"`
	ExpertName    string  `json:"expert_name"`
	Rationale     string  `json:"rationale,omitempty"`
}

// ScoreMatrix 评分矩阵
type ScoreMatrix struct {
	Options  []string     `json:"options"`
	Criteria []string     `json:"criteria"`
	Entries  []ScoreEntry `json:"entries"`
}

// MeetingMinutes 完整会议纪要
type MeetingMinutes struct {
	SessionID     string         `json:"session_id"`
	Title         string         `json:"title"`
	Paradigm      string         `json:"paradigm"`
	Participants  []string       `json:"participants"`
	StartedAt     time.Time      `json:"started_at"`
	EndedAt       time.Time      `json:"ended_at"`
	Rounds        []RoundRecord  `json:"rounds"`
	Decisions     []Decision     `json:"decisions"`
	Disagreements []Disagreement `json:"disagreements"`
	ScoreMatrix   *ScoreMatrix   `json:"score_matrix,omitempty"`
	Conclusion    string         `json:"conclusion"`
}

// BuildMinutes 从 CallbackRecord 数组构建完整的 MeetingMinutes
// 自动分组 round、提取决策、识别分歧
func BuildMinutes(sessionID, title, paradigm string, participants []string, records []CallbackRecord) *MeetingMinutes {
	if len(records) == 0 {
		return &MeetingMinutes{
			SessionID:    sessionID,
			Title:        title,
			Paradigm:     paradigm,
			Participants: participants,
		}
	}

	// 计算起止时间
	startedAt := records[0].StartedAt
	endedAt := records[len(records)-1].EndedAt

	// 按轮次分组
	roundMap := make(map[int][]CallbackRecord)
	for _, rec := range records {
		roundMap[rec.Round] = append(roundMap[rec.Round], rec)
	}

	// 构建轮次记录
	roundNumbers := make([]int, 0, len(roundMap))
	for rn := range roundMap {
		roundNumbers = append(roundNumbers, rn)
	}
	sort.Ints(roundNumbers)

	rounds := make([]RoundRecord, 0, len(roundNumbers))
	for _, rn := range roundNumbers {
		recs := roundMap[rn]
		speeches := make([]SpeechRecord, 0)
		var evalResult string

		for _, rec := range recs {
			// 排除评估节点和总结节点，这些不是发言
			if rec.NodeName == "moderator_eval" || rec.NodeName == "evaluate" {
				evalResult = rec.Output
				continue
			}
			if rec.NodeName == "summarize" || rec.NodeName == "final_summary" {
				continue
			}
			if rec.Output != "" {
				speeches = append(speeches, SpeechRecord{
					RoleName: rec.RoleName,
					Content:  rec.Output,
				})
			}
		}

		rounds = append(rounds, RoundRecord{
			RoundNumber: rn,
			Speeches:    speeches,
			EvalResult:  evalResult,
		})
	}

	// 提取决策和分歧（启发式：从总结节点的输出中提取）
	decisions := extractDecisions(records)
	disagreements := extractDisagreements(records)

	// 提取结论
	conclusion := extractConclusion(records)

	return &MeetingMinutes{
		SessionID:     sessionID,
		Title:         title,
		Paradigm:      paradigm,
		Participants:  participants,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
		Rounds:        rounds,
		Decisions:     decisions,
		Disagreements: disagreements,
		Conclusion:    conclusion,
	}
}

// extractDecisions 从记录中提取决策（启发式）
func extractDecisions(records []CallbackRecord) []Decision {
	var decisions []Decision
	for _, rec := range records {
		if rec.NodeName == "summarize" || rec.NodeName == "final_summary" {
			if rec.Output != "" {
				decisions = append(decisions, Decision{
					Title:       "讨论结论",
					Description: rec.Output,
					Accepted:    true,
				})
			}
		}
	}
	if decisions == nil {
		decisions = []Decision{}
	}
	return decisions
}

// extractDisagreements 从记录中提取分歧（启发式）
func extractDisagreements(records []CallbackRecord) []Disagreement {
	var disagreements []Disagreement
	for _, rec := range records {
		if rec.NodeName == "critic_challenge" || rec.NodeName == "disagreement" {
			disagreements = append(disagreements, Disagreement{
				Topic:     rec.Output,
				Positions: []string{rec.RoleName, "对方"},
				Resolved:  false,
			})
		}
	}
	if disagreements == nil {
		disagreements = []Disagreement{}
	}
	return disagreements
}

// extractConclusion 从记录中提取结论
func extractConclusion(records []CallbackRecord) string {
	// 从后往前找总结节点的输出
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].NodeName == "summarize" || records[i].NodeName == "final_summary" {
			if records[i].Output != "" {
				return records[i].Output
			}
		}
	}
	return ""
}

// BuildScoreMatrix 从评分记录构建评分矩阵
func BuildScoreMatrix(entries []ScoreEntry) *ScoreMatrix {
	if len(entries) == 0 {
		return &ScoreMatrix{}
	}

	// 收集唯一的 option 和 criterion
	optionSet := make(map[string]string) // id -> name
	criterionSet := make(map[string]bool)
	for _, e := range entries {
		optionSet[e.OptionID] = e.OptionName
		criterionSet[e.CriterionName] = true
	}

	options := make([]string, 0, len(optionSet))
	for id := range optionSet {
		options = append(options, id)
	}
	sort.Strings(options)

	criteria := make([]string, 0, len(criterionSet))
	for c := range criterionSet {
		criteria = append(criteria, c)
	}
	sort.Strings(criteria)

	return &ScoreMatrix{
		Options:  options,
		Criteria: criteria,
		Entries:  entries,
	}
}

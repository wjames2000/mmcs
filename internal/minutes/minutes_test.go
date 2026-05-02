package minutes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildMinutes_EmptyRecords(t *testing.T) {
	minutes := BuildMinutes("s_test", "测试会议", "round_robin", []string{"专家A", "专家B"}, []CallbackRecord{})
	assert.Equal(t, "s_test", minutes.SessionID)
	assert.Equal(t, "测试会议", minutes.Title)
	assert.Equal(t, "round_robin", minutes.Paradigm)
	assert.Equal(t, 2, len(minutes.Participants))
	assert.Empty(t, minutes.Rounds)
}

func TestBuildMinutes_WithRecords(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "我认为方案A更好。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家B",
			Output:    "我同意方案A，但需要注意性能问题。",
			StartedAt: now.Add(2 * time.Second),
			EndedAt:   now.Add(3 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "moderator_eval",
			RoleName:  "主持人",
			Output:    "讨论充分，可以结束。",
			StartedAt: now.Add(4 * time.Second),
			EndedAt:   now.Add(5 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "summarize",
			RoleName:  "主持人",
			Output:    "最终结论：选择方案A。",
			StartedAt: now.Add(6 * time.Second),
			EndedAt:   now.Add(7 * time.Second),
			Round:     1,
		},
	}

	minutes := BuildMinutes("s_records", "评审会议", "court", []string{"专家A", "专家B", "主持人"}, records)
	assert.Equal(t, "s_records", minutes.SessionID)
	assert.Equal(t, 1, len(minutes.Rounds))
	assert.Equal(t, 2, len(minutes.Rounds[0].Speeches))
	assert.Equal(t, "最终结论：选择方案A。", minutes.Conclusion)
}

func TestBuildMinutes_MultipleRounds(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "第一轮发言。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
		{
			NodeName:  "moderator_eval",
			RoleName:  "主持人",
			Output:    "继续第二轮。",
			StartedAt: now.Add(2 * time.Second),
			EndedAt:   now.Add(3 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家B",
			Output:    "第二轮发言。",
			StartedAt: now.Add(4 * time.Second),
			EndedAt:   now.Add(5 * time.Second),
			Round:     2,
		},
	}

	minutes := BuildMinutes("s_multi_round", "多轮讨论", "round_robin", []string{"专家A", "专家B"}, records)
	assert.Equal(t, 2, len(minutes.Rounds))
	assert.Equal(t, 1, len(minutes.Rounds[0].Speeches))
	assert.Equal(t, "专家A", minutes.Rounds[0].Speeches[0].RoleName)
	assert.Equal(t, "专家B", minutes.Rounds[1].Speeches[0].RoleName)
}

func TestBuildMinutes_FreeChatParadigm(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "free_chat",
			RoleName:  "用户A",
			Output:    "你好。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     0,
		},
	}

	minutes := BuildMinutes("s_free", "自由讨论", "free_chat", []string{"用户A"}, records)
	assert.Equal(t, "free_chat", minutes.Paradigm)
	assert.Equal(t, 1, len(minutes.Rounds))
}

func TestExtractDecisions(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "summarize",
			Output:    "决策：采用方案A。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
	}

	minutes := BuildMinutes("s_dec", "决策会议", "round_robin", []string{"专家A"}, records)
	assert.Equal(t, 1, len(minutes.Decisions))
	assert.True(t, minutes.Decisions[0].Accepted)
	assert.Contains(t, minutes.Decisions[0].Description, "方案A")
}

func TestExtractDisagreements(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "critic_challenge",
			RoleName:  "批评者",
			Output:    "我质疑这个假设。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
	}

	minutes := BuildMinutes("s_dis", "分歧讨论", "evaluation", []string{"批评者", "专家"}, records)
	assert.Equal(t, 1, len(minutes.Disagreements))
	assert.Contains(t, minutes.Disagreements[0].Positions, "批评者")
}

func TestBuildScoreMatrix(t *testing.T) {
	entries := []ScoreEntry{
		{OptionID: "opt_1", OptionName: "方案A", CriterionName: "质量", Score: 85, ExpertName: "专家1"},
		{OptionID: "opt_1", OptionName: "方案A", CriterionName: "成本", Score: 70, ExpertName: "专家1"},
		{OptionID: "opt_2", OptionName: "方案B", CriterionName: "质量", Score: 90, ExpertName: "专家1"},
		{OptionID: "opt_2", OptionName: "方案B", CriterionName: "成本", Score: 60, ExpertName: "专家1"},
	}

	matrix := BuildScoreMatrix(entries)
	assert.Equal(t, 2, len(matrix.Options))
	assert.Equal(t, 2, len(matrix.Criteria))
	assert.Equal(t, 4, len(matrix.Entries))
}

func TestBuildScoreMatrix_EmptyEntries(t *testing.T) {
	matrix := BuildScoreMatrix([]ScoreEntry{})
	assert.NotNil(t, matrix)
	assert.Empty(t, matrix.Options)
	assert.Empty(t, matrix.Criteria)
	assert.Empty(t, matrix.Entries)
}

func TestBuildMinutes_NoConclusion(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "发言内容。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
	}

	minutes := BuildMinutes("s_no_conc", "无结论讨论", "round_robin", []string{"专家A"}, records)
	assert.Empty(t, minutes.Conclusion)
}

func TestBuildMinutes_TimeRange(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "第一句。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家B",
			Output:    "第二句。",
			StartedAt: now.Add(10 * time.Second),
			EndedAt:   now.Add(11 * time.Second),
			Round:     1,
		},
	}

	minutes := BuildMinutes("s_time", "时间测试", "round_robin", []string{}, records)
	assert.Equal(t, now.Unix(), minutes.StartedAt.Unix())
	assert.Equal(t, now.Add(11*time.Second).Unix(), minutes.EndedAt.Unix())
}

func TestBuildMinutes_WithDecisionAndDisagreement(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "我支持方案A。",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
		{
			NodeName:  "critic_challenge",
			RoleName:  "批评者",
			Output:    "方案A有风险。",
			StartedAt: now.Add(2 * time.Second),
			EndedAt:   now.Add(3 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "moderator_eval",
			RoleName:  "主持人",
			Output:    "达成一致。",
			StartedAt: now.Add(4 * time.Second),
			EndedAt:   now.Add(5 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "final_summary",
			RoleName:  "主持人",
			Output:    "最终决定采用方案A。",
			StartedAt: now.Add(6 * time.Second),
			EndedAt:   now.Add(7 * time.Second),
			Round:     1,
		},
	}

	minutes := BuildMinutes("s_full", "完整会议", "court", []string{"专家A", "批评者", "主持人"}, records)

	// 应该有决策
	assert.Equal(t, 1, len(minutes.Decisions))
	assert.Contains(t, minutes.Decisions[0].Description, "方案A")

	// 应该有分歧
	assert.Equal(t, 1, len(minutes.Disagreements))

	// 应该有结论
	assert.Contains(t, minutes.Conclusion, "方案A")
}

func TestBuildChain_EmptyRecords(t *testing.T) {
	chain := BuildChain([]CallbackRecord{})
	assert.Equal(t, 0, len(chain.Nodes))
}

func TestBuildChain_WithRecords(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "round_start",
			Input:     "开始第一轮讨论",
			Output:    "",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "第一轮发言",
			StartedAt: now.Add(2 * time.Second),
			EndedAt:   now.Add(3 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家B",
			Output:    "第一轮回应",
			StartedAt: now.Add(4 * time.Second),
			EndedAt:   now.Add(5 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "moderator_eval",
			RoleName:  "主持人",
			Output:    "继续讨论",
			StartedAt: now.Add(6 * time.Second),
			EndedAt:   now.Add(7 * time.Second),
			Round:     1,
		},
	}

	chain := BuildChain(records)
	assert.GreaterOrEqual(t, len(chain.Nodes), 1)

	// 第一个节点应该是 round_start
	if len(chain.Nodes) > 0 {
		assert.Equal(t, "round_start", chain.Nodes[0].Name)
		// 应该有子节点
		assert.GreaterOrEqual(t, len(chain.Nodes[0].Children), 2)
	}
}

func TestBuildChain_MultipleRounds(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "round_start",
			Input:     "第一轮",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
			Round:     1,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家A",
			Output:    "第一轮内容",
			StartedAt: now.Add(2 * time.Second),
			EndedAt:   now.Add(3 * time.Second),
			Round:     1,
		},
		{
			NodeName:  "round_start",
			Input:     "第二轮",
			StartedAt: now.Add(4 * time.Second),
			EndedAt:   now.Add(5 * time.Second),
			Round:     2,
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "专家B",
			Output:    "第二轮内容",
			StartedAt: now.Add(6 * time.Second),
			EndedAt:   now.Add(7 * time.Second),
			Round:     2,
		},
	}

	chain := BuildChain(records)
	assert.Equal(t, 2, len(chain.Nodes))
	assert.Equal(t, "round_start", chain.Nodes[0].Name)
	assert.Equal(t, "round_start", chain.Nodes[1].Name)
}

func TestBuildChain_CourtFlow(t *testing.T) {
	now := time.Now()
	records := []CallbackRecord{
		{
			NodeName:  "author_statement",
			Input:     "作者陈述",
			RoleName:  "作者",
			Output:    "设计方案内容",
			StartedAt: now,
			EndedAt:   now.Add(time.Second),
		},
		{
			NodeName:  "expert_speak",
			RoleName:  "审查员",
			Output:    "审查意见",
			StartedAt: now.Add(2 * time.Second),
			EndedAt:   now.Add(3 * time.Second),
		},
		{
			NodeName:  "author_response",
			RoleName:  "作者",
			Output:    "回应审查",
			StartedAt: now.Add(4 * time.Second),
			EndedAt:   now.Add(5 * time.Second),
		},
	}

	chain := BuildChain(records)
	assert.GreaterOrEqual(t, len(chain.Nodes), 1)
}

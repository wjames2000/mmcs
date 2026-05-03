package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareRetry_NeedsRevision(t *testing.T) {
	task := &Task{
		ID:          "t1",
		Description: "原描述",
		Status:      StatusReviewing,
	}
	result := &ValidationResult{
		Verdict: "needs_revision",
		Reason:  "缺少边界检查",
		Detail: map[string]any{
			"checked_items": []any{
				map[string]any{"item": "边界检查", "result": "fail"},
			},
		},
	}
	newDesc, err := PrepareRetry(task, result)
	assert.NoError(t, err)
	assert.Contains(t, newDesc, "原描述")
	assert.Contains(t, newDesc, "修订意见")
	assert.Contains(t, newDesc, "缺少边界检查")

	// 验证 task 的 Description 已被更新
	assert.Equal(t, newDesc, task.Description)
}

func TestPrepareRetry_NoDetail(t *testing.T) {
	task := &Task{
		ID:          "t2",
		Description: "任务描述",
		Status:      StatusReviewing,
	}
	result := &ValidationResult{
		Verdict: "needs_revision",
		Reason:  "代码质量不达标",
		Detail:  nil, // detail 为空
	}
	newDesc, err := PrepareRetry(task, result)
	assert.NoError(t, err)
	assert.Contains(t, newDesc, "任务描述")
	assert.Contains(t, newDesc, "修订意见")
	assert.Contains(t, newDesc, "代码质量不达标")
}

func TestPrepareRetry_EmptyReason(t *testing.T) {
	task := &Task{
		ID:          "t3",
		Description: "任务描述",
		Status:      StatusInProgress,
	}
	result := &ValidationResult{
		Verdict: "needs_revision",
		Reason:  "", // reason 为空
	}
	newDesc, err := PrepareRetry(task, result)
	assert.NoError(t, err)
	assert.Contains(t, newDesc, "任务描述")
	assert.Contains(t, newDesc, "修订意见")
}

func TestPrepareRetry_NilResult(t *testing.T) {
	_, err := PrepareRetry(&Task{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "验证结果不能为空")
}

func TestPrepareRetry_NilTask(t *testing.T) {
	result := &ValidationResult{Verdict: "needs_revision"}
	_, err := PrepareRetry(nil, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不能为空")
}

func TestPrepareRetry_WrongVerdict(t *testing.T) {
	task := &Task{ID: "t4", Description: "desc"}
	tests := []string{"passed", "rejected", ""}
	for _, v := range tests {
		result := &ValidationResult{Verdict: v}
		_, err := PrepareRetry(task, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "只有 needs_revision")
	}
}

func TestIsRetryable_Pending(t *testing.T) {
	task := &Task{Status: StatusPending}
	assert.False(t, IsRetryable(task), "pending 不可重试")
}

func TestIsRetryable_InProgress(t *testing.T) {
	task := &Task{Status: StatusInProgress}
	assert.True(t, IsRetryable(task), "in_progress 可重试")
}

func TestIsRetryable_Reviewing(t *testing.T) {
	task := &Task{Status: StatusReviewing}
	assert.True(t, IsRetryable(task), "reviewing 可重试")
}

func TestIsRetryable_Completed(t *testing.T) {
	task := &Task{Status: StatusCompleted}
	assert.False(t, IsRetryable(task), "completed 不可重试")
}

func TestIsRetryable_Rejected(t *testing.T) {
	task := &Task{Status: StatusRejected}
	assert.False(t, IsRetryable(task), "rejected 不可重试")
}

func TestIsRetryable_NilTask(t *testing.T) {
	assert.False(t, IsRetryable(nil))
}

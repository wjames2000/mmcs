package task

import "fmt"

// PrepareRetry 准备任务退回重试
// 将验证结果中的 reason 附加到任务描述，用于通知执行 Agent 修订方向
// 不修改任务状态，由调用方负责持久化和状态转换
//
// 参数:
//   - task: 待退回的任务（会被修改 Description 字段）
//   - result: 验证结果，包含退回原因
//
// 返回修订后的描述文本
func PrepareRetry(task *Task, result *ValidationResult) (string, error) {
	if result == nil {
		return "", fmt.Errorf("验证结果不能为空")
	}

	if result.Verdict != "needs_revision" {
		return "", fmt.Errorf("只有 needs_revision 状态可以退回重试，当前: %s", result.Verdict)
	}

	if task == nil {
		return "", fmt.Errorf("任务不能为空")
	}

	revisionNote := fmt.Sprintf("\n\n修订意见: %s", result.Reason)
	updatedDescription := task.Description + revisionNote
	task.Description = updatedDescription

	return updatedDescription, nil
}

// IsRetryable 判断任务是否可以退回重试
// 只有非终止状态的任务可以退回
func IsRetryable(task *Task) bool {
	if task == nil {
		return false
	}
	return !TerminalStatus(task.Status)
}

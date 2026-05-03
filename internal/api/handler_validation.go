package api

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/task"
	"github.com/wjames2000/mmcs/internal/validation"
)

// ValidationHandler 验证相关 HTTP handler
type ValidationHandler struct {
	validationService *validation.Service
	taskService       *task.Service
}

// NewValidationHandler 创建验证 handler
func NewValidationHandler(validationService *validation.Service, taskService *task.Service) *ValidationHandler {
	return &ValidationHandler{
		validationService: validationService,
		taskService:       taskService,
	}
}

// TriggerValidation 触发任务验证
// POST /api/v1/tasks/{id}/validate
//
// 验证在 goroutine 中异步执行，立即返回 status=reviewing
// 响应:
//
//	200: {task_id, status: "reviewing"}
//	400: 任务不存在或状态不正确
func (h *ValidationHandler) TriggerValidation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少任务 ID")
		return
	}

	// 先获取任务，检查是否存在
	t, err := h.taskService.Get(r.Context(), id)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	// 只有 in_progress 或 reviewing 状态的任务可以验证
	if t.Status != task.StatusInProgress && t.Status != task.StatusReviewing {
		middleware.WriteError(w, http.StatusBadRequest,
			"任务状态不正确，只有 in_progress 或 reviewing 状态的任务可以验证")
		return
	}

	// 异步触发验证
	if err := h.validationService.ValidateTaskAsync(r.Context(), id); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "触发验证失败: "+err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]interface{}{
		"task_id": id,
		"status":  "reviewing",
		"message": "验证已触发，请稍后查看结果",
	})
}

// GetValidationResult 获取验证结果
// GET /api/v1/tasks/{id}/validation
//
// 返回任务的验证结果，如果没有验证过则返回 null
// 响应:
//
//	200: {validation_result: ValidationResult | null}
//	404: 任务不存在
func (h *ValidationHandler) GetValidationResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少任务 ID")
		return
	}

	t, err := h.taskService.Get(r.Context(), id)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]interface{}{
		"task_id":           id,
		"validation_result": t.ValidationResult,
	})
}

// RetryValidation 触发验证退回重试
// POST /api/v1/tasks/{id}/retry-validation
//
// 手动触发退回重试，将任务状态设为 in_progress 并重新分配 Agent
// 响应:
//
//	200: {task_id, status: "in_progress"}
//	400: 任务状态不正确或未验证
func (h *ValidationHandler) RetryValidation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少任务 ID")
		return
	}

	t, err := h.taskService.Get(r.Context(), id)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	if t.ValidationResult == nil {
		middleware.WriteError(w, http.StatusBadRequest, "该任务尚未进行验证")
		return
	}

	if t.ValidationResult.Verdict != "needs_revision" && t.ValidationResult.Verdict != "rejected" {
		middleware.WriteError(w, http.StatusBadRequest, "该验证结果不允许退回重试")
		return
	}

	if err := h.validationService.HandleRetry(r.Context(), t, t.ValidationResult); err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("手动退回重试失败")
		middleware.WriteError(w, http.StatusBadRequest, "退回重试失败: "+err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]interface{}{
		"task_id": id,
		"status":  "in_progress",
		"message": "任务已退回重试",
	})
}

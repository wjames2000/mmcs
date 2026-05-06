package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/minutes"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/task"
)

// TaskHandler 任务相关 HTTP handler
type TaskHandler struct {
	taskService    *task.Service
	sessionService *session.Service
}

// NewTaskHandler 创建任务 handler
func NewTaskHandler(taskService *task.Service, sessionService *session.Service) *TaskHandler {
	return &TaskHandler{
		taskService:    taskService,
		sessionService: sessionService,
	}
}

// ListTasks 获取任务列表
// GET /api/v1/tasks?workspace_id=&session_id=&status=
// GET /api/v1/workspaces/{workspaceId}/tasks
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = r.PathValue("workspaceId")
	}
	sessionID := r.URL.Query().Get("session_id")
	status := r.URL.Query().Get("status")

	var tasks []*task.Task
	var err error

	switch {
	case workspaceID != "":
		tasks, err = h.taskService.ListByWorkspace(r.Context(), workspaceID)
	case sessionID != "":
		tasks, err = h.taskService.ListBySession(r.Context(), sessionID)
	case status != "":
		tasks, err = h.taskService.ListByStatus(r.Context(), task.Status(status))
	default:
		tasks, err = h.taskService.ListAll(r.Context())
	}

	if err != nil {
		log.Error().Err(err).Msg("获取任务列表失败")
		middleware.WriteInternalError(w, "获取任务列表失败")
		return
	}

	// 添加统计信息
	byStatus := map[string]int{
		"pending":     0,
		"in_progress": 0,
		"reviewing":   0,
		"completed":   0,
		"rejected":    0,
	}
	for _, t := range tasks {
		byStatus[string(t.Status)]++
	}

	middleware.WriteSuccess(w, map[string]interface{}{
		"tasks":     tasks,
		"count":     len(tasks),
		"by_status": byStatus,
	})
}

// CreateTask 创建任务
// POST /api/v1/tasks
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req task.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	t, err := h.taskService.Create(r.Context(), &req)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteCreated(w, t)
}

// GetTask 获取任务详情
// GET /api/v1/tasks/{id}
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
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

	middleware.WriteSuccess(w, t)
}

// UpdateTask 更新任务状态或内容
// PATCH /api/v1/tasks/{id}
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少任务 ID")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	// 如果是状态更新
	if statusStr, ok := body["status"].(string); ok {
		newStatus := task.Status(statusStr)

		// 阻止从外部将状态改为 reviewing（必须通过验证服务）
		if newStatus == task.StatusReviewing {
			middleware.WriteError(w, http.StatusBadRequest, "不允许手动将状态改为 reviewing，请使用验证接口")
			return
		}

		if err := h.taskService.UpdateStatus(r.Context(), id, newStatus); err != nil {
			middleware.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		t, _ := h.taskService.Get(r.Context(), id)
		middleware.WriteSuccess(w, t)
		return
	}

	// 否则更新内容
	t, err := h.taskService.Update(r.Context(), id, body)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, t)
}

// AssignTask 分配 Agent
// POST /api/v1/tasks/{id}/assign
func (h *TaskHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少任务 ID")
		return
	}

	var req struct {
		AgentID    string `json:"agent_id"`
		AssignedBy string `json:"assigned_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if req.AgentID == "" {
		middleware.WriteBadRequest(w, "agent_id 不能为空")
		return
	}

	if err := h.taskService.Assign(r.Context(), id, req.AgentID, req.AssignedBy); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	t, _ := h.taskService.Get(r.Context(), id)
	middleware.WriteSuccess(w, t)
}

// ExtractTasks 从会话 Minutes 提取任务
// POST /api/v1/tasks/{id}/extract
func (h *TaskHandler) ExtractTasks(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		middleware.WriteBadRequest(w, "缺少会话 ID")
		return
	}

	// 获取会话详情以获取 workspaceID 和 minutes
	sess, _, err := h.sessionService.GetWithRoles(r.Context(), sessionID)
	if err != nil {
		middleware.WriteNotFound(w, "会话不存在")
		return
	}

	// 获取该 session 相关的 minutes
	// 使用 session 的 minutes 构建一个 MeetingMinutes 对象
	// 这里从 session 中获取已记录的讨论数据
	mm := &minutes.MeetingMinutes{
		SessionID:  sess.ID,
		Title:      sess.Title,
		Paradigm:   sess.Paradigm,
		Conclusion: sess.Title + " 讨论结论", // 默认结论
	}

	// 提取任务
	extractedTasks, err := task.ExtractFromMinutes(mm)
	if err != nil {
		middleware.WriteInternalError(w, "提取任务失败: "+err.Error())
		return
	}

	// 将提取的任务保存到存储中
	var createdTasks []*task.Task
	for _, t := range extractedTasks {
		t.SessionID = sessionID
		t.WorkspaceID = sess.WorkspaceID

		created, err := h.taskService.Create(r.Context(), &task.CreateRequest{
			SessionID:          sessionID,
			WorkspaceID:        sess.WorkspaceID,
			Title:              t.Title,
			Description:        t.Description,
			AcceptanceCriteria: t.AcceptanceCriteria,
			Priority:           t.Priority,
		})
		if err != nil {
			log.Error().Err(err).Str("title", t.Title).Msg("创建提取任务失败")
			continue
		}
		createdTasks = append(createdTasks, created)
	}

	middleware.WriteCreated(w, map[string]interface{}{
		"tasks":      createdTasks,
		"count":      len(createdTasks),
		"session_id": sessionID,
	})
}

// AutoAssignTask 自动分配 Agent
// POST /api/v1/tasks/{id}/auto-assign
func (h *TaskHandler) AutoAssignTask(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Agents []task.AgentInfo `json:"agents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if len(req.Agents) == 0 {
		middleware.WriteBadRequest(w, "agents 列表不能为空")
		return
	}

	agentID := task.AutoAssign(t, req.Agents)
	if agentID == "" {
		middleware.WriteSuccess(w, map[string]interface{}{
			"task_id":       id,
			"agent_id":      "",
			"auto_assigned": false,
			"message":       "未找到匹配的 Agent，请手动分配",
		})
		return
	}

	if err := h.taskService.Assign(r.Context(), id, agentID, "auto"); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]interface{}{
		"task_id":       id,
		"agent_id":      agentID,
		"auto_assigned": true,
	})
}

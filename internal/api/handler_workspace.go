package api

import (
	"encoding/json"
	"net/http"

	"github.com/mmcs/internal/api/middleware"
	"github.com/mmcs/internal/user"
	"github.com/mmcs/internal/workspace"
)

// WorkspaceHandler 工作区相关 HTTP handler
type WorkspaceHandler struct {
	workspaceService *workspace.Service
}

// NewWorkspaceHandler 创建工作区 handler
func NewWorkspaceHandler(workspaceService *workspace.Service) *WorkspaceHandler {
	return &WorkspaceHandler{workspaceService: workspaceService}
}

// List 获取工作区列表
// GET /api/v1/workspaces
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	workspaces, err := h.workspaceService.List(r.Context(), userID)
	if err != nil {
		middleware.WriteInternalError(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, workspaces)
}

// Create 创建工作区
// POST /api/v1/workspaces
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	var req workspace.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	wsp, err := h.workspaceService.Create(r.Context(), userID, &req)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteCreated(w, wsp)
}

// Get 获取工作区详情
// GET /api/v1/workspaces/{id}
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少工作区 ID")
		return
	}

	wsp, err := h.workspaceService.Get(r.Context(), id, userID)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, wsp)
}

// Update 更新工作区
// PATCH /api/v1/workspaces/{id}
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少工作区 ID")
		return
	}

	var req workspace.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	wsp, err := h.workspaceService.Update(r.Context(), id, userID, &req)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, wsp)
}

// Archive 归档工作区
// POST /api/v1/workspaces/{id}/archive
func (h *WorkspaceHandler) Archive(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少工作区 ID")
		return
	}

	if err := h.workspaceService.Archive(r.Context(), id, userID); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]string{"status": "archived"})
}

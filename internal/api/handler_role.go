package api

import (
	"encoding/json"
	"net/http"

	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/user"
)

// RoleHandler 角色相关 HTTP handler
type RoleHandler struct {
	roleService *role.Service
}

// NewRoleHandler 创建角色 handler
func NewRoleHandler(roleService *role.Service) *RoleHandler {
	return &RoleHandler{roleService: roleService}
}

// List 获取角色列表
// GET /api/v1/roles
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())

	roles, err := h.roleService.List(r.Context(), nil, userID)
	if err != nil {
		middleware.WriteInternalError(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, roles)
}

// Create 创建角色
// POST /api/v1/roles
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	var req role.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	rl, err := h.roleService.Create(r.Context(), userID, &req)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteCreated(w, rl)
}

// Get 获取角色详情
// GET /api/v1/roles/{id}
func (h *RoleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少角色 ID")
		return
	}

	rl, err := h.roleService.Get(r.Context(), id)
	if err != nil {
		middleware.WriteNotFound(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, rl)
}

// Update 更新角色
// PUT /api/v1/roles/{id}
func (h *RoleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少角色 ID")
		return
	}

	var req role.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	rl, err := h.roleService.Update(r.Context(), id, userID, &req)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, rl)
}

// Delete 删除角色
// DELETE /api/v1/roles/{id}
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := user.UserIDFromContext(r.Context())
	if userID == "" {
		middleware.WriteUnauthorized(w, "未认证")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少角色 ID")
		return
	}

	if err := h.roleService.Delete(r.Context(), id, userID); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	middleware.WriteSuccess(w, map[string]string{"status": "deleted"})
}

// ListSkills 列出所有可用技能
// GET /api/v1/roles/skills
func (h *RoleHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	skills := h.roleService.ListSkills(r.Context())
	middleware.WriteSuccess(w, skills)
}

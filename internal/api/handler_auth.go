package api

import (
	"encoding/json"
	"net/http"

	"github.com/mmcs/internal/api/middleware"
	"github.com/mmcs/internal/user"
)

// AuthHandler 认证相关 HTTP handler
type AuthHandler struct {
	userService *user.Service
}

// NewAuthHandler 创建认证 handler
func NewAuthHandler(userService *user.Service) *AuthHandler {
	return &AuthHandler{userService: userService}
}

// Register 用户注册
// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req user.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		middleware.WriteBadRequest(w, "用户名、邮箱和密码不能为空")
		return
	}

	resp, err := h.userService.Register(r.Context(), &req)
	if err != nil {
		middleware.WriteError(w, http.StatusConflict, err.Error())
		return
	}

	middleware.WriteCreated(w, resp)
}

// Login 用户登录
// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req user.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if req.Email == "" || req.Password == "" {
		middleware.WriteBadRequest(w, "邮箱和密码不能为空")
		return
	}

	resp, err := h.userService.Login(r.Context(), &req)
	if err != nil {
		middleware.WriteUnauthorized(w, err.Error())
		return
	}

	middleware.WriteSuccess(w, resp)
}

// Refresh 刷新令牌
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if req.Token == "" {
		middleware.WriteBadRequest(w, "令牌不能为空")
		return
	}

	newToken, err := h.userService.RefreshToken(r.Context(), req.Token)
	if err != nil {
		middleware.WriteUnauthorized(w, "令牌刷新失败")
		return
	}

	middleware.WriteSuccess(w, map[string]string{"token": newToken})
}

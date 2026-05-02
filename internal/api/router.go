// Package api 提供 HTTP API 路由注册和依赖注入
package api

import (
	"net/http"

	"github.com/mmcs/internal/agent"
	"github.com/mmcs/internal/api/middleware"
	"github.com/mmcs/internal/orchestrator"
	"github.com/mmcs/internal/role"
	"github.com/mmcs/internal/session"
	"github.com/mmcs/internal/stream"
	"github.com/mmcs/internal/task"
	"github.com/mmcs/internal/user"
	"github.com/mmcs/internal/workspace"
)

// Dependencies API 依赖集合
type Dependencies struct {
	UserService         *user.Service
	AuthMiddleware      *user.AuthMiddleware
	WorkspaceService    *workspace.Service
	RoleService         *role.Service
	SessionService      *session.Service
	OrchestratorFactory *orchestrator.Factory
	HubRegistry         *stream.HubRegistry
	AgentExecutor       *agent.Executor
	TaskService         *task.Service
	TaskStore           task.Store
}

// NewRouter 创建并注册所有路由
// 使用 Go 1.22+ 的 ServeMux 模式匹配
func NewRouter(deps *Dependencies) http.Handler {
	mux := http.NewServeMux()
	rl := middleware.NewRateLimiter(100, 200)

	// Auth handlers
	authHandler := NewAuthHandler(deps.UserService)
	workspaceHandler := NewWorkspaceHandler(deps.WorkspaceService, deps.TaskService)
	roleHandler := NewRoleHandler(deps.RoleService)
	sessionHandler := NewSessionHandler(deps.SessionService, deps.OrchestratorFactory, deps.HubRegistry)
	agentHandler := NewAgentHandler(deps.AgentExecutor)
	taskHandler := NewTaskHandler(deps.TaskService, deps.SessionService)

	// 健康检查
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"api-server"}`))
	})

	// ===== 认证（无需 JWT） =====
	mux.Handle("POST /api/v1/auth/register", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(authHandler.Register))))
	mux.Handle("POST /api/v1/auth/login", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(authHandler.Login))))
	mux.Handle("POST /api/v1/auth/refresh", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(authHandler.Refresh))))

	// ===== 工作区（需 JWT） =====
	workspaceAuth := deps.AuthMiddleware.Authenticate
	mux.Handle("GET /api/v1/workspaces", rl.Limit(middleware.PanicRecovery(workspaceAuth(http.HandlerFunc(workspaceHandler.List)))))
	mux.Handle("POST /api/v1/workspaces", rl.Limit(middleware.PanicRecovery(workspaceAuth(http.HandlerFunc(workspaceHandler.Create)))))
	mux.Handle("GET /api/v1/workspaces/{id}", rl.Limit(middleware.PanicRecovery(workspaceAuth(http.HandlerFunc(workspaceHandler.Get)))))
	mux.Handle("PATCH /api/v1/workspaces/{id}", rl.Limit(middleware.PanicRecovery(workspaceAuth(http.HandlerFunc(workspaceHandler.Update)))))
	mux.Handle("POST /api/v1/workspaces/{id}/archive", rl.Limit(middleware.PanicRecovery(workspaceAuth(http.HandlerFunc(workspaceHandler.Archive)))))

	// ===== 角色（需 JWT） =====
	roleAuth := deps.AuthMiddleware.Authenticate
	mux.Handle("GET /api/v1/roles", rl.Limit(middleware.PanicRecovery(roleAuth(http.HandlerFunc(roleHandler.List)))))
	mux.Handle("POST /api/v1/roles", rl.Limit(middleware.PanicRecovery(roleAuth(http.HandlerFunc(roleHandler.Create)))))
	mux.Handle("GET /api/v1/roles/{id}", rl.Limit(middleware.PanicRecovery(roleAuth(http.HandlerFunc(roleHandler.Get)))))
	mux.Handle("PUT /api/v1/roles/{id}", rl.Limit(middleware.PanicRecovery(roleAuth(http.HandlerFunc(roleHandler.Update)))))
	mux.Handle("DELETE /api/v1/roles/{id}", rl.Limit(middleware.PanicRecovery(roleAuth(http.HandlerFunc(roleHandler.Delete)))))
	mux.Handle("GET /api/v1/roles/skills", rl.Limit(middleware.PanicRecovery(roleAuth(http.HandlerFunc(roleHandler.ListSkills)))))

	// ===== 会话（需 JWT） =====
	sessionAuth := deps.AuthMiddleware.Authenticate
	mux.Handle("GET /api/v1/workspaces/{workspaceId}/sessions", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.List)))))
	mux.Handle("POST /api/v1/sessions", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Create)))))
	mux.Handle("GET /api/v1/sessions/{id}", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Get)))))
	mux.Handle("POST /api/v1/sessions/{id}/start", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Start)))))
	mux.Handle("POST /api/v1/sessions/{id}/pause", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Pause)))))
	mux.Handle("POST /api/v1/sessions/{id}/resume", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Resume)))))
	mux.Handle("POST /api/v1/sessions/{id}/terminate", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Terminate)))))

	// ===== SSE 流式推送（无需 JWT，使用 token 参数） =====
	mux.Handle("GET /api/v1/sessions/{id}/stream", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(sessionHandler.Stream))))

	// ===== Agent（无需 JWT，后续版本添加认证） =====
	mux.Handle("POST /api/v1/agents", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(agentHandler.Register))))
	mux.Handle("GET /api/v1/agents", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(agentHandler.ListAgents))))
	mux.Handle("POST /api/v1/agents/{id}/execute", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(agentHandler.ExecuteSync))))
	mux.Handle("POST /api/v1/agents/{id}/execute-async", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(agentHandler.ExecuteAsync))))

	// ===== 任务（需 JWT） =====
	taskAuth := deps.AuthMiddleware.Authenticate
	mux.Handle("GET /api/v1/tasks", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.ListTasks)))))
	mux.Handle("POST /api/v1/tasks", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.CreateTask)))))
	mux.Handle("GET /api/v1/tasks/{id}", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.GetTask)))))
	mux.Handle("PATCH /api/v1/tasks/{id}", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.UpdateTask)))))
	mux.Handle("POST /api/v1/tasks/{id}/assign", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.AssignTask)))))
	mux.Handle("POST /api/v1/tasks/{id}/extract", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.ExtractTasks)))))
	mux.Handle("POST /api/v1/tasks/{id}/auto-assign", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(taskHandler.AutoAssignTask)))))

	return middleware.CORS(middleware.PanicRecovery(mux))
}

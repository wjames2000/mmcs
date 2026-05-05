// Package api 提供 HTTP API 路由注册和依赖注入
package api

import (
	"net/http"

	"github.com/wjames2000/mmcs/internal/agent"
	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/orchestrator"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/task"
	"github.com/wjames2000/mmcs/internal/user"
	"github.com/wjames2000/mmcs/internal/validation"
	"github.com/wjames2000/mmcs/internal/workspace"
)

// MetricsHandler 指标 HTTP handler 类型
type MetricsHandler http.Handler

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
	ValidationService   *validation.Service
	HealthHandler       *HealthHandler
	MetricsHandler      MetricsHandler
	MaterialStore       *session.MaterialStore
	ModelGateway        *model_gateway.Gateway
	SessionMessageStore *session.MessageStore
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
	sessionHandler := NewSessionHandler(deps.SessionService, deps.OrchestratorFactory, deps.HubRegistry, deps.MaterialStore, deps.SessionMessageStore)
	agentHandler := NewAgentHandler(deps.AgentExecutor)
	taskHandler := NewTaskHandler(deps.TaskService, deps.SessionService)

	// 健康检查
	if deps.HealthHandler != nil {
		mux.Handle("GET /healthz", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(deps.HealthHandler.Liveness))))
		mux.Handle("GET /healthz/ready", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(deps.HealthHandler.Readiness))))
		mux.Handle("GET /healthz/deps", rl.Limit(middleware.PanicRecovery(http.HandlerFunc(deps.HealthHandler.Deps))))
	} else {
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","service":"api-server"}`))
		})
	}

	// Prometheus 指标
	if deps.MetricsHandler != nil {
		mux.Handle("GET /metrics", rl.Limit(middleware.PanicRecovery(http.Handler(deps.MetricsHandler))))
	}

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
	mux.Handle("GET /api/v1/sessions/{id}/minutes", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.GetMinutes)))))
	mux.Handle("POST /api/v1/sessions/restart", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Restart)))))
	mux.Handle("GET /api/v1/sessions/{sessionId}/merged-minutes", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.GetMergedMinutes)))))
	mux.Handle("GET /api/v1/sessions/{sessionId}/messages", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.ListMessages)))))
	mux.Handle("DELETE /api/v1/sessions/{id}", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.Delete)))))
	mux.Handle("GET /api/v1/sessions/{sessionId}/tasks", rl.Limit(middleware.PanicRecovery(sessionAuth(http.HandlerFunc(sessionHandler.ExtractTasks)))))

	// ===== 会议材料（需 JWT） =====
	materialAuth := deps.AuthMiddleware.Authenticate
	mux.Handle("POST /api/v1/sessions/{sessionId}/materials", rl.Limit(middleware.PanicRecovery(materialAuth(http.HandlerFunc(sessionHandler.UploadMaterial)))))
	mux.Handle("GET /api/v1/sessions/{sessionId}/materials", rl.Limit(middleware.PanicRecovery(materialAuth(http.HandlerFunc(sessionHandler.ListMaterials)))))
	mux.Handle("DELETE /api/v1/materials/{id}", rl.Limit(middleware.PanicRecovery(materialAuth(http.HandlerFunc(sessionHandler.DeleteMaterial)))))

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

	// ===== 验证（需 JWT） =====
	if deps.ValidationService != nil {
		validationHandler := NewValidationHandler(deps.ValidationService, deps.TaskService)
		mux.Handle("POST /api/v1/tasks/{id}/validate", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(validationHandler.TriggerValidation)))))
		mux.Handle("GET /api/v1/tasks/{id}/validation", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(validationHandler.GetValidationResult)))))
		mux.Handle("POST /api/v1/tasks/{id}/retry-validation", rl.Limit(middleware.PanicRecovery(taskAuth(http.HandlerFunc(validationHandler.RetryValidation)))))
	}

	// ===== 模型配置（需 JWT） =====
	if deps.ModelGateway != nil {
		modelHandler := NewModelHandler(deps.ModelGateway)
		modelAuth := deps.AuthMiddleware.Authenticate
		mux.Handle("GET /api/v1/models", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.ListModels)))))
		mux.Handle("GET /api/v1/models/providers", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.ListProviders)))))
		mux.Handle("POST /api/v1/models/providers", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.CreateProvider)))))
		mux.Handle("PUT /api/v1/models/providers/{id}", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.UpdateProvider)))))
		mux.Handle("DELETE /api/v1/models/providers/{id}", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.DeleteProvider)))))
		mux.Handle("POST /api/v1/models/providers/{id}/toggle", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.ToggleProvider)))))
		mux.Handle("GET /api/v1/models/refresh/{providerName}", rl.Limit(middleware.PanicRecovery(modelAuth(http.HandlerFunc(modelHandler.RefreshModels)))))
	}

	return middleware.CORS(middleware.PanicRecovery(mux))
}

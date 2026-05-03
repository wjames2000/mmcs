package main

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/wjames2000/mmcs/config"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/task"
	"github.com/wjames2000/mmcs/internal/user"
	"github.com/wjames2000/mmcs/internal/workspace"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// App Wails 应用主结构
type App struct {
	ctx context.Context

	// 配置
	cfg *config.Config

	// 基础设施
	dbPool *pgxpool.Pool
	redis  *redis.Client

	// 服务
	userSvc      *user.Service
	workspaceSvc *workspace.Service
	sessionSvc   *session.Service
	roleSvc      *role.Service
	taskSvc      *task.Service
	gateway      *model_gateway.Gateway

	// 流式推送
	hubRegistry *stream.HubRegistry
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// startup 应用启动时初始化所有依赖
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Info().Msg("MMCS 桌面应用启动中...")

	// 1. 加载配置
	cfg, err := config.Load("development")
	if err != nil {
		log.Fatal().Err(err).Msg("加载配置失败")
	}
	a.cfg = cfg

	// 2. 连接数据库
	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("连接数据库失败")
	}
	a.dbPool = pool

	// 3. 连接 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("连接 Redis 失败")
	}
	a.redis = rdb

	// 4. 初始化服务
	jwtManager := user.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)
	userRepo := user.NewRepository(pool)
	a.userSvc = user.NewService(userRepo, jwtManager)

	wsRepo := workspace.NewRepository(pool)
	a.workspaceSvc = workspace.NewService(wsRepo)

	roleRepo := role.NewRepository(pool)
	skillRegistry := role.NewSkillRegistry()
	a.roleSvc = role.NewService(roleRepo, skillRegistry)

	sessionRepo := session.NewRepository(pool)
	graphPool := session.NewGraphPool(cfg.Session.GraphPoolSize)
	a.sessionSvc = session.NewService(sessionRepo, graphPool, a.roleSvc)

	taskStore := task.NewStore(pool)
	a.taskSvc = task.NewService(taskStore)

	a.gateway = model_gateway.NewGateway(&cfg.ModelGateway)

	// 5. 初始化 SSE Hub 注册表
	a.hubRegistry = stream.NewHubRegistry()
	a.sessionSvc.SetHubRegistry(a.hubRegistry)

	log.Info().Msg("MMCS 桌面应用启动完成")
}

// shutdown 应用关闭时清理资源
func (a *App) shutdown(ctx context.Context) {
	log.Info().Msg("MMCS 桌面应用关闭中...")

	if a.hubRegistry != nil {
		a.hubRegistry.Cleanup()
	}
	if a.dbPool != nil {
		a.dbPool.Close()
	}
	if a.redis != nil {
		a.redis.Close()
	}

	log.Info().Msg("MMCS 桌面应用已关闭")
}

// emitSessionEvent 向 Wails 前端发射会话事件
func (a *App) emitSessionEvent(sessionID, eventType string, data interface{}) {
	runtime.EventsEmit(a.ctx, fmt.Sprintf("session:%s", sessionID), map[string]interface{}{
		"type": eventType,
		"data": data,
	})
}

// ==================== Auth ====================

// Login 用户登录
func (a *App) Login(email, password string) (*user.LoginResponse, error) {
	return a.userSvc.Login(a.ctx, &user.LoginRequest{Email: email, Password: password})
}

// Register 用户注册
func (a *App) Register(name, email, password string) (*user.RegisterResponse, error) {
	return a.userSvc.Register(a.ctx, &user.RegisterRequest{
		Name:     name,
		Email:    email,
		Password: password,
	})
}

// RefreshToken 刷新令牌
func (a *App) RefreshToken(token string) (string, error) {
	return a.userSvc.RefreshToken(a.ctx, token)
}

// GetCurrentUser 获取当前用户信息
func (a *App) GetCurrentUser(userID string) (*user.User, error) {
	return a.userSvc.GetUser(a.ctx, userID)
}

// ==================== Workspace ====================

// GetWorkspaces 获取用户的工作区列表
func (a *App) GetWorkspaces(userID string) ([]*workspace.Workspace, error) {
	return a.workspaceSvc.List(a.ctx, userID)
}

// CreateWorkspace 创建工作区
func (a *App) CreateWorkspace(creatorID string, name, description, mode string) (*workspace.Workspace, error) {
	return a.workspaceSvc.Create(a.ctx, creatorID, &workspace.CreateRequest{
		Name:        name,
		Description: description,
		Mode:        mode,
	})
}

// GetWorkspaceDetail 获取工作区详情
func (a *App) GetWorkspaceDetail(id, userID string) (*workspace.Workspace, error) {
	return a.workspaceSvc.Get(a.ctx, id, userID)
}

// UpdateWorkspace 更新工作区
func (a *App) UpdateWorkspace(id, userID, name, description, mode string) (*workspace.Workspace, error) {
	return a.workspaceSvc.Update(a.ctx, id, userID, &workspace.UpdateRequest{
		Name:        name,
		Description: description,
		Mode:        mode,
	})
}

// ArchiveWorkspace 归档工作区
func (a *App) ArchiveWorkspace(id, userID string) error {
	return a.workspaceSvc.Archive(a.ctx, id, userID)
}

// ==================== Session ====================

// GetSessions 获取工作区下的会话列表
func (a *App) GetSessions(workspaceID string) ([]*session.Session, error) {
	return a.sessionSvc.ListByWorkspace(a.ctx, workspaceID)
}

// CreateSession 创建会话
func (a *App) CreateSession(creatorID, workspaceID, title, paradigm string, maxRounds int, roleIDs []string) (*session.CreateResponse, error) {
	return a.sessionSvc.Create(a.ctx, creatorID, &session.CreateRequest{
		WorkspaceID: workspaceID,
		Title:       title,
		Paradigm:    paradigm,
		MaxRounds:   maxRounds,
		RoleIDs:     roleIDs,
	})
}

// GetSessionDetail 获取会话详情（含角色绑定）
func (a *App) GetSessionDetail(id string) (*session.Session, []*session.SessionRole, error) {
	return a.sessionSvc.GetWithRoles(a.ctx, id)
}

// StartSession 启动会话
func (a *App) StartSession(id string) error {
	// 通知前端会话开始事件
	a.emitSessionEvent(id, "session.starting", nil)
	return a.sessionSvc.Start(a.ctx, id)
}

// PauseSession 暂停会话
func (a *App) PauseSession(id, nodeName, message string) error {
	return a.sessionSvc.Pause(a.ctx, id, nodeName, message)
}

// ResumeSession 恢复会话
func (a *App) ResumeSession(id, message string) error {
	return a.sessionSvc.Resume(a.ctx, id, message)
}

// TerminateSession 终止会话
func (a *App) TerminateSession(id string) error {
	return a.sessionSvc.Terminate(a.ctx, id)
}

// ==================== Role ====================

// GetRoles 获取角色列表
func (a *App) GetRoles(creatorID string) ([]*role.Role, error) {
	return a.roleSvc.List(a.ctx, nil, creatorID)
}

// CreateRole 创建角色
func (a *App) CreateRole(creatorID string, req *role.CreateRequest) (*role.Role, error) {
	return a.roleSvc.Create(a.ctx, creatorID, req)
}

// UpdateRole 更新角色
func (a *App) UpdateRole(id, userID string, req *role.UpdateRequest) (*role.Role, error) {
	return a.roleSvc.Update(a.ctx, id, userID, req)
}

// DeleteRole 删除角色
func (a *App) DeleteRole(id, userID string) error {
	return a.roleSvc.Delete(a.ctx, id, userID)
}

// GetSkills 获取可用技能列表
func (a *App) GetSkills() []*role.SkillDefinition {
	return a.roleSvc.ListSkills(a.ctx)
}

// ==================== Task ====================

// GetTasks 获取工作区下的任务列表
func (a *App) GetTasks(workspaceID string) ([]*task.Task, error) {
	return a.taskSvc.ListByWorkspace(a.ctx, workspaceID)
}

// CreateTask 创建任务
func (a *App) CreateTask(req *task.CreateRequest) (*task.Task, error) {
	return a.taskSvc.Create(a.ctx, req)
}

// UpdateTaskStatus 更新任务状态
func (a *App) UpdateTaskStatus(id string, status task.Status) error {
	return a.taskSvc.UpdateStatus(a.ctx, id, status)
}

// AssignTask 分配任务
func (a *App) AssignTask(taskID, agentID, assignedBy string) error {
	return a.taskSvc.Assign(a.ctx, taskID, agentID, assignedBy)
}

// GetTaskDetail 获取任务详情
func (a *App) GetTaskDetail(id string) (*task.Task, error) {
	return a.taskSvc.Get(a.ctx, id)
}

// ==================== Model ====================

// GetModels 获取已配置的模型提供商列表
func (a *App) GetModels() []string {
	return a.gateway.ListProviders()
}

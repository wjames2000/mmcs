package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/wjames2000/mmcs/config"
	"github.com/wjames2000/mmcs/internal/minutes"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/orchestrator"
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

	// 编排
	orchFactory *orchestrator.Factory

	// 流式推送
	hubRegistry *stream.HubRegistry

	// 会议材料
	materialStore *session.MaterialStore

	// 讨论消息持久化
	messageStore *session.MessageStore
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// startup 应用启动时初始化所有依赖
// 所有初始化失败均不会导致应用崩溃，仅记录错误并通过 dialog 提示
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.LogInfo(a.ctx, "MMCS 桌面应用启动中...")

	// 1. 加载配置
	// 优先级: MMCS_CONFIG 环境变量 > ~/.config/mmcs/config.yaml > config/config.dev.yaml
	var (
		cfg *config.Config
		err error
	)
	if envPath := os.Getenv("MMCS_CONFIG"); envPath != "" {
		cfg, err = config.LoadFromFile(envPath)
	} else {
		cfg, err = config.Load("development")
	}
	if err != nil {
		// 尝试从用户目录加载
		homeCfg := os.ExpandEnv("$HOME/.config/mmcs/config.yaml")
		cfg, err = config.LoadFromFile(homeCfg)
		if err != nil {
			runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Title:   "配置错误",
				Message: fmt.Sprintf("加载配置文件失败: %v\n\n请检查配置:\n  1. config/config.dev.yaml（DST=postgres://postgres:admin@localhost:5432/mmcs）\n  2. ~/.config/mmcs/config.yaml\n  3. MMCS_CONFIG 环境变量", err),
				Type:    runtime.ErrorDialog,
			})
			return
		}
	}
	a.cfg = cfg
	runtime.LogInfo(a.ctx, "配置加载成功")

	// 2. 连接数据库（非致命，失败后服务不可用但 UI 仍可显示）
	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		runtime.LogWarning(a.ctx, fmt.Sprintf("创建数据库连接池失败: %v", err))
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "数据库连接失败",
			Message: fmt.Sprintf("无法创建数据库连接池:\n%s\n\n请检查配置 DSN:\n%s", err, cfg.Database.DSN),
			Type:    runtime.WarningDialog,
		})
	} else if err := pool.Ping(ctx); err != nil {
		// 连接池创建成功但无法连通（例如数据库不存在）
		pool.Close()
		runtime.LogWarning(a.ctx, fmt.Sprintf("数据库无法连通: %v (DSN: %s)", err, cfg.Database.DSN))
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "数据库无法连通",
			Message: fmt.Sprintf("数据库地址可达但连接失败:\n%s\n\n请检查:\n1. 数据库 'mmcs' 是否已创建\n2. 用户名和密码是否正确\n3. 运行迁移: docker exec hpds-pgsql psql -U postgres -c 'CREATE DATABASE mmcs;'", err),
			Type:    runtime.WarningDialog,
		})
	} else {
		a.dbPool = pool
		runtime.LogInfo(a.ctx, "数据库连接成功")
	}

	// 3. 连接 Redis（非致命）
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("连接 Redis 失败: %v", err))
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Redis 连接失败",
			Message: fmt.Sprintf("无法连接到 Redis:\n%s\n\n请确保 Redis 已启动:\ndocker compose up -d redis", err),
			Type:    runtime.WarningDialog,
		})
	} else {
		a.redis = rdb
		runtime.LogInfo(a.ctx, "Redis 连接成功")
	}

	// 4. 初始化服务（使用内存存储，不依赖数据库连接）
	jwtExpiry, err := time.ParseDuration(cfg.Auth.JWTExpiry)
	if err != nil {
		jwtExpiry = 15 * time.Minute
	}
	jwtManager := user.NewJWTManager(cfg.Auth.JWTSecret, jwtExpiry)

	// 无论数据库是否可用，都初始化服务（Wails 绑定需要非 nil 服务）
	taskStore := task.NewMemoryStore()
	a.taskSvc = task.NewService(taskStore)

	skillRegistry := role.NewSkillRegistry()
	a.roleSvc = role.NewService(nil, skillRegistry)

	if a.dbPool != nil {
		userRepo := user.NewRepository(a.dbPool)
		a.userSvc = user.NewService(userRepo, jwtManager)

		wsRepo := workspace.NewRepository(a.dbPool)
		a.workspaceSvc = workspace.NewService(wsRepo)

		roleRepo := role.NewRepository(a.dbPool)
		a.roleSvc = role.NewService(roleRepo, skillRegistry)

		sessionRepo := session.NewRepository(a.dbPool)
		graphPool := session.NewGraphPool(cfg.Session.GraphPoolSize)
		a.sessionSvc = session.NewService(sessionRepo, graphPool, a.roleSvc)

		a.gateway = model_gateway.NewGateway(&cfg.ModelGateway)

		// 5. 初始化编排工厂
		a.orchFactory = orchestrator.NewFactory(a.roleSvc, skillRegistry, a.gateway)

		// 6. 初始化会议材料存储
		a.materialStore = session.NewMaterialStore()
		a.sessionSvc.SetMaterialStore(a.materialStore)

		// 6b. 初始化讨论消息持久化存储
		a.messageStore = session.NewMessageStore()

		// 7. 初始化 SSE Hub 注册表
		a.hubRegistry = stream.NewHubRegistry()
		a.sessionSvc.SetHubRegistry(a.hubRegistry)

		runtime.LogInfo(a.ctx, "MMCS 桌面应用启动完成（完整模式）")
	} else {
		runtime.LogInfo(a.ctx, "MMCS 桌面应用启动完成（离线模式 - 数据库未连接，部分功能不可用）")
	}
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
	if a.workspaceSvc == nil {
		return nil, fmt.Errorf("数据库未连接，工作区服务不可用")
	}
	return a.workspaceSvc.List(a.ctx, userID)
}

// CreateWorkspace 创建工作区
func (a *App) CreateWorkspace(creatorID string, name, description, mode string) (*workspace.Workspace, error) {
	if a.workspaceSvc == nil {
		return nil, fmt.Errorf("数据库未连接，无法创建工作区")
	}
	return a.workspaceSvc.Create(a.ctx, creatorID, &workspace.CreateRequest{
		Name:        name,
		Description: description,
		Mode:        mode,
	})
}

// GetWorkspaceDetail 获取工作区详情
func (a *App) GetWorkspaceDetail(id, userID string) (*workspace.Workspace, error) {
	if a.workspaceSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return a.workspaceSvc.Get(a.ctx, id, userID)
}

// UpdateWorkspace 更新工作区
func (a *App) UpdateWorkspace(id, userID, name, description, mode string) (*workspace.Workspace, error) {
	if a.workspaceSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return a.workspaceSvc.Update(a.ctx, id, userID, &workspace.UpdateRequest{
		Name:        name,
		Description: description,
		Mode:        mode,
	})
}

// ArchiveWorkspace 归档工作区
func (a *App) ArchiveWorkspace(id, userID string) error {
	if a.workspaceSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.workspaceSvc.Archive(a.ctx, id, userID)
}

// ==================== Session ====================

// GetSessions 获取工作区下的会话列表
func (a *App) GetSessions(workspaceID string) ([]*session.Session, error) {
	if a.sessionSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.ListByWorkspace(a.ctx, workspaceID)
}

// CreateSession 创建会话
// roleBindingsJSON 是 JSON 字符串，如 '[{"role_id":"r1","model_override":{"provider":"openai","model_name":"gpt-4"}}]'
// 为空字符串 "" 时使用 roleIDs 兼容旧模式
// topic 是讨论主题/背景描述（可选，传入空字符串则不设置）
// moderatorModel 是主持人模型绑定，如 "openai:gpt-4"（可选，传入空字符串则不设置）
func (a *App) CreateSession(creatorID, workspaceID, title, topic, paradigm string, maxRounds int, roleIDs []string, roleBindingsJSON string, moderatorModel string) (*session.CreateResponse, error) {
	if a.sessionSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}

	// 构建 session config（含主持人模型等额外配置）
	configMap := make(map[string]interface{})
	if moderatorModel != "" {
		configMap["moderator_model"] = moderatorModel
	}
	configJSON, _ := json.Marshal(configMap)

	req := &session.CreateRequest{
		WorkspaceID: workspaceID,
		Title:       title,
		Topic:       topic,
		Paradigm:    paradigm,
		MaxRounds:   maxRounds,
		RoleIDs:     roleIDs,
		Config:      configJSON,
	}

	if roleBindingsJSON != "" {
		var bindings []session.RoleBinding
		if err := json.Unmarshal([]byte(roleBindingsJSON), &bindings); err != nil {
			return nil, fmt.Errorf("解析角色绑定 JSON 失败: %w", err)
		}
		req.RoleBindings = bindings
	}

	return a.sessionSvc.Create(a.ctx, creatorID, req)
}

// GetSessionDetail 获取会话详情（含角色绑定）
func (a *App) GetSessionDetail(id string) (map[string]interface{}, error) {
	if a.sessionSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	sess, roles, err := a.sessionSvc.GetWithRoles(a.ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session": sess,
		"roles":   roles,
	}, nil
}

// AddSessionRole 向会话添加角色（仅草稿状态）
// modelOverrideJSON 是 JSON 字符串，如 '{"provider":"openai","model_name":"gpt-4"}'
func (a *App) AddSessionRole(sessionID, roleID, modelOverrideJSON string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	var override json.RawMessage
	if modelOverrideJSON != "" {
		override = json.RawMessage(modelOverrideJSON)
	}
	return a.sessionSvc.AddSessionRole(a.ctx, sessionID, roleID, override)
}

// RemoveSessionRole 从会话移除角色（仅草稿状态）
func (a *App) RemoveSessionRole(sessionID, roleID string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.RemoveSessionRole(a.ctx, sessionID, roleID)
}

// StartSession 启动会话并异步执行编排
func (a *App) StartSession(id string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}

	// 通知前端会话开始事件
	a.emitSessionEvent(id, "session.starting", nil)

	// 1. 更新状态为 running
	if err := a.sessionSvc.Start(a.ctx, id); err != nil {
		return err
	}

	// 2. 异步启动编排
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("编排执行异常: %v", r)
				runtime.LogErrorf(a.ctx, "编排执行 panic [%s]: %v", id, r)
				a.emitSessionEvent(id, "session.error", map[string]interface{}{
					"session_id": id,
					"error":      errMsg,
				})
			}
		}()
		if err := a.startOrchestration(a.ctx, id); err != nil {
			runtime.LogErrorf(a.ctx, "编排执行失败 [%s]: %v", id, err)
			// 通知前端编排失败
			a.emitSessionEvent(id, "session.error", map[string]interface{}{
				"session_id": id,
				"error":      err.Error(),
			})
		}
	}()

	return nil
}

// startOrchestration 启动异步编排
// 与 HTTP handler 中的实现保持一致，使用 topic 作为讨论主题描述（回退到 title）
func (a *App) startOrchestration(ctx context.Context, sessionID string) error {
	// 获取会话详情 + 角色绑定
	sess, sessionRoles, err := a.sessionSvc.GetWithRoles(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("获取会话失败: %w", err)
	}

	// 获取 Hub 和 Bridge
	hub := a.hubRegistry.GetOrCreate(sessionID)
	bridge := stream.NewBridge(hub, 128)
	bridge.Start(ctx)

	// 将 Hub 事件桥接到 Wails 运行时事件（供前端 useSSE 接收）
	hubSub := &stream.Subscriber{
		ID:      "wails-bridge-" + sessionID,
		Events:  make(chan *stream.Event, 256),
		CloseCh: make(chan struct{}),
	}
	hub.Subscribe(hubSub)
	go func() {
		for {
			select {
			case ev := <-hubSub.Events:
				// Bridge 的 Event.Data 是 JSON 字符串，需要解析为对象
				payload := ev.Data
				if str, ok := ev.Data.(string); ok {
					var parsed interface{}
					if err := json.Unmarshal([]byte(str), &parsed); err == nil {
						payload = parsed
					}
				}
				a.emitSessionEvent(sessionID, ev.Type, payload)
			case <-hubSub.CloseCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	defer hub.Unsubscribe(hubSub.ID)

	// 初始化中断/恢复 channel
	ch := a.sessionSvc.InitChannels(sessionID)
	defer a.sessionSvc.RemoveChannels(sessionID)

	// 获取编排器
	orch, err := a.orchFactory.CreateOrchestrator(orchestrator.ParadigmType(sess.Paradigm))
	if err != nil {
		return fmt.Errorf("创建编排器失败: %w", err)
	}

	// 提取角色 ID 和主持人配置
	roleIDs := make([]string, len(sessionRoles))
	var moderatorModel string
	if sess.Config != nil {
		var cfg struct {
			ModeratorModel string `json:"moderator_model"`
		}
		if err := json.Unmarshal(sess.Config, &cfg); err == nil {
			moderatorModel = cfg.ModeratorModel
		}
	}
	for i, sr := range sessionRoles {
		roleIDs[i] = sr.RoleID
		_ = i
	}

	// 使用 topic 作为讨论主题描述（优先 topic，回退到 title）
	topic := ""
	if sess.Topic != nil {
		topic = *sess.Topic
	}
	if topic == "" {
		topic = sess.Title
	}

	progressCh := make(chan string, 10)
	go func() {
		for msg := range progressCh {
			runtime.LogDebug(a.ctx, fmt.Sprintf("讨论进度 [%s]: %s", sessionID, msg))
		}
	}()

	// 根据范式类型执行
	var execErr error
	switch o := orch.(type) {
	case *orchestrator.RoundRobinOrchestrator:
		config := &orchestrator.RoundRobinConfig{
			RoleIDs:         roleIDs,
			Topic:           topic,
			MaxRounds:       sess.MaxRounds,
			ModeratorModel:  moderatorModel,
			ModeratorPrompt: "",
			InterruptCh:     ch.InterruptCh,
			ResumeCh:        ch.ResumeCh,
			MsgStore:        a.messageStore,
		}
		_, execErr = o.Execute(ctx, sessionID, config, bridge, progressCh)

	case *orchestrator.CourtOrchestrator:
		config := &orchestrator.CourtConfig{
			RoleIDs:     roleIDs,
			Topic:       topic,
			MaxRounds:   sess.MaxRounds,
			InterruptCh: ch.InterruptCh,
			ResumeCh:    ch.ResumeCh,
			MsgStore:    a.messageStore,
		}
		_, execErr = o.Execute(ctx, sessionID, config, bridge, progressCh)

	case *orchestrator.EvaluationOrchestrator:
		config := &orchestrator.EvaluationConfig{
			RoleIDs:     roleIDs,
			Topic:       topic,
			InterruptCh: ch.InterruptCh,
			ResumeCh:    ch.ResumeCh,
			MsgStore:    a.messageStore,
		}
		_, execErr = o.Execute(ctx, sessionID, config, bridge, progressCh)

	case *orchestrator.FreeChatOrchestrator:
		config := &orchestrator.FreeChatConfig{
			RoleIDs:     roleIDs,
			Topic:       topic,
			MaxRounds:   sess.MaxRounds,
			InterruptCh: ch.InterruptCh,
			ResumeCh:    ch.ResumeCh,
			MsgStore:    a.messageStore,
		}
		_, execErr = o.Execute(ctx, sessionID, config, bridge, progressCh)

	default:
		execErr = fmt.Errorf("不支持的编排器类型: %T", orch)
	}

	// 关闭 progressCh（避免 goroutine 泄漏）
	close(progressCh)

	// 仅当执行出错时才终止会话（让用户手动终止正常完成的讨论）
	if execErr != nil {
		_ = a.sessionSvc.Terminate(ctx, sessionID)
		return fmt.Errorf("讨论执行失败: %w", execErr)
	}

	// 讨论正常结束，不自动终止，保持 running 状态
	// 用户可以通过 UI 手动点击"结束"来终止会话，期间可随时暂停/恢复
	log.Info().Str("session_id", sessionID).Msg("讨论执行完成，等待用户手动终止")
	return nil
}

// PauseSession 暂停会话
func (a *App) PauseSession(id, nodeName, message string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.Pause(a.ctx, id, nodeName, message)
}

// ResumeSession 恢复会话
func (a *App) ResumeSession(id, message string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.Resume(a.ctx, id, message)
}

// TerminateSession 终止会话
func (a *App) TerminateSession(id string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.Terminate(a.ctx, id)
}

// GetSessionMinutes 获取会话会议纪要
func (a *App) GetSessionMinutes(sessionID string) (*minutes.MeetingMinutes, error) {
	if a.sessionSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.GetMinutes(a.ctx, sessionID)
}

// ArchiveSession 归档会话
func (a *App) ArchiveSession(id string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.Archive(a.ctx, id)
}

// DeleteSession 删除会话
func (a *App) DeleteSession(id string) error {
	if a.sessionSvc == nil {
		return fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.Delete(a.ctx, id)
}

// ==================== Restart ====================

// RestartSession 重启已结束的会议
// roleBindingsJSON 是 JSON 字符串，如 '[{"role_id":"r1","model_override":{"provider":"openai","model_name":"gpt-4"}}]'
// 为空字符串 "" 时使用 roleIDs 兼容旧模式
func (a *App) RestartSession(originalID, creatorID, newTitle, newTopic string, roleIDs []string, roleBindingsJSON string) (*session.CreateResponse, error) {
	if a.sessionSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var bindings []session.RoleBinding
	if roleBindingsJSON != "" {
		if err := json.Unmarshal([]byte(roleBindingsJSON), &bindings); err != nil {
			return nil, fmt.Errorf("解析角色绑定 JSON 失败: %w", err)
		}
	}

	// 确保第一个角色绑定携带模型 override（主持人模型）
	return a.sessionSvc.Restart(a.ctx, originalID, creatorID, newTitle, newTopic, roleIDs, bindings)
}

// GetMergedMinutes 获取合并会议纪要
func (a *App) GetMergedMinutes(newSessionID, originalSessionID string) (*session.MergedMinutes, error) {
	if a.sessionSvc == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return a.sessionSvc.GetMergedMinutes(a.ctx, newSessionID, originalSessionID)
}

// ==================== Material ====================

// UploadSessionMaterial 上传会议材料
// base64Data 是文件的 base64 编码内容（不含 data: URL 前缀）
func (a *App) UploadSessionMaterial(sessionID, fileName, mimeType, base64Data string) (*session.Material, error) {
	if a.materialStore == nil {
		return nil, fmt.Errorf("材料存储未初始化")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("会话 ID 不能为空")
	}
	if fileName == "" {
		return nil, fmt.Errorf("文件名不能为空")
	}

	data, err := decodeBase64Data(base64Data)
	if err != nil {
		return nil, fmt.Errorf("解码 base64 数据失败: %w", err)
	}

	m := a.materialStore.Add(sessionID, fileName, mimeType, data)
	return m, nil
}

// GetSessionMaterials 获取会话的所有材料
func (a *App) GetSessionMaterials(sessionID string) []*session.Material {
	if a.materialStore == nil {
		return nil
	}
	return a.materialStore.ListBySession(sessionID)
}

// DeleteSessionMaterial 删除会议材料
func (a *App) DeleteSessionMaterial(materialID string) error {
	if a.materialStore == nil {
		return fmt.Errorf("材料存储未初始化")
	}
	return a.materialStore.Delete(materialID)
}

// ==================== Session Messages ====================

// GetSessionMessages 获取会话的所有讨论消息
func (a *App) GetSessionMessages(sessionID string) ([]*session.ChatMessage, error) {
	if a.messageStore == nil {
		return nil, fmt.Errorf("消息存储未初始化")
	}
	return a.messageStore.ListBySession(sessionID)
}

// decodeBase64Data 解码 base64 数据
// 支持标准 base64 和 data URL 格式（如 "data:image/png;base64,iVBOR..."）
func decodeBase64Data(s string) ([]byte, error) {
	// 如果是 data URL 格式，提取 base64 部分
	if idx := strings.Index(s, ";base64,"); idx >= 0 {
		s = s[idx+8:] // 跳过 ";base64,"
	} else if idx := strings.Index(s, "base64,"); idx >= 0 {
		s = s[idx+7:] // 跳过 "base64,"
	}
	return base64.StdEncoding.DecodeString(s)
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

// GetModels 获取已配置的模型提供商名称列表
func (a *App) GetModels() []string {
	return a.gateway.ListProviders()
}

// GetModelProviders 获取完整的模型提供商配置列表
func (a *App) GetModelProviders() []*model_gateway.ModelProvider {
	return a.gateway.ListModelProviders()
}

// CreateModelProvider 创建新的模型提供商配置
func (a *App) CreateModelProvider(name, provider, apiKey, baseUrl, defaultModel string) error {
	return a.gateway.CreateModelProvider(&model_gateway.ModelProvider{
		Name:         name,
		Provider:     provider,
		APIKey:       apiKey,
		BaseURL:      baseUrl,
		DefaultModel: defaultModel,
	})
}

// UpdateModelProvider 更新已有的模型提供商配置
func (a *App) UpdateModelProvider(id, name, provider, apiKey, baseUrl, defaultModel string) error {
	return a.gateway.UpdateModelProvider(&model_gateway.ModelProvider{
		ID:           id,
		Name:         name,
		Provider:     provider,
		APIKey:       apiKey,
		BaseURL:      baseUrl,
		DefaultModel: defaultModel,
	})
}

// DeleteModelProvider 删除模型提供商配置
func (a *App) DeleteModelProvider(id string) error {
	return a.gateway.DeleteModelProvider(id)
}

// ToggleModelProvider 切换模型提供商的启用/禁用状态
func (a *App) ToggleModelProvider(id string) error {
	return a.gateway.ToggleModelProvider(id)
}

// RefreshModelsFromProvider 调用提供商的 /v1/models API 刷新可用模型列表
func (a *App) RefreshModelsFromProvider(providerName string) ([]string, error) {
	return a.gateway.RefreshModelsFromProvider(providerName)
}

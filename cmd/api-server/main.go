// api-server 是 MMCS 的唯一 HTTP 服务入口
// 提供 REST API + SSE 流式推送，其他服务通过 gRPC 和 Asynq 通信
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/config"
	"github.com/wjames2000/mmcs/internal/api"
	"github.com/wjames2000/mmcs/internal/model_gateway"
	"github.com/wjames2000/mmcs/internal/model_gateway/provider"
	"github.com/wjames2000/mmcs/internal/orchestrator"
	"github.com/wjames2000/mmcs/internal/role"
	"github.com/wjames2000/mmcs/internal/session"
	"github.com/wjames2000/mmcs/internal/stream"
	"github.com/wjames2000/mmcs/internal/user"
	"github.com/wjames2000/mmcs/internal/workspace"
	"github.com/wjames2000/mmcs/pkg/logger"
	"github.com/wjames2000/mmcs/pkg/postgres"
	"github.com/wjames2000/mmcs/pkg/redis"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := config.Load("development")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	logCfg := logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
		File:   cfg.Log.File,
		Rotation: logger.RotationConfig{
			MaxSize:    cfg.Log.Rotation.MaxSize,
			MaxAge:     cfg.Log.Rotation.MaxAge,
			MaxBackups: cfg.Log.Rotation.MaxBackups,
			Compress:   cfg.Log.Rotation.Compress,
		},
	}
	lg, err := logger.New(logCfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	logger.SetDefault(lg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化 PostgreSQL 连接
	dbCfg := postgres.DefaultConfig(cfg.Database.DSN)
	dbCfg.MaxConns = cfg.Database.MaxOpen
	dbCfg.MinConns = cfg.Database.MaxIdle
	dbPool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化 PostgreSQL 连接池失败")
	}
	defer dbPool.Close()

	// 初始化 Redis 连接（缓存 + SSE Hub + Asynq 共用）
	redisCfg := redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	}
	rdb, err := redis.NewClient(ctx, redisCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化 Redis 客户端失败")
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Error().Err(err).Msg("关闭 Redis 客户端失败")
		}
	}()

	// ===== 依赖注入 =====

	// 1. 技能注册表
	skillRegistry := role.NewSkillRegistry()
	log.Info().Int("skills", len(skillRegistry.ListNames())).Msg("技能注册表初始化完成")

	// 2. JWT 管理器
	jwtExpiry, err := time.ParseDuration(cfg.Auth.JWTExpiry)
	if err != nil {
		log.Fatal().Err(err).Msg("解析 JWT 过期时间失败")
	}
	jwtManager := user.NewJWTManager(cfg.Auth.JWTSecret, jwtExpiry)

	// 3. 仓储层
	userRepo := user.NewRepository(dbPool.Pool)
	workspaceRepo := workspace.NewRepository(dbPool.Pool)
	roleRepo := role.NewRepository(dbPool.Pool)
	sessionRepo := session.NewRepository(dbPool.Pool)

	// 4. 服务层
	userSvc := user.NewService(userRepo, jwtManager)
	workspaceSvc := workspace.NewService(workspaceRepo)
	roleSvc := role.NewService(roleRepo, skillRegistry)

	// 5. 模型网关
	gateway := model_gateway.NewGateway(&cfg.ModelGateway)

	// 注册提供商
	gateway.RegisterProvider("openai", provider.NewOpenAIProvider)
	gateway.RegisterProvider("ollama", provider.NewOllamaProvider)

	// 初始化模型（懒加载，此处只是注册工厂）

	// 6. Graph 池 & 会话服务
	graphPool := session.NewGraphPool(cfg.Session.GraphPoolSize)
	sessionSvc := session.NewService(sessionRepo, graphPool, roleSvc)

	// 7. 编排工厂
	orchFactory := orchestrator.NewFactory(roleSvc, skillRegistry, gateway)

	// 8. SSE Hub 注册表
	hubRegistry := stream.NewHubRegistry()

	// 9. 认证中间件
	authMiddleware := user.NewAuthMiddleware(jwtManager)

	// ===== 注册路由 =====
	deps := &api.Dependencies{
		UserService:         userSvc,
		AuthMiddleware:      authMiddleware,
		WorkspaceService:    workspaceSvc,
		RoleService:         roleSvc,
		SessionService:      sessionSvc,
		OrchestratorFactory: orchFactory,
		HubRegistry:         hubRegistry,
	}
	handler := api.NewRouter(deps)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler: handler,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info().Int("port", cfg.Server.HTTPPort).Msg("API Server 启动")
		return httpServer.ListenAndServe()
	})

	g.Go(func() error {
		<-gCtx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// 优雅关闭：先停止接收新请求
		log.Info().Msg("正在关闭 HTTP 服务器...")
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("HTTP 服务器关闭异常")
		}

		// 停止所有运行中的 Graph 实例
		log.Info().Msg("正在停止所有运行中的 Graph 实例...")
		graphPool.CancelAll()

		// 清理所有 SSE Hub
		log.Info().Msg("正在清理 SSE Hub...")
		hubRegistry.Cleanup()

		return nil
	})

	// 监听退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	g.Go(func() error {
		select {
		case sig := <-sigCh:
			log.Info().Str("signal", sig.String()).Msg("收到退出信号")
			cancel()
		case <-gCtx.Done():
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Error().Err(err).Msg("服务异常退出")
	}

	log.Info().Msg("API Server 已关闭")
}

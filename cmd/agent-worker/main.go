// agent-worker 是 MMCS 的异步任务消费者
// 不提供 HTTP 服务，仅通过 Asynq 与 api-server 通信
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/mmcs/config"
	"github.com/mmcs/pkg/logger"
	"github.com/mmcs/pkg/postgres"
	"github.com/mmcs/pkg/redis"
	"github.com/rs/zerolog/log"
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

	// 初始化 Redis 连接（Asynq 后端）
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

	g, gCtx := errgroup.WithContext(ctx)

	// 启动 Asynq Worker Server
	asynqServer := rdb.AsynqServer(redis.AsynqServerConfig{
		Concurrency: cfg.Asynq.Concurrency,
		Queues:      cfg.Asynq.Queues,
	})

	mux := asynq.NewServeMux()
	// 注册任务处理器（后续 Phase 3+ 各模块实现后添加）
	// mux.HandleFunc(agent.TypeAgentTask, agentHandler.HandleAgentTask)

	g.Go(func() error {
		log.Info().
			Int("concurrency", cfg.Asynq.Concurrency).
			Interface("queues", cfg.Asynq.Queues).
			Msg("Agent Worker 启动")
		if err := asynqServer.Run(mux); err != nil {
			return fmt.Errorf("Asynq Server 运行失败: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		asynqServer.Shutdown()
		log.Info().Msg("Agent Worker 已关闭")
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
		log.Error().Err(err).Msg("Agent Worker 异常退出")
	}

	log.Info().Msg("Agent Worker 已关闭")
}

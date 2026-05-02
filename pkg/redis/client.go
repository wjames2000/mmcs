// Package redis 提供 Redis 客户端初始化和 Asynq 专用客户端
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Config Redis 客户端配置
type Config struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

// Client 封装通用 Redis 客户端
type Client struct {
	*redis.Client
	cfg Config
}

// NewClient 创建并验证 Redis 通用客户端
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// 验证连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	log.Info().
		Str("addr", cfg.Addr).
		Int("db", cfg.DB).
		Int("poolSize", cfg.PoolSize).
		Msg("Redis 客户端已建立")

	return &Client{
		Client: rdb,
		cfg:    cfg,
	}, nil
}

// Close 关闭 Redis 连接
func (c *Client) Close() error {
	if c.Client != nil {
		log.Info().Msg("正在关闭 Redis 客户端...")
		return c.Client.Close()
	}
	return nil
}

// AsynqClient 返回 Asynq 任务队列客户端
func (c *Client) AsynqClient() *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     c.cfg.Addr,
		Password: c.cfg.Password,
		DB:       c.cfg.DB,
	})
}

// AsynqServer 返回 Asynq 任务队列服务端
func (c *Client) AsynqServer(cfg AsynqServerConfig) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     c.cfg.Addr,
			Password: c.cfg.Password,
			DB:       c.cfg.DB,
		},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues:      cfg.Queues,
			LogLevel:    asynq.DebugLevel,
		},
	)
}

// AsynqInspector 返回 Asynq 任务队列监视器
func (c *Client) AsynqInspector() *asynq.Inspector {
	return asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     c.cfg.Addr,
		Password: c.cfg.Password,
		DB:       c.cfg.DB,
	})
}

// AsynqServerConfig Asynq 服务端配置
type AsynqServerConfig struct {
	Concurrency int
	Queues      map[string]int
}

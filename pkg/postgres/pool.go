// Package postgres 提供 PostgreSQL 数据库连接池的管理
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Pool 封装 pgxpool.Pool，提供健康检查和优雅关闭
type Pool struct {
	*pgxpool.Pool
	cfg Config
}

// Config 数据库连接池配置
type Config struct {
	DSN             string
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthCheckFreq time.Duration
}

// DefaultConfig 返回默认连接池配置
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxConns:        50,
		MinConns:        10,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		HealthCheckFreq: 1 * time.Minute,
	}
}

// NewPool 创建并初始化 PostgreSQL 连接池
func NewPool(ctx context.Context, cfg Config) (*Pool, error) {
	pgxCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("解析 DSN 失败: %w", err)
	}

	pgxCfg.MaxConns = int32(cfg.MaxConns)
	pgxCfg.MinConns = int32(cfg.MinConns)
	pgxCfg.MaxConnLifetime = cfg.MaxConnLifetime
	pgxCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pgxCfg.HealthCheckPeriod = cfg.HealthCheckFreq

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}

	p := &Pool{
		Pool: pool,
		cfg:  cfg,
	}

	// 健康检查：验证数据库连通性
	if err := p.HealthCheck(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("数据库健康检查失败: %w", err)
	}

	log.Info().
		Int32("maxConns", pgxCfg.MaxConns).
		Int32("minConns", pgxCfg.MinConns).
		Str("maxLifetime", pgxCfg.MaxConnLifetime.String()).
		Msg("PostgreSQL 连接池已建立")

	return p, nil
}

// HealthCheck 执行数据库 Ping 健康检查
func (p *Pool) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := p.Ping(ctx); err != nil {
		return fmt.Errorf("数据库 Ping 失败: %w", err)
	}
	return nil
}

// Stats 返回连接池统计信息
func (p *Pool) Stats() *pgxpool.Stat {
	return p.Pool.Stat()
}

// Close 优雅关闭连接池
func (p *Pool) Close() {
	if p.Pool != nil {
		log.Info().Msg("正在关闭 PostgreSQL 连接池...")
		p.Pool.Close()
		log.Info().Msg("PostgreSQL 连接池已关闭")
	}
}

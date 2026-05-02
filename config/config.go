// Package config 提供应用配置的加载与管理
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用根配置结构
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Asynq        AsynqConfig        `mapstructure:"asynq"`
	ModelGateway ModelGatewayConfig `mapstructure:"model_gateway"`
	Session      SessionConfig      `mapstructure:"session"`
	Context      ContextConfig      `mapstructure:"context"`
	Auth         AuthConfig         `mapstructure:"auth"`
	Crypto       CryptoConfig       `mapstructure:"crypto"`
	Log          LogConfig          `mapstructure:"log"`
}

// ServerConfig 服务端口配置
type ServerConfig struct {
	HTTPPort int `mapstructure:"http_port"`
	GRPCPort int `mapstructure:"grpc_port"`
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	DSN     string `mapstructure:"dsn"`
	MaxOpen int    `mapstructure:"max_open"`
	MaxIdle int    `mapstructure:"max_idle"`
}

// RedisConfig Redis 客户端配置
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// AsynqConfig 异步任务队列配置
type AsynqConfig struct {
	Concurrency int            `mapstructure:"concurrency"`
	Queues      map[string]int `mapstructure:"queues"`
}

// ModelGatewayConfig 模型网关配置
type ModelGatewayConfig struct {
	Providers map[string]ProviderConfig `mapstructure:"providers"`
	Timeout   int                       `mapstructure:"timeout"`
}

// ProviderConfig 单个模型提供商配置
type ProviderConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	BaseURL      string `mapstructure:"base_url"`
	DefaultModel string `mapstructure:"default_model"`
	APIKey       string `mapstructure:"api_key"`
}

// SessionConfig 会话管理配置
type SessionConfig struct {
	MaxRounds     int `mapstructure:"max_rounds"`
	RoundTimeout  int `mapstructure:"round_timeout"`
	GraphPoolSize int `mapstructure:"graph_pool_size"`
}

// ContextConfig 上下文管理配置
type ContextConfig struct {
	MaxTokens int    `mapstructure:"max_tokens"`
	Strategy  string `mapstructure:"strategy"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
	JWTExpiry string `mapstructure:"jwt_expiry"`
}

// CryptoConfig 加密配置
type CryptoConfig struct {
	MasterKey string `mapstructure:"master_key"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string         `mapstructure:"level"`
	Format   string         `mapstructure:"format"`
	Output   string         `mapstructure:"output"`
	File     string         `mapstructure:"file"`
	Rotation LogRotationCfg `mapstructure:"rotation"`
}

// LogRotationCfg 日志轮转配置
type LogRotationCfg struct {
	MaxSize    int  `mapstructure:"max_size"`    // 单文件最大 MB
	MaxAge     int  `mapstructure:"max_age"`     // 最长保留天数
	MaxBackups int  `mapstructure:"max_backups"` // 最多保留文件数
	Compress   bool `mapstructure:"compress"`    // 是否压缩
}

// Load 加载配置文件，依次读取：
//  1. config/config.yaml（基础配置）
//  2. config/config.{env}.yaml（环境特定覆盖）
//  3. 环境变量（最高优先级）
//
// env 参数指定当前环境：development / production / test
func Load(env string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")

	// 读取基础配置
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取基础配置文件失败: %w", err)
	}

	// 读取环境特定配置（如 config.production.yaml）
	if env != "" {
		envConfigName := fmt.Sprintf("config.%s", env)
		v.SetConfigName(envConfigName)
		if err := v.MergeInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("读取环境配置文件 %s 失败: %w", envConfigName, err)
			}
			// 环境配置文件不存在不是错误
		}
	}

	// 设置环境变量覆盖
	v.SetEnvPrefix("MMCS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 解析
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 应用默认值
	applyDefaults(&cfg)

	return &cfg, nil
}

// LoadFromFile 从指定路径加载配置文件
func LoadFromFile(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	v.SetEnvPrefix("MMCS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.HTTPPort == 0 {
		cfg.Server.HTTPPort = 8080
	}
	if cfg.Server.GRPCPort == 0 {
		cfg.Server.GRPCPort = 9090
	}
	if cfg.Database.MaxOpen == 0 {
		cfg.Database.MaxOpen = 50
	}
	if cfg.Database.MaxIdle == 0 {
		cfg.Database.MaxIdle = 10
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 100
	}
	if cfg.Asynq.Concurrency == 0 {
		cfg.Asynq.Concurrency = 10
	}
	if cfg.Session.MaxRounds == 0 {
		cfg.Session.MaxRounds = 10
	}
	if cfg.Session.RoundTimeout == 0 {
		cfg.Session.RoundTimeout = 300
	}
	if cfg.Session.GraphPoolSize == 0 {
		cfg.Session.GraphPoolSize = 100
	}
	if cfg.Context.MaxTokens == 0 {
		cfg.Context.MaxTokens = 128000
	}
	if cfg.Context.Strategy == "" {
		cfg.Context.Strategy = "truncate_summarize"
	}
	if cfg.ModelGateway.Timeout == 0 {
		cfg.ModelGateway.Timeout = 60
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Log.Output == "" {
		cfg.Log.Output = "stdout"
	}
	if cfg.Log.File == "" {
		cfg.Log.File = "logs/mmcs.log"
	}
	if cfg.Log.Rotation.MaxSize == 0 {
		cfg.Log.Rotation.MaxSize = 100
	}
	if cfg.Log.Rotation.MaxAge == 0 {
		cfg.Log.Rotation.MaxAge = 30
	}
	if cfg.Log.Rotation.MaxBackups == 0 {
		cfg.Log.Rotation.MaxBackups = 0 // 0 = 不限制
	}
	if !cfg.Log.Rotation.Compress {
		cfg.Log.Rotation.Compress = true
	}
}

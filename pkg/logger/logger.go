// Package logger 提供基于 zerolog 的结构化日志功能
// 支持按大小和日期自动轮转，最长保留 30 天，自动压缩归档
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

// CtxKey 请求 ID 上下文键类型
type CtxKey string

const (
	RequestIDKey CtxKey = "request_id"
)

// Logger 封装 zerolog.Logger，提供便捷方法
type Logger struct {
	zerolog.Logger
	level zerolog.Level
	mu    sync.RWMutex
}

// Config 日志配置
type Config struct {
	Level    string // debug / info / warn / error / fatal / panic / no / disabled
	Format   string // json / text
	Output   string // stdout / stderr / file
	File     string // output=file 时的路径
	Rotation RotationConfig
}

// RotationConfig 日志轮转配置
type RotationConfig struct {
	MaxSize    int  // 单文件最大 MB
	MaxAge     int  // 最长保留天数
	MaxBackups int  // 最多保留文件数（0=不限制）
	Compress   bool // 是否压缩
}

// New 创建 Logger 实例
func New(cfg Config) (*Logger, error) {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}

	output := resolveOutput(cfg)

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.MessageFieldName = "message"

	var logger zerolog.Logger
	switch strings.ToLower(cfg.Format) {
	case "text", "console":
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339Nano,
			NoColor:    false,
		}
		logger = zerolog.New(output).Level(level).With().Timestamp().Caller().Logger()
	default:
		logger = zerolog.New(output).Level(level).With().Timestamp().Caller().Logger()
	}

	return &Logger{Logger: logger, level: level}, nil
}

// resolveOutput 根据配置决定日志输出目标
func resolveOutput(cfg Config) io.Writer {
	switch strings.ToLower(cfg.Output) {
	case "stderr":
		return os.Stderr
	case "file":
		if cfg.File == "" {
			cfg.File = "logs/mmcs.log"
		}
		return &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    max(cfg.Rotation.MaxSize, 1),
			MaxAge:     max(cfg.Rotation.MaxAge, 1),
			MaxBackups: cfg.Rotation.MaxBackups,
			Compress:   cfg.Rotation.Compress,
			LocalTime:  true,
		}
	default:
		return os.Stdout
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// WithRequestID 从上下文中提取请求 ID 并注入到日志中
func (l *Logger) WithRequestID(ctx context.Context) *Logger {
	if ctx != nil {
		if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
			logger := l.Logger.With().Str("request_id", reqID).Logger()
			return &Logger{Logger: logger, level: l.level}
		}
	}
	return l
}

// SetLevel 动态调整日志级别
func (l *Logger) SetLevel(levelStr string) error {
	level, err := zerolog.ParseLevel(strings.ToLower(levelStr))
	if err != nil {
		return fmt.Errorf("无效日志级别 %q: %w", levelStr, err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	l.Logger = l.Logger.Level(level)
	return nil
}

// Level 返回当前日志级别
func (l *Logger) Level() zerolog.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// CtxWithRequestID 将请求 ID 注入到上下文中
func CtxWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID 从上下文中获取请求 ID
func GetRequestID(ctx context.Context) string {
	if ctx != nil {
		if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
			return reqID
		}
	}
	return ""
}

var defaultLogger *Logger

func init() {
	var err error
	defaultLogger, err = New(Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
	if err != nil {
		panic(fmt.Sprintf("初始化默认日志失败: %v", err))
	}
}

func L() *Logger {
	return defaultLogger
}

func SetDefault(l *Logger) {
	defaultLogger = l
}

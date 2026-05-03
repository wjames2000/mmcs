// Package metrics 提供 MMCS Prometheus 指标注册和暴露
// 所有指标均以 mmcs_ 前缀命名，遵循 Prometheus 命名规范
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Namespace 指标命名空间
const Namespace = "mmcs"

// 服务级别指标

// HTTPRequestDuration HTTP 请求耗时（秒）
// 标签: method, path, status
var HTTPRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP 请求耗时分布（秒）",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	},
	[]string{"method", "path", "status"},
)

// HTTPRequestTotal HTTP 请求总数
// 标签: method, path, status
var HTTPRequestTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "http_request_total",
		Help:      "HTTP 请求总数",
	},
	[]string{"method", "path", "status"},
)

// ActiveSessions 当前活跃会话数
var ActiveSessions = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "active_sessions",
		Help:      "当前正在运行中的会话数量",
	},
)

// SessionTotal 历史会话总数
var SessionTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "session_total",
		Help:      "历史会话总数",
	},
)

// Provider 相关指标

// ProviderCallCount Provider 调用次数
// 标签: provider, model, status
var ProviderCallCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "provider_call_total",
		Help:      "模型提供商调用总次数",
	},
	[]string{"provider", "model", "status"},
)

// ProviderCallDuration Provider 调用耗时（秒）
// 标签: provider, model
var ProviderCallDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "provider_call_duration_seconds",
		Help:      "模型提供商调用耗时分布（秒）",
		Buckets:   []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
	},
	[]string{"provider", "model"},
)

// ProviderCacheHit Provider 缓存命中次数
// 标签: provider
var ProviderCacheHit = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "provider_cache_hit_total",
		Help:      "Provider 缓存命中总次数",
	},
	[]string{"provider"},
)

// ProviderCacheMiss Provider 缓存未命中次数
// 标签: provider
var ProviderCacheMiss = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "provider_cache_miss_total",
		Help:      "Provider 缓存未命中总次数",
	},
	[]string{"provider"},
)

// Agent 相关指标

// AgentExecDuration Agent 执行耗时（秒）
// 标签: agent_type, status
var AgentExecDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "agent_execution_duration_seconds",
		Help:      "Agent 执行耗时分布（秒）",
		Buckets:   []float64{0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0, 120.0},
	},
	[]string{"agent_type", "status"},
)

// AgentExecTotal Agent 执行总次数
// 标签: agent_type, status
var AgentExecTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "agent_execution_total",
		Help:      "Agent 执行总次数",
	},
	[]string{"agent_type", "status"},
)

// 系统指标

// GoroutineCount 当前 goroutine 数量
// 使用 Gauge（而非 GaugeFunc），由 main.go 注册实际 collector
var GoroutineCount = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "goroutine_count",
		Help:      "当前 runtime goroutine 数量（由主进程定期更新）",
	},
)

// GraphPoolSize Graph 池大小
// 标签: state (active / capacity)
var GraphPoolSize = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "graph_pool_size",
		Help:      "Graph 实例池大小",
	},
	[]string{"state"},
)

// AuditEntryTotal 审计日志条目总数
var AuditEntryTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "audit_entry_total",
		Help:      "审计日志条目总数",
	},
)

// TaskQueueDepth Asynq 队列深度
// 标签: queue
var TaskQueueDepth = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "task_queue_depth",
		Help:      "Asynq 任务队列深度",
	},
	[]string{"queue"},
)

// Register 将所有指标注册到指定 Registry
func Register(r *prometheus.Registry) {
	r.MustRegister(
		HTTPRequestDuration,
		HTTPRequestTotal,
		ActiveSessions,
		SessionTotal,
		ProviderCallCount,
		ProviderCallDuration,
		ProviderCacheHit,
		ProviderCacheMiss,
		AgentExecDuration,
		AgentExecTotal,
		GoroutineCount,
		GraphPoolSize,
		AuditEntryTotal,
		TaskQueueDepth,
	)
}

// DefaultRegistry 创建带有默认配置的 Registry
func DefaultRegistry() *prometheus.Registry {
	r := prometheus.NewRegistry()
	Register(r)
	return r
}

// MetricsHandler 返回 /metrics 的 HTTP handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

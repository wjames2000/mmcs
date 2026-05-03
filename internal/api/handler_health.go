package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// DependencyStatus 依赖项状态
type DependencyStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "up" / "down"
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status"` // "ok" / "degraded" / "down"
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version,omitempty"`
	Uptime    string `json:"uptime,omitempty"`
}

// ReadinessResponse 就绪检查响应
type ReadinessResponse struct {
	Status       string             `json:"status"`
	Service      string             `json:"service"`
	Dependencies []DependencyStatus `json:"dependencies"`
}

// HealthChecker 健康检查依赖接口
type HealthChecker interface {
	// CheckPostgres 检查 PostgreSQL 连接
	CheckPostgres() error
	// CheckRedis 检查 Redis 连接
	CheckRedis() error
}

// HealthHandler 健康检查 HTTP handler
type HealthHandler struct {
	checker        HealthChecker
	startTime      time.Time
	serviceName    string
	serviceVersion string
}

// NewHealthHandler 创建健康检查 handler
func NewHealthHandler(checker HealthChecker) *HealthHandler {
	return &HealthHandler{
		checker:     checker,
		startTime:   time.Now(),
		serviceName: "api-server",
	}
}

// SetVersion 设置服务版本号
func (h *HealthHandler) SetVersion(version string) {
	h.serviceVersion = version
}

// Liveness 存活检查
// GET /healthz
// 返回服务基本存活状态
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "ok",
		Service:   h.serviceName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(h.startTime).Round(time.Second).String(),
	}
	if h.serviceVersion != "" {
		resp.Version = h.serviceVersion
	}

	writeJSON(w, http.StatusOK, resp)
}

// Readiness 就绪检查
// GET /healthz/ready
// 检查所有外部依赖是否可用
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	var deps []DependencyStatus
	overallStatus := "ok"

	// 检查 PostgreSQL
	pgStatus := checkDependency("postgresql", h.checker.CheckPostgres)
	deps = append(deps, pgStatus)
	if pgStatus.Status == "down" {
		overallStatus = "down"
	}

	// 检查 Redis
	redisStatus := checkDependency("redis", h.checker.CheckRedis)
	deps = append(deps, redisStatus)
	if redisStatus.Status == "down" && overallStatus != "down" {
		overallStatus = "degraded"
	}

	resp := ReadinessResponse{
		Status:       overallStatus,
		Service:      h.serviceName,
		Dependencies: deps,
	}

	statusCode := http.StatusOK
	if overallStatus == "down" {
		statusCode = http.StatusServiceUnavailable
	} else if overallStatus == "degraded" {
		statusCode = http.StatusOK // degraded 仍返回 200，但标记状态
	}

	writeJSON(w, statusCode, resp)
}

// Deps 依赖详情
// GET /healthz/deps
// 返回所有外部依赖的详细状态
func (h *HealthHandler) Deps(w http.ResponseWriter, r *http.Request) {
	var deps []DependencyStatus

	pgStatus := checkDependency("postgresql", h.checker.CheckPostgres)
	deps = append(deps, pgStatus)

	redisStatus := checkDependency("redis", h.checker.CheckRedis)
	deps = append(deps, redisStatus)

	resp := map[string]interface{}{
		"service":      h.serviceName,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"uptime":       time.Since(h.startTime).Round(time.Second).String(),
		"dependencies": deps,
	}

	overallStatus := "ok"
	for _, dep := range deps {
		if dep.Status == "down" {
			overallStatus = "down"
			break
		} else if dep.Status == "error" {
			overallStatus = "degraded"
		}
	}
	resp["status"] = overallStatus

	statusCode := http.StatusOK
	if overallStatus == "down" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, resp)
}

// NoopHealthChecker 无操作健康检查器（当没有真实依赖时使用）
type NoopHealthChecker struct{}

func (n *NoopHealthChecker) CheckPostgres() error { return nil }
func (n *NoopHealthChecker) CheckRedis() error    { return nil }

// checkDependency 执行依赖检查并返回状态
func checkDependency(name string, checkFn func() error) DependencyStatus {
	start := time.Now()
	err := checkFn()
	latency := time.Since(start)

	dep := DependencyStatus{
		Name:    name,
		Latency: latency.Round(time.Millisecond).String(),
	}

	if err != nil {
		dep.Status = "down"
		dep.Error = err.Error()
	} else {
		dep.Status = "up"
	}

	return dep
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

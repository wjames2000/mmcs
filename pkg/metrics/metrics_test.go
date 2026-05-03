package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsRegistration 验证指标注册不会 panic
func TestMetricsRegistration(t *testing.T) {
	r := prometheus.NewRegistry()

	// 正常注册
	require.NotPanics(t, func() {
		Register(r)
	}, "Register should not panic")

	// 重复注册同一 Registry 不应 panic（由于使用 MustRegister）
	// MustRegister 在重复注册时会 panic，所以要用新 Registry
	r2 := prometheus.NewRegistry()
	require.NotPanics(t, func() {
		Register(r2)
	}, "Register on new registry should not panic")
}

// TestDefaultRegistry 验证默认 Registry 创建
func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	require.NotNil(t, r)

	// 验证指标已注册
	metrics, err := r.Gather()
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

// TestMetricsHandler 验证 /metrics HTTP handler
func TestMetricsHandler(t *testing.T) {
	handler := MetricsHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestMetricNaming 验证指标命名是否符合规范 (mmcs_ 前缀)
func TestMetricNaming(t *testing.T) {
	r := DefaultRegistry()
	metricFamilies, err := r.Gather()
	require.NoError(t, err)

	for _, mf := range metricFamilies {
		name := mf.GetName()
		if len(name) < 5 || name[:5] != "mmcs_" {
			t.Errorf("Metric %s should have mmcs_ prefix", name)
		}
	}
}

// TestHTTPRequestDuration 验证 Histogram 指标可正常使用
func TestHTTPRequestDuration(t *testing.T) {
	r := prometheus.NewRegistry()
	Register(r)

	hist := HTTPRequestDuration.WithLabelValues("GET", "/test", "200")
	hist.Observe(0.1)
	hist.Observe(0.2)
	hist.Observe(0.3)

	metricFamilies, err := r.Gather()
	require.NoError(t, err)

	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "mmcs_http_request_duration_seconds" {
			found = true
			break
		}
	}
	assert.True(t, found, "mmcs_http_request_duration_seconds should be registered")
}

// TestProviderCallCount 验证 Counter 指标可正常使用
func TestProviderCallCount(t *testing.T) {
	r := prometheus.NewRegistry()
	Register(r)

	ProviderCallCount.WithLabelValues("openai", "gpt-4", "success").Inc()
	ProviderCallCount.WithLabelValues("openai", "gpt-4", "error").Inc()

	metricFamilies, err := r.Gather()
	require.NoError(t, err)

	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "mmcs_provider_call_total" {
			found = true
			break
		}
	}
	assert.True(t, found, "mmcs_provider_call_total should be registered")
}

// TestActiveSessions 验证 Gauge 指标可正常使用
func TestActiveSessions(t *testing.T) {
	r := prometheus.NewRegistry()
	Register(r)

	// 验证指标可通过 /metrics 暴露
	metricFamilies, err := r.Gather()
	require.NoError(t, err)

	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "mmcs_active_sessions" {
			found = true
			break
		}
	}
	assert.True(t, found, "mmcs_active_sessions should be registered")
}

package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter 基于 IP 的简单限流中间件
type RateLimiter struct {
	mu       sync.RWMutex
	visitors map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

// NewRateLimiter 创建限流器
// r: 每秒允许的请求数
// burst: 突发允许的请求数
func NewRateLimiter(r float64, burst int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     rate.Limit(r),
		burst:    burst,
	}
}

// getLimiter 获取或创建客户端的限流器
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.visitors[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 双重检查
	if limiter, exists = rl.visitors[ip]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.visitors[ip] = limiter
	return limiter
}

// Limit HTTP 限流中间件
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从 X-Forwarded-For 或 RemoteAddr 获取客户端 IP
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			WriteError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CleanupVisitors 定期清理不活跃的访问者记录
// 应该在后台 goroutine 中调用
func (rl *RateLimiter) CleanupVisitors(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		rl.mu.Lock()
		for ip, limiter := range rl.visitors {
			// 如果限流器最近没有使用，删除记录
			if limiter.Tokens() == float64(rl.burst) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

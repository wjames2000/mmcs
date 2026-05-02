package user

import (
	"context"
	"net/http"
	"strings"
)

// 上下文键类型
type contextKey string

const (
	// UserIDKey 用于从 context 中提取 user_id
	UserIDKey contextKey = "user_id"
	// UserEmailKey 用于从 context 中提取 user_email
	UserEmailKey contextKey = "user_email"
)

// AuthMiddleware JWT 认证中间件
// 从 Authorization: Bearer <token> 中提取并验证令牌
// 验证通过后将 user_id 和 email 注入到 request context
type AuthMiddleware struct {
	jwtManager *JWTManager
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(jwtManager *JWTManager) *AuthMiddleware {
	return &AuthMiddleware{jwtManager: jwtManager}
}

// Authenticate HTTP 中间件处理函数
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtManager.ValidateToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken 从 HTTP 请求头中提取 Bearer Token
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// UserIDFromContext 从 context 中获取 user_id
func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// UserEmailFromContext 从 context 中获取 email
func UserEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value(UserEmailKey).(string); ok {
		return email
	}
	return ""
}

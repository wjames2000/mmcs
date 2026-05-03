package user

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ===== extractBearerToken 测试 =====

func TestExtractBearerToken_Valid(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer xxx")
	token := extractBearerToken(req)
	assert.Equal(t, "xxx", token)
}

func TestExtractBearerToken_NoHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	token := extractBearerToken(req)
	assert.Equal(t, "", token)
}

func TestExtractBearerToken_WrongFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic xxx")
	token := extractBearerToken(req)
	assert.Equal(t, "", token)
}

func TestExtractBearerToken_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	token := extractBearerToken(req)
	assert.Equal(t, "", token)
}

func TestExtractBearerToken_CaseInsensitive(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "bearer xxx")
	token := extractBearerToken(req)
	assert.Equal(t, "xxx", token)
}

func TestExtractBearerToken_MixedCase(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "BEARER xxx")
	token := extractBearerToken(req)
	assert.Equal(t, "xxx", token)
}

func TestExtractBearerToken_MultipleSpaces(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer  xxx")
	token := extractBearerToken(req)
	assert.Equal(t, " xxx", token) // SplitN(" ", 2) → [Bearer, " xxx"]
}

// ===== Authenticate 测试 =====

// testAuthenticate 调用 Authenticate 中间件并返回响应 recorder 的结果
func testAuthenticate(t *testing.T, middleware *AuthMiddleware, req *http.Request) *http.Response {
	recorder := httptest.NewRecorder()
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		email := UserEmailFromContext(r.Context())
		w.Header().Set("X-User-Id", userID)
		w.Header().Set("X-User-Email", email)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(userID))
	}))
	handler.ServeHTTP(recorder, req)
	return recorder.Result()
}

func TestAuthenticate_ValidToken(t *testing.T) {
	jwtManager := NewJWTManager("test-secret", time.Hour)
	middleware := NewAuthMiddleware(jwtManager)

	token, err := jwtManager.GenerateToken("user_123", "test@example.com")
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp := testAuthenticate(t, middleware, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "user_123", resp.Header.Get("X-User-Id"))
	assert.Equal(t, "test@example.com", resp.Header.Get("X-User-Email"))
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	jwtManager := NewJWTManager("test-secret", time.Hour)
	middleware := NewAuthMiddleware(jwtManager)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")

	resp := testAuthenticate(t, middleware, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthenticate_NoToken(t *testing.T) {
	jwtManager := NewJWTManager("test-secret", time.Hour)
	middleware := NewAuthMiddleware(jwtManager)

	req := httptest.NewRequest("GET", "/", nil)

	resp := testAuthenticate(t, middleware, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthenticate_ExpiredToken(t *testing.T) {
	// 使用负过期时间，签发的令牌立即过期
	jwtManager := NewJWTManager("test-secret", -time.Hour)
	middleware := NewAuthMiddleware(jwtManager)

	token, err := jwtManager.GenerateToken("user_123", "test@example.com")
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp := testAuthenticate(t, middleware, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthenticate_WrongSignature(t *testing.T) {
	// 用一个不同密钥签署的令牌
	signer := NewJWTManager("different-secret", time.Hour)
	middleware := NewAuthMiddleware(NewJWTManager("test-secret", time.Hour))

	token, err := signer.GenerateToken("user_123", "test@example.com")
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp := testAuthenticate(t, middleware, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthenticate_WrongAuthScheme(t *testing.T) {
	jwtManager := NewJWTManager("test-secret", time.Hour)
	middleware := NewAuthMiddleware(jwtManager)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	resp := testAuthenticate(t, middleware, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

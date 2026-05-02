package user

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// mockRepository 实现 UserRepository 接口
type mockRepository struct {
	users map[string]*User
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		users: make(map[string]*User),
	}
}

func (m *mockRepository) Create(ctx context.Context, u *User) error {
	if _, exists := m.users[u.Email]; exists {
		return assert.AnError
	}
	m.users[u.Email] = u
	m.users[u.ID] = u
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, assert.AnError
	}
	return u, nil
}

func (m *mockRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, assert.AnError
	}
	return u, nil
}

func (m *mockRepository) Update(ctx context.Context, u *User) error {
	m.users[u.ID] = u
	m.users[u.Email] = u
	return nil
}

func setupTestService() (*Service, *mockRepository, *JWTManager) {
	repo := newMockRepository()
	jwtManager := NewJWTManager("test-secret-key-for-testing-only-123456", 15*time.Minute)
	svc := NewService(repo, jwtManager)
	return svc, repo, jwtManager
}

func TestRegister_Success(t *testing.T) {
	svc, _, _ := setupTestService()

	resp, err := svc.Register(context.Background(), &RegisterRequest{
		Name:     "测试用户",
		Email:    "test@example.com",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "测试用户", resp.User.Name)
	assert.Equal(t, "test@example.com", resp.User.Email)
	assert.Equal(t, "active", resp.User.Status)
	assert.NotEmpty(t, resp.User.ID)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Name:     "用户A",
		Email:    "duplicate@example.com",
		Password: "password123",
	})
	assert.NoError(t, err)

	_, err = svc.Register(context.Background(), &RegisterRequest{
		Name:     "用户B",
		Email:    "duplicate@example.com",
		Password: "password456",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已被注册")
}

func TestRegister_EmptyFields(t *testing.T) {
	svc, _, _ := setupTestService()

	tests := []struct {
		name string
		req  RegisterRequest
	}{
		{name: "空密码", req: RegisterRequest{Name: "用户", Email: "a@b.com", Password: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(context.Background(), &tt.req)
			assert.Error(t, err)
		})
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Name:     "登录用户",
		Email:    "login@example.com",
		Password: "password123",
	})
	assert.NoError(t, err)

	resp, err := svc.Login(context.Background(), &LoginRequest{
		Email:    "login@example.com",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "登录用户", resp.User.Name)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Name:     "密码测试",
		Email:    "pwd@example.com",
		Password: "correct-password",
	})
	assert.NoError(t, err)

	_, err = svc.Login(context.Background(), &LoginRequest{
		Email:    "pwd@example.com",
		Password: "wrong-password",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "邮箱或密码错误")
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Login(context.Background(), &LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "邮箱或密码错误")
}

func TestJWT_GenerateAndValidate(t *testing.T) {
	jwtManager := NewJWTManager("test-secret-for-jwt-testing-000000000000", 15*time.Minute)

	token, err := jwtManager.GenerateToken("u_test123", "test@example.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := jwtManager.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "u_test123", claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
}

func TestJWT_ExpiredToken(t *testing.T) {
	jwtManager := NewJWTManager("test-secret-expired-00000000000000000000000", -1*time.Minute)

	token, err := jwtManager.GenerateToken("u_expired", "expired@example.com")
	assert.NoError(t, err)

	_, err = jwtManager.ValidateToken(token)
	assert.Error(t, err)
}

func TestJWT_InvalidToken(t *testing.T) {
	jwtManager := NewJWTManager("test-secret-000000000000000000000000000000", 15*time.Minute)

	_, err := jwtManager.ValidateToken("invalid-token-string")
	assert.Error(t, err)
}

func TestJWT_RefreshToken(t *testing.T) {
	jwtManager := NewJWTManager("test-secret-refresh-000000000000000000000000", 15*time.Minute)

	token, err := jwtManager.GenerateToken("u_refresh", "refresh@example.com")
	assert.NoError(t, err)

	// 短暂等待确保时间戳不同
	time.Sleep(10 * time.Millisecond)

	newToken, err := jwtManager.RefreshToken(token)
	assert.NoError(t, err)
	assert.NotEmpty(t, newToken)

	// 验证新 token 的有效性
	claims, err := jwtManager.ValidateToken(newToken)
	assert.NoError(t, err)
	assert.Equal(t, "u_refresh", claims.UserID)
	assert.Equal(t, "refresh@example.com", claims.Email)
}

func TestBcryptHash(t *testing.T) {
	password := "my-secure-password-123!@#"

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashed)

	err = bcrypt.CompareHashAndPassword(hashed, []byte(password))
	assert.NoError(t, err)

	err = bcrypt.CompareHashAndPassword(hashed, []byte("wrong-password"))
	assert.Error(t, err)
}

func TestAuthMiddleware_Basic(t *testing.T) {
	jwtManager := NewJWTManager("test-secret-middleware-0000000000000000000", 15*time.Minute)
	mw := NewAuthMiddleware(jwtManager)
	assert.NotNil(t, mw)

	token, err := jwtManager.GenerateToken("u_mw_test", "mw@example.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := jwtManager.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "u_mw_test", claims.UserID)
	assert.Equal(t, "mw@example.com", claims.Email)
}

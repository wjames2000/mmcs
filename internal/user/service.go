package user

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/pkg/util"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
}

// Service 用户服务
type Service struct {
	repo       UserRepository
	jwtManager *JWTManager
}

// NewService 创建用户服务
func NewService(repo UserRepository, jwtManager *JWTManager) *Service {
	return &Service{repo: repo, jwtManager: jwtManager}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}

// Register 用户注册
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	// 字段校验
	if req.Name == "" || req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("用户名、邮箱和密码不能为空")
	}

	// 检查邮箱是否已存在
	existing, _ := s.repo.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, fmt.Errorf("邮箱已被注册")
	}

	// bcrypt 哈希密码
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	now := time.Now()
	user := &User{
		ID:           util.NewID("u"),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hashedBytes),
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	// 签发 JWT
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("签发令牌失败: %w", err)
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Msg("用户注册成功")
	return &RegisterResponse{User: user, Token: token}, nil
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("邮箱或密码错误")
	}

	if user.Status != "active" {
		return nil, fmt.Errorf("账号已被禁用")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("邮箱或密码错误")
	}

	// 签发 JWT
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("签发令牌失败: %w", err)
	}

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Msg("用户登录成功")
	return &LoginResponse{User: user, Token: token}, nil
}

// RefreshToken 刷新令牌
func (s *Service) RefreshToken(ctx context.Context, token string) (string, error) {
	newToken, err := s.jwtManager.RefreshToken(token)
	if err != nil {
		return "", fmt.Errorf("刷新令牌失败: %w", err)
	}
	return newToken, nil
}

// GetUser 获取用户信息
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return user, nil
}

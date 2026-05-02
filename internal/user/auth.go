package user

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT Claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager JWT 令牌管理
type JWTManager struct {
	secret []byte
	expiry time.Duration
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(secret string, expiry time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		expiry: expiry,
	}
}

// GenerateToken 签发 JWT 令牌
func (m *JWTManager) GenerateToken(userID, email string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "mmcs",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("JWT 签发失败: %w", err)
	}
	return tokenString, nil
}

// ValidateToken 验证 JWT 令牌并返回 Claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名方法: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("JWT 验证失败: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("无效的 JWT 令牌")
	}

	return claims, nil
}

// RefreshToken 刷新令牌（签发新令牌）
func (m *JWTManager) RefreshToken(oldToken string) (string, error) {
	claims, err := m.ValidateToken(oldToken)
	if err != nil {
		return "", fmt.Errorf("刷新令牌失败: %w", err)
	}
	return m.GenerateToken(claims.UserID, claims.Email)
}

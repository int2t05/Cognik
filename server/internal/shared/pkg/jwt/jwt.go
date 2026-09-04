// Package jwt 提供 JWT 令牌生成、解析和刷新工具。
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 声明
type Claims struct {
	UserID      int64    `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"` // 权限码列表
	TokenType   string   `json:"token_type"`  // access 或 refresh
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成访问令牌
func GenerateAccessToken(userID int64, username string, roles []string, permissions []string, secret string, expire time.Duration) (string, error) {
	return generateToken(userID, username, roles, permissions, "access", secret, expire)
}

// GenerateRefreshToken 生成刷新令牌
func GenerateRefreshToken(userID int64, username string, roles []string, permissions []string, secret string, expire time.Duration) (string, error) {
	return generateToken(userID, username, roles, permissions, "refresh", secret, expire)
}

// ParseToken 解析并验证令牌
func ParseToken(tokenString string, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("secret cannot be empty")
	}

	// 严格限制 alg 为 HS256
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// generateToken 内部令牌生成函数。
func generateToken(userID int64, username string, roles []string, permissions []string, tokenType string, secret string, expire time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("JWT secret 不能为空")
	}

	now := time.Now()
	// jti 使用纳秒时间戳保证唯一性
	claims := &Claims{
		UserID:      userID,
		Username:    username,
		Roles:       roles,
		Permissions: permissions,
		TokenType:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cognos",
			Subject:   fmt.Sprintf("%d", userID),
			ID:        fmt.Sprintf("%d-%d", userID, now.UnixNano()),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

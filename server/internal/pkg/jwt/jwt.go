// Package jwt 提供 JWT 令牌生成、解析和刷新工具。
//
// 使用 golang-jwt/v5 库实现，支持访问令牌和刷新令牌。
// Claims 包含 UserID、Username、Roles。
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
	Permissions []string `json:"permissions"` // 从 Role.Permissions 解析，避免中间件硬编码
	TokenType   string   `json:"token_type"`  // "access" 或 "refresh"，用于区分令牌类型
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
	// 参数验证
	if secret == "" {
		return nil, errors.New("secret cannot be empty")
	}

	// 解析令牌
	// WithValidMethods 限制 alg 严格为 HS256，拒绝 HS384/HS512 及非对称算法。
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
//
// secret 为空时直接返回错误——调用方应通过 config.JWT.Secret 注入，
// main.go 在 release 模式下也已校验非空，此处为纵深防御。
func generateToken(userID int64, username string, roles []string, permissions []string, tokenType string, secret string, expire time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("JWT secret 不能为空")
	}

	now := time.Now()
	// jti 使用纳秒时间戳保证唯一性，TokenVersion 预留用于权限变更后强制旧 token 失效。
	claims := &Claims{
		UserID:      userID,
		Username:    username,
		Roles:       roles,
		Permissions: permissions,
		TokenType:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "opsmind",
			Subject:   fmt.Sprintf("%d", userID),
			ID:        fmt.Sprintf("%d-%d", userID, now.UnixNano()),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

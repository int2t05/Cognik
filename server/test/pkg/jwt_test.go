// Package pkg_test 测试公共工具包的导出 API。
//
// 本文件测试 JWT 生成、解析和刷新工具。
package pkg_test

import (
	"testing"
	"time"

	"opsmind/pkg/jwt"
)

const testSecret = "test-secret-key-for-unit-testing"

// TestGenerateAccessToken 测试访问令牌生成
func TestGenerateAccessToken(t *testing.T) {
	token, err := jwt.GenerateAccessToken(1, "admin", []string{"admin"}, nil, testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken 失败: %v", err)
	}

	// 令牌不应为空
	if token == "" {
		t.Error("令牌不应为空")
	}
}

// TestParseToken 测试令牌解析
func TestParseToken(t *testing.T) {
	// 生成令牌
	token, err := jwt.GenerateAccessToken(1, "admin", []string{"admin"}, nil, testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken 失败: %v", err)
	}

	// 解析令牌
	claims, err := jwt.ParseToken(token, testSecret)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}

	// 验证 Claims 内容
	if claims.UserID != 1 {
		t.Errorf("期望 UserID=1，实际 %d", claims.UserID)
	}
	if claims.Username != "admin" {
		t.Errorf("期望 Username=\"admin\"，实际 %q", claims.Username)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "admin" {
		t.Errorf("期望 Roles=[admin]，实际 %v", claims.Roles)
	}
}

// TestParseTokenExpired 测试过期令牌解析
func TestParseTokenExpired(t *testing.T) {
	// 生成一个过期的令牌（过期时间为负数）
	token, err := jwt.GenerateAccessToken(1, "admin", []string{"admin"}, nil, testSecret, -1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateAccessToken 失败: %v", err)
	}

	// 解析过期令牌应该失败
	_, err = jwt.ParseToken(token, testSecret)
	if err == nil {
		t.Error("解析过期令牌应该返回错误")
	}
}

// TestParseTokenInvalidSecret 测试使用错误密钥解析
func TestParseTokenInvalidSecret(t *testing.T) {
	// 生成令牌
	token, err := jwt.GenerateAccessToken(1, "admin", []string{"admin"}, nil, testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken 失败: %v", err)
	}

	// 使用错误密钥解析应该失败
	_, err = jwt.ParseToken(token, "wrong-secret")
	if err == nil {
		t.Error("使用错误密钥解析应该返回错误")
	}
}

// TestGenerateRefreshToken 测试刷新令牌生成
func TestGenerateRefreshToken(t *testing.T) {
	token, err := jwt.GenerateRefreshToken(1, "admin", []string{"admin"}, nil, testSecret, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateRefreshToken 失败: %v", err)
	}

	// 令牌不应为空
	if token == "" {
		t.Error("刷新令牌不应为空")
	}

	// 解析刷新令牌
	claims, err := jwt.ParseToken(token, testSecret)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("期望 UserID=1，实际 %d", claims.UserID)
	}
}

// TestParseTokenInvalidFormat 测试无效格式令牌解析
func TestParseTokenInvalidFormat(t *testing.T) {
	_, err := jwt.ParseToken("invalid-token-format", testSecret)
	if err == nil {
		t.Error("解析无效格式令牌应该返回错误")
	}
}

// TestTokenType_AccessToken 验证访问令牌的 TokenType 为 "access"。
//
// 双令牌安全模型要求 access token 和 refresh token 在结构上可区分，
// 中间件通过 TokenType 字段拒绝 refresh token 用于 API 认证。
func TestTokenType_AccessToken(t *testing.T) {
	token, err := jwt.GenerateAccessToken(1, "admin", []string{"admin"}, nil, testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken 失败: %v", err)
	}

	claims, err := jwt.ParseToken(token, testSecret)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}

	if claims.TokenType != "access" {
		t.Errorf("Access Token 的 TokenType 应为 \"access\"，实际 %q", claims.TokenType)
	}
}

// TestTokenType_RefreshToken 验证刷新令牌的 TokenType 为 "refresh"。
func TestTokenType_RefreshToken(t *testing.T) {
	token, err := jwt.GenerateRefreshToken(1, "admin", []string{"admin"}, nil, testSecret, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateRefreshToken 失败: %v", err)
	}

	claims, err := jwt.ParseToken(token, testSecret)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}

	if claims.TokenType != "refresh" {
		t.Errorf("Refresh Token 的 TokenType 应为 \"refresh\"，实际 %q", claims.TokenType)
	}
}

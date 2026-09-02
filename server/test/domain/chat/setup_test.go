//go:build integration

// setup_test.go 提供 chat 测试包的共享变量与辅助函数。
package chat_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"opsmind/internal/shared/model"

	"gorm.io/gorm"
)

// bgCtx 测试用根上下文，供包内所有测试复用。
var bgCtx = context.Background()

// getEnv 读环境变量，缺省返回默认值，适配不同开发环境。
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// requireNoErr 断言无错误，有错立即终止测试。
func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
}

// itoa 将 int64 转字符串，用于 URL 拼接。
func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}

// hashToPhone 由字符串生成 11 位手机号，避免唯一索引冲突。
func hashToPhone(s string) string {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	phone := make([]byte, 11)
	phone[0] = '1'
	for i := 1; i < 11; i++ {
		h = h*31 + uint32(i)
		phone[i] = byte('0' + (h % 10))
	}
	return string(phone)
}

// createTestUser 创建测试用户并返回，供会话外键依赖。
func createTestUser(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	now := time.Now()
	u := &model.User{
		Username:     username,
		PasswordHash: "$2a$10$hash",
		RealName:     "测试用户",
		Phone:        hashToPhone(username),
		Status:       1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	return u
}

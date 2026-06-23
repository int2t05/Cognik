// Package pkg_test 测试公共工具包的导出 API。
//
// 本文件测试密码哈希和验证工具。
package pkg_test

import (
	"testing"

	"opsmind/pkg/hash"
)

// TestHashPassword 测试密码哈希生成
func TestHashPassword(t *testing.T) {
	password := "TestPass123"

	// 生成哈希
	hashed, err := hash.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}

	// 验证哈希不为空
	if hashed == "" {
		t.Error("哈希结果不应为空")
	}

	// 验证哈希不等于原密码
	if hashed == password {
		t.Error("哈希结果不应等于原密码")
	}
}

// TestCheckPassword 测试密码验证
func TestCheckPassword(t *testing.T) {
	password := "TestPass123"

	// 先生成哈希
	hashed, err := hash.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}

	// 验证正确密码
	if !hash.CheckPassword(hashed, password) {
		t.Error("CheckPassword 应该验证正确密码")
	}

	// 验证错误密码
	if hash.CheckPassword(hashed, "WrongPass123") {
		t.Error("CheckPassword 不应该验证错误密码")
	}
}

// TestValidatePassword 测试密码策略校验
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{"有效密码", "TestPass123", false},
		{"有效密码-边界长度", "Abc12345", false},
		{"有效密码-最大长度", "Abcdefghijklmnop12345678901234", false},
		{"太短", "Abc1234", true},
		{"太长", "Abcdefghijklmnop12345678901234567", true},
		{"缺少大写字母", "testpass123", true},
		{"缺少小写字母", "TESTPASS123", true},
		{"缺少数字", "TestPassWord", true},
		{"空密码", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hash.ValidatePassword(tt.pass)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr = %v", tt.pass, err, tt.wantErr)
			}
		})
	}
}

// TestHashPasswordConsistency 测试相同密码产生不同哈希（因为 salt 不同）
func TestHashPasswordConsistency(t *testing.T) {
	password := "TestPass123"

	hash1, err := hash.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 第一次失败: %v", err)
	}

	hash2, err := hash.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 第二次失败: %v", err)
	}

	// 相同密码产生不同哈希（因为 bcrypt 使用随机 salt）
	if hash1 == hash2 {
		t.Error("相同密码应该产生不同哈希（因为随机 salt）")
	}

	// 但两个哈希都应该能验证原密码
	if !hash.CheckPassword(hash1, password) {
		t.Error("第一个哈希应该能验证原密码")
	}
	if !hash.CheckPassword(hash2, password) {
		t.Error("第二个哈希应该能验证原密码")
	}
}

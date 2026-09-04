// Package hash 提供密码哈希和验证工具（bcrypt）。
package hash

import (
	"errors"
	"os"
	"strconv"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// ErrPasswordTooShort 密码太短
var ErrPasswordTooShort = errors.New("密码长度不足 8 位")

// ErrPasswordTooLong 密码太长
var ErrPasswordTooLong = errors.New("密码长度超过 32 位")

// ErrPasswordWeak 密码强度不足
var ErrPasswordWeak = errors.New("密码必须包含大写字母、小写字母和数字")

// bcryptCost 从环境变量读取成本参数，默认 10。
func bcryptCost() int {
	if v := os.Getenv("COGNOS_BCRYPT_COST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 4 && n <= 31 {
			return n
		}
	}
	return 10
}

// HashPassword 使用 bcrypt 对密码进行单向哈希。
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost())
	return string(bytes), err
}

// CheckPassword 验证密码是否匹配哈希值
func CheckPassword(hashed, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	return err == nil
}

// ValidatePassword 校验密码策略：8-32 字符，含大小写字母和数字。
func ValidatePassword(password string) error {
	// rune 计数，与用户认知一致
	if n := utf8.RuneCountInString(password); n < 8 {
		return ErrPasswordTooShort
	} else if n > 32 {
		return ErrPasswordTooLong
	}

	var hasLower, hasUpper, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
		if hasLower && hasUpper && hasDigit {
			return nil
		}
	}

	return ErrPasswordWeak
}

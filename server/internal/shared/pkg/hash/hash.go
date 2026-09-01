// Package hash 提供密码哈希和验证工具。
//
// 使用 bcrypt 算法进行密码哈希。
// 密码策略：
// - 长度 8-32 位
// - 必须包含大写字母、小写字母和数字
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

// bcryptCost 返回 bcrypt 哈希成本参数。
// 从环境变量 OPSMIND_BCRYPT_COST 读取，默认 10。
func bcryptCost() int {
	if v := os.Getenv("OPSMIND_BCRYPT_COST"); v != "" {
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

// ValidatePassword 校验密码是否符合策略要求。
//
// 策略：长度 8-32 字符、至少一个小写字母、一个大写字母、一个数字。
// 使用 utf8.RuneCountInString 计算字符数（非字节数），
// 使用 unicode.IsLower/IsUpper/IsDigit 统一检测逻辑（全 Unicode 范围）。
func ValidatePassword(password string) error {
	// 长度用 rune 计数，与用户认知一致（中/英/emoji 各算 1 个字符）
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

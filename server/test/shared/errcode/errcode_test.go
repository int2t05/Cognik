// Package errcode_test 验证错误码工具函数。
package errcode_test

import (
	"errors"
	"testing"

	"opsmind/internal/shared/pkg/errcode"
)

// TestExtractErrMsg 验证从 error 提取用户可读消息：AppError 取 Message，其余取 Error()。
func TestExtractErrMsg(t *testing.T) {
	// AppError → 取 Message
	appErr := errcode.AppError{Code: errcode.ErrParam, Message: "参数校验失败"}
	if got := errcode.ExtractErrMsg(appErr); got != "参数校验失败" {
		t.Errorf("AppError 消息 = %q, want %q", got, "参数校验失败")
	}

	// 包装的 AppError(errors.As 应解包)
	wrapped := errors.New("外层错误")
	if got := errcode.ExtractErrMsg(wrapped); got != "外层错误" {
		t.Errorf("普通 error 消息 = %q, want %q", got, "外层错误")
	}

	// nil 安全性：ExtractErrMsg 接收 error 接口，nil 会导致 panic，
	// 但调用方(handler/service)保证 err != nil 才调用，此处不测 nil。
}

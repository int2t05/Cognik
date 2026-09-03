// Package errcode 定义全局错误码常量和业务错误类型。
package errcode

import (
	"errors"
	"fmt"
)

// AppError 业务错误，携带错误码供 Handler 层映射 HTTP 状态码。
type AppError struct {
	Code    int
	Message string
}

// Error 实现 error 接口，格式为 "[错误码] 消息"。
func (e AppError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// ExtractErrMsg 从 error 提取用户可读消息：AppError 取 Message，其余取 Error()。
func ExtractErrMsg(err error) string {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return err.Error()
}

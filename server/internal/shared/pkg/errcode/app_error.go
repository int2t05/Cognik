// Package errcode 定义全局错误码常量和业务错误类型。
package errcode

import "fmt"

// AppError 业务错误，携带错误码供 Handler 层映射 HTTP 状态码。
type AppError struct {
	Code    int
	Message string
}

// Error 实现 error 接口，格式为 "[错误码] 消息"。
func (e AppError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

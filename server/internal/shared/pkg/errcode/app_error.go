// Package errcode 定义全局错误码常量和业务错误类型。
package errcode

import "fmt"

// AppError 业务错误，携带错误码供 Handler 层按不同 HTTP 状态码响应。
//
// 为什么放在 errcode 包而非 service 包：
// AppError 被所有 service 包文件使用。与错误码常量放在同一包中，调用方 import 即可同时获得
// 错误码常量和 AppError 类型。
type AppError struct {
	Code    int
	Message string
}

// Error 实现 error 接口，格式为 "[错误码] 消息"。
func (e AppError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

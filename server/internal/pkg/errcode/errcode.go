// Package errcode 定义全局错误码常量。
//
// 错误码分段约定：
// - 0: 成功
// - 1xxxx: 认证和授权
// - 2xxxx: 外部服务不可用
// - 99999: 未知错误
package errcode

// 成功
const Success = 0

// 认证和授权错误码（1xxxx）
const (
	// ErrAuth 未登录或令牌过期
	ErrAuth = 10001
	// ErrForbidden 无权限
	ErrForbidden = 10002
	// ErrParam 参数校验失败
	ErrParam = 10003
	// ErrNotFound 资源不存在
	ErrNotFound = 10004
	// ErrConflict 资源冲突（如账号名重复）
	ErrConflict = 10005
	// ErrAlreadyFrozen 用户已被冻结
	ErrAlreadyFrozen = 10006
	// ErrAlreadyActive 用户已处于正常状态
	ErrAlreadyActive = 10007
)

// 外部服务错误码（2xxxx）
const (
	// ErrAIUnavailable AI 服务不可用
	ErrAIUnavailable = 20001
	// ErrRAGUnavailable RAG 服务不可用
	ErrRAGUnavailable = 20002
	// ErrStorageUnavailable 存储服务不可用
	ErrStorageUnavailable = 20003
)

// 未知错误
const ErrUnknown = 99999

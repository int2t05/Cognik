// Package request 定义用户管理相关的请求结构体。
package request

// CreateUserRequest 创建用户请求。
//
// RoleIDs 允许为空（不分配角色），Password 由后端校验密码策略。
type CreateUserRequest struct {
	Username string  `json:"username" binding:"required"` // 用户名（唯一）
	Password string  `json:"password" binding:"required"` // 密码（需满足策略）
	RealName string  `json:"real_name" binding:"required"` // 真实姓名
	Phone    string  `json:"phone" binding:"required"`    // 手机号
	Email    string  `json:"email"`                       // 邮箱（可选）
	RoleIDs  []int64 `json:"role_ids"`                    // 分配角色 ID 列表
}

// UpdateUserRequest 更新用户请求。
//
// 仅更新基本信息，密码修改走独立接口 POST /api/v1/auth/change-password。
type UpdateUserRequest struct {
	RealName string  `json:"real_name" binding:"required"` // 真实姓名
	Phone    string  `json:"phone" binding:"required"`     // 手机号
	Email    string  `json:"email"`                        // 邮箱（可选）
	RoleIDs  []int64 `json:"role_ids"`                     // 重新分配角色 ID 列表
}

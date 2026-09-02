// Package request 定义用户管理相关的请求结构体。
package request

// CreateUserRequest 创建用户请求。
type CreateUserRequest struct {
	Username string  `json:"username" binding:"required"`  // 用户名（唯一）
	Password string  `json:"password" binding:"required"`  // 密码（需满足策略）
	RealName string  `json:"real_name" binding:"required"` // 真实姓名
	Phone    string  `json:"phone" binding:"required"`     // 手机号
	Email    string  `json:"email"`                        // 邮箱（可选）
	RoleIDs  []int64 `json:"role_ids"`                     // 分配角色 ID 列表
}

// UpdateUserRequest 更新用户请求。
type UpdateUserRequest struct {
	RealName string  `json:"real_name" binding:"required"` // 真实姓名
	Phone    string  `json:"phone" binding:"required"`     // 手机号
	Email    string  `json:"email"`                        // 邮箱（可选）
	RoleIDs  []int64 `json:"role_ids"`                     // 重新分配角色 ID 列表
}

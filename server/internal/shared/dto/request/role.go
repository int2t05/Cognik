// Package request 定义请求 DTO。
//
// role.go 提供角色管理相关的请求结构体。
package request

// CreateRoleRequest 创建角色请求。
type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleRequest 更新角色请求。
type UpdateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

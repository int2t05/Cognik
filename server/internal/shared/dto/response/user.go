// Package response 定义用户管理相关的响应结构体。
package response

// UserListResponse 用户列表响应（含分页）。
type UserListResponse struct {
	Users []UserDetailResponse `json:"users"` // 用户列表
	Total int64                `json:"total"` // 总数
}

// UserDetailResponse 用户详情响应（含角色列表）。
type UserDetailResponse struct {
	ID         int64    `json:"id"`          // 用户 ID
	Username   string   `json:"username"`    // 用户名
	RealName   string   `json:"real_name"`   // 真实姓名
	Phone      string   `json:"phone"`       // 手机号
	Email      string   `json:"email"`       // 邮箱
	Status     int16    `json:"status"`      // 状态（1=正常, 2=冻结）
	FirstLogin bool     `json:"first_login"` // 是否首次登录
	Roles      []string `json:"roles"`       // 角色名列表
	CreatedAt  string   `json:"created_at"`  // 创建时间
	UpdatedAt  string   `json:"updated_at"`  // 更新时间
}

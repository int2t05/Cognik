// Package response 定义认证相关的响应结构体。
//
// LoginResponse 包含令牌和用户信息。
package response

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`  // 访问令牌
	RefreshToken string   `json:"refresh_token"` // 刷新令牌
	User         UserInfo `json:"user"`           // 用户信息
	Roles        []string `json:"roles"`          // 角色名列表
	Permissions  []string `json:"permissions"`    // 权限列表
	Menus        []MenuItem `json:"menus"`         // 菜单树
}

// UserInfo 用户基本信息
type UserInfo struct {
	ID         int64  `json:"id"`          // 用户 ID
	Username   string `json:"username"`    // 用户名
	RealName   string `json:"real_name"`   // 真实姓名
	Phone      string `json:"phone"`       // 手机号
	Email      string `json:"email"`       // 邮箱
	FirstLogin bool   `json:"first_login"` // 是否首次登录
}

// MenuItem 菜单项
type MenuItem struct {
	ID        int64      `json:"id"`         // 菜单 ID
	Name      string     `json:"name"`       // 菜单名称
	Path      string     `json:"path"`       // 路由路径
	Icon      string     `json:"icon"`       // 图标
	ParentID  int64      `json:"parent_id"`  // 父菜单 ID
	SortOrder int        `json:"sort_order"` // 排序
	Type      string     `json:"type"`       // 菜单类型
	Children  []MenuItem `json:"children"`   // 子菜单
}

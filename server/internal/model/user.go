package model

import (
	"time"

	"gorm.io/datatypes"
)

// User 用户表
type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"type:varchar(255);not null;column:password_hash" json:"-"`
	RealName     string    `gorm:"type:varchar(64);not null;column:real_name" json:"real_name"`
	Phone        string    `gorm:"type:varchar(11);uniqueIndex;not null" json:"phone"`
	Email        string    `gorm:"type:varchar(128)" json:"email"`
	Status       int16     `gorm:"not null;default:1" json:"status"`
	FirstLogin   bool      `gorm:"not null;default:true;column:first_login" json:"first_login"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (User) TableName() string { return "users" }

// RoleNameAdmin 系统管理员角色名（唯一可识别来源，避免硬编码字符串分散）。
// 所有需要根据角色名做判断的逻辑均应引用此常量。
const RoleNameAdmin = "系统管理员"

// Role 角色表
//
// Permissions 使用 JSONB 存储权限列表，例如 ["ticket:read", "ticket:write"]。
// 选择 JSONB 而非 text[] 的原因：JSONB 可直接在 Go 中用 datatypes.JSON 序列化/反序列化，
// 无需自定义 Scanner/Valuer。
type Role struct {
	ID          int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string          `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	Description string          `gorm:"type:varchar(255)" json:"description"`
	Permissions datatypes.JSON  `gorm:"type:jsonb" json:"permissions"`
	IsSystem    bool            `gorm:"not null;default:false;column:is_system" json:"is_system"`
	CreatedAt   time.Time       `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"not null" json:"updated_at"`
}

func (Role) TableName() string { return "roles" }

// UserRole 用户-角色关联表
type UserRole struct {
	UserID int64 `gorm:"primaryKey;column:user_id" json:"user_id"`
	RoleID int64 `gorm:"primaryKey;column:role_id" json:"role_id"`
}

func (UserRole) TableName() string { return "user_roles" }

// Menu 菜单表
type Menu struct {
	ID        int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string `gorm:"type:varchar(64);not null" json:"name"`
	Path      string `gorm:"type:varchar(255)" json:"path"`
	Icon      string `gorm:"type:varchar(64)" json:"icon"`
	ParentID  int64  `gorm:"default:0;column:parent_id" json:"parent_id"`
	SortOrder int    `gorm:"default:0;column:sort_order" json:"sort_order"`
	Type      string `gorm:"type:varchar(32);not null" json:"type"`
}

func (Menu) TableName() string { return "menus" }

// RoleMenu 角色-菜单关联表
type RoleMenu struct {
	RoleID int64 `gorm:"primaryKey;column:role_id" json:"role_id"`
	MenuID int64 `gorm:"primaryKey;column:menu_id" json:"menu_id"`
}

func (RoleMenu) TableName() string { return "role_menus" }

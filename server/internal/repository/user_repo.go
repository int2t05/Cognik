// Package repository 提供用户的数据访问层。
//
// UserRepo 封装 users 表的查询和创建操作，供 UserService 调用。
// 为什么独立于 ConfigRepo：User 表的查询模式更丰富（按 ID/用户名/手机号），
// 且后续会扩展分页查询、角色关联等功能，独立 Repo 更利于演进。
package repository

import (
	"encoding/json"

	"opsmind/internal/model"

	"gorm.io/gorm"
)

// UserRepo 用户数据访问
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 创建 UserRepo 实例
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// GetByID 按 ID 查询用户。
//
// ID 是主键，查询不到时返回 gorm.ErrRecordNotFound。
func (r *UserRepo) GetByID(id int64) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername 按用户名查询用户。
//
// username 具有唯一索引，用于登录验证。
func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByIDs 按 ID 列表批量查询用户（仅返回 id + real_name，供审计等服务使用）。
func (r *UserRepo) FindByIDs(ids []int64) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []model.User
	err := r.db.Where("id IN ?", ids).Select("id, real_name").Find(&users).Error
	return users, err
}

// GetByPhone 按手机号查询用户。
//
// phone 用于报障人身份识别和注册校验。
func (r *UserRepo) GetByPhone(phone string) (*model.User, error) {
	var user model.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ExistsByPhone 检查手机号是否已注册。
//
// 为什么单独封装而非复用 GetByPhone：
// 语义更清晰（布尔返回值 vs 结构体），优化为 SELECT 1 提升性能。
func (r *UserRepo) ExistsByPhone(phone string) (bool, error) {
	// 输入验证
	if phone == "" {
		return false, nil
	}

	var id uint
	err := r.db.Model(&model.User{}).Where("phone = ?", phone).Limit(1).Pluck("id", &id).Error
	if err != nil {
		return false, err
	}
	return id != 0, nil
}

// Create 新增用户。
//
// 创建后 user.ID 会被 GORM 自动填充（autoIncrement）。
// 用户名唯一约束由数据库保证，重复时返回 PostgreSQL 错误。
func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户全部字段。
//
// 为什么用 Save 而非 Updates：Save 会更新所有字段（包括零值），
// 适用于修改密码等需要更新 password_hash、first_login、updated_at 的场景。
func (r *UserRepo) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// List 分页查询用户列表，支持关键词搜索。
func (r *UserRepo) List(page, pageSize int, keyword string) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.Model(&model.User{})
	if keyword != "" {
		query = query.Where("username LIKE ? OR real_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// UpdateStatus 更新用户状态（冻结/恢复）。
//
// 为什么单独封装而非复用 Update：仅更新 status 字段，避免意外覆盖其他字段。
func (r *UserRepo) UpdateStatus(id int64, status int) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}

// ExistsByUsername 检查用户名是否已存在。
func (r *UserRepo) ExistsByUsername(username string) (bool, error) {
	// 输入验证
	if username == "" {
		return false, nil
	}

	var id uint
	err := r.db.Model(&model.User{}).Where("username = ?", username).Limit(1).Pluck("id", &id).Error
	if err != nil {
		return false, err
	}
	return id != 0, nil
}

// --- 角色/菜单/权限关联查询 ---

// GetUserRoles 查询用户关联的角色列表。
func (r *UserRepo) GetUserRoles(userID int64) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// CountActiveAdmins 统计除指定用户外的活跃系统管理员数量。
//
// 用于冻结/降级管理员前的安全检查：确保至少还有一个活跃管理员可操作系统。
func (r *UserRepo) CountActiveAdmins(excludeUserID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.UserRole{}).
		Joins("JOIN users ON users.id = user_roles.user_id").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("roles.name = ?", "系统管理员").
		Where("users.status = 1").
		Where("users.id != ?", excludeUserID).
		Count(&count).Error
	return count, err
}

// CountUsersByRole 统计拥有指定角色的用户数。
//
// 用于角色删除前校验：若关联用户 > 0 则拒绝删除，避免产生孤儿 user_roles 记录。
func (r *UserRepo) CountUsersByRole(roleID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.UserRole{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

// AssignRoles 分配用户角色（先删后插，无内部事务，由调用方管理事务边界）。
//
// 自动过滤 roleID ≤ 0 的非法值并去重，避免重复主键和无效外键。
func (r *UserRepo) AssignRoles(userID int64, roleIDs []int64) error {
	seen := make(map[int64]bool)
	valid := make([]int64, 0, len(roleIDs))
	for _, rid := range roleIDs {
		if rid > 0 && !seen[rid] {
			seen[rid] = true
			valid = append(valid, rid)
		}
	}

	if err := r.db.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return err
	}
	for _, roleID := range valid {
		if err := r.db.Create(&model.UserRole{UserID: userID, RoleID: roleID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListMenus 查询全部菜单（按排序字段升序）。
func (r *UserRepo) ListMenus() ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Order("sort_order ASC, id ASC").Find(&menus).Error
	return menus, err
}

// GetRoleMenus 查询角色关联的菜单列表。
func (r *UserRepo) GetRoleMenus(roleID int64) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Joins("JOIN role_menus ON role_menus.menu_id = menus.id").
		Where("role_menus.role_id = ?", roleID).
		Order("menus.sort_order ASC, menus.id ASC").
		Find(&menus).Error
	return menus, err
}

// BatchGetRoleMenus 批量查询多个角色的菜单（去重）。
//
// 为什么批量查询：用户拥有 N 个角色时，逐角色查询产生 N 次 DB 往返。
// 批量查询合并为一次 JOIN，避免 N+1 问题。
func (r *UserRepo) BatchGetRoleMenus(roleIDs []int64) ([]model.Menu, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var menus []model.Menu
	err := r.db.Joins("JOIN role_menus ON role_menus.menu_id = menus.id").
		Where("role_menus.role_id IN ?", roleIDs).
		Order("menus.sort_order ASC, menus.id ASC").
		Distinct().
		Find(&menus).Error
	return menus, err
}

// UpdateRoleMenus 更新角色菜单关联（先删后批量插入，单事务保证原子性）。
func (r *UserRepo) UpdateRoleMenus(roleID int64, menuIDs []int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		menus := make([]model.RoleMenu, len(menuIDs))
		for i, mid := range menuIDs {
			menus[i] = model.RoleMenu{RoleID: roleID, MenuID: mid}
		}
		return tx.Create(&menus).Error
	})
}

// GetUserPermissions 聚合用户所有角色的权限列表（去重）。
//
// 权限从 Role.Permissions (jsonb) 字段解析，不再使用硬编码映射。
// 新增角色或修改权限无需改代码，只需更新数据库 role 记录即可生效。
func (r *UserRepo) GetUserPermissions(userID int64) ([]string, error) {
	// TODO(repository/user): 权限解析失败时当前 continue 静默跳过。
	// 角色权限 JSON 损坏属于数据完整性问题，应记录日志或返回错误，避免隐式降权难排查。
	// 从数据库 Role.Permissions (jsonb) 字段读取实际权限
	var roles []model.Role
	if err := r.db.Model(&model.Role{}).
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error; err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, role := range roles {
		var perms []string
		if len(role.Permissions) > 0 {
			if err := json.Unmarshal(role.Permissions, &perms); err != nil {
				continue // 跳过解析失败的角色权限
			}
		}
		for _, perm := range perms {
			seen[perm] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for perm := range seen {
		result = append(result, perm)
	}
	return result, nil
}


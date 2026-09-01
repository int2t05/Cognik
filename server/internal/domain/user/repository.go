// Package user 实现用户/权限领域的数据访问、业务逻辑与 HTTP Handler。
//
// repository.go 合并 users / roles / menus 三张表的数据访问层。
// UserRepo 封装用户表与用户角色关联查询；RoleRepo 封装角色表 CRUD；
// MenuRepo 独立管理 menus / role_menus 表操作——菜单是权限的载体，归入用户/权限领域。
package user

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"opsmind/internal/shared/model"

	"gorm.io/gorm"
)

// =============================================================================
// 用户数据访问
// =============================================================================

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
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername 按用户名查询用户。
//
// username 具有唯一索引，用于登录验证。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByIDs 按 ID 列表批量查询用户（仅返回 id + real_name，供审计等服务使用）。
func (r *UserRepo) FindByIDs(ctx context.Context, ids []int64) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []model.User
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Select("id, real_name").Find(&users).Error
	return users, err
}

// GetByPhone 按手机号查询用户。
//
// phone 用于报障人身份识别和注册校验。
func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ExistsByPhone 检查手机号是否已注册。
//
// 为什么单独封装而非复用 GetByPhone：
// 语义更清晰（布尔返回值 vs 结构体），优化为 SELECT 1 提升性能。
func (r *UserRepo) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	if phone == "" {
		return false, nil
	}
	var id uint
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("phone = ?", phone).Limit(1).Pluck("id", &id).Error
	if err != nil {
		return false, err
	}
	return id != 0, nil
}

// Create 新增用户。
//
// 创建后 user.ID 会被 GORM 自动填充（autoIncrement）。
// 用户名唯一约束由数据库保证，重复时返回 PostgreSQL 错误。
func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// Update 更新用户全部字段。
//
// 为什么用 Save 而非 Updates：Save 会更新所有字段（包括零值），
// 适用于修改密码等需要更新 password_hash、first_login、updated_at 的场景。
func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// BatchDelete 批量删除用户。
func (r *UserRepo) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Delete(&model.User{}, ids)
	return res.RowsAffected, res.Error
}

// List 分页查询用户列表，支持关键词搜索。
func (r *UserRepo) List(ctx context.Context, page, pageSize int, keyword string) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{})
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
func (r *UserRepo) UpdateStatus(ctx context.Context, id int64, status int) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}

// ExistsByUsername 检查用户名是否已存在。
func (r *UserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	if username == "" {
		return false, nil
	}
	var id uint
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Limit(1).Pluck("id", &id).Error
	if err != nil {
		return false, err
	}
	return id != 0, nil
}

// --- 角色关联查询 ---

// GetUserRoles 查询用户关联的角色列表。
func (r *UserRepo) GetUserRoles(ctx context.Context, userID int64) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// BatchGetUserRoles 批量查询多个用户的角色名（用于列表场景消除 N+1）。
func (r *UserRepo) BatchGetUserRoles(ctx context.Context, userIDs []int64) (map[int64][]string, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	type row struct {
		UserID   int64  `gorm:"column:user_id"`
		RoleName string `gorm:"column:role_name"`
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("user_roles").
		Select("user_roles.user_id, roles.name AS role_name").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id IN ?", userIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]string, len(userIDs))
	for _, r := range rows {
		result[r.UserID] = append(result[r.UserID], r.RoleName)
	}
	return result, nil
}

// CountActiveAdmins 统计除指定用户外的活跃系统管理员数量。
//
// 用于冻结/降级管理员前的安全检查：确保至少还有一个活跃管理员可操作系统。
func (r *UserRepo) CountActiveAdmins(ctx context.Context, excludeUserID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UserRole{}).
		Joins("JOIN users ON users.id = user_roles.user_id").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("roles.name = ?", model.RoleNameAdmin).
		Where("users.status = 1").
		Where("users.id != ?", excludeUserID).
		Count(&count).Error
	return count, err
}

// CountUsersByRole 统计拥有指定角色的用户数。
//
// 用于角色删除前校验：若关联用户 > 0 则拒绝删除，避免产生孤儿 user_roles 记录。
func (r *UserRepo) CountUsersByRole(ctx context.Context, roleID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UserRole{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

// AssignRoles 分配用户角色（先删后插，无内部事务，由调用方管理事务边界）。
//
// 自动过滤 roleID ≤ 0 的非法值并去重，避免重复主键和无效外键。
func (r *UserRepo) AssignRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	seen := make(map[int64]bool)
	valid := make([]int64, 0, len(roleIDs))
	for _, rid := range roleIDs {
		if rid > 0 && !seen[rid] {
			seen[rid] = true
			valid = append(valid, rid)
		}
	}

	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return err
	}
	for _, roleID := range valid {
		if err := r.db.WithContext(ctx).Create(&model.UserRole{UserID: userID, RoleID: roleID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetUserPermissions 聚合用户所有角色的权限列表（去重）。
//
// 权限从 Role.Permissions (jsonb) 字段解析，不再使用硬编码映射。
// 新增角色或修改权限无需改代码，只需更新数据库 role 记录即可生效。
func (r *UserRepo) GetUserPermissions(ctx context.Context, userID int64) ([]string, error) {
	var roles []model.Role
	if err := r.db.WithContext(ctx).Model(&model.Role{}).
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
				slog.Warn("角色权限 JSON 解析失败，已跳过", "role_id", role.ID, "role_name", role.Name, "error", err)
				continue
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

// =============================================================================
// 角色数据访问
// =============================================================================

// RoleRepo 角色数据访问。
type RoleRepo struct {
	db *gorm.DB
}

// NewRoleRepo 创建 RoleRepo 实例。
func NewRoleRepo(db *gorm.DB) *RoleRepo {
	return &RoleRepo{db: db}
}

// Create 创建角色。
func (r *RoleRepo) Create(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// GetByID 查询角色详情。
func (r *RoleRepo) GetByID(ctx context.Context, id int64) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// ExistsByName 检查角色名是否已存在。
//
// 用于 Service 层唯一性校验，避免绕过 Repository 直接操作 DB。
// excludeID > 0 时排除自身（用于修改场景）。
func (r *RoleRepo) ExistsByName(ctx context.Context, name string, excludeID int64) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.Role{}).Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// List 查询角色列表（分页 + 关键词搜索）。
func (r *RoleRepo) List(ctx context.Context, page, pageSize int, keyword string) ([]model.Role, int64, error) {
	var roles []model.Role
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Role{})
	if keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&roles).Error; err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}

// Update 更新角色。
func (r *RoleRepo) Update(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete 删除角色。
//
// 添加 id <= 0 守卫防止 GORM 零值陷阱（Delete(&Role{}, 0) 会删除全部角色）。
// 返回 ErrDeleteFailed 当 RowsAffected == 0（已被并发删除或 id 不存在）。
func (r *RoleRepo) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Role{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// IsBuiltinRole 判断角色是否为系统内置角色（不可删除）。
func (r *RoleRepo) IsBuiltinRole(ctx context.Context, id int64) (bool, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Where("id = ? AND is_system = ?", id, true).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// =============================================================================
// 菜单数据访问
// =============================================================================

// MenuRepo 菜单数据访问，独立管理 menus / role_menus 表操作。
type MenuRepo struct {
	db *gorm.DB
}

// NewMenuRepo 创建 MenuRepo 实例。
func NewMenuRepo(db *gorm.DB) *MenuRepo {
	return &MenuRepo{db: db}
}

// ListMenus 查询全部菜单（按排序字段）。
func (r *MenuRepo) ListMenus(ctx context.Context) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&menus).Error
	return menus, err
}

// GetRoleMenus 查询指定角色的菜单列表。
func (r *MenuRepo) GetRoleMenus(ctx context.Context, roleID int64) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).Joins("JOIN role_menus ON role_menus.menu_id = menus.id").
		Where("role_menus.role_id = ?", roleID).
		Order("menus.sort_order ASC, menus.id ASC").
		Find(&menus).Error
	return menus, err
}

// BatchGetRoleMenus 批量查询多个角色的菜单（去重）。
func (r *MenuRepo) BatchGetRoleMenus(ctx context.Context, roleIDs []int64) ([]model.Menu, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var menus []model.Menu
	err := r.db.WithContext(ctx).Joins("JOIN role_menus ON role_menus.menu_id = menus.id").
		Where("role_menus.role_id IN ?", roleIDs).
		Order("menus.sort_order ASC, menus.id ASC").
		Distinct().
		Find(&menus).Error
	return menus, err
}

// ValidateMenuIDs 校验菜单 ID 列表，返回不存在的 ID。
func (r *MenuRepo) ValidateMenuIDs(ctx context.Context, menuIDs []int64) ([]int64, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var existing []int64
	if err := r.db.WithContext(ctx).Model(&model.Menu{}).Where("id IN ?", menuIDs).Pluck("id", &existing).Error; err != nil {
		return nil, err
	}
	existingSet := make(map[int64]bool, len(existing))
	for _, id := range existing {
		existingSet[id] = true
	}
	var missing []int64
	for _, id := range menuIDs {
		if !existingSet[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// UpdateRoleMenus 全量替换角色的菜单绑定（先删后插）。
func (r *MenuRepo) UpdateRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

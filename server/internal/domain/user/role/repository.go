// Package role 角色权限数据访问。
//
// repository.go 管理 roles / menus / role_menus 表 CRUD。
package role

import (
	"context"
	"errors"

	"cognos/internal/shared/model"

	"gorm.io/gorm"
)

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

// ExistsByName 检查角色名是否已存在。excludeID > 0 时排除自身。
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

// MenuRepo 菜单数据访问。
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

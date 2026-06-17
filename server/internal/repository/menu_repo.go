// Package repository 实现菜单数据访问层。
//
// MenuRepo 从 UserRepo 中拆分出来，独立管理 menus/role_menus 表操作。
// 拆分原因：UserRepo 承担了用户、角色、菜单三种职责，违反单一职责原则。
package repository

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// MenuRepo 菜单数据访问。
type MenuRepo struct {
	db *gorm.DB
}

// NewMenuRepo 创建 MenuRepo 实例。
func NewMenuRepo(db *gorm.DB) *MenuRepo {
	return &MenuRepo{db: db}
}

// ListMenus 查询全部菜单（按排序字段升序）。
func (r *MenuRepo) ListMenus() ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Order("sort_order ASC, id ASC").Find(&menus).Error
	return menus, err
}

// GetRoleMenus 查询角色关联的菜单列表。
func (r *MenuRepo) GetRoleMenus(roleID int64) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Joins("JOIN role_menus ON role_menus.menu_id = menus.id").
		Where("role_menus.role_id = ?", roleID).
		Order("menus.sort_order ASC, menus.id ASC").
		Find(&menus).Error
	return menus, err
}

// BatchGetRoleMenus 批量查询多个角色的菜单（去重）。
func (r *MenuRepo) BatchGetRoleMenus(roleIDs []int64) ([]model.Menu, error) {
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
func (r *MenuRepo) UpdateRoleMenus(roleID int64, menuIDs []int64) error {
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

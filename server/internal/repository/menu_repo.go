// Package repository 实现菜单数据访问层。
//
// MenuRepo 独立管理 menus/role_menus 表操作。
package repository

import (
	"context"

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

func (r *MenuRepo) ListMenus(ctx context.Context) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *MenuRepo) GetRoleMenus(ctx context.Context, roleID int64) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).Joins("JOIN role_menus ON role_menus.menu_id = menus.id").
		Where("role_menus.role_id = ?", roleID).
		Order("menus.sort_order ASC, menus.id ASC").
		Find(&menus).Error
	return menus, err
}

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

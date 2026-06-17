// Package repository 实现数据访问层。
//
// role_repo.go 提供角色表的数据访问方法。
// 角色的 Permissions 字段使用 JSONB 存储，GORM 通过 datatypes.JSON 自动处理序列化。
package repository

import (
	"errors"
	"opsmind/internal/model"

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
func (r *RoleRepo) Create(role *model.Role) error {
	return r.db.Create(role).Error
}

// GetByID 根据 ID 获取角色。
func (r *RoleRepo) GetByID(id int64) (*model.Role, error) {
	var role model.Role
	err := r.db.First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// ExistsByName 检查角色名是否已存在。
//
// 用于 Service 层唯一性校验，避免绕过 Repository 直接操作 DB。
// excludeID > 0 时排除自身（用于修改场景）。
func (r *RoleRepo) ExistsByName(name string, excludeID int64) (bool, error) {
	var count int64
	query := r.db.Model(&model.Role{}).Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// List 查询角色列表（分页）。
func (r *RoleRepo) List(page, pageSize int) ([]model.Role, int64, error) {
	var roles []model.Role
	var total int64

	query := r.db.Model(&model.Role{})
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
func (r *RoleRepo) Update(role *model.Role) error {
	return r.db.Save(role).Error
}

// Delete 删除角色。
//
// 添加 id <= 0 守卫防止 GORM 零值陷阱（Delete(&Role{}, 0) 会删除全部角色）。
// 返回 ErrDeleteFailed 当 RowsAffected == 0（已被并发删除或 id 不存在）。
func (r *RoleRepo) Delete(id int64) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	result := r.db.Where("id = ?", id).Delete(&model.Role{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// IsBuiltinRole 判断角色是否为系统内置角色（不可删除）。
func (r *RoleRepo) IsBuiltinRole(id int64) (bool, error) {
	var role model.Role
	err := r.db.Where("id = ? AND is_system = ?", id, true).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

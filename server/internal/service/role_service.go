// Package service 实现角色权限管理业务逻辑。
//
// RoleService 提供角色 CRUD 功能。
// 角色的 Permissions 使用 JSONB 存储权限列表，序列化/反序列化由 GORM datatypes.JSON 自动处理。
package service

import (
	"errors"
	"encoding/json"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RoleService 角色管理服务。
type RoleService struct {
	repo     *repository.RoleRepo
	menuRepo *repository.MenuRepo
	db       *gorm.DB
}

// NewRoleService 创建 RoleService 实例。
func NewRoleService(repo *repository.RoleRepo, menuRepo *repository.MenuRepo, db *gorm.DB) *RoleService {
	return &RoleService{repo: repo, menuRepo: menuRepo, db: db}
}

// validPermissions 权限白名单。
//
// 仅允许写入已定义的权限标识，防止拼写错误导致权限静默失效。
var validPermissions = map[string]bool{
	"user:manage":      true,
	"ticket:read":      true,
	"ticket:write":     true,
	"ticket:manage":    true,
	"knowledge:create": true,
	"knowledge:manage": true,
	"audit:read":       true,
	"dashboard:read":   true,
	"system:config":    true,
}

// validatePermissions 校验权限列表是否全部在白名单中。
func validatePermissions(perms []string) error {
	for _, p := range perms {
		if !validPermissions[p] {
			return AppError{Code: errcode.ErrParam, Message: "无效的权限标识: " + p}
		}
	}
	return nil
}

// Create 创建角色。
//
// 校验角色名唯一性，重复返回 10005。
func (s *RoleService) Create(name, description string, permissions []string) error {
	// 校验权限白名单
	if err := validatePermissions(permissions); err != nil {
		return err
	}

	// 校验角色名唯一（通过 Repository 层，保证三层架构一致）
	exists, err := s.repo.ExistsByName(name, 0)
	if err != nil {
		return err
	}
	if exists {
		return AppError{Code: errcode.ErrConflict, Message: "角色名已存在"}
	}

	permsJSON, err := json.Marshal(permissions)
	if err != nil {
		return err
	}

	role := &model.Role{
		Name:        name,
		Description: description,
		Permissions: datatypes.JSON(permsJSON),
	}

	return s.repo.Create(role)
}

// GetByID 根据 ID 获取角色。
func (s *RoleService) GetByID(id int64) (*model.Role, error) {
	role, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return nil, err
	}
	return role, nil
}

// List 查询角色列表（分页 + 关键词搜索）。
func (s *RoleService) List(page, pageSize int, keyword string) ([]model.Role, int64, error) {
	return s.repo.List(page, pageSize, keyword)
}

// Update 更新角色。
//
// 校验新名称是否与其他角色冲突（排除自身），
// 与 Create 保持一致的唯一性约束。
func (s *RoleService) Update(id int64, name, description string, permissions []string) error {
	role, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return err
	}

	// 校验权限白名单
	if err := validatePermissions(permissions); err != nil {
		return err
	}

	// 校验角色名唯一（排除自身，通过 Repository 层）
	exists, err := s.repo.ExistsByName(name, id)
	if err != nil {
		return err
	}
	if exists {
		return AppError{Code: errcode.ErrConflict, Message: "角色名已存在"}
	}

	permsJSON, err := json.Marshal(permissions)
	if err != nil {
		return err
	}

	role.Name = name
	role.Description = description
	role.Permissions = datatypes.JSON(permsJSON)

	return s.repo.Update(role)
}

// Delete 删除角色。
//
// 使用事务包裹存在性检查+删除，防止 TOCTOU 竞态：
// 并发 AssignRoles 可能在 CountUsersByRole 检查通过后分配用户到此角色。
func (s *RoleService) Delete(id int64) error {
	// 禁止删除内置角色
	if isBuiltin, err := s.repo.IsBuiltinRole(id); err != nil {
		return err
	} else if isBuiltin {
		return AppError{Code: errcode.ErrForbidden, Message: "不能删除系统内置角色"}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewRoleRepo(tx)
		txUserRepo := repository.NewUserRepo(tx)

		if _, err := txRepo.GetByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
			}
			return err
		}

		count, err := txUserRepo.CountUsersByRole(id)
		if err != nil {
			return err
		}
		if count > 0 {
			return AppError{Code: errcode.ErrConflict, Message: "角色下存在关联用户，无法删除"}
		}

		return txRepo.Delete(id)
	})
}

// ListMenus 获取全部菜单列表（树形结构）。
//
// 菜单权限绑定是本模块的核心功能之一，Menu 存储在独立的 menus 表中，
// 但菜单管理归入角色模块，因为菜单是权限的载体。
func (s *RoleService) ListMenus() ([]model.Menu, error) {
	return s.menuRepo.ListMenus()
}

// GetRoleMenus 获取指定角色的菜单 ID 列表。
func (s *RoleService) GetRoleMenus(roleID int64) ([]model.Menu, error) {
	// 先确认角色存在
	if _, err := s.repo.GetByID(roleID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return nil, err
	}
	return s.menuRepo.GetRoleMenus(roleID)
}

// UpdateRoleMenus 更新角色的菜单权限绑定。
//
// 采用全量替换策略：先清空角色的所有菜单关联，再插入新关联。
// 为什么全量替换而非增量更新：前端提交的是完整菜单 ID 列表，
// 全量替换避免了前端需要追踪增删的复杂性。
func (s *RoleService) UpdateRoleMenus(roleID int64, menuIDs []int64) error {
	// 校验 menuIDs 是否全部存在
	if missing, err := s.menuRepo.ValidateMenuIDs(menuIDs); err != nil {
		return err
	} else if len(missing) > 0 {
		return AppError{Code: errcode.ErrParam, Message: "菜单 ID 不存在"}
	}

	// 先确认角色存在
	if _, err := s.repo.GetByID(roleID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return err
	}
	return s.menuRepo.UpdateRoleMenus(roleID, menuIDs)
}

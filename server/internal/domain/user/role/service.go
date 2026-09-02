// Package role 角色权限业务逻辑（角色 CRUD、菜单绑定、权限校验）。
package role

import (
	"context"
	"encoding/json"
	"errors"

	"opsmind/internal/domain/user/account"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AppError 是 errcode.AppError 的类型别名。
type AppError = errcode.AppError

// AuditWriter 审计日志写入接口（消费者接口）。
type AuditWriter interface {
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
	WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// 权限标识常量
const (
	PermUserManage      = "user:manage"
	PermTicketRead      = "ticket:read"
	PermTicketWrite     = "ticket:write"
	PermTicketManage    = "ticket:manage"
	PermKnowledgeRead   = "knowledge:read"
	PermKnowledgeWrite  = "knowledge:write"
	PermKnowledgeCreate = "knowledge:create"
	PermKnowledgeManage = "knowledge:manage"
	PermKnowledgeReview = "knowledge:review"
	PermAuditRead       = "audit:read"
	PermDashboardRead   = "dashboard:read"
	PermSystemConfig    = "system:config"
)

// validPermissions 权限白名单。
var validPermissions = map[string]bool{
	PermUserManage:      true,
	PermTicketRead:      true,
	PermTicketWrite:     true,
	PermTicketManage:    true,
	PermKnowledgeRead:   true,
	PermKnowledgeWrite:  true,
	PermKnowledgeCreate: true,
	PermKnowledgeManage: true,
	PermKnowledgeReview: true,
	PermAuditRead:       true,
	PermDashboardRead:   true,
	PermSystemConfig:    true,
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

// RoleService 角色管理服务。
type RoleService struct {
	repo        *RoleRepo
	menuRepo    *MenuRepo
	auditWriter AuditWriter
	db          *gorm.DB
}

// NewRoleService 创建 RoleService 实例。
func NewRoleService(repo *RoleRepo, menuRepo *MenuRepo, auditWriter AuditWriter, db *gorm.DB) *RoleService {
	return &RoleService{repo: repo, menuRepo: menuRepo, auditWriter: auditWriter, db: db}
}

// Create 创建角色。
func (s *RoleService) Create(ctx context.Context, name, description string, permissions []string) error {
	if err := validatePermissions(permissions); err != nil {
		return err
	}

	exists, err := s.repo.ExistsByName(ctx, name, 0)
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

	if err := s.repo.Create(ctx, role); err != nil {
		return err
	}
	s.auditWriter.Write(ctx, 0, "role.create", "role", role.ID, "")
	return nil
}

// GetByID 获取角色详情。
func (s *RoleService) GetByID(ctx context.Context, id int64) (*model.Role, error) {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return nil, err
	}
	return role, nil
}

// List 查询角色列表（分页 + 关键词搜索）。
func (s *RoleService) List(ctx context.Context, page, pageSize int, keyword string) ([]model.Role, int64, error) {
	return s.repo.List(ctx, page, pageSize, keyword)
}

// Update 更新角色。
func (s *RoleService) Update(ctx context.Context, id int64, name, description string, permissions []string) error {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return err
	}

	if err := validatePermissions(permissions); err != nil {
		return err
	}

	exists, err := s.repo.ExistsByName(ctx, name, id)
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

	if err := s.repo.Update(ctx, role); err != nil {
		return err
	}
	s.auditWriter.Write(ctx, 0, "role.update", "role", id, "")
	return nil
}

// Delete 删除角色（事务内存在性检查+删除，防止 TOCTOU 竞态）。
func (s *RoleService) Delete(ctx context.Context, id int64) error {
	if isBuiltin, err := s.repo.IsBuiltinRole(ctx, id); err != nil {
		return err
	} else if isBuiltin {
		return AppError{Code: errcode.ErrForbidden, Message: "不能删除系统内置角色"}
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := NewRoleRepo(tx)
		txUserRepo := account.NewUserRepo(tx)

		if _, err := txRepo.GetByID(ctx, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
			}
			return err
		}

		count, err := txUserRepo.CountUsersByRole(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return AppError{Code: errcode.ErrConflict, Message: "角色下存在关联用户，无法删除"}
		}

		if err := txRepo.Delete(ctx, id); err != nil {
			return err
		}
		return s.auditWriter.WriteWithTx(ctx, tx, 0, "role.delete", "role", id, "")
	})
}

// ListMenus 获取全部菜单列表。
func (s *RoleService) ListMenus(ctx context.Context) ([]model.Menu, error) {
	return s.menuRepo.ListMenus(ctx)
}

// GetRoleMenus 获取指定角色的菜单 ID 列表。
func (s *RoleService) GetRoleMenus(ctx context.Context, roleID int64) ([]model.Menu, error) {
	if _, err := s.repo.GetByID(ctx, roleID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return nil, err
	}
	return s.menuRepo.GetRoleMenus(ctx, roleID)
}

// UpdateRoleMenus 更新角色的菜单权限绑定。
func (s *RoleService) UpdateRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	if missing, err := s.menuRepo.ValidateMenuIDs(ctx, menuIDs); err != nil {
		return err
	} else if len(missing) > 0 {
		return AppError{Code: errcode.ErrParam, Message: "菜单 ID 不存在"}
	}

	if _, err := s.repo.GetByID(ctx, roleID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return err
	}
	return s.menuRepo.UpdateRoleMenus(ctx, roleID, menuIDs)
}

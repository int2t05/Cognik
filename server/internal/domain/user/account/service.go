// Package account 用户账户业务逻辑（CRUD、冻结/恢复、管理员安全检查）。
package account

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"opsmind/internal/infra/cache"
	"opsmind/internal/shared/dto/request"
	respDto "opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/hash"

	"gorm.io/gorm"
)

// AppError 是 errcode.AppError 的类型别名。
type AppError = errcode.AppError

// AuditWriter 审计日志写入接口（消费者接口）。
type AuditWriter interface {
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
	WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// UserService 用户管理服务。
type UserService struct {
	repo        *UserRepo
	auditWriter AuditWriter
	db          *gorm.DB
	userCache   *cache.UserStatusCache
}

// NewUserService 创建 UserService 实例。
func NewUserService(repo *UserRepo, auditWriter AuditWriter, db *gorm.DB, userCache *cache.UserStatusCache) *UserService {
	return &UserService{repo: repo, auditWriter: auditWriter, db: db, userCache: userCache}
}

// GetByID 根据 ID 获取用户详情（含角色列表）。
func (s *UserService) GetByID(ctx context.Context, id int64) (*respDto.UserDetailResponse, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "用户不存在"}
		}
		return nil, err
	}

	return s.toDetailResponse(ctx, user)
}

// List 查询用户列表（分页 + 关键词搜索）。
func (s *UserService) List(ctx context.Context, page, pageSize int, keyword string) (*respDto.UserListResponse, error) {
	users, total, err := s.repo.List(ctx, page, pageSize, keyword)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int64, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}
	roleNames, err := s.repo.BatchGetUserRoles(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	details := make([]respDto.UserDetailResponse, 0, len(users))
	for _, user := range users {
		names := roleNames[user.ID]
		if names == nil {
			names = []string{}
		}
		details = append(details, respDto.UserDetailResponse{
			ID:         user.ID,
			Username:   user.Username,
			RealName:   user.RealName,
			Phone:      user.Phone,
			Email:      user.Email,
			Status:     user.Status,
			FirstLogin: user.FirstLogin,
			Roles:      names,
			CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  user.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &respDto.UserListResponse{
		Users: details,
		Total: total,
	}, nil
}

// Create 创建用户。
func (s *UserService) Create(ctx context.Context, req request.CreateUserRequest) error {
	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return err
	}
	if exists {
		return AppError{Code: errcode.ErrConflict, Message: "用户名已存在"}
	}

	if err := hash.ValidatePassword(req.Password); err != nil {
		return AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	passwordHash, err := hash.HashPassword(req.Password)
	if err != nil {
		return err
	}

	if err := validateUserInput(req.Username, req.RealName, req.Phone, req.Email); err != nil {
		return err
	}

	user := &model.User{
		Username:     strings.TrimSpace(req.Username),
		PasswordHash: passwordHash,
		RealName:     strings.TrimSpace(req.RealName),
		Phone:        strings.TrimSpace(req.Phone),
		Email:        strings.TrimSpace(req.Email),
		Status:       1,
		FirstLogin:   true,
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		if len(req.RoleIDs) > 0 {
			txRepo := NewUserRepo(tx)
			if err := txRepo.AssignRoles(ctx, user.ID, req.RoleIDs); err != nil {
				return err
			}
		}

		if err := s.auditWriter.WriteWithTx(ctx, tx, 0, "user.create", "user", user.ID, ""); err != nil {
			return err
		}
		return nil
	})
}

// Update 更新用户基本信息。
func (s *UserService) Update(ctx context.Context, id int64, req request.UpdateUserRequest) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "用户不存在"}
		}
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(user).UpdateColumns(map[string]interface{}{
			"real_name": strings.TrimSpace(req.RealName),
			"phone":     strings.TrimSpace(req.Phone),
			"email":     strings.TrimSpace(req.Email),
		}).Error; err != nil {
			return err
		}

		txRepo := NewUserRepo(tx)
		if err := txRepo.AssignRoles(ctx, id, req.RoleIDs); err != nil {
			return err
		}

		if err := s.auditWriter.WriteWithTx(ctx, tx, 0, "user.update", "user", id, ""); err != nil {
			return err
		}
		return nil
	})
}

// Freeze 冻结用户。
func (s *UserService) Freeze(ctx context.Context, id int64, operatorID int64) error {
	if id == operatorID {
		return AppError{Code: errcode.ErrParam, Message: "不能冻结自己的账号"}
	}

	target, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "用户不存在"}
		}
		return err
	}

	if target.Status == model.StatusInactive {
		return AppError{Code: errcode.ErrAlreadyFrozen, Message: "用户已被冻结"}
	}

	if err := s.assertNotLastAdmin(ctx, id); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, int(model.StatusInactive)); err != nil {
		return err
	}
	s.invalidateCache(id)
	s.auditWriter.Write(ctx, operatorID, "user.freeze", "user", id, "")
	return nil
}

// Restore 恢复已冻结用户。
func (s *UserService) Restore(ctx context.Context, id int64) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "用户不存在"}
		}
		return err
	}

	if user.Status == model.StatusActive {
		return AppError{Code: errcode.ErrAlreadyActive, Message: "用户已处于正常状态"}
	}

	if err := s.repo.UpdateStatus(ctx, id, int(model.StatusActive)); err != nil {
		return err
	}
	s.invalidateCache(id)
	s.auditWriter.Write(ctx, 0, "user.restore", "user", id, "")
	return nil
}

// invalidateCache 清除用户缓存（状态变更后调用）。
func (s *UserService) invalidateCache(userID int64) {
	if s.userCache != nil {
		s.userCache.Invalidate(userID)
	}
}

// assertNotLastAdmin 检查目标用户是否为最后一个管理员。
func (s *UserService) assertNotLastAdmin(ctx context.Context, targetUserID int64) error {
	roles, err := s.repo.GetUserRoles(ctx, targetUserID)
	if err != nil {
		return err
	}
	isAdmin := false
	for _, r := range roles {
		if r.Name == model.RoleNameAdmin {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		return nil
	}

	adminCount, err := s.repo.CountActiveAdmins(ctx, targetUserID)
	if err != nil {
		return err
	}
	if adminCount == 0 {
		return AppError{Code: errcode.ErrParam, Message: "不能冻结/移除最后一个系统管理员"}
	}
	return nil
}

// validateUserInput 校验用户输入字段格式。
func validateUserInput(username, realName, phone, email string) error {
	if strings.TrimSpace(username) == "" {
		return AppError{Code: errcode.ErrParam, Message: "用户名不能为空"}
	}
	if strings.TrimSpace(realName) == "" {
		return AppError{Code: errcode.ErrParam, Message: "姓名不能为空"}
	}
	if strings.TrimSpace(phone) == "" {
		return AppError{Code: errcode.ErrParam, Message: "手机号不能为空"}
	}
	phoneRe := regexp.MustCompile(`^1[3-9]\d{9}$`)
	if !phoneRe.MatchString(strings.TrimSpace(phone)) {
		return AppError{Code: errcode.ErrParam, Message: "手机号格式不正确"}
	}
	if email != "" {
		emailRe := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
		if !emailRe.MatchString(strings.TrimSpace(email)) {
			return AppError{Code: errcode.ErrParam, Message: "邮箱格式不正确"}
		}
	}
	return nil
}

// toDetailResponse 将 User 模型转为 UserDetailResponse。
func (s *UserService) toDetailResponse(ctx context.Context, user *model.User) (*respDto.UserDetailResponse, error) {
	roles, err := s.repo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	roleNames := make([]string, len(roles))
	for i, role := range roles {
		roleNames[i] = role.Name
	}

	return &respDto.UserDetailResponse{
		ID:         user.ID,
		Username:   user.Username,
		RealName:   user.RealName,
		Phone:      user.Phone,
		Email:      user.Email,
		Status:     user.Status,
		FirstLogin: user.FirstLogin,
		Roles:      roleNames,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:  user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// BatchDelete 批量删除用户。
func (s *UserService) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	return s.repo.BatchDelete(ctx, ids)
}

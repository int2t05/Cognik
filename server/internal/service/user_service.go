// Package service 实现用户管理业务逻辑。
//
// UserService 提供用户 CRUD、冻结/恢复功能。
// 为什么冻结/恢复放在 Service 而非 Repository：
// 冻结前需校验当前状态（已冻结不能重复冻结），这类业务规则属于 Service 层职责。
package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"opsmind/internal/cache"
	"opsmind/internal/dto/request"
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"
	"opsmind/pkg/hash"

	"gorm.io/gorm"
)

// UserService 用户管理服务。
type UserService struct {
	repo        *repository.UserRepo
	auditWriter AuditWriter
	db          *gorm.DB
	userCache   *cache.UserStatusCache
}

// NewUserService 创建 UserService 实例。
// auditWriter 通过 AuditWriter 接口注入，而非直接依赖 AuditRepo——遵循"消费者接口"模式。
func NewUserService(repo *repository.UserRepo, auditWriter AuditWriter, db *gorm.DB, userCache *cache.UserStatusCache) *UserService {
	return &UserService{repo: repo, auditWriter: auditWriter, db: db, userCache: userCache}
}

// GetByID 根据 ID 获取用户详情（含角色列表）。
func (s *UserService) GetByID(ctx context.Context, id int64) (*response.UserDetailResponse, error) {
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
//
// 使用 BatchGetUserRoles 一次查询所有用户的角色名，消除 N+1 问题。
func (s *UserService) List(ctx context.Context, page, pageSize int, keyword string) (*response.UserListResponse, error) {
	users, total, err := s.repo.List(ctx, page, pageSize, keyword)
	if err != nil {
		return nil, err
	}

	// 批量查询所有用户的角色名（消除 N+1）
	userIDs := make([]int64, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}
	roleNames, err := s.repo.BatchGetUserRoles(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	details := make([]response.UserDetailResponse, 0, len(users))
	for _, user := range users {
		names := roleNames[user.ID]
		if names == nil {
			names = []string{}
		}
		details = append(details, response.UserDetailResponse{
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

	return &response.UserListResponse{
		Users: details,
		Total: total,
	}, nil
}

// Create 创建用户。
//
// 流程：校验用户名唯一 → 校验密码策略 → bcrypt 哈希 → 事务(创建用户 + 分配角色)。
// 为什么包裹在事务中：若用户创建成功但角色分配失败，事务回滚保证数据一致性。
func (s *UserService) Create(ctx context.Context, req request.CreateUserRequest) error {
	// 校验用户名唯一
	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return err
	}
	if exists {
		return AppError{Code: errcode.ErrConflict, Message: "用户名已存在"}
	}

	// 校验密码策略
	if err := hash.ValidatePassword(req.Password); err != nil {
		return AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	// bcrypt 哈希
	passwordHash, err := hash.HashPassword(req.Password)
	if err != nil {
		return err
	}

	// 输入校验与清洗
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

	// 包裹在事务中：Create + AssignRoles 原子执行。
	// AssignRoles 不再自开事务，直接使用当前 tx 保证同一事务边界。
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		if len(req.RoleIDs) > 0 {
			txRepo := repository.NewUserRepo(tx)
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
//
// 仅更新 RealName/Phone/Email 和角色分配，密码修改走独立接口。
// 使用 UpdateColumns 只写三列，避免并发 ChangePassword 的密码哈希被 Save 全字段覆盖。
// 包裹在事务中保证 Update + AssignRoles 原子性。
func (s *UserService) Update(ctx context.Context, id int64, req request.UpdateUserRequest) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "用户不存在"}
		}
		return err
	}

	// 包裹在事务中：Update + AssignRoles 原子执行
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 使用 UpdateColumns 只写目标列，避免 Save 全字段覆盖（特别是 password_hash）
		if err := tx.Model(user).UpdateColumns(map[string]interface{}{
			"real_name": strings.TrimSpace(req.RealName),
			"phone":     strings.TrimSpace(req.Phone),
			"email":     strings.TrimSpace(req.Email),
		}).Error; err != nil {
			return err
		}

		txRepo := repository.NewUserRepo(tx)
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
//
// 冻结前校验：目标存在、非自己、非最后一个系统管理员。
func (s *UserService) Freeze(ctx context.Context, id int64, operatorID int64) error {
	// 禁止冻结自己
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

	// 检查目标是否为最后一个系统管理员
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
//
// 恢复前校验：目标存在、已处于冻结状态。
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

// invalidateCache 清除指定用户的缓存条目（状态变更后调用）。
func (s *UserService) invalidateCache(userID int64) {
	if s.userCache != nil {
		s.userCache.Invalidate(userID)
	}
}

// assertNotLastAdmin 检查目标用户是否为最后一个系统管理员。
//
// 系统管理员角色 id=1（name="系统管理员"），冻结或移除该角色前必须确保至少
// 还有另一个活跃管理员可操作系统。
func (s *UserService) assertNotLastAdmin(ctx context.Context, targetUserID int64) error {
	// 检查目标是否拥有系统管理员角色
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

	// 统计其他活跃系统管理员数量
	adminCount, err := s.repo.CountActiveAdmins(ctx, targetUserID)
	if err != nil {
		return err
	}
	if adminCount == 0 {
		return AppError{Code: errcode.ErrParam, Message: "不能冻结/移除最后一个系统管理员"}
	}
	return nil
}

// validateUserInput 校验用户输入字段格式并做空白裁剪前检查。
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

// toDetailResponse 将 User 模型转换为 UserDetailResponse。
func (s *UserService) toDetailResponse(ctx context.Context, user *model.User) (*response.UserDetailResponse, error) {
	roles, err := s.repo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	roleNames := make([]string, len(roles))
	for i, role := range roles {
		roleNames[i] = role.Name
	}

	return &response.UserDetailResponse{
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

// Package user 实现用户/权限领域的业务逻辑。
//
// service.go 合并认证、角色、用户三个 Service：
//   - AuthService 处理登录、令牌刷新、密码修改、登出
//   - RoleService 处理角色 CRUD 与菜单权限绑定
//   - UserService 处理用户 CRUD 与冻结/恢复
//
// AuditWriter 通过消费者接口注入——本包只依赖接口而非具体实现，
// Go 结构化类型系统使 *service.AuditService 自动满足此接口。
package user

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"opsmind/internal/infra/cache"
	"opsmind/internal/infra/config"
	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/hash"
	"opsmind/internal/shared/pkg/jwt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AppError 是 errcode.AppError 的类型别名，供本包内使用。
type AppError = errcode.AppError

// AuditWriter 定义审计日志写入接口（消费者接口模式——本包只依赖接口而非具体实现）。
//
// 各 Service 通过此接口写入审计日志，而非直接依赖 AuditRepo。
// Go 结构化类型系统使任何实现了这两个方法的类型自动满足此接口，
// 无需显式 import service 包，避免跨领域循环依赖。
type AuditWriter interface {
	// Write 写入一条审计日志（使用默认 DB 连接）。
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
	// WriteWithTx 在事务中写入审计日志，与业务操作在同一事务中提交或回滚。
	WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// =============================================================================
// 认证服务
// =============================================================================

// loginFailRecord 记录单个用户的登录失败信息。
//
// 使用滑动窗口计数：firstAt 为窗口起始时间，count 为窗口内失败次数。
type loginFailRecord struct {
	count   int
	firstAt time.Time
}

// AuthService 认证业务逻辑。
//
// jwtCfg 在构造时注入，使得令牌有效期可通过 config 控制，
// 而非写死 2h/7d——环境变量 OPSMIND_JWT_* 调整后无需改代码。
//
// tokenBlacklist 为内存级已失效 refresh token 集合，key 为原始 token 字符串。
// 为什么用内存而非 DB：MVP 阶段单实例足够；token 到期后自动从 map 清理。
type AuthService struct {
	userRepo       *UserRepo
	menuRepo       *MenuRepo
	db             *gorm.DB
	jwtCfg         config.JWTConfig
	rateLimiter    *loginRateLimiter
	tokenBlacklist map[string]time.Time // 已失效 refresh token -> 到期时间
	blMu           sync.Mutex
	stopCh         chan struct{} // 关闭信号，用于停止 blacklistCleanupLoop
}

// loginRateLimiter 基于内存的登录失败限流器。
//
// 为什么用内存而非 Redis：MVP 阶段单实例部署足够，避免引入额外依赖。
// 限制策略：同一用户名在 window 内连续失败 maxFails 次后，后续尝试直接拒绝。
// 成功登录会清除该用户的失败记录。
type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginFailRecord
	maxFails int
	window   time.Duration
}

// NewAuthService 创建 AuthService 实例。
func NewAuthService(userRepo *UserRepo, menuRepo *MenuRepo, db *gorm.DB, jwtCfg config.JWTConfig) *AuthService {
	s := &AuthService{
		userRepo: userRepo,
		menuRepo: menuRepo,
		db:       db,
		jwtCfg:   jwtCfg,
		rateLimiter: &loginRateLimiter{
			attempts: make(map[string]*loginFailRecord),
			maxFails: 5,
			window:   15 * time.Minute,
		},
		tokenBlacklist: make(map[string]time.Time),
		stopCh:         make(chan struct{}),
	}
	go s.blacklistCleanupLoop()
	return s
}

// Shutdown 优雅关闭 AuthService。
//
// 关闭 blacklistCleanupLoop goroutine，释放 tokenBlacklist map 的引用，
// 确保服务关闭后 goroutine 不会阻止 GC 回收。
func (s *AuthService) Shutdown() {
	close(s.stopCh)
}

// blacklistCleanupLoop 每 10 分钟清理一次已到期的黑名单条目。
//
// 通过 stopCh 接收关闭信号，确保 Shutdown() 调用后 goroutine 退出。
func (s *AuthService) blacklistCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.blMu.Lock()
			now := time.Now()
			for token, exp := range s.tokenBlacklist {
				if now.After(exp) {
					delete(s.tokenBlacklist, token)
				}
			}
			s.blMu.Unlock()
		}
	}
}

// allowLogin 检查是否允许该用户名尝试登录。
//
// 返回 nil 表示允许；返回 error 表示被限流。
func (r *loginRateLimiter) allowLogin(username string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, exists := r.attempts[username]
	if !exists {
		return nil
	}

	// 窗口已过期，重置
	if time.Since(rec.firstAt) > r.window {
		delete(r.attempts, username)
		return nil
	}

	if rec.count >= r.maxFails {
		return AppError{Code: 10003, Message: "登录失败次数过多，请15分钟后再试"}
	}
	return nil
}

// recordFail 记录一次登录失败。
func (r *loginRateLimiter) recordFail(username string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, exists := r.attempts[username]
	if !exists || time.Since(rec.firstAt) > r.window {
		r.attempts[username] = &loginFailRecord{count: 1, firstAt: time.Now()}
		return
	}
	rec.count++
}

// recordSuccess 登录成功后清除失败记录。
func (r *loginRateLimiter) recordSuccess(username string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, username)
}

// Login 用户登录。
//
// 流程：查用户 → bcrypt 校验 → 检查状态 → 生成令牌 → 组装返回。
// 为什么密码错误和用户不存在返回相同错误码（10003）：
// 避免用户名枚举攻击，不暴露"用户是否存在"信息。
func (s *AuthService) Login(ctx context.Context, username, password string) (*response.LoginResponse, error) {
	// 限流检查：同一用户名在 15 分钟内最多失败 5 次
	if err := s.rateLimiter.allowLogin(username); err != nil {
		slog.Warn("登录被限流拒绝", "username", username)
		return nil, err
	}

	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.rateLimiter.recordFail(username)
			slog.Warn("登录失败：用户不存在", "username", username)
			return nil, AppError{Code: 10003, Message: "用户名或密码错误"}
		}
		return nil, AppError{Code: errcode.ErrUnknown, Message: "查询用户失败: " + err.Error()}
	}

	if !hash.CheckPassword(user.PasswordHash, password) {
		s.rateLimiter.recordFail(username)
		slog.Warn("登录失败：密码错误", "username", username)
		return nil, AppError{Code: 10003, Message: "用户名或密码错误"}
	}

	if user.Status == 2 {
		slog.Warn("登录被拒：账号已冻结", "username", username, "user_id", user.ID)
		return nil, AppError{Code: 10002, Message: "账号已被冻结"}
	}

	s.rateLimiter.recordSuccess(username)
	slog.Info("登录成功", "user_id", user.ID, "username", username)

	// 首次登录后清除 first_login 标记（异步，失败不影响登录流程）
	if user.FirstLogin {
		_ = s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Update("first_login", false).Error
		user.FirstLogin = false
	}

	return s.buildLoginResponse(ctx, user)
}

// Logout 使当前 refresh token 失效。
//
// 将 token 加入内存黑名单，阻止其被用于刷新。
// 黑名单条目在 token 到期后由后台 goroutine 自动清理。
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := jwt.ParseToken(refreshToken, s.jwtCfg.Secret)
	if err != nil {
		// token 已过期或无效——仍视为退出成功（不需要再失效）
		slog.Info("Logout：token 已无效，跳过黑名单", "error", err)
		return nil
	}

	s.blMu.Lock()
	s.tokenBlacklist[refreshToken] = claims.ExpiresAt.Time
	s.blMu.Unlock()

	slog.Info("用户已退出登录，refresh token 已失效", "user_id", claims.UserID)
	return nil
}

// RefreshToken 刷新令牌。
//
// 解析 refresh_token 后重新生成令牌对。
// 为什么不直接生成新 access_token：统一走令牌对刷新，客户端逻辑更简单。
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*response.LoginResponse, error) {
	// 检查 token 是否已被登出（黑名单）
	s.blMu.Lock()
	if _, blacklisted := s.tokenBlacklist[refreshToken]; blacklisted {
		s.blMu.Unlock()
		slog.Warn("刷新令牌已被登出失效")
		return nil, AppError{Code: 10001, Message: "刷新令牌已失效，请重新登录"}
	}
	s.blMu.Unlock()

	claims, err := jwt.ParseToken(refreshToken, s.jwtCfg.Secret)
	if err != nil {
		slog.Warn("刷新令牌无效", "error", err)
		return nil, AppError{Code: 10001, Message: "刷新令牌无效或已过期"}
	}
	if claims.TokenType != "refresh" {
		slog.Warn("令牌类型错误：用 access token 刷新", "user_id", claims.UserID)
		return nil, AppError{Code: 10001, Message: "令牌类型错误，请使用刷新令牌"}
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, AppError{Code: 10001, Message: "用户不存在"}
	}

	if user.Status == 2 {
		slog.Warn("刷新令牌被拒：账号已冻结", "user_id", user.ID, "username", user.Username)
		return nil, AppError{Code: 10002, Message: "账号已被冻结"}
	}

	slog.Info("令牌刷新成功", "user_id", user.ID)
	return s.buildLoginResponse(ctx, user)
}

// ChangePassword 修改密码。
//
// 流程：查用户 → 校验旧密码 → 校验新密码策略 → 更新哈希 → 设置 first_login=false。
// 为什么先校验旧密码再校验新密码策略：旧密码错误是更常见的场景，先返回更有用的错误信息。
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, oldPwd, newPwd string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return AppError{Code: errcode.ErrUnknown, Message: "查询用户失败: " + err.Error()}
	}

	if !hash.CheckPassword(user.PasswordHash, oldPwd) {
		slog.Warn("修改密码失败：旧密码错误", "user_id", userID)
		return AppError{Code: 10003, Message: "旧密码错误"}
	}

	if oldPwd == newPwd {
		return AppError{Code: errcode.ErrParam, Message: "新密码不能与旧密码相同"}
	}

	if err := hash.ValidatePassword(newPwd); err != nil {
		slog.Warn("修改密码失败：新密码不符合策略", "user_id", userID)
		return AppError{Code: 10003, Message: err.Error()}
	}

	newHash, err := hash.HashPassword(newPwd)
	if err != nil {
		return AppError{Code: errcode.ErrUnknown, Message: "密码哈希失败: " + err.Error()}
	}

	// 仅更新 password_hash 和 first_login，避免 Save 全字段覆盖并发的 user.Update
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password_hash": newHash,
		"first_login":   false,
	}).Error; err != nil {
		return AppError{Code: errcode.ErrUnknown, Message: "更新密码失败: " + err.Error()}
	}

	slog.Info("密码修改成功", "user_id", userID)
	return nil
}

// buildLoginResponse 根据用户信息组装登录响应。
//
// 查询用户角色、权限、菜单树，组装完整的 LoginResponse。
// 菜单树构建思路：先从全部菜单中分离一级菜单，再递归挂载子菜单。
func (s *AuthService) buildLoginResponse(ctx context.Context, user *model.User) (*response.LoginResponse, error) {
	// 查询用户角色
	roles, err := s.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "查询用户角色失败: " + err.Error()}
	}

	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}

	// 查询用户权限
	permissions, err := s.userRepo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "查询用户权限失败: " + err.Error()}
	}
	if permissions == nil {
		permissions = []string{}
	}

	// 查询用户菜单树
	menuTree, err := s.buildMenuTree(ctx, roles)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "查询用户菜单失败: " + err.Error()}
	}

	accessToken, err := jwt.GenerateAccessToken(
		user.ID, user.Username, roleNames, permissions,
		s.jwtCfg.Secret, s.jwtCfg.AccessExpire,
	)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "生成 access_token 失败: " + err.Error()}
	}

	refreshToken, err := jwt.GenerateRefreshToken(
		user.ID, user.Username, roleNames, permissions,
		s.jwtCfg.Secret, s.jwtCfg.RefreshExpire,
	)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "生成 refresh_token 失败: " + err.Error()}
	}

	return &response.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: response.UserInfo{
			ID:         user.ID,
			Username:   user.Username,
			RealName:   user.RealName,
			Phone:      user.Phone,
			Email:      user.Email,
			FirstLogin: user.FirstLogin,
		},
		Roles:       roleNames,
		Permissions: permissions,
		Menus:       menuTree,
	}, nil
}

// buildMenuTree 构建用户的菜单树。
//
// 为什么在 Service 层而非 Repository 层构建树结构：
// 树构建是展示逻辑，属于业务层的职责。Repository 只负责数据查询。
//
// 系统管理员自动获得全部菜单。
func (s *AuthService) buildMenuTree(ctx context.Context, roles []model.Role) ([]response.MenuItem, error) {
	// 判断是否为系统管理员（使用常量避免角色更名后静默失效）
	isAdmin := false
	for _, role := range roles {
		if role.Name == model.RoleNameAdmin {
			isAdmin = true
			break
		}
	}

	var menus []model.Menu
	var err error

	if isAdmin {
		// 系统管理员获取全部菜单
		menus, err = s.menuRepo.ListMenus(ctx)
	} else {
		// 其他用户：批量查询所有角色的菜单（一次 DB 查询，避免 N+1）
		roleIDSlice := make([]int64, len(roles))
		for i, role := range roles {
			roleIDSlice[i] = role.ID
		}
		allMenus, menuErr := s.menuRepo.BatchGetRoleMenus(ctx, roleIDSlice)
		if menuErr != nil {
			return nil, menuErr
		}
		menuMap := make(map[int64]model.Menu)
		for _, m := range allMenus {
			menuMap[m.ID] = m
		}
		for _, m := range menuMap {
			menus = append(menus, m)
		}
	}

	if err != nil {
		return nil, err
	}

	// 构建菜单树
	return buildTree(menus, 0), nil
}

// buildTree 递归构建菜单树。
//
// parentID=0 表示一级菜单,子菜单通过 parentID 关联。
func buildTree(menus []model.Menu, parentID int64) []response.MenuItem {
	// 按 parent_id 构建索引 map，避免每层都扫描完整 menus
	childrenMap := make(map[int64][]model.Menu)
	for _, m := range menus {
		childrenMap[m.ParentID] = append(childrenMap[m.ParentID], m)
	}

	return buildTreeWithMap(childrenMap, parentID)
}

// buildTreeWithMap 使用预构建的 map 递归构建树结构
func buildTreeWithMap(childrenMap map[int64][]model.Menu, parentID int64) []response.MenuItem {
	children := childrenMap[parentID]
	if len(children) == 0 {
		return []response.MenuItem{}
	}

	result := make([]response.MenuItem, 0, len(children))
	for _, m := range children {
		item := response.MenuItem{
			ID:        m.ID,
			Name:      m.Name,
			Path:      m.Path,
			Icon:      m.Icon,
			ParentID:  m.ParentID,
			SortOrder: m.SortOrder,
			Type:      m.Type,
			Children:  buildTreeWithMap(childrenMap, m.ID),
		}
		result = append(result, item)
	}

	// 按 sort_order 排序，保证稳定的输出顺序
	sort.Slice(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// =============================================================================
// 角色服务
// =============================================================================

// 权限标识常量——router/permissions.go 通过别名引用此处。
// 新增权限时只需在此处添加，router 自动同步。
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
//
// 仅允许写入已定义的权限标识，防止拼写错误导致权限静默失效。
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
//
// 校验角色名唯一性，重复返回 10005。
func (s *RoleService) Create(ctx context.Context, name, description string, permissions []string) error {
	// 校验权限白名单
	if err := validatePermissions(permissions); err != nil {
		return err
	}

	// 校验角色名唯一（通过 Repository 层，保证三层架构一致）
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
//
// 校验新名称是否与其他角色冲突（排除自身），
// 与 Create 保持一致的唯一性约束。
func (s *RoleService) Update(ctx context.Context, id int64, name, description string, permissions []string) error {
	role, err := s.repo.GetByID(ctx, id)
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

// Delete 删除角色。
//
// 使用事务包裹存在性检查+删除，防止 TOCTOU 竞态：
// 并发 AssignRoles 可能在 CountUsersByRole 检查通过后分配用户到此角色。
func (s *RoleService) Delete(ctx context.Context, id int64) error {
	// 禁止删除内置角色
	if isBuiltin, err := s.repo.IsBuiltinRole(ctx, id); err != nil {
		return err
	} else if isBuiltin {
		return AppError{Code: errcode.ErrForbidden, Message: "不能删除系统内置角色"}
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := NewRoleRepo(tx)
		txUserRepo := NewUserRepo(tx)

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
		// 通过 AuditWriter 接口写入审计日志，与 Create/Update 方法保持一致
		return s.auditWriter.WriteWithTx(ctx, tx, 0, "role.delete", "role", id, "")
	})
}

// ListMenus 获取全部菜单列表（树形结构）。
//
// 菜单权限绑定是本模块的核心功能之一，Menu 存储在独立的 menus 表中，
// 但菜单管理归入角色模块，因为菜单是权限的载体。
func (s *RoleService) ListMenus(ctx context.Context) ([]model.Menu, error) {
	return s.menuRepo.ListMenus(ctx)
}

// GetRoleMenus 获取指定角色的菜单 ID 列表。
func (s *RoleService) GetRoleMenus(ctx context.Context, roleID int64) ([]model.Menu, error) {
	// 先确认角色存在
	if _, err := s.repo.GetByID(ctx, roleID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return nil, err
	}
	return s.menuRepo.GetRoleMenus(ctx, roleID)
}

// UpdateRoleMenus 更新角色的菜单权限绑定。
//
// 采用全量替换策略：先清空角色的所有菜单关联，再插入新关联。
// 为什么全量替换而非增量更新：前端提交的是完整菜单 ID 列表，
// 全量替换避免了前端需要追踪增删的复杂性。
func (s *RoleService) UpdateRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	// 校验 menuIDs 是否全部存在
	if missing, err := s.menuRepo.ValidateMenuIDs(ctx, menuIDs); err != nil {
		return err
	} else if len(missing) > 0 {
		return AppError{Code: errcode.ErrParam, Message: "菜单 ID 不存在"}
	}

	// 先确认角色存在
	if _, err := s.repo.GetByID(ctx, roleID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "角色不存在"}
		}
		return err
	}
	return s.menuRepo.UpdateRoleMenus(ctx, roleID, menuIDs)
}

// =============================================================================
// 用户服务
// =============================================================================

// UserService 用户管理服务。
type UserService struct {
	repo        *UserRepo
	auditWriter AuditWriter
	db          *gorm.DB
	userCache   *cache.UserStatusCache
}

// NewUserService 创建 UserService 实例。
// auditWriter 通过 AuditWriter 接口注入，而非直接依赖 AuditRepo——遵循"消费者接口"模式。
func NewUserService(repo *UserRepo, auditWriter AuditWriter, db *gorm.DB, userCache *cache.UserStatusCache) *UserService {
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
	// AssignRoles 使用当前 tx 保证同一事务边界。
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

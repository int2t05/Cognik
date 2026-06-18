// Package service 实现认证业务逻辑。
//
// AuthService 处理登录、刷新令牌、修改密码等认证相关操作。
// 依赖 UserRepo 进行用户数据访问，依赖 pkg/jwt 生成令牌。
package service

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"
	"opsmind/pkg/hash"
	"opsmind/pkg/jwt"

	"gorm.io/gorm"
)

// AppError 是 errcode.AppError 的类型别名，供 service 包内其他文件使用。
type AppError = errcode.AppError

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
	userRepo       *repository.UserRepo
	menuRepo       *repository.MenuRepo
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
func NewAuthService(userRepo *repository.UserRepo, menuRepo *repository.MenuRepo, db *gorm.DB, jwtCfg config.JWTConfig) *AuthService {
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

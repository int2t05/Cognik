// Package auth 认证业务逻辑（登录、令牌刷新、密码修改、登出）。
package auth

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"cognik/internal/domain/user/account"
	"cognik/internal/domain/user/role"
	"cognik/internal/infra/config"
	respDto "cognik/internal/shared/dto/response"
	"cognik/internal/shared/model"
	"cognik/internal/shared/pkg/errcode"
	"cognik/internal/shared/pkg/hash"
	"cognik/internal/shared/pkg/jwt"

	"gorm.io/gorm"
)

// AppError 是 errcode.AppError 的类型别名。
type AppError = errcode.AppError

// AuthService 认证业务逻辑。
type AuthService struct {
	userRepo       *account.UserRepo
	menuRepo       *role.MenuRepo
	db             *gorm.DB
	jwtCfg         config.JWTConfig
	rateLimiter    *loginRateLimiter
	tokenBlacklist map[string]time.Time
	blMu           sync.Mutex
	stopCh         chan struct{}
}

// NewAuthService 创建 AuthService 实例。
func NewAuthService(userRepo *account.UserRepo, menuRepo *role.MenuRepo, db *gorm.DB, jwtCfg config.JWTConfig) *AuthService {
	s := &AuthService{
		userRepo:       userRepo,
		menuRepo:       menuRepo,
		db:             db,
		jwtCfg:         jwtCfg,
		rateLimiter:    newLoginRateLimiter(),
		tokenBlacklist: make(map[string]time.Time),
		stopCh:         make(chan struct{}),
	}
	go s.blacklistCleanupLoop()
	return s
}

// Shutdown 优雅关闭 AuthService。
func (s *AuthService) Shutdown() {
	close(s.stopCh)
}

// blacklistCleanupLoop 定期清理过期黑名单条目。
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

// Login 用户登录。
func (s *AuthService) Login(ctx context.Context, username, password string) (*respDto.LoginResponse, error) {
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

	if user.FirstLogin {
		_ = s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Update("first_login", false).Error
		user.FirstLogin = false
	}

	return s.buildLoginResponse(ctx, user)
}

// Logout 使当前 refresh token 失效。
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := jwt.ParseToken(refreshToken, s.jwtCfg.Secret)
	if err != nil {
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
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*respDto.LoginResponse, error) {
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

	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password_hash": newHash,
		"first_login":   false,
	}).Error; err != nil {
		return AppError{Code: errcode.ErrUnknown, Message: "更新密码失败: " + err.Error()}
	}

	slog.Info("密码修改成功", "user_id", userID)
	return nil
}

// buildLoginResponse 组装登录响应（含角色、权限、菜单树、双令牌）。
func (s *AuthService) buildLoginResponse(ctx context.Context, user *model.User) (*respDto.LoginResponse, error) {
	roles, err := s.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "查询用户角色失败: " + err.Error()}
	}

	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}

	permissions, err := s.userRepo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return nil, AppError{Code: errcode.ErrUnknown, Message: "查询用户权限失败: " + err.Error()}
	}
	if permissions == nil {
		permissions = []string{}
	}

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

	return &respDto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: respDto.UserInfo{
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

// buildMenuTree 构建用户菜单树（admin 获取全部，非 admin 按角色聚合）。
func (s *AuthService) buildMenuTree(ctx context.Context, roles []model.Role) ([]respDto.MenuItem, error) {
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
		menus, err = s.menuRepo.ListMenus(ctx)
	} else {
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

	return buildTree(menus, 0), nil
}

// buildTree 递归构建菜单树。
func buildTree(menus []model.Menu, parentID int64) []respDto.MenuItem {
	childrenMap := make(map[int64][]model.Menu)
	for _, m := range menus {
		childrenMap[m.ParentID] = append(childrenMap[m.ParentID], m)
	}

	return buildTreeWithMap(childrenMap, parentID)
}

// buildTreeWithMap 使用预构建 map 递归构建树。
func buildTreeWithMap(childrenMap map[int64][]model.Menu, parentID int64) []respDto.MenuItem {
	children := childrenMap[parentID]
	if len(children) == 0 {
		return []respDto.MenuItem{}
	}

	result := make([]respDto.MenuItem, 0, len(children))
	for _, m := range children {
		item := respDto.MenuItem{
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

	sort.Slice(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

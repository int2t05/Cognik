# Auth 数据流 — 每个 API 端点

> 涉及文件: `handler/auth.go`, `service/auth_service.go`, `repository/user_repo.go`, `repository/menu_repo.go`, `pkg/jwt/jwt.go`, `pkg/hash/hash.go`, `middleware/auth.go`, `middleware/rbac.go`, `model/user.go`

---

## POST /api/v1/auth/login

**输入** `{"username":"admin","password":"Admin@123"}`

```
1. AuthHandler.Login (handler/auth.go:33)
   └─ c.ShouldBindJSON → request.LoginRequest{Username, Password}

2. AuthService.Login (service/auth_service.go:166)
   ├─ rateLimiter.allowLogin(username) → 15min/5次限流检查
   ├─ UserRepo.GetByUsername (repository/user_repo.go:43)
   │   → SQL: SELECT * FROM users WHERE username = ?
   ├─ hash.CheckPassword (pkg/hash/hash.go:46)
   │   → bcrypt.CompareHashAndPassword
   ├─ user.Status == 2 → 冻结拒绝
   ├─ user.FirstLogin → 异步清除 first_login 标志
   └─ AuthService.buildLoginResponse → 组装返回

3. buildLoginResponse (service/auth_service.go 内部)
   ├─ UserRepo.GetUserRoles (repository/user_repo.go:151)
   │   → JOIN user_roles + roles
   ├─ UserRepo.GetUserPermissions (repository/user_repo.go:236)
   │   → 聚合 roles.permissions (JSONB) 去重
   ├─ buildMenuTree:
   │   ├─ 系统管理员 → MenuRepo.ListMenus (repository/menu_repo.go:24)
   │   └─ 其他 → MenuRepo.BatchGetRoleMenus (repository/menu_repo.go:39)
   │   └─ buildTreeWithMap 递归建树
   ├─ jwt.GenerateAccessToken (pkg/jwt/jwt.go:26) → HS256, TokenType:"access"
   └─ jwt.GenerateRefreshToken (pkg/jwt/jwt.go:31) → HS256, TokenType:"refresh"
```

**输出** `{access_token, refresh_token, user, roles, permissions, menus}`

**分支:**
- 限流拒绝 → ErrParam "登录失败次数过多，请15分钟后再试"
- 密码/用户名错误 → ErrParam "用户名或密码错误"（防枚举，同错误码）
- 账号已冻结 → ErrForbidden "账号已被冻结"

---

## POST /api/v1/auth/refresh

**输入** `{"refresh_token":"eyJ..."}`

```
1. AuthHandler.Refresh (handler/auth.go:52)
   └─ c.ShouldBindJSON → request.RefreshRequest

2. AuthService.RefreshToken (service/auth_service.go:230)
   ├─ tokenBlacklist 检查 → 已登出则拒绝
   ├─ jwt.ParseToken (pkg/jwt/jwt.go:36)
   │   → 校验签名 + 过期 + TokenType=="refresh"
   ├─ UserRepo.GetByID (repository/user_repo.go:31)
   │   → 校验用户存在且未冻结
   └─ buildLoginResponse → 重新生成令牌对 + 用户信息 + 菜单树
```

**输出** 同登录响应

---

## POST /api/v1/auth/me/change-password

**输入** `{"old_password":"Admin@123","new_password":"NewPass@456"}`

```
1. AuthHandler.ChangePassword (handler/auth.go:72)
   └─ getCurrentUserID → 从 JWT context 取 userID

2. AuthService.ChangePassword (service/auth_service.go:268)
   ├─ UserRepo.GetByID → 查用户
   ├─ hash.CheckPassword → 旧密码校验
   ├─ oldPwd == newPwd → 拒绝
   ├─ hash.ValidatePassword (pkg/hash/hash.go:56)
   │   → 正则 ^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$
   ├─ hash.HashPassword (pkg/hash/hash.go:40)
   │   → bcrypt, cost=OPSMIND_BCRYPT_COST(默认10)
   └─ db.Model(&User{}).Updates({password_hash, first_login:false})
```

**输出** `{code:0}`

---

## POST /api/v1/auth/me/logout

**输入** `{"refresh_token":"eyJ..."}`

```
1. AuthHandler.Logout (handler/auth.go:104)

2. AuthService.Logout (service/auth_service.go:210)
   ├─ jwt.ParseToken → 解析（已过期也接受）
   └─ tokenBlacklist[refreshToken] = expiresAt → 写入内存黑名单
       └─ blacklistCleanupLoop 每 10min 清理过期条目
```

**输出** `{code:0}`

---

## 中间件链

所有 `/api/v1/auth/me/*`, `/api/v1/portal/*`, `/api/v1/admin/*` 经过:

```
middleware.JWTAuth (middleware/auth.go:37)
  ├─ Authorization header → Bearer token
  ├─ jwt.ParseToken → HS256 验证 + TokenType=="access"
  ├─ UserStatusCache.GetStatus → 内存缓存(30s TTL) → 未命中回退 DB
  ├─ status == 2 → "账户已冻结"
  └─ gin.Context ← CurrentUser{UserID, Username, Roles, Permissions}

middleware.RequirePermission (middleware/rbac.go:24)
  └─ currentUser.Permissions 匹配 → 精确匹配 / 通配符前缀 / "*" all
```

## 关键组件

| 组件 | 文件 | 说明 |
|------|------|------|
| 密码 bcrypt | `pkg/hash/hash.go` | cost=10(可配), 正则 8-32 位大小写+数字 |
| JWT HS256 | `pkg/jwt/jwt.go` | Claims: UserID,Username,Roles,Permissions,TokenType |
| 登录限流 | `service/auth_service.go` | 滑动窗口 15min/5次 |
| 令牌黑名单 | `service/auth_service.go` | 内存 map, sync.Mutex, 10min 清理 |
| 用户状态缓存 | `cache/user_status.go` | 内存 TTL 30s, GetStatus/Invalidate |

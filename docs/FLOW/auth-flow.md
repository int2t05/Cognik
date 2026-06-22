# 认证数据流 — 登录 / 刷新 / 改密 / 登出

> **聚焦：后端数据逻辑。不含前端调用链和用户故事。**

---

## 1. 路由注册与中间件链

```
router.Setup()
  ├─ public:  POST /api/v1/auth/login       → AuthHandler.Login
  ├─ public:  POST /api/v1/auth/refresh     → AuthHandler.Refresh
  ├─ JWT:     POST /api/v1/auth/me/change-password → AuthHandler.ChangePassword
  └─ JWT:     POST /api/v1/auth/me/logout   → AuthHandler.Logout

JWT 中间件链 (/api/v1/auth/me/*, /api/v1/portal/*, /api/v1/admin/*):
  middleware.JWTAuth(userCache, jwtSecret)
    ├─ 1. 从 Authorization header 提取 Bearer <token>
    ├─ 2. jwt.ParseToken(token, secret) — HS256 签名验证 + 过期检查
    ├─ 3. claims.TokenType == "access"（拒绝 refresh token 访问业务接口）
    ├─ 4. userCache.GetStatus(userID) — 内存 TTL 缓存(30s)查询用户状态
    │     └─ 缓存未命中 → userRepo.GetByID(ctx, userID) 回退 DB
    ├─ 5. status == 2 → "账户已冻结" 中止
    └─ 6. 写入 gin.Context: CurrentUser{UserID, Username, Roles, Permissions} + userID
```

---

## 2. 登录 — POST /api/v1/auth/login

### 2.1 Handler: `AuthHandler.Login(c)`

```
1. c.ShouldBindJSON(&request.LoginRequest{Username, Password})
2. h.authService.Login(ctx, req.Username, req.Password)
3. response.Success(c, LoginResponse{...})
```

### 2.2 Service: `AuthService.Login(ctx, username, password) → *LoginResponse`

```
1. rateLimiter.allowLogin(username)
   └─ 滑动窗口: 15 分钟内同一 username 失败次数 < 5
   └─ 超限 → AppError{10003, "登录失败次数过多，请15分钟后再试"}

2. userRepo.GetByUsername(ctx, username)
   └─ SQL: SELECT * FROM users WHERE username = ?
   └─ 未找到 → AppError{10003, "用户名或密码错误"}（与密码错误同码，防用户名枚举）

3. hash.CheckPassword(user.PasswordHash, password)
   └─ bcrypt.CompareHashAndPassword(hashed, password)
   └─ 不匹配 → rateLimiter.recordFail(username) → AppError{10003, "用户名或密码错误"}

4. user.Status == 2（冻结态）
   └─ → AppError{10003, "账号已被冻结"}

5. rateLimiter.recordSuccess(username) — 清除失败计数

6. user.FirstLogin == true
   └─ go db.Model(&User{}).Where("id=?", id).Update("first_login", false) — 异步清除

7. buildLoginResponse(ctx, user) — 组装完整响应（见 §2.3）
```

### 2.3 Service: `AuthService.buildLoginResponse(ctx, user) → *LoginResponse`

```
1. repo.GetUserRoles(ctx, user.ID)
   └─ SQL: JOIN user_roles + roles WHERE user_roles.user_id = ?

2. repo.GetUserPermissions(ctx, user.ID)
   └─ SQL: JOIN user_roles + roles → 聚合 roles.permissions(jsonb) → 去重

3. buildMenuTree(ctx, roles)
   ├─ 系统管理员（roles 含 "系统管理员"）:
   │   └─ menuRepo.ListMenus(ctx)
   │       └─ SQL: SELECT * FROM menus ORDER BY sort_order ASC, id ASC
   │
   └─ 其他用户:
       └─ menuRepo.BatchGetRoleMenus(ctx, roleIDs)
           └─ SQL: SELECT DISTINCT m.* FROM menus m
               JOIN role_menus rm ON rm.menu_id = m.id WHERE rm.role_id IN ?
               ORDER BY sort_order ASC, id ASC
       └─ buildTreeWithMap(childrenMap, parentID=0) — 递归构建树，按 sort_order 排序

4. jwt.GenerateAccessToken(user.ID, username, roles, permissions, secret, expire)
   └─ Claims{UserID, Username, Roles, Permissions, TokenType:"access"}
   └─ jwt.SigningMethodHS256 签名

5. jwt.GenerateRefreshToken(...) — 同上，TokenType:"refresh"，expire 更长

6. 返回 *LoginResponse{AccessToken, RefreshToken, User, Roles, Permissions, Menus}
```

### 2.4 数据层调用汇总

| 步骤 | 调用 | SQL |
|------|------|-----|
| 2.2.2 | `UserRepo.GetByUsername` | `SELECT * FROM users WHERE username = ?` |
| 2.3.1 | `UserRepo.GetUserRoles` | `JOIN user_roles + roles WHERE user_id = ?` |
| 2.3.2 | `UserRepo.GetUserPermissions` | 聚合 `roles.permissions` JSONB |
| 2.3.3a | `MenuRepo.ListMenus` | `SELECT * FROM menus ORDER BY sort_order, id` |
| 2.3.3b | `MenuRepo.BatchGetRoleMenus` | `SELECT DISTINCT m.* FROM menus m JOIN role_menus ...` |

---

## 3. 令牌刷新 — POST /api/v1/auth/refresh

### 3.1 Handler: `AuthHandler.Refresh(c)`

```
1. c.ShouldBindJSON(&request.RefreshRequest{RefreshToken})
2. h.authService.RefreshToken(ctx, refreshToken)
3. response.Success(c, newLoginResponse)
```

### 3.2 Service: `AuthService.RefreshToken(ctx, refreshToken) → *LoginResponse`

```
1. tokenBlacklist 检查 — refreshToken 是否已在内存黑名单中
   └─ 在黑名单 → AppError{10001, "令牌已失效"}

2. jwt.ParseToken(refreshToken, secret)
   └─ 校验签名 + 过期 + TokenType == "refresh"

3. userRepo.GetByID(ctx, claims.UserID)
   └─ 未找到 → "用户不存在"
   └─ Status == 2 → "账号已被冻结"

4. buildLoginResponse(ctx, user) — 重新生成令牌对（同 §2.3）
```

---

## 4. 修改密码 — POST /api/v1/auth/me/change-password

### 4.1 Handler: `AuthHandler.ChangePassword(c)`

```
1. getCurrentUserID(c) → 从 JWT context 提取 userID
2. c.ShouldBindJSON(&request.ChangePasswordRequest{OldPassword, NewPassword})
3. h.authService.ChangePassword(ctx, userID, oldPwd, newPwd)
```

### 4.2 Service: `AuthService.ChangePassword(ctx, userID, oldPwd, newPwd)`

```
1. userRepo.GetByID(ctx, userID)

2. hash.CheckPassword(user.PasswordHash, oldPwd)
   └─ 不匹配 → AppError{10003, "旧密码错误"}

3. oldPwd == newPwd
   └─ → AppError{10003, "新密码不能与旧密码相同"}

4. hash.ValidatePassword(newPwd)
   └─ 正则: ^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$
   └─ 不满足 → AppError{10003, "密码需8-32位，含大小写字母和数字"}

5. hash.HashPassword(newPwd)
   └─ bcrypt.GenerateFromPassword(password, cost)
   └─ cost 从 OPSMIND_BCRYPT_COST 环境变量读取（默认 10, 范围 4-31）

6. db.Model(&User{}).Where("id=?", userID).Updates(map{
     "password_hash": hash,
     "first_login": false,
   })
   └─ 直接使用 DB 更新（单表双字段，不需要 Repository 全字段 Save）
```

---

## 5. 退出登录 — POST /api/v1/auth/me/logout

### 5.1 Handler: `AuthHandler.Logout(c)`

```
1. c.ShouldBindJSON(&request.LogoutRequest{RefreshToken})
2. h.authService.Logout(ctx, refreshToken)
```

### 5.2 Service: `AuthService.Logout(ctx, refreshToken)`

```
1. jwt.ParseToken(refreshToken, secret)
   └─ 已过期的 token 也接受解析（仍可写入黑名单）

2. tokenBlacklist[refreshToken] = claims.ExpiresAt.Time
   └─ 写入内存 map（sync.Mutex 保护）

3. 后台 goroutine: blacklistCleanupLoop()
   └─ 每 10 分钟扫描一次，删除已过期的黑名单条目
```

---

## 6. 关键数据结构

### JWT Claims (`pkg/jwt/jwt.go`)

```go
type Claims struct {
    UserID      int64    `json:"user_id"`
    Username    string   `json:"username"`
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
    TokenType   string   `json:"token_type"` // "access" | "refresh"
    jwt.RegisteredClaims
}
```

### 密码策略 (`pkg/hash/hash.go`)

| 函数 | 说明 |
|------|------|
| `HashPassword(plain) → hash` | bcrypt, cost 可配 `OPSMIND_BCRYPT_COST`（默认 10） |
| `CheckPassword(hash, plain) → bool` | bcrypt.CompareHashAndPassword |
| `ValidatePassword(plain) → error` | 正则: `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$` |

### 登录限流 (`service/auth_service.go`)

- 滑动窗口: 15 分钟 / 5 次失败上限
- `rateLimiter.allowLogin(username)` — 超限返回 error
- `rateLimiter.recordFail(username)` — 失败计数 +1
- `rateLimiter.recordSuccess(username)` — 清除计数

### 令牌黑名单 (`service/auth_service.go`)

- 内存 `map[string]time.Time`，`sync.Mutex` 保护
- 登出时写入 `refresh_token → expires_at`
- `blacklistCleanupLoop()` 每 10 分钟清理过期条目

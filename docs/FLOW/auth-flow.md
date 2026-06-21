# 认证数据流 — 登录 / 刷新 / 改密 / 登出

---

## 用户故事

| 角色 | 目标 | 价值 |
|------|------|------|
| 报障人 | 使用工号密码登录门户，获取智能问答和申告权限 | 自助获取运维支持 |
| 运维人员 / 管理员 | 登录后台管理申告、知识库、系统配置 | 日常运维工作入口 |
| 首次登录用户 | 登录后强制修改初始密码 | 安全合规 |
| 会话过期用户 | 无感刷新令牌，不中断当前操作 | 体验连贯 |

---

## 前端调用链路

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          前端组件 → API 映射                              │
├───────────────┬────────────────────────────────┬────────────────────────┤
│     页面       │           组件                  │      API 调用           │
├───────────────┼────────────────────────────────┼────────────────────────┤
│ /login        │ LoginPage                      │ POST /api/v1/auth/login│
│               │  ├─ <input> 用户名/密码         │  → apiFetch<LoginResp> │
│               │  └─ AppleButton type="submit"   │                        │
│               │                                │                        │
│ /change-      │ ChangePasswordPage             │ POST /api/v1/auth/me/  │
│ password      │  ├─ AppleInput 旧密码/新密码     │   change-password      │
│               │  └─ AppleButton 提交            │  → changePassword()    │
│               │                                │                        │
│ 全局          │ PortalLayout / AdminLayout      │ POST /api/v1/auth/me/  │
│ (导航栏)       │  └─ "退出登录" 按钮             │   logout               │
│               │     → useAuth().logout()        │  → logout(refreshToken)│
│               │                                │                        │
│ 全局          │ proxy.ts (middleware)            │ POST /api/v1/auth/     │
│ (路由守卫)     │  自动检测 token 过期             │   refresh              │
│               │  透明刷新后重放请求               │  → refreshToken()      │
└───────────────┴────────────────────────────────┴────────────────────────┘
```

> **注意：** `LoginPage` 直接调用 `apiFetch<LoginResponse>('/api/v1/auth/login', ...)`（内联类型），
> 未使用 `lib/api/auth.ts` 中的 `login()` 封装函数。登录成功后通过 `useAuth().login()` 将
> token/user/roles/permissions/menus 写入 localStorage + cookie，后续所有请求由
> `apiFetch` 自动附加 `Authorization: Bearer <token>` 头。

---

# 认证数据流 — 登录

## 输入
```
{
  "username": "admin",
  "password": "Admin@123"
}
```

## 分层数据流

### 接入层 — 路由 & 中间件

1. `router.Setup()` 注册 `POST /api/v1/auth/login` → `AuthHandler.Login`

### 接入层 — Handler

2. 经由 `AuthHandler.Login()` 处理：
   - `c.ShouldBindJSON(&req)` 将 body 反序列化为 `request.LoginRequest`
   - 校验失败 → `response.Error(c, ErrParam, ...)` 返回 400

### 业务层 — Service

3. 经由 `AuthService.Login(ctx, username, password)` 处理：
   - `rateLimiter.allowLogin(username)` — 检查滑动窗口限流（15 分钟内 ≤ 5 次失败）
   - `userRepo.GetByUsername(ctx, username)` 查询用户记录
   - `hash.CheckPassword(user.PasswordHash, password)` bcrypt 校验
   - `user.Status == 2` 冻结检查
   - `rateLimiter.recordSuccess(username)` 或 `recordFail(username)`

4. 经由 `AuthService.buildLoginResponse(ctx, user)` 组装返回：
   - `userRepo.GetUserRoles(ctx, user.ID)` — 查询用户角色
   - `userRepo.GetUserPermissions(ctx, user.ID)` — 查询用户权限
   - `buildMenuTree(ctx, roles)` — 构建菜单树
     - 系统管理员 → `menuRepo.ListMenus(ctx)` 获取全部菜单
     - 其他用户 → `menuRepo.BatchGetRoleMenus(ctx, roleIDSlice)` 批量查询
     - `buildTree(menus, 0)` → `buildTreeWithMap(childrenMap, 0)` 递归构建
   - `jwt.GenerateAccessToken(user.ID, username, roles, permissions, secret, expire)` 生成 access_token
   - `jwt.GenerateRefreshToken(...)` 生成 refresh_token

### 数据层 — Repository

5. `UserRepo.GetByUsername(ctx, username)` — `SELECT * FROM users WHERE username = ?`
6. `UserRepo.GetUserRoles(ctx, userID)` — JOIN user_roles + roles
7. `UserRepo.GetUserPermissions(ctx, userID)` — JOIN 角色 → JSONB 权限聚合
8. `MenuRepo.ListMenus(ctx)` 或 `MenuRepo.BatchGetRoleMenus(ctx, roleIDs)` — 菜单查询

## 输出
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "user": { "id": 1, "username": "admin", "real_name": "管理员", ... },
    "roles": ["系统管理员"],
    "permissions": ["*"],
    "menus": [{ "id": 1, "name": "知识管理", "children": [...] }]
  }
}
```

## 关键分支

| 分支 | 条件 | 产出 |
|------|------|------|
| 限流拒绝 | 同一用户名 15 分钟内失败 ≥ 5 次 | `ErrParam` — "登录失败次数过多，请15分钟后再试" |
| 密码/用户名错误 | bcrypt 校验失败或用户不存在 | `ErrParam` — "用户名或密码错误"（统一错误码，防枚举） |
| 账号已冻结 | `user.Status == 2` | `ErrForbidden` — "账号已被冻结" |
| 首次登录 | `user.FirstLogin == true` | 异步 `db.Model(&User{}).Where("id=?",id).Update("first_login",false)` |

---

# 认证数据流 — 刷新令牌

## 用户故事
> 作为门户用户，我希望在会话期间令牌过期时自动续期，不被打断当前操作。

## 前端调用链
`proxy.ts (middleware)` 拦截请求 → 检测 token 过期 → `POST /api/v1/auth/refresh` → 更新 cookie → 重放原始请求

## 输入
```
{ "refresh_token": "eyJhbGciOi..." }
```

## 分层数据流

1. `AuthHandler.Refresh(c)` — Gin Handler，`ShouldBindJSON`
2. `AuthService.RefreshToken(ctx, refreshToken)`
   - `tokenBlacklist` map 检查是否已登出
   - `jwt.ParseToken(refreshToken, secret)` 解析 claims
   - `claims.TokenType != "refresh"` 类型校验
   - `userRepo.GetByID(ctx, claims.UserID)` 校验用户仍存在且未冻结
   - `buildLoginResponse(ctx, user)` 重新生成令牌对
3. 同 Login 的数据层调用

## 输出
同登录响应（新令牌对 + 用户信息 + 菜单树）

---

# 认证数据流 — 修改密码

## 用户故事
> 作为首次登录用户，我需要按安全策略修改初始密码后才能使用系统功能。

## 前端调用链
`ChangePasswordPage` → 用户填写旧密码/新密码/确认 → AppleButton 提交 → `changePassword(old, new)` → `POST /api/v1/auth/me/change-password`

## 输入
```
{ "old_password": "Admin@123", "new_password": "NewPass@456" }
```

## 分层数据流

1. `AuthHandler.ChangePassword(c)` — 从 JWT context 获取 `userID`
2. `AuthService.ChangePassword(ctx, userID, oldPwd, newPwd)`
   - `userRepo.GetByID(ctx, userID)` 查询用户
   - `hash.CheckPassword(user.PasswordHash, oldPwd)` 旧密码校验
   - `oldPwd == newPwd` 新旧相同校验
   - `hash.ValidatePassword(newPwd)` 正则校验 `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`
   - `hash.HashPassword(newPwd)` bcrypt 哈希 (cost=10)
   - `db.Model(&User{}).Where("id=?",userID).Updates(map)` 更新 password_hash + first_login=false

## 输出
```json
{ "code": 0, "message": "success", "data": null }
```

---

# 认证数据流 — 退出登录

## 用户故事
> 作为用户，退出后我的会话应立即失效，其他人无法用同一令牌访问系统。

## 前端调用链
`PortalLayout` 或 `AdminLayout` 导航栏 → 点击"退出登录" → `useAuth().logout()` → 清除 localStorage + cookie → `logout(refreshToken)` → `POST /api/v1/auth/me/logout` → 跳转到 `/login`

## 输入
```
{ "refresh_token": "eyJhbGciOi..." }
```

## 分层数据流

1. `AuthHandler.Logout(c)` — Gin Handler
2. `AuthService.Logout(ctx, refreshToken)`
   - `jwt.ParseToken(refreshToken, secret)` 解析（已过期也接受）
   - `tokenBlacklist[refreshToken] = claims.ExpiresAt.Time` 写入内存黑名单
3. `blacklistCleanupLoop()` 后台 goroutine 每 10 分钟清理过期条目

## 输出
```json
{ "code": 0, "message": "success", "data": null }
```

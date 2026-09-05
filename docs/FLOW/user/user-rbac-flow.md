# 用户与 RBAC 数据流 — 每个 API 端点

> 涉及文件: `domain/user/account/handler.go`, `domain/user/role/handler.go`, `domain/user/account/service.go`, `domain/user/role/service.go`, `domain/user/auth/service.go`, `domain/user/account/repository.go`, `domain/user/role/repository.go`, `domain/user/role/repository.go`, `domain/system/audit/repository.go`, `shared/model/user.go`, `shared/model/user.go`, `shared/model/user.go`, `infra/cache/user_status.go`

---

## 用户管理

### GET /api/v1/admin/users &emsp; 用户列表 &emsp; [PermUserManage]

**输入** `?page=1&page_size=20&keyword=张三`

```
UserHandler.List (domain/user/account/handler.go:66)
  └─ parsePagination (shared/handler/common.go:23) → page, pageSize
  → UserService.List (domain/user/account/service.go:54)
    └─ UserRepo.List (domain/user/account/repository.go:107)
        → SELECT COUNT(*) FROM users
        → SELECT * FROM users WHERE username ILIKE '%keyword%' OR real_name ILIKE '%keyword%'
          ORDER BY created_at DESC LIMIT ? OFFSET ?
```

### POST /api/v1/admin/users &emsp; 创建用户 &emsp; [PermUserManage]

**输入** `{"username":"zhangsan","password":"Zh@ng123","real_name":"张三","roles":[3],"phone":"13800138000","email":"zs@ops.com"}`

```
UserHandler.Create (domain/user/account/handler.go:30)
  → UserService.Create (domain/user/account/service.go:100)
    ├─ validateUserInput (domain/user/account/service.go:299)
    │   → username 正则 ^[a-zA-Z][a-zA-Z0-9_]{3,31}$, phone 正则, email 正则
    ├─ hash.ValidatePassword → ^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$
    ├─ UserRepo.ExistsByUsername (domain/user/account/repository.go:136)
    │   → SELECT COUNT(*) FROM users WHERE username=? → 冲突 10005
    ├─ UserRepo.GetByPhone → 可选唯一性校验
    ├─ hash.HashPassword → bcrypt cost=10
    ├─ UserRepo.Create (domain/user/account/repository.go:94)
    │   → INSERT INTO users (...)
    ├─ UserRepo.AssignRoles (domain/user/account/repository.go:211)
    │   → INSERT INTO user_roles (user_id, role_id) VALUES ...
    ├─ AuditRepo.Create → "user.create"
    └─ UserStatusCache.Invalidate (infra/cache/user_status.go)
```

### GET /api/v1/admin/users/:id &emsp; 用户详情 &emsp; [PermUserManage]

```
UserHandler.GetByID (domain/user/account/handler.go:48)
  └─ parseID → userID
  → UserService.GetByID (domain/user/account/service.go:39)
    ├─ UserRepo.GetByID (domain/user/account/repository.go:31)
    │   → SELECT * FROM users WHERE id=?
    ├─ UserRepo.GetUserRoles (domain/user/account/repository.go:151)
    │   → SELECT r.* FROM roles r JOIN user_roles ur ON r.id=ur.role_id WHERE ur.user_id=?
    └─ toDetailResponse (domain/user/account/service.go:323)
        → 组装 UserDetailResponse{user, roles}
```

### PUT /api/v1/admin/users/:id &emsp; 更新用户 &emsp; [PermUserManage]

**输入** `{"real_name":"张三丰","roles":[3,4],"phone":"13900139000"}` (支持部分更新)

```
UserHandler.Update (domain/user/account/handler.go:82)
  → UserService.Update (domain/user/account/service.go:164)
    ├─ UserRepo.GetByID → 校验存在
    ├─ UserRepo.ExistsByUsername → 改名需唯一
    ├─ validateUserInput → 更新字段校验
    ├─ UserRepo.Update → UPDATE users SET real_name=?, phone=?, email=? WHERE id=?
    ├─ UserRepo.AssignRoles → 事务: DELETE old + INSERT new
    ├─ AuditRepo.Create → "user.update"
    └─ UserStatusCache.Invalidate → 清除缓存
```

### PATCH /api/v1/admin/users/:id/freeze &emsp; 冻结用户 &emsp; [PermUserManage]

```
UserHandler.Freeze (domain/user/account/handler.go:105)
  → UserService.Freeze (domain/user/account/service.go:200)
    ├─ UserRepo.GetByID → 校验
    ├─ status==2(frozen) → ErrAlreadyFrozen(10006)
    ├─ assertNotLastAdmin (domain/user/account/service.go:270)
    │   └─ UserRepo.CountActiveAdmins (domain/user/account/repository.go:187)
    │       → SELECT COUNT(*) FROM users u JOIN user_roles ur ... WHERE u.status=1 AND ...
    │       → count≤1 → 拒绝（不能冻结最后一个管理员）
    ├─ UserRepo.UpdateStatus (domain/user/account/repository.go:131)
    │   → UPDATE users SET status=2 WHERE id=?
    ├─ AuditRepo.Create → "user.freeze"
    └─ UserStatusCache.Invalidate
```

### PATCH /api/v1/admin/users/:id/unfreeze &emsp; 恢复用户 &emsp; [PermUserManage]

```
UserHandler.Restore (domain/user/account/handler.go:124)
  → UserService.Restore (domain/user/account/service.go:236)
    ├─ UserRepo.GetByID → 校验
    ├─ status!=2(frozen) → ErrAlreadyActive(10007)
    ├─ UserRepo.UpdateStatus → UPDATE users SET status=1 WHERE id=?
    ├─ AuditRepo.Create → "user.restore"
    └─ UserStatusCache.Invalidate
```

---

## 角色管理

### GET /api/v1/admin/roles &emsp; 角色列表 &emsp; [PermUserManage]

**输入** `?page=1&page_size=20&keyword=技术`

```
RoleHandler.List (domain/user/role/handler.go:59)
  └─ parsePagination → page, pageSize
  → RoleService.List (domain/user/role/service.go:112)
    └─ RoleRepo.List (domain/user/role/repository.go:58)
        → SELECT * FROM roles WHERE name ILIKE '%keyword%' ORDER BY id ASC LIMIT ? OFFSET ?
```

### POST /api/v1/admin/roles &emsp; 创建角色 &emsp; [PermUserManage]

**输入** `{"name":"审计员","description":"只读审计日志","permissions":["audit.read"]}`

```
RoleHandler.Create (domain/user/role/handler.go:27)
  → RoleService.Create (domain/user/role/service.go:64)
    ├─ validatePermissions (domain/user/role/service.go:52)
    │   → 逐个校验 validPermissions 白名单
    ├─ RoleRepo.ExistsByName (domain/user/role/repository.go:45)
    │   → SELECT COUNT(*) FROM roles WHERE name=? → 冲突
    ├─ permissions → JSON via pq.Array → JSONB
    ├─ RoleRepo.Create (domain/user/role/repository.go:27)
    │   → INSERT INTO roles (name, description, permissions, is_builtin=false)
    └─ AuditRepo.Create → "role.create"
```

### GET /api/v1/admin/roles/:id &emsp; 角色详情 &emsp; [PermUserManage]

```
RoleHandler.GetByID (domain/user/role/handler.go:43)
  → RoleService.GetByID (domain/user/role/service.go:100)
    └─ RoleRepo.GetByID (domain/user/role/repository.go:32)
        → SELECT * FROM roles WHERE id=?
```

### PUT /api/v1/admin/roles/:id &emsp; 更新角色 &emsp; [PermUserManage]

**输入** `{"name":"高级审计员","permissions":["audit.read","dashboard.read"]}`

```
RoleHandler.Update (domain/user/role/handler.go:73)
  → RoleService.Update (domain/user/role/service.go:120)
    ├─ RoleRepo.GetByID → 校验存在
    ├─ RoleRepo.IsBuiltinRole (domain/user/role/repository.go:102)
    │   → SELECT is_builtin FROM roles WHERE id=? → 内置角色不可修改权限
    ├─ validatePermissions → 白名单校验
    ├─ RoleRepo.ExistsByName → 改名唯一性（排除自身）
    ├─ RoleRepo.Update (domain/user/role/repository.go:79)
    │   → UPDATE roles SET name=?, description=?, permissions=? WHERE id=?
    └─ AuditRepo.Create → "role.update"
```

### DELETE /api/v1/admin/roles/:id &emsp; 删除角色 &emsp; [PermUserManage]

```
RoleHandler.Delete (domain/user/role/handler.go:94)
  → RoleService.Delete (domain/user/role/service.go:165)
    ├─ RoleRepo.GetByID → 校验
    ├─ RoleRepo.IsBuiltinRole → 内置角色不可删除
    ├─ UserRepo.CountUsersByRole (domain/user/account/repository.go:202)
    │   → SELECT COUNT(*) FROM user_roles WHERE role_id=? → count>0 拒绝
    ├─ RoleRepo.Delete (domain/user/role/repository.go:87)
    │   → DELETE FROM roles WHERE id=? AND is_builtin=false
    └─ AuditRepo.Create → "role.delete"
```

---

## 菜单管理

### GET /api/v1/admin/menus &emsp; 全部菜单 &emsp; [PermUserManage]

```
RoleHandler.ListMenus (domain/user/role/handler.go:115)
  → RoleService.ListMenus (domain/user/role/service.go:207)
    └─ MenuRepo.ListMenus (domain/user/role/repository.go:24)
        → SELECT * FROM menus ORDER BY sort_order ASC
```

### PUT /api/v1/admin/roles/:id/menus &emsp; 更新角色菜单绑定 &emsp; [PermUserManage]

**输入** `{"menu_ids":[1,2,5,6]}`

```
RoleHandler.UpdateRoleMenus (domain/user/role/handler.go:127)
  → RoleService.UpdateRoleMenus (domain/user/role/service.go:228)
    ├─ RoleRepo.GetByID → 校验角色存在
    ├─ MenuRepo.ValidateMenuIDs (domain/user/role/repository.go:52)
    │   → SELECT id FROM menus WHERE id IN ? → 不存在的 ID 拒绝
    └─ MenuRepo.UpdateRoleMenus (domain/user/role/repository.go:73)
        → 事务: DELETE FROM role_menus WHERE role_id=? → INSERT role_menus 批量
```

---

## 权限常量

定义于 `router/permissions.go:10-22`：

```
user:manage       — 用户与角色管理
ticket:read       — 申告查看
ticket:write      — 申告操作（状态变更/记录/知识候选）
ticket:manage     — 申告全管理（=read+write）
knowledge:read    — 知识库查看
knowledge:write   — 知识库编辑
knowledge:create  — 知识创建
knowledge:manage  — 知识全管理（=read+write+review）
knowledge:review  — 知识审核与发布
audit:read        — 审计日志查看
dashboard:read    — 看板查看
system:config     — 系统配置与 LLM 配置管理
```

## RBAC 中间件校验链

```
middleware.JWTAuth (infra/middleware/auth.go:37)
  → JWT 解析 → 用户状态缓存校验 → CurrentUser 写入 context

middleware.RequirePermission (infra/middleware/rbac.go:24)
  → 从 context 取 CurrentUser.Permissions
  → 精确匹配 / 通配符前缀 ("ticket:*" 匹配 "ticket:read") / "*" 全局
  → 无匹配 → 403 ErrForbidden
```

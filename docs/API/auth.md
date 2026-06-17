# 认证接口

> **Base URL:** `/api/v1/auth` | **Auth:** Public (login/refresh) or JWT (change-password/logout)

## 1. 登录

```http
POST /api/v1/auth/login
```

**请求体：**

```json
{
  "username": "admin",
  "password": "Admin@123"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | ✓ | 用户名 |
| password | string | ✓ | 密码 |

**成功响应 (200)：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "username": "admin",
      "real_name": "系统管理员",
      "phone": "13800000001",
      "email": "admin@opsmind.local",
      "first_login": false
    },
    "roles": ["系统管理员"],
    "permissions": ["user:manage", "ticket:manage", "knowledge:manage"],
    "menus": [
      {
        "id": 1,
        "name": "仪表盘",
        "path": "/admin/dashboard",
        "icon": "dashboard",
        "parent_id": 0,
        "sort_order": 1,
        "type": "menu",
        "children": []
      }
    ]
  }
}
```

**错误响应：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 用户名或密码错误（同一用户名 15 分钟内最多 5 次失败尝试，超额后返回此错误） |
| 10002 | 403 | 账号已被冻结 |
| 99999 | 500 | 服务器内部错误（DB 查询失败、角色/权限/菜单加载失败、Token 生成失败） |

> 登录失败审计日志由服务端 `slog` 记录，涵盖限流拒绝、用户不存在、密码错误、账号已冻结等场景。用户名不存在与密码错误返回相同错误码，以防止用户名枚举攻击。

---

## 2. 刷新令牌

```http
POST /api/v1/auth/refresh
```

**请求体：**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

> refresh_token 必须是 refresh 类型的 token（非 access_token）。使用 access_token 将返回 10001。

**成功响应 (200)：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "username": "admin",
      "real_name": "系统管理员",
      "phone": "13800000001",
      "email": "admin@opsmind.local",
      "first_login": false
    },
    "roles": ["系统管理员"],
    "permissions": ["user:manage", "ticket:manage", "knowledge:manage"],
    "menus": [
      {
        "id": 1,
        "name": "仪表盘",
        "path": "/admin/dashboard",
        "icon": "dashboard",
        "parent_id": 0,
        "sort_order": 1,
        "type": "menu",
        "children": []
      }
    ]
  }
}
```

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10001 | 401 | Token 无效或已过期、已被拉黑、使用了 access_token 而非 refresh_token、用户不存在 |
| 10002 | 403 | 用户已被冻结 |
| 10003 | 400 | 请求体格式错误（缺少 refresh_token） |
| 99999 | 500 | 服务器内部错误 |

> 刷新令牌有效期为 7 天。令牌过期后，用户需重新登录以获取新的令牌。

---

## 3. 修改密码

```http
POST /api/v1/auth/me/change-password
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "old_password": "Admin@123",
  "new_password": "NewPass456"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| old_password | string | ✓ | 旧密码 |
| new_password | string | ✓ | 新密码（8-32 位，须含大小写字母+数字） |

**密码策略：**
- 长度 8-32 位
- 必须包含大写字母、小写字母、数字
- 正则：`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10001 | 401 | 未登录或 Token 无效（无法获取当前用户） |
| 10003 | 400 | 参数校验失败、旧密码错误、新密码不满足策略 |
| 99999 | 500 | 服务器内部错误 |

**成功响应 (200)：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

---

## 4. 登出

```http
POST /api/v1/auth/me/logout
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refresh_token | string | ✓ | 需失效的刷新令牌 |

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 请求体格式错误（缺少 refresh_token） |

**成功响应 (200)：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

> 登出时将 refresh token 加入内存黑名单，以防止其被用于令牌刷新。黑名单条目在 token 到期后自动清理。即使 refresh token 已过期或无效，登出操作仍视为成功响应。

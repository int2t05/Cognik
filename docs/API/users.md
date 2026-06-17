# 用户管理接口

> **Base URL:** `/api/v1/admin/users` | **Auth:** JWT + `user:manage`

## 1. 用户列表

```http
GET /api/v1/admin/users?page=1&page_size=10&keyword=admin
Authorization: Bearer <token>
```

**查询参数：**

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 10 | 每页条数（最大 100） |
| keyword | string | — | 按用户名/姓名/手机号模糊搜索（可选） |

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "username": "admin",
      "real_name": "系统管理员",
      "phone": "13800000001",
      "email": "admin@opsmind.local",
      "status": 1,
      "first_login": false,
      "roles": ["系统管理员"],
      "created_at": "2026-06-11 19:27:43",
      "updated_at": "2026-06-11 19:27:43"
    }
  ],
  "total": 7,
  "page": 1,
  "page_size": 10
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| status | int16 | 1=正常, 2=冻结 |
| first_login | bool | 是否首次登录（首次需强制改密） |

---

## 2. 创建用户

```http
POST /api/v1/admin/users
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "username": "newuser",
  "password": "NewUser123",
  "real_name": "新用户",
  "phone": "13800001000",
  "email": "newuser@opsmind.local",
  "role_ids": [4]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | ✓ | 用户名（唯一） |
| password | string | ✓ | 密码（8-32位，须含大小写+数字） |
| real_name | string | ✓ | 真实姓名 |
| phone | string | ✓ | 手机号 |
| email | string | | 邮箱 |
| role_ids | int64[] | | 分配的角色 ID 列表 |

**密码策略：** 正则 `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`

**错误：**

| code | HTTP | 说明 |
|------|------|------|
| 10003 | 400 | 参数校验失败（含密码策略不满足） |
| 10005 | 409 | 用户名已存在 |

---

## 3. 用户详情

```http
GET /api/v1/admin/users/:id
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "username": "admin",
    "real_name": "系统管理员",
    "phone": "13800000001",
    "email": "admin@opsmind.local",
    "status": 1,
    "first_login": false,
    "roles": ["系统管理员"],
    "created_at": "2026-06-11 19:27:43",
    "updated_at": "2026-06-11 19:27:43"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| status | int16 | 1=正常, 2=冻结 |
| first_login | bool | 是否首次登录（首次需强制改密） |
| roles | string[] | 已分配的角色名列表 |

**错误：**

| code | HTTP | 说明 |
|------|------|------|
| 10004 | 404 | 用户不存在 |

---

## 4. 更新用户

```http
PUT /api/v1/admin/users/:id
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "real_name": "新姓名",
  "phone": "13800001001",
  "email": "updated@opsmind.local",
  "role_ids": [4]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| real_name | string | ✓ | 真实姓名 |
| phone | string | ✓ | 手机号 |
| email | string | | 邮箱（可选） |
| role_ids | int64[] | | 重新分配的角色 ID 列表（全量替换） |

> 仅更新基本信息，不修改密码。密码修改走 `/api/v1/auth/change-password`。
>
> 角色分配采用全量替换策略：传入 `role_ids` 将覆盖用户现有的角色关联。

**错误：**

| code | HTTP | 说明 |
|------|------|------|
| 10003 | 400 | 参数校验失败（real_name/phone 未提供） |
| 10004 | 404 | 用户不存在 |

---

## 5. 冻结用户

```http
PATCH /api/v1/admin/users/:id/freeze
Authorization: Bearer <token>
```

> 状态：正常(1) → 冻结(2)
>
> 冻结后用户无法登录，已有 token 在后续请求中被拒绝。

**错误：**

| code | HTTP | 说明 |
|------|------|------|
| 10003 | 400 | 不能冻结自己的账号 |
| 10003 | 400 | 不能冻结/移除最后一个系统管理员 |
| 10004 | 404 | 用户不存在 |
| 10006 | 400 | 用户已被冻结（重复冻结） |

**成功响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

---

## 6. 恢复用户

```http
PATCH /api/v1/admin/users/:id/unfreeze
Authorization: Bearer <token>
```

> 状态：冻结(2) → 正常(1)

**错误：**

| code | HTTP | 说明 |
|------|------|------|
| 10004 | 404 | 用户不存在 |
| 10007 | 400 | 用户已处于正常状态（重复恢复） |

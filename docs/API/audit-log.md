# 审计日志、系统配置与站内消息管理

## 审计日志管理

> 基础路径：`/api/v1/admin/audit-logs` | 认证：JWT + RBAC（权限：`audit:read`）

### 查询审计日志

```http
GET /api/v1/admin/audit-logs?page=1&page_size=10&operator_id=0&action=&target_type=&target_id=0&date_from=&date_to=
Authorization: Bearer <token>
```

**查询参数：**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| page | int | 否 | 1 | 页码 |
| page_size | int | 否 | 10 | 每页条数（最大 100） |
| operator_id | int | 否 | 0 | 操作人 ID（0=全部，包括系统自动操作） |
| action | string | 否 | "" | 操作类型，精确匹配（空=全部） |
| target_type | string | 否 | "" | 操作对象类型（空=全部，可选：user/role/ticket/knowledge_article/llm_config） |
| target_id | int64 | 否 | 0 | 操作对象 ID（0=全部） |
| date_from | string | 否 | "" | 起始日期（格式：YYYY-MM-DD，含当日） |
| date_to | string | 否 | "" | 结束日期（格式：YYYY-MM-DD，含当日） |

**响应体字段（AuditLogItem）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int64 | 审计日志 ID |
| operator_id | int64 | 操作人 ID |
| operator_name | string | 操作人姓名（JOIN users 表） |
| action | string | 操作类型 |
| target_type | string | 操作对象类型 |
| target_id | int64 | 操作对象 ID |
| detail | string | 操作详情（JSON 字符串） |
| ip_address | string | 操作人 IP 地址 |
| created_at | string | 操作时间（格式：2006-01-02 15:04:05） |

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "operator_id": 1,
      "operator_name": "admin",
      "action": "knowledge.publish",
      "target_type": "knowledge_article",
      "target_id": 5,
      "detail": "{\"title\":\"VPN 密码重置 FAQ\"}",
      "ip_address": "192.168.1.1",
      "created_at": "2026-06-11 20:30:00"
    }
  ],
  "total": 150,
  "page": 1,
  "page_size": 10
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10003 | 400 | 参数校验失败（page、page_size 等参数格式错误） |
| 99999 | 500 | 服务器内部错误 |

**记录的操作类型：**

| action | 说明 |
|--------|------|
| `user.create` | 创建用户 |
| `user.update` | 更新用户 |
| `user.freeze` | 冻结用户 |
| `user.restore` | 恢复用户 |
| `role.create` | 创建角色 |
| `role.update` | 更新角色 |
| `role.delete` | 删除角色 |
| `ticket.start` | 开始处理申告 |
| `ticket.request_info` | 请求补充信息 |
| `ticket.resolve` | 解决申告 |
| `ticket.close` | 关闭申告 |
| `ticket.auto_close` | 系统自动关闭超期申告 |
| `knowledge.publish` | 发布知识文章 |
| `knowledge.disable` | 停用知识文章 |
| `config.update` | 更新系统配置 |
| `llm_config.update` | 更新 LLM 配置 |

### 批量删除审计日志

```http
POST /api/v1/admin/audit-logs/batch-delete
Authorization: Bearer <token>
```

**请求体：**

```json
{ "ids": [1, 2, 3] }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 参数校验失败 |

---

## 系统配置管理

> 基础路径：`/api/v1/admin/configs` | 认证：JWT + RBAC（权限：`system:config`）

### 获取配置

```http
GET /api/v1/admin/configs/:key
Authorization: Bearer <token>
```

**URL 参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 配置键名 |

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": "0.6"
}
```

> `data` 字段返回配置值的原始 JSON 解析结果（string、number、boolean、object 等类型取决于配置项）。

**可用配置键：**

| key | 类型 | 说明 |
|-----|------|------|
| `app_name` | string | 应用名称，显示在页面标题和系统通知中 |
| `ai.top_k` | number | RAG 默认检索 Top K |
| `ai.threshold` | number | AI 置信度阈值 |

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10003 | 400 | 参数校验失败（key 为空） |
| 10004 | 404 | 配置项不存在 |
| 99999 | 500 | 服务器内部错误 |

### 更新配置

```http
PUT /api/v1/admin/configs/:key
Authorization: Bearer <token>
Content-Type: application/json
```

**URL 参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 配置键名 |

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| value | any | 是 | 配置值（会被序列化为 JSONB 存储；不能为 null） |

**请求示例：**

```json
{
  "value": "OpsMind"
}
```

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10003 | 400 | 参数校验失败（key 为空、value 为 null 或格式错误） |
| 10004 | 404 | 配置项不存在（更新不存在配置时创建新配置，通常不会返回此错误） |
| 99999 | 500 | 服务器内部错误 |

### 公开配置

```http
GET /api/v1/public/configs/:key
```

> 无需认证，供登录页等公开页面读取系统级配置（如应用名称）。

**响应示例：**

```json
{ "code": 0, "message": "success", "data": { "value": "OpsMind" } }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10004 | 404 | 配置项不存在 |

---

## 站内消息管理（门户端）

> 基础路径：`/api/v1/portal/messages` | 认证：JWT

### 消息列表

```http
GET /api/v1/portal/messages?page=1&page_size=10&is_read=false&type=ticket_supplement
Authorization: Bearer <token>
```

**查询参数：**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| page | int | 否 | 1 | 页码 |
| page_size | int | 否 | 10 | 每页条数（最大 100） |
| is_read | bool | 否 | — | 是否已读（true/false，不传不过滤） |
| type | string | 否 | — | 消息类型（不传不过滤，可选值：`ticket_supplement` / `ticket_resolved` / `system_notice`） |

**响应体字段（Message）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int64 | 消息 ID |
| user_id | int64 | 接收用户 ID |
| title | string | 消息标题 |
| content | string | 消息正文 |
| type | string | 消息类型：`ticket_supplement` / `ticket_resolved` / `system_notice` |
| related_type | string | 关联对象类型（如 `ticket`） |
| related_id | int64 | 关联对象 ID |
| is_read | bool | 是否已读 |
| created_at | string | 创建时间 |

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "user_id": 3,
      "title": "申告「公司邮箱无法登录」需补充信息",
      "content": "请提供错误截图和发生时间",
      "type": "ticket_supplement",
      "related_type": "ticket",
      "related_id": 5,
      "is_read": false,
      "created_at": "2026-06-11 15:30:00"
    }
  ],
  "total": 3,
  "page": 1,
  "page_size": 10
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10003 | 400 | 参数校验失败（page、page_size 格式错误） |
| 99999 | 500 | 服务器内部错误 |

### 标记已读

```http
PUT /api/v1/portal/messages/:id/read
Authorization: Bearer <token>
```

**URL 参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | int64 | 是 | 消息 ID |

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "unread_count": 3
  }
}
```

> 标记已读后返回当前未读消息总数，便于前端同步 badge 计数。

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10003 | 400 | 参数校验失败（ID 格式无效） |
| 10004 | 404 | 消息不存在或不属于当前用户 |
| 99999 | 500 | 服务器内部错误 |

### 未读计数

```http
GET /api/v1/portal/messages/unread-count
Authorization: Bearer <token>
```

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 5
  }
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 10003 | 400 | 参数校验失败 |
| 99999 | 500 | 服务器内部错误 |

### 全部标记已读

```http
PUT /api/v1/portal/messages/read-all
Authorization: Bearer <token>
```

> 将当前用户所有未读消息标记为已读，请求体为空。

**响应示例：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-------------|------|
| 99999 | 500 | 服务器内部错误 |

---

## 健康检查

> 无需认证，供 Docker/K8s 存活探针和就绪探针使用。

### 存活探针

```http
GET /health
```

**响应示例：**

```json
{
  "status": "ok"
}
```

### 就绪探针

```http
GET /readyz
```

**响应示例（就绪）：**

```json
{
  "status": "ready"
}
```

**响应示例（未就绪，503）：**

```json
{
  "status": "not ready"
}
```

> `/readyz` 验证数据库连接可达性。数据库不可达时返回 HTTP 503，K8s 将停止向该 Pod 路由流量。

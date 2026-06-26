# 申告管理接口

> **Base URL:** `/api/v1/portal` + `/api/v1/admin` | **Auth:** Portal: JWT only, Admin: JWT + RBAC

## 申告状态机

```
待处理(1) ──→ 处理中(2) ──→ 已解决(4)
                  │
                  ↓
            需补充信息(3)
                  │
                  ↓
            处理中(2)

待处理(1) ──→ 已关闭(5)
处理中(2) ──→ 已关闭(5)
需补充信息(3) → 已关闭(5)
```

## 门户端（报障人）

### 1. 创建申告

```http
POST /api/v1/portal/tickets
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "title": "公司邮箱无法登录",
  "description": "从今天上午开始，Outlook 一直提示密码错误，已尝试修改密码无效",
  "urgency": 2,
  "impact_scope": 1,
  "affected_systems": ["Exchange", "Outlook"],
  "contact_phone": "13800001111",
  "contact_email": "user@company.com",
  "chat_context": {
    "session_id": 42,
    "question": "邮箱无法登录怎么办？",
    "answer": "您可以尝试以下步骤...",
    "confidence": 0.85
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | ✓ | 申告标题 |
| description | string | ✓ | 详细描述 |
| urgency | int | ✓ | 紧急程度：1=低, 2=中, 3=高 |
| impact_scope | int | | 影响范围：1=个人, 2=部门, 3=全公司（对应 model/enums.go 中的枚举定义） |
| affected_systems | string[] | | 受影响系统列表 |
| contact_phone | string | ✓ | 联系电话 |
| contact_email | string | | 联系邮箱 |
| chat_context | object | | 从问答转申告时的上下文（结构化对象，含 session_id / question / answer / confidence） |

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 参数校验失败 |
| 99999 | 500 | 未知错误 |

### 2. 查询我的申告

```http
GET /api/v1/portal/tickets?page=1&page_size=10
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "ticket_no": "TK-20260611-000001",
      "user_id": 1,
      "submitter_name": "张三",
      "title": "公司邮箱无法登录",
      "urgency": 2,
      "impact_scope": 1,
      "contact_phone": "13800000001",
      "status": 1,
      "status_text": "待处理",
      "supplement_count": 0,
      "created_at": "2026-06-11 10:30:00",
      "updated_at": "2026-06-11 10:30:00"
    }
  ],
  "total": 5,
  "page": 1,
  "page_size": 10
}
```

### 3. 申告详情

```http
GET /api/v1/portal/tickets/:id
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "ticket_no": "TK-20260611-000001",
    "user_id": 1,
    "submitter_name": "张三",
    "title": "公司邮箱无法登录",
    "urgency": 2,
    "impact_scope": 1,
    "contact_phone": "13800000001",
    "status": 3,
    "status_text": "需补充信息",
    "supplement_count": 1,
    "created_at": "2026-06-11 10:30:00",
    "updated_at": "2026-06-12 14:00:00",
    "description": "从今天上午开始，Outlook 一直提示密码错误",
    "affected_systems": ["Exchange", "Outlook"],
    "contact_email": "user@company.com",
    "source": 1,
    "records": [
      {
        "id": 1,
        "ticket_id": 1,
        "operator_id": 2,
        "action": "request_info",
        "content": "请提供错误截图",
        "detail": "",
        "created_at": "2026-06-12 10:00:00"
      }
    ]
  }
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 无效的申告 ID |
| 10004 | 404 | 申告不存在 |

### 3a. 更新申告（门户）

```http
PATCH /api/v1/portal/tickets/:id
Authorization: Bearer <token>
```

> 报障人可更新标题、描述、联系方式等字段。

**请求体：**

```json
{
  "title": "更新后的标题",
  "description": "更新后的描述",
  "tags": "网络,邮箱",
  "contact_phone": "13800000000",
  "contact_email": "user@example.com"
}
```

**错误码：**

| code | HTTP | 说明 |
|------|------|------|
| 10003 | 400 | 参数校验失败 |
| 10004 | 404 | 申告不存在 |

### 4. 补充信息

```http
PATCH /api/v1/portal/tickets/:id/supplement
Authorization: Bearer <token>
```

> 仅在状态为「需补充信息(3)」时可执行补充操作。补充完成后状态恢复为「处理中(2)」。

**请求体：**

```json
{
  "content": "补充的截图和信息..."
}
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 参数校验失败 / 内容为空 / 非需补充信息状态 |
| 10004 | 404 | 申告不存在 |
| 10002 | 403 | 非本人申告 |

---

## 后台管理（运维人员/管理员）

### 5. 全部申告列表

```http
GET /api/v1/admin/tickets?page=1&page_size=10&status=1&urgency=0
Authorization: Bearer <token>
```

| 查询参数 | 类型 | 默认 | 说明 |
|----------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 10 | 每页条数 |
| status | int | -1 | 按状态筛选（-1=全部, 1=待处理, 2=处理中, 3=需补充, 4=已解决, 5=已关闭） |
| urgency | int | 0 | 按紧急程度筛选（0=全部, 1=低, 2=中, 3=高） |

### 5a. 申告详情（后台）

```http
GET /api/v1/admin/tickets/:id
Authorization: Bearer <token>
```

> 后台详情接口与门户端（3. 申告详情）使用相同 Handler，响应格式一致。
> 主要区别在于：后台不限制所有权，可查看任意申告的详情；门户端仅限查看本人的申告。

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 无效的申告 ID |
| 10004 | 404 | 申告不存在 |

### 6. 更新申告状态

```http
PATCH /api/v1/admin/tickets/:id/status
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "action": "start",
  "result": "已分配工程师处理",
  "to_knowledge_candidate": false
}
```

| action | 状态转换 | 说明 |
|--------|----------|------|
| `start` | 待处理(1) → 处理中(2) | 开始处理 |
| `request_info` | 处理中(2) → 需补充信息(3) | 向报障人请求补充信息（同一申告最多 3 次） |
| `resolve` | 处理中(2) → 已解决(4) | 标记已解决 |
| `close` | 待处理(1) / 处理中(2) / 需补充信息(3) → 已关闭(5) | 关闭申告（已解决和已关闭状态不允许关闭） |

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 参数校验失败 / 不支持的操作类型 / 状态机转换违规 |
| 10004 | 404 | 申告不存在 |

### 7. 添加处理记录

```http
POST /api/v1/admin/tickets/:id/records
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "action": "note",
  "content": "已联系用户确认问题复现步骤",
  "detail": "{\"contact_method\":\"phone\"}"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| action | string | ✓ | 操作类型（note/callback/escalate） |
| content | string | | 处理描述 |
| detail | string | | 结构化详情（JSON 字符串） |

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 参数校验失败 / 不支持的操作类型 |
| 10004 | 404 | 申告不存在 |

### 8. 生成知识库候选

```http
POST /api/v1/admin/tickets/:id/knowledge-candidate
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "kb_id": 1
}
```

> 基于申告标题和描述自动创建知识文章草稿，标题格式为「申告经验 - {原申告标题}」。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

**错误码：**

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 10003 | 400 | 参数校验失败（kb_id 必填） |
| 10004 | 404 | 申告不存在 |

### 9. 批量删除申告

```http
POST /api/v1/admin/tickets/batch-delete
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

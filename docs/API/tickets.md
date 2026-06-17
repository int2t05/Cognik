# 申告管理接口

> 基础路径：`/api/v1/portal` + `/api/v1/admin` | 认证：JWT + RBAC（后台）

## 申告状态机

```
待处理(1) → 处理中(2) → 已解决(4)
                ↓              ↓
          需补充信息(3)    已关闭(5)
                ↓
           处理中(2)
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
  "chat_context": "{\"question\":\"邮箱无法登录怎么办？\"}"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | ✓ | 申告标题 |
| description | string | ✓ | 详细描述 |
| urgency | int | ✓ | 紧急程度：1=普通, 2=紧急, 3=非常紧急 |
| impact_scope | int | | 影响范围：1=个人, 2=部门, 3=全公司（对齐 model/enums.go） |
| affected_systems | string[] | | 受影响系统列表 |
| contact_phone | string | ✓ | 联系电话 |
| contact_email | string | | 联系邮箱 |
| chat_context | string | | 从问答转申告时的上下文（JSON 字符串） |

### 2. 查询我的申告

```http
GET /api/v1/portal/tickets?page=1&page_size=10
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "ticket_no": "TK-20260611-0001",
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

### 4. 补充信息

```http
PATCH /api/v1/portal/tickets/:id/supplement
Authorization: Bearer <token>
```

> 仅在状态为「需补充信息(3)」时可操作。最多 3 次。

**请求体：**

```json
{
  "content": "补充的截图和信息..."
}
```

---

## 后台管理（运维人员/管理员）

### 5. 全部申告列表

```http
GET /api/v1/admin/tickets?page=1&page_size=10&status=1&urgency=0
Authorization: Bearer <token>
```

| 查询参数 | 类型 | 默认 | 说明 |
|----------|------|------|------|
| status | int | -1 | 按状态筛选（-1=全部, 1=待处理, 2=处理中, 3=需补充, 4=已解决, 5=已关闭） |
| urgency | int | 0 | 按紧急程度筛选（0=全部, 1=普通, 2=紧急, 3=非常紧急） |

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
| `request_info` | 处理中(2) → 需补充信息(3) | 向报障人索要更多信息 |
| `resolve` | 处理中(2) → 已解决(4) | 标记已解决 |
| `close` | 任意 → 已关闭(5) | 关闭申告 |

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
| action | string | ✓ | 操作类型（note/visit/escalate 等） |
| content | string | | 处理描述 |
| detail | string | | 结构化详情（JSON 字符串） |

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

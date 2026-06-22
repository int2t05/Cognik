# 📡 API 参考

## 概述

OpsMind 提供 RESTful JSON API，Base URL 为 `/api/v1`。

## 路由分组

| 路由组 | 前缀 | 认证 | 说明 |
|--------|------|------|------|
| Public | `/api/v1/auth` | 无 | 登录、刷新令牌 |
| Portal | `/api/v1/portal` | JWT | 智能问答（SSE 流式）、申告提交与查询、站内消息 |
| Admin | `/api/v1/admin` | JWT + RBAC | 申告处理、知识库管理、LLM 配置、用户/角色、看板、审计 |

## 统一响应格式

所有 API 使用统一的 JSON 响应结构：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

分页响应附加分页元数据：

```json
{
  "code": 0,
  "message": "success",
  "data": [],
  "total": 100,
  "page": 1,
  "page_size": 10
}
```

## 认证方式

需要认证的接口携带 JWT 令牌：

```http
Authorization: Bearer <access_token>
```

- `access_token` 有效期 2 小时，通过 `/api/v1/auth/login` 获取
- 过期后使用 `/api/v1/auth/refresh` 刷新（`refresh_token` 有效期 7 天）

## 分页参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码（从 1 开始） |
| `page_size` | int | 10 | 每页条数（最大 100） |

## 错误码速查表

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 0 | 200 | 成功 |
| 10001 | 401 | 未登录或令牌过期 |
| 10002 | 403 | 无权限 / 账号已被冻结 |
| 10003 | 400 | 参数校验失败 / 状态机转换违规 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突（名称重复 / 角色有关联用户） |
| 10006 | 400 | 用户已被冻结 |
| 10007 | 400 | 用户已处于正常状态 |
| 20001 | 503 | AI 服务不可用 |
| 20002 | 503 | RAG 服务不可用 |
| 20003 | 503 | 存储服务不可用 |
| 99999 | 500 | 未知错误 / 服务未初始化 |

## 接口索引

详细接口文档位于仓库 `docs/API/` 目录下：

| 文档 | 端点数 | 说明 |
|------|--------|------|
| [auth.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/auth.md) | 4 | 登录、刷新令牌、登出、修改密码 |
| [chat.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/chat.md) | 6 | 会话管理、SSE 流式问答、反馈、RAG 管道 |
| [tickets.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/tickets.md) | 12 | 申告门户提交、后台处理、状态机、操作记录、知识候选 |
| [knowledge.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/knowledge.md) | 16 | 知识库 CRUD、文章生命周期、审核发布、文档上传/状态 |
| [llm-config.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/llm-config.md) | 6 | LLM/Embedding 配置管理、测试连接 |
| [users.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/users.md) | 7 | 用户 CRUD、冻结/恢复 |
| [roles.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/roles.md) | 8 | 角色 CRUD、菜单分配、角色-菜单关联 |
| [dashboard.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/dashboard.md) | 2 | 统计数据、趋势 |
| [audit-log.md](https://github.com/int2t05/OpsMind/blob/main/docs/API/audit-log.md) | 2 | 审计日志查询、系统配置、站内消息 |

> 涵盖全部 **63 个 API 端点**。

## 核心接口速览

### 认证

```http
POST /api/v1/auth/login        # 登录，返回 access_token + refresh_token
POST /api/v1/auth/refresh      # 刷新 access_token
POST /api/v1/auth/logout       # 登出
POST /api/v1/auth/change-password  # 修改密码
```

### 智能问答（SSE 流式）

```http
POST /api/v1/portal/chat-sessions              # 创建会话
POST /api/v1/portal/chat-sessions/:id/stream   # 发送消息（SSE 流式返回）
GET  /api/v1/portal/chat-sessions              # 会话列表
GET  /api/v1/portal/chat-sessions/:id          # 会话详情（含消息）
POST /api/v1/portal/messages/:id/feedback      # 消息反馈（已解决/未解决）
DELETE /api/v1/portal/chat-sessions/:id         # 删除会话
```

### 申告管理

```http
# 门户端
POST /api/v1/portal/tickets          # 提交申告
GET  /api/v1/portal/tickets          # 我的申告列表
GET  /api/v1/portal/tickets/:id      # 申告详情

# 后台管理端
GET    /api/v1/admin/tickets              # 全部申告列表（筛选+分页）
GET    /api/v1/admin/tickets/:id          # 申告详情（含处理记录）
PUT    /api/v1/admin/tickets/:id/status   # 状态变更（开始处理/解决/关闭/索要补充）
POST   /api/v1/admin/tickets/:id/records  # 添加处理记录
```

### 知识库管理

```http
# 知识库
GET    /api/v1/admin/knowledge-bases          # 知识库列表
POST   /api/v1/admin/knowledge-bases          # 创建知识库
PUT    /api/v1/admin/knowledge-bases/:id      # 编辑知识库
DELETE /api/v1/admin/knowledge-bases/:id      # 删除知识库

# 文章
GET    /api/v1/admin/knowledge-bases/:kb_id/articles       # 文章列表
POST   /api/v1/admin/knowledge-bases/:kb_id/articles       # 创建文章/上传文档
PUT    /api/v1/admin/knowledge-articles/:id                # 编辑文章
DELETE /api/v1/admin/knowledge-articles/:id                # 删除文章
POST   /api/v1/admin/knowledge-articles/:id/submit-review  # 提交审核
POST   /api/v1/admin/knowledge-articles/:id/review         # 审核（通过/驳回）
POST   /api/v1/admin/knowledge-articles/:id/publish        # 发布
POST   /api/v1/admin/knowledge-articles/:id/disable        # 停用
POST   /api/v1/admin/knowledge-articles/:id/enable         # 启用

# 文档处理状态
GET /api/v1/admin/knowledge-articles/:id/process-status    # 查询处理进度
POST /api/v1/admin/knowledge-articles/:id/retry            # 重试失败文档
```

## API 文档维护

所有 API 文档位于 `docs/API/` 目录，共 9 份 Markdown 文件。修改接口时需同步更新对应文档。文档使用标准格式：

- 接口 URL 和 HTTP 方法
- 认证要求
- 请求体/查询参数（含类型、必填、说明）
- 成功/错误响应示例
- 错误码说明

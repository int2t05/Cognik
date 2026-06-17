# OpsMind API 文档

> **Base URL:** `/api/v1`

## Overview

OpsMind 后端提供 RESTful JSON API，分为三组路由：

| Route Group | Prefix | Auth | Description |
|--------|------|----------|------|
| 公开 | `/api/v1/auth` | 无 | 登录、刷新令牌 |
| 门户端 | `/api/v1/portal` | JWT | 智能问答（SSE 流式 + RAG 管道）、申告提交、进度查询、站内消息 |
| 后台管理 | `/api/v1/admin` | JWT + RBAC | 申告处理、知识库管理（含文档上传）、LLM 配置、用户/角色、看板、审计 |

## Response Format

所有 API 响应使用统一 JSON 结构：

```json
{
  "code": 0,
  "message": "success",
  "data": { }
}
```

分页响应附加分页字段：

```json
{
  "code": 0,
  "message": "success",
  "data": [ ],
  "total": 100,
  "page": 1,
  "page_size": 10
}
```

## Error Codes

所有 API 响应通过 `code` 字段标识业务结果，HTTP 状态码反映传输层状态。

| Code | HTTP Status | Description |
|------|-------------|-------------|
| 0 | 200 | Success |
| 10001 | 401 | 未登录或令牌过期 |
| 10002 | 403 | 无权限 |
| 10003 | 400 | 参数校验失败 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突（如账号名重复） |
| 10006 | 400 | 用户已被冻结 |
| 10007 | 400 | 用户已处于正常状态 |
| 20001 | 503 | AI 服务不可用 |
| 20002 | 503 | RAG 服务不可用 |
| 20003 | 503 | 存储服务不可用 |
| 99999 | 500 | 未知错误 |

## Authentication

所有需要认证的接口需携带 JWT 令牌：

```http
Authorization: Bearer <access_token>
```

令牌通过 `/api/v1/auth/login` 获取，有效期 2 小时。过期后使用 `/api/v1/auth/refresh` 刷新。

## Pagination

支持分页的接口接受以下查询参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码（从 1 开始） |
| `page_size` | int | 10 | 每页条数（最大 100） |

## API Reference

| 文档 | 说明 |
|------|------|
| [auth.md](auth.md) | 认证接口（登录/刷新/登出/修改密码） |
| [chat.md](chat.md) | 智能问答接口（SSE 流式 + RAG 管道） |
| [tickets.md](tickets.md) | 申告管理接口（门户提交 + 后台处理） |
| [knowledge.md](knowledge.md) | 知识库管理接口（KB/文章/审核/发布/文档上传） |
| [llm-config.md](llm-config.md) | LLM 配置接口（llama.cpp / OpenAI-compatible） |
| [users.md](users.md) | 用户管理接口（CRUD + 冻结/恢复） |
| [roles.md](roles.md) | 角色与菜单管理接口 |
| [dashboard.md](dashboard.md) | 数据看板接口（统计 + 趋势） |
| [audit-log.md](audit-log.md) | 审计日志 + 系统配置 + 站内消息 |

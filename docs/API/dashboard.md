# 数据看板接口

> **Base URL:** `/api/v1/admin/dashboard` | **Auth:** JWT + `audit:read` | **Module:** Dashboard

## 1. 统计数据

```http
GET /api/v1/admin/dashboard/stats
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "today_tickets": 3,
    "pending_tickets": 2,
    "processing_tickets": 1,
    "resolved_tickets": 15,
    "today_chats": 42,
    "avg_confidence": 0.78,
    "knowledge_count": 25
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| today_tickets | int64 | 今日新增申告数 |
| pending_tickets | int64 | 待处理申告数（status=1） |
| processing_tickets | int64 | 处理中申告数（status=2） |
| resolved_tickets | int64 | 已解决申告数（status=4） |
| today_chats | int64 | 今日问答次数 |
| avg_confidence | float64 | 今日平均置信度 |
| knowledge_count | int64 | 知识条目总数 |

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 99999 | 500 | 服务器内部错误（数据库查询失败） |

---

## 2. 趋势数据

```http
GET /api/v1/admin/dashboard/trends?start_date=2026-06-01&end_date=2026-06-11&granularity=day
Authorization: Bearer <token>
```

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start_date | string | ✓ | 开始日期（YYYY-MM-DD） |
| end_date | string | ✓ | 结束日期（YYYY-MM-DD） |
| granularity | string | | 粒度：`day`（默认）或 `week` |

> granularity 参数当前保留但未生效，Service 始终按日聚合。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "data_points": [
      {
        "date": "2026-06-01",
        "ticket_count": 5,
        "chat_count": 23
      },
      {
        "date": "2026-06-02",
        "ticket_count": 3,
        "chat_count": 18
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| date | string | 日期 YYYY-MM-DD |
| ticket_count | int64 | 该日新增申告数 |
| chat_count | int64 | 该日问答数 |

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 参数校验失败（日期格式错误、结束日期早于开始日期） |
| 99999 | 500 | 服务器内部错误（数据库查询失败） |

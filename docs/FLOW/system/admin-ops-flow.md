# 管理后台数据流 — 每个 API 端点

> 涉及文件: `domain/system/dashboard/handler.go`, `domain/system/audit/handler.go`, `domain/system/config/handler.go`, `domain/system/message/handler.go`, `domain/system/dashboard/service.go`, `domain/system/audit/service.go`, `domain/system/config/service.go`, `domain/system/message/service.go`, `domain/system/audit/repository.go`, `domain/system/config/repository.go`, `domain/system/message/repository.go`, `shared/model/audit.go`, `shared/model/system.go`, `shared/model/message.go`

---

## 数据看板

### GET /api/v1/admin/dashboard/stats &emsp; 统计概览 &emsp; [PermDashboardRead]

```
DashboardHandler.GetStats (domain/system/dashboard/handler.go:30)
  → DashboardService.GetStats (domain/system/dashboard/service.go:42)
    ├─ dashboardRepo.CountTodayTickets → SELECT COUNT(*) FROM tickets WHERE DATE(created_at)=CURRENT_DATE
    ├─ dashboardRepo.CountByStatus(1) → Pending
    ├─ dashboardRepo.CountByStatus(2) → Processing
    ├─ dashboardRepo.CountByStatus(4) → Resolved
    ├─ dashboardRepo.CountTodayChats → SELECT COUNT(*) FROM chat_sessions WHERE DATE(created_at)=CURRENT_DATE
    ├─ dashboardRepo.AvgTodayConfidence → SELECT AVG(confidence) FROM chat_sessions WHERE DATE(created_at)=CURRENT_DATE
    └─ dashboardRepo.CountKnowledgeArticles → SELECT COUNT(*) FROM knowledge_articles WHERE status=4
```

**输出** `{today_tickets, pending, processing, resolved, today_chats, avg_confidence, total_articles}`

### GET /api/v1/admin/dashboard/trends &emsp; 趋势数据 &emsp; [PermDashboardRead]

**输入** `?start_date=2026-06-15&end_date=2026-06-22&granularity=day`

```
DashboardHandler.GetTrends (domain/system/dashboard/handler.go:43)
  → DashboardService.GetTrends (domain/system/dashboard/service.go:141)
    ├─ dashboardRepo.GetTicketTrends (repository 内部)
    │   → SELECT DATE(created_at), COUNT(*) FROM tickets WHERE DATE(created_at) BETWEEN ? AND ?
    │     GROUP BY DATE(created_at) ORDER BY 1
    └─ dashboardRepo.GetChatTrends (repository 内部)
        → SELECT DATE(created_at), COUNT(*) FROM chat_sessions WHERE DATE(created_at) BETWEEN ? AND ?
          GROUP BY DATE(created_at) ORDER BY 1
```

**输出** `{ticket_trends:[{date,count}...], chat_trends:[{date,count}...]}`

---

## 审计日志

### GET /api/v1/admin/audit-logs &emsp; 操作日志 &emsp; [PermAuditRead]

**输入** `?page=1&page_size=20&operator_id=1&action=user.create&target_type=user&date_from=2026-06-01&date_to=2026-06-22`

```
AuditHandler.List (domain/system/audit/handler.go:30)
  └─ parsePagination → page, pageSize
  → AuditService.List (domain/system/audit/service.go:24)
    └─ AuditRepo.List (domain/system/audit/repository.go:55)
        → SELECT COUNT(*) FROM audit_logs [WHERE 动态过滤]
        → SELECT * FROM audit_logs [WHERE ...] ORDER BY created_at DESC LIMIT ? OFFSET ?
```

**输出** `{items:[{id,user_id,username,action,resource_type,resource_id,detail,ip,created_at}...], total}`

---

## 系统配置

### GET /api/v1/admin/configs/:key &emsp; 获取配置 &emsp; [PermSystemConfig]

```
ConfigHandler.Get (domain/system/config/handler.go:29)
  → ConfigService.GetConfig (domain/system/config/service.go:55)
    ├─ validConfigKeys[key] → 白名单校验 (app_name / ai.top_k / ai.threshold)
    └─ ConfigRepo.GetByKey (domain/system/config/repository.go:27)
        → SELECT * FROM system_configs WHERE config_key=?
```

### PUT /api/v1/admin/configs/:key &emsp; 更新配置 &emsp; [PermSystemConfig]

**输入** `{"value":"Cognos 知识管理系统"}`

```
ConfigHandler.Update (domain/system/config/handler.go:51)
  → ConfigService.UpdateConfig (domain/system/config/service.go:80)
    ├─ validConfigKeys[key] → 白名单校验 (app_name: string / ai.top_k: number / ai.threshold: number)
    ├─ configKeyMeta.ValueType → 值类型校验（string/number）
    ├─ ConfigRepo.Upsert (domain/system/config/repository.go:37)
    │   → INSERT INTO system_configs (...) ON CONFLICT(config_key) DO UPDATE ...
    └─ AuditRepo.Create → "config.update"
```

---

## 站内消息

### GET /api/v1/portal/messages &emsp; 消息列表

**输入** `?page=1&page_size=20&is_read=false&type=ticket_supplement`

```
MessageHandler.ListMessages (domain/system/message/handler.go:35)
  └─ parsePagination → page, pageSize
  → MessageService.ListMessages (domain/system/message/service.go:90)
    └─ MessageRepo.ListByUser (domain/system/message/repository.go:34)
        → SELECT COUNT(*) FROM messages WHERE user_id=?
        → SELECT * FROM messages WHERE user_id=? [AND is_read=?] [AND type=?]
          ORDER BY created_at DESC LIMIT ? OFFSET ?
```

### PUT /api/v1/portal/messages/:id/read &emsp; 标记已读

```
MessageHandler.MarkAsRead (domain/system/message/handler.go:61)
  → MessageService.MarkAsRead (domain/system/message/service.go:101)
    ├─ MessageRepo.MarkAsRead (domain/system/message/repository.go:59)
    │   → UPDATE messages SET is_read=true WHERE id=? AND user_id=?
    └─ invalidateUnread → 清除 unread 缓存
```

### GET /api/v1/portal/messages/unread-count &emsp; 未读计数

```
MessageHandler.CountUnread (domain/system/message/handler.go:82)
  → MessageService.CountUnread (domain/system/message/service.go:132)
    ├─ getCachedUnread (domain/system/message/service.go 内部)
    │   → unreadCountCache 内存 map, TTL 可配 → 命中直接返回
    └─ 未命中:
        MessageRepo.CountUnread (domain/system/message/repository.go:71)
          → SELECT COUNT(*) FROM messages WHERE user_id=? AND is_read=false
        └─ setCachedUnread → 写入缓存
```

**消息类型**: `ticket_supplement`（申告补充信息通知）、`system`（系统通知）

---

## 设计要点

| 模块 | 要点 |
|------|------|
| Dashboard | Service 依赖 `dashboardRepo` 接口而非具体 Repo，统计实时查询无缓存 |
| Audit | 支持多维度过滤（用户/操作/资源/时间范围），日志只增不删 |
| Config | 静态白名单 `validConfigKeys` + 类型校验，防止注入未知 key |
| Message | 未读数内存缓存 TTL，读操作清除缓存，`NotifySupplement` 由 `TicketService.UpdateStatus` 同步调用 |

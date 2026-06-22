# 管理运维数据流 — 每个 API 端点

> 涉及文件: `handler/dashboard.go`, `handler/audit.go`, `handler/config.go`, `handler/message.go`, `service/dashboard_service.go`, `service/audit_service.go`, `service/config_service.go`, `service/message_service.go`, `repository/audit_repo.go`, `repository/config_repo.go`, `repository/message_repo.go`, `model/audit.go`, `model/config.go`, `model/message.go`

---

## 数据看板

### GET /api/v1/admin/dashboard/stats &emsp; 统计概览 &emsp; [PermDashboardRead]

```
DashboardHandler.GetStats (handler/dashboard.go:30)
  → DashboardService.GetStats (service/dashboard_service.go:42)
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
DashboardHandler.GetTrends (handler/dashboard.go:43)
  → DashboardService.GetTrends (service/dashboard_service.go:141)
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

**输入** `?page=1&page_size=20&user_id=1&action=user.create&resource_type=user&start_date=2026-06-01&end_date=2026-06-22`

```
AuditHandler.List (handler/audit.go:30)
  └─ parsePagination → page, pageSize
  → AuditService.List (service/audit_service.go:24)
    └─ AuditRepo.List (repository/audit_repo.go:55)
        → SELECT COUNT(*) FROM audit_logs [WHERE 动态过滤]
        → SELECT * FROM audit_logs [WHERE ...] ORDER BY created_at DESC LIMIT ? OFFSET ?
```

**输出** `{items:[{id,user_id,username,action,resource_type,resource_id,detail,ip,created_at}...], total}`

---

## 系统配置

### GET /api/v1/admin/configs/:key &emsp; 获取配置 &emsp; [PermSystemConfig]

```
ConfigHandler.Get (handler/config.go:29)
  → ConfigService.GetConfig (service/config_service.go:55)
    ├─ validConfigKeys[key] → 静态白名单校验
    │   (system_name, welcome_message, ticket_auto_close_days, ai_confidence_threshold, ai_default_top_k)
    └─ ConfigRepo.GetByKey (repository/config_repo.go:27)
        → SELECT * FROM system_configs WHERE config_key=?
```

### PUT /api/v1/admin/configs/:key &emsp; 更新配置 &emsp; [PermSystemConfig]

**输入** `{"value":"运维数字员工系统 v2"}`

```
ConfigHandler.Update (handler/config.go:51)
  → ConfigService.UpdateConfig (service/config_service.go:80)
    ├─ validConfigKeys[key] → 白名单校验
    ├─ configKeyMeta.typeValidation → 值类型校验（string/int/float）
    ├─ ConfigRepo.Upsert (repository/config_repo.go:37)
    │   → INSERT INTO system_configs (...) ON CONFLICT(config_key) DO UPDATE ...
    └─ AuditRepo.Create → "config.update"
```

---

## 站内消息

### GET /api/v1/portal/messages &emsp; 消息列表

**输入** `?page=1&page_size=20&is_read=false&type=ticket_supplement`

```
MessageHandler.ListMessages (handler/message.go:35)
  └─ parsePagination → page, pageSize
  → MessageService.ListMessages (service/message_service.go:90)
    └─ MessageRepo.ListByUser (repository/message_repo.go:34)
        → SELECT COUNT(*) FROM messages WHERE user_id=?
        → SELECT * FROM messages WHERE user_id=? [AND is_read=?] [AND type=?]
          ORDER BY created_at DESC LIMIT ? OFFSET ?
```

### PUT /api/v1/portal/messages/:id/read &emsp; 标记已读

```
MessageHandler.MarkAsRead (handler/message.go:61)
  → MessageService.MarkAsRead (service/message_service.go:101)
    ├─ MessageRepo.MarkAsRead (repository/message_repo.go:59)
    │   → UPDATE messages SET is_read=true WHERE id=? AND user_id=?
    └─ invalidateUnread → 清除 unread 缓存
```

### GET /api/v1/portal/messages/unread-count &emsp; 未读计数

```
MessageHandler.CountUnread (handler/message.go:82)
  → MessageService.CountUnread (service/message_service.go:132)
    ├─ getCachedUnread (service/message_service.go 内部)
    │   → unreadCountCache 内存 map, TTL 可配 → 命中直接返回
    └─ 未命中:
        MessageRepo.CountUnread (repository/message_repo.go:71)
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

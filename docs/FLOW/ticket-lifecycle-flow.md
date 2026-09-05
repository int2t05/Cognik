# Ticket 数据流 — 每个 API 端点

> 涉及文件: `domain/ticket/handler.go`, `domain/ticket/service.go`, `domain/knowledge/service.go`, `domain/system/message/service.go`, `infra/runtime/scheduler.go`, `infra/runtime/tx_manager.go`, `domain/ticket/repository.go`, `domain/system/audit/repository.go`, `domain/system/message/repository.go`, `shared/model/ticket.go`, `shared/model/message.go`, `shared/model/audit.go`

---

## 系统自动创建（source=3）

### 知识库复核工单

**触发场景**：
1. **文件上传**（`KnowledgeService.UploadDocuments`）→ 解析后创建草稿文章 → 自动创建 `【文档复核】` 工单
2. **LLM 元数据补全**（`KnowledgeService.republishFromApproved`）→ 发布时 `article_type` 缺失/非法 → LLM 推断 → 自动创建 `【元数据复核】` 工单

```
KnowledgeService.UploadDocuments / republishFromApproved
  → ticketCreator.CreateSystemTicket (domain/ticket/service.go:133)
    ├─ generateTicketNo → "TK-YYYYMMDD-" + crypto/rand(6位数)
    ├─ source = KnowledgeReview(3), user_id = 1(admin)
    ├─ contact_phone 留空（系统工单无联系电话）
    ├─ related_article_id / related_kb_id 关联文章
    ├─ TicketRepo.Create → INSERT INTO tickets (status=1 Pending)
    └─ TicketRepo.CreateRecord → action="auto_created", content=触发原因
```

> 系统工单从「待处理(1)」进入，状态机与门户工单一致，无联系电话，关联 article_id + kb_id。

---

## 门户端

### POST /api/v1/portal/tickets &emsp; 创建申告

**输入** `{"title":"数据库超时","description":"MySQL间歇性超时...","urgency":2,"impact_scope":2,`<br/>`"affected_systems":["mysql"],"contact_phone":"13800138000","chat_context":{...}}`

```
1. TicketHandler.CreateTicket (domain/ticket/handler.go:37)
   └─ c.ShouldBindJSON → request.CreateTicketRequest

2. TicketService.CreateTicket (domain/ticket/service.go:72)
   ├─ 参数校验: title/description/contact_phone 非空, urgency∈[1,3]
   ├─ generateTicketNo → "TK-YYYYMMDD-" + crypto/rand(6位数)
   ├─ marshalTicketTags(affectedSystems) → JSONB
   ├─ json.Marshal(chatContext) → JSONB
   └─ TicketRepo.Create (domain/ticket/repository.go:32)
       → INSERT INTO tickets (status=1 Pending, source=1 Portal)
```

**输出** `{code:0}`

### GET /api/v1/portal/tickets &emsp; 我的申告列表

```
TicketHandler.ListByUser (domain/ticket/handler.go:56)
  → TicketService.ListByUser (domain/ticket/service.go:349)
    └─ TicketRepo.ListByUser (domain/ticket/repository.go:72)
        → SELECT ... WHERE user_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?
```

### GET /api/v1/portal/tickets/:id &emsp; 申告详情

```
TicketHandler.GetDetail (domain/ticket/handler.go:119)
  → TicketService.GetDetail (domain/ticket/service.go:391)
    ├─ TicketRepo.FindByID (domain/ticket/repository.go:37)
    │   → Preload User + TicketRecords (ORDER BY created_at ASC)
    └─ portal: ticket.UserID != currentUserID → ErrForbidden
```

### PATCH /api/v1/portal/tickets/:id/supplement &emsp; 补充信息

**输入** `{"content":"报错日志: connect timeout after 30s"}`

```
TicketHandler.SupplementTicket (domain/ticket/handler.go:72)
  → TicketService.SupplementTicket (domain/ticket/service.go:140)
    ├─ TicketRepo.FindByID → 归属校验 (UserID != current → 403)
    ├─ Status != NeedSupplement(3) → 拒绝
    └─ GormTxManager.Transaction (infra/runtime/tx_manager.go:32):
        ├─ TicketRepo.CreateRecord → INSERT ticket_records (action="supplement")
        └─ TicketRepo.UpdateStatus (domain/ticket/repository.go:57)
            → UPDATE tickets SET status=2 WHERE id=? AND status=3 (CAS)
            → RowsAffected==0 → "状态已变更，请刷新"
```

---

## 后台管理端

### GET /api/v1/admin/tickets &emsp; 全部申告 &emsp; [PermTicketRead]

```
TicketHandler.ListAll (domain/ticket/handler.go:101)
  → TicketService.ListAll (domain/ticket/service.go:369)
    └─ TicketRepo.ListAll (domain/ticket/repository.go:91)
        → 分页 + status/urgency 可选过滤 + 批量用户姓名查询
```

### GET /api/v1/admin/tickets/:id &emsp; 申告详情 &emsp; [PermTicketRead]

```
TicketHandler.GetDetail → TicketService.GetDetail
  └─ admin 模式: 不校验所有权（getDetailID=0）
```

### PATCH /api/v1/admin/tickets/:id/status &emsp; 状态转换 &emsp; [PermTicketWrite]

**输入** `{"action":"start","result":"已接单，开始排查"}`

```
TicketHandler.UpdateStatus (domain/ticket/handler.go:144)
  → TicketService.UpdateStatus (domain/ticket/service.go:200)
    ├─ TicketRepo.FindByID → 加载申告
    ├─ switch-case 状态机:
    │   ├─ start:      Pending(1) → Processing(2)
    │   ├─ request_info: Processing(2) → NeedSupplement(3)
    │   │   └─ TicketRepo.IncrementSupplementCount (domain/ticket/repository.go:65)
    │   │       → UPDATE SET supplement_count+1 WHERE id=? AND supplement_count<3
    │   │       → 超限拒绝 "已达上限(3次)"
    │   ├─ resolve:    Processing(2) → Resolved(4)
    │   └─ close:      Pending/Processing/NeedSupplement → Closed(5)
    │                   (Resolved/Closed 不可关闭)
    │
    └─ GormTxManager.Transaction:
        ├─ TicketRepo.UpdateStatus → CAS: WHERE id=? AND status=?
        │   → RowsAffected==0 → 并发冲突
        ├─ TicketRepo.CreateRecord → INSERT ticket_records
        └─ AuditRepo.Create → INSERT audit_logs

附加逻辑 — request_info:
  MessageService.NotifySupplement (domain/system/message/service.go:64)
    └─ MessageRepo.Create → INSERT messages (type="ticket_supplement")
```

### POST /api/v1/admin/tickets/:id/records &emsp; 添加处理记录 &emsp; [PermTicketWrite]

**输入** `{"action":"note","content":"已联系用户确认","detail":"{...}"}`

```
TicketHandler.AddRecord (domain/ticket/handler.go:169)
  → TicketService.AddRecord (domain/ticket/service.go:311)
    ├─ isValidRecordAction → 白名单: note/callback/escalate
    ├─ isValidJSON(detail) → 可选 JSON 校验
    └─ TicketRepo.CreateRecord → INSERT ticket_records (不更新状态)
```

### POST /api/v1/admin/tickets/:id/knowledge-candidate &emsp; 生成知识候选 &emsp; [PermTicketWrite]

**输入** `{"kb_id":1}`

```
TicketHandler.CreateKnowledgeCandidate (domain/ticket/handler.go:198)
  → TicketService.CreateKnowledgeCandidate (domain/ticket/service.go:547)
    ├─ GetDetail(ctx, id, 0) → 管理员模式加载
    ├─ 拼接: title="申告经验 - {title}", content="问题描述：{title}\n\n解决方案：{description}"
    └─ KnowledgeService.CreateArticle → 草稿状态(1), 同手动创建
```

---

## 定时任务: 自动关闭超期申告

```
Scheduler.Start (infra/runtime/scheduler.go:35)
  → goroutine: 首次立即执行, 之后每 1h

TicketService.AutoClose (domain/ticket/service.go:499)
  └─ GormTxManager.Transaction:
      ├─ TicketRepo.AutoCloseTickets (domain/ticket/repository.go:137)
      │   → UPDATE tickets SET status=5 WHERE status IN(1,2,3) AND created_at<?
      │       RETURNING id
      ├─ for each closedID: TicketRepo.CreateRecord (action="auto_close", operatorID=0)
      └─ AuditRepo.Create (action="ticket.auto_close", operatorID=0)
```

---

## 状态机

```
Pending(1) ── start ──→ Processing(2) ── resolve ──→ Resolved(4)
    │                        │  │
    │                        │  ├── request_info ──→ NeedSupplement(3)
    │                        │  │                      │
    │                        │  │   ←── supplement ────┘
    │                        │  │       (CAS: 3→2)
    │                        │  └── close ──→ Closed(5)
    │                        │
    ├── close ──→ Closed(5)  │
    └── (auto_close 7天) ──→ Closed(5)
```

## 关键设计

| 机制 | 实现 |
|------|------|
| CAS 更新 | `UPDATE ... WHERE id=? AND status=?` → RowsAffected==0 检测并发冲突 |
| 补充次数 | 原子自增 `WHERE supplement_count<3`, 数据库级别 |
| 事务边界 | UpdateStatus + CreateRecord + AuditLog 同事务, 任一失败全回滚 |
| 消息通知 | request_info 成功后同步调用 MessageService.NotifySupplement |

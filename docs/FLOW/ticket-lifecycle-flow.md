# 申告生命周期数据流 — 从创建到关闭

> **聚焦：后端数据逻辑。不含前端调用链和用户故事。**

---

## 1. 路由注册

```
router.Setup() → registerPortalRoutes():
  POST   /api/v1/portal/tickets            → TicketHandler.CreateTicket
  GET    /api/v1/portal/tickets            → TicketHandler.ListByUser
  GET    /api/v1/portal/tickets/:id        → TicketHandler.GetDetail
  PATCH  /api/v1/portal/tickets/:id/supplement → TicketHandler.SupplementTicket

router.Setup() → registerAdminRoutes():
  GET    /api/v1/admin/tickets             → TicketHandler.ListAll        [PermTicketRead]
  GET    /api/v1/admin/tickets/:id         → TicketHandler.GetDetail      [PermTicketRead]
  PATCH  /api/v1/admin/tickets/:id/status  → TicketHandler.UpdateStatus   [PermTicketWrite]
  POST   /api/v1/admin/tickets/:id/records → TicketHandler.AddRecord      [PermTicketWrite]
  POST   /api/v1/admin/tickets/:id/knowledge-candidate → TicketHandler.CreateKnowledgeCandidate [PermTicketWrite]
```

---

## 2. 申告状态机

```
状态码: 1=待处理  2=处理中  3=需补充信息  4=已解决  5=已关闭

                  ┌── CreateTicket ──→ Pending(1)
                  │                    │
                  │                    ├── start ──→ Processing(2)
                  │                    │            │  │
                  │                    │            │  ├── request_info ──→ NeedSupplement(3)
                  │                    │            │  │  (supplement_count < 3)
                  │                    │            │  │                    │
                  │                    │            │  │←── supplement ─────┘
                  │                    │            │  │     (CAS: status=3→2)
                  │                    │            │  │
                  │                    │            │  ├── resolve ──→ Resolved(4)
                  │                    │            │  │
                  │                    │            │  └── close ──→ Closed(5)
                  │                    │            │
                  │                    │            └── close ──→ Closed(5)
                  │                    │
                  │                    └── close ──→ Closed(5)

Pending/Processing/NeedSupplement ── 超过7天 ──→ Scheduler.AutoClose → Closed(5)
```

---

## 3. 创建申告 — POST /api/v1/portal/tickets

### Handler: `TicketHandler.CreateTicket(c)`

```
1. c.ShouldBindJSON(&request.CreateTicketRequest{
     Title, Description, Urgency, ImpactScope, AffectedSystems,
     ContactPhone, ContactEmail, ChatContext
   })
2. getCurrentUserID(c) → userID
3. h.svc.CreateTicket(ctx, req, userID)
```

### Service: `TicketService.CreateTicket(ctx, req, userID)`

```
1. 参数校验:
   ├─ TrimSpace(Title) == ""        → "标题不能为空"
   ├─ TrimSpace(Description) == ""  → "描述不能为空"
   ├─ TrimSpace(ContactPhone) == "" → "联系电话不能为空"
   └─ Urgency < 1 || Urgency > 3    → "紧急程度必须为1-3"

2. generateTicketNo() — 工单编号 "TK-YYYYMMDD-XXXXXX":
   ├─ crypto/rand.Int(rand.Reader, big.NewInt(1000000)) → 6位随机数
   └─ fmt.Sprintf("TK-%s-%06d", time.Now().Format("20060102"), n)

3. marshalTicketTags(AffectedSystems) → JSONB

4. json.Marshal(ChatContext) → JSONB

5. 构建 Ticket{UserID, TicketNo, Title, Description, Urgency, ImpactScope,
     AffectedSystems(JSONB), ContactPhone, ContactEmail, ChatContext(JSONB),
     Status:1(Pending), Source:1(Portal)}

6. TicketRepo.Create(ctx, ticket)
   → SQL: INSERT INTO tickets (...) VALUES (...)
```

---

## 4. 状态转换 — PATCH /api/v1/admin/tickets/:id/status

### Handler: `TicketHandler.UpdateStatus(c)`

```
1. parseID(c, "id") → ticketID
2. c.ShouldBindJSON(&request.UpdateTicketStatusRequest{Action, Result})
3. getCurrentUserID(c) → operatorID
4. h.svc.UpdateStatus(ctx, ticketID, operatorID, req)
```

### Service: `TicketService.UpdateStatus(ctx, id, operatorID, req)`

```
1. repo.FindByID(ctx, id) — 加载申告（Preload User + TicketRecords）

2. switch-case 状态机校验:

   action="start":       Status(A)=1(Pending)  → Status(B)=2(Processing)
   action="request_info": Status(A)=2(Processing) → Status(B)=3(NeedSupplement)
     附加: repo.IncrementSupplementCount(ctx, id)
       → SQL: UPDATE tickets SET supplement_count = supplement_count + 1
           WHERE id=? AND supplement_count < 3
       → RowsAffected==0 → "补充信息次数已达上限（3次）"
   action="resolve":     Status(A)=2(Processing) → Status(B)=4(Resolved)
   action="close":       Status(A)∈{1,2,3} → Status(B)=5(Closed)
                         (不可从 Resolved 或已 Closed 关闭)

3. txManager.Transaction(ctx, func(tx) {
     txRepo := repository.NewTicketRepo(tx)

     a. txRepo.UpdateStatus(ctx, id, oldStatus, newStatus) — CAS 原子更新
        → SQL: UPDATE tickets SET status=? WHERE id=? AND status=?
        → RowsAffected==0 → "申告状态已变更，请刷新后重试"（并发冲突）

     b. txRepo.CreateRecord(ctx, &TicketRecord{
          TicketID:id, OperatorID:operatorID, Action:req.Action, Content:req.Result
        })
        → SQL: INSERT INTO ticket_records (...)

     c. repository.NewAuditRepo(tx).Create(ctx, &AuditLog{
          OperatorID:operatorID, Action:"ticket."+req.Action, TargetType:"ticket", TargetID:id
        })
   })

4. request_info 特殊逻辑 — 事务完成后:
   if req.Action == "request_info" && msgSvc != nil:
     msgSvc.NotifySupplement(ctx, ticketID, ticket.UserID, ticket.Title)
       → MessageRepo.Create(ctx, &Message{
           UserID, Title:"申告需补充信息",
           Content:"您的申告「{title}」需要补充更多信息...",
           Type:"ticket_supplement", RelatedType:"ticket", RelatedID:ticketID, IsRead:false
         })
       → invalidateUnread(userID) — 清除未读数缓存
```

### 状态转换矩阵

| action | 前置状态 | 后置状态 | 额外条件 |
|--------|---------|---------|---------|
| `start` | Pending(1) | Processing(2) | — |
| `request_info` | Processing(2) | NeedSupplement(3) | supplement_count < 3 |
| `resolve` | Processing(2) | Resolved(4) | — |
| `close` | Pending(1)/Processing(2)/NeedSupplement(3) | Closed(5) | 不可从 Resolved/Closed 关闭 |

---

## 5. 补充信息 — PATCH /api/v1/portal/tickets/:id/supplement

### Service: `TicketService.SupplementTicket(ctx, id, userID, req)`

```
1. repo.FindByID(ctx, id) — 加载申告

2. 归属校验: ticket.UserID != userID → ErrForbidden

3. 状态校验: ticket.Status != NeedSupplement(3) → 拒绝

4. txManager.Transaction(ctx, func(tx) {
     txRepo := repository.NewTicketRepo(tx)

     a. txRepo.CreateRecord(ctx, &TicketRecord{Action:"supplement", Content:req.Content})
        → SQL: INSERT INTO ticket_records (...)

     b. txRepo.UpdateStatus(ctx, id, 3, 2) — CAS: NeedSupplement → Processing
        → SQL: UPDATE tickets SET status=2 WHERE id=? AND status=3
        → RowsAffected==0 → "申告状态已变更，请刷新后重试"
   })
```

---

## 6. 添加处理记录 — POST /api/v1/admin/tickets/:id/records

### Service: `TicketService.AddRecord(ctx, id, operatorID, req)`

```
1. isValidRecordAction(req.Action) — 白名单: note / callback / escalate

2. repo.FindByID(ctx, id) — 校验申告存在

3. req.Detail != "" → isValidJSON(req.Detail) — JSON 合法性校验

4. repo.CreateRecord(ctx, &TicketRecord{
     TicketID:id, OperatorID:operatorID,
     Action:req.Action, Content:req.Content, Detail:req.Detail
   })
   → SQL: INSERT INTO ticket_records (...)
   → 注意: 不更新申告状态
```

---

## 7. 自动关闭超期申告 — Scheduler

### 触发: `Scheduler.Start(ctx)`

```
1. sync.Once 保护，防止重复启动
2. 启动 goroutine runAutoCloseLoop:
   ├─ 立即执行一次 doAutoClose()
   └─ 之后每 1 小时执行一次 (time.NewTicker)
```

### 核心: `TicketService.AutoClose(ctx, olderThan=7天前)`

```
1. txManager.Transaction(ctx, func(tx) {
     txRepo := repository.NewTicketRepo(tx)

     a. txRepo.AutoCloseTickets(ctx, olderThan)
        → SQL: UPDATE tickets SET status=5
            WHERE status IN (1,2,3) AND created_at < $1
            RETURNING id

     b. 遍历 closed IDs，创建 auto_close 记录
        → SQL: INSERT INTO ticket_records (ticket_id, operator_id=0, action='auto_close', ...)

     c. repository.NewAuditRepo(tx).Create(ctx, &AuditLog{
          OperatorID:0, Action:"ticket.auto_close", TargetType:"ticket", TargetID:id
        })
   })

2. 返回 closedCount
```

---

## 8. 创建知识候选 — POST /api/v1/admin/tickets/:id/knowledge-candidate

### Service: `TicketService.CreateKnowledgeCandidate(ctx, id, kbID, userID)`

```
1. GetDetail(ctx, id, 0) — 管理员模式不限制所有权

2. 拼接内容:
   title = "申告经验 - " + ticket.Title
   content = "问题描述：" + ticket.Title + "\n\n解决方案：" + ticket.Description

3. kbSvc.CreateArticle(ctx, CreateArticleRequest{kbID, title, content}, userID)
   └─ 与手动创建文章相同路径 → 草稿状态(status=1)
   └─ 走 KnowledgeService.CreateArticle 标准流程
```

---

## 9. 关键数据表

### `tickets`

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | |
| ticket_no | VARCHAR UNIQUE INDEX | TK-YYYYMMDD-XXXXXX |
| user_id | BIGINT FK→users | 创建人 |
| title | VARCHAR | |
| description | TEXT | |
| urgency | INT | 1=低 2=中 3=高 |
| impact_scope | INT | 1=个人 2=部门 3=全公司 |
| affected_systems | JSONB | ["mysql","app-server"] |
| contact_phone | VARCHAR | |
| contact_email | VARCHAR | |
| status | INT INDEX | 1/2/3/4/5 |
| supplement_count | INT | 索要补充次数（上限3） |
| chat_context | JSONB | {session_id, kb_id} |
| source | INT | 1=门户 2=后台 |
| created_at | TIMESTAMP INDEX | |
| updated_at | TIMESTAMP | |

### `ticket_records`

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | |
| ticket_id | BIGINT FK INDEX | |
| operator_id | BIGINT | 0=系统 |
| action | VARCHAR | create/start/request_info/supplement/resolve/close/auto_close/note/callback/escalate |
| content | TEXT | |
| detail | JSONB | |
| created_at | TIMESTAMP | |

### `messages`（站内通知）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | |
| user_id | BIGINT INDEX | |
| title | VARCHAR | |
| content | TEXT | |
| type | VARCHAR | "ticket_supplement" |
| related_type | VARCHAR | "ticket" |
| related_id | BIGINT | |
| is_read | BOOL INDEX | DEFAULT false |
| created_at | TIMESTAMP | |

---

## 10. 关键设计决策

### CAS (Compare-And-Swap) 状态更新

所有状态转换使用 `WHERE id=? AND status=?` 条件更新：
- `RowsAffected == 0` → 并发冲突 → "申告状态已变更，请刷新后重试"
- 防止 start/close 等并发操作导致状态混乱

### 补充次数原子自增

```sql
UPDATE tickets SET supplement_count = supplement_count + 1
WHERE id = ? AND supplement_count < 3
```
- 数据库级别原子操作，无需应用层锁
- 超限时 `RowsAffected == 0`

### 统一事务边界

所有状态转换 + 记录创建 + 审计日志在同一事务内完成，任一步骤失败全部回滚。

### 消息通知同步执行

`request_info` 触发 `msgSvc.NotifySupplement` 是同步的：
- 消息写入是轻量单行 INSERT
- 同步执行保证事务一致性
- MVP 阶段避免异步消息队列

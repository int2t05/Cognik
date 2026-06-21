# 申告生命周期数据流 — 从创建到关闭

---

## 用户故事

| 角色 | 目标 | 价值 |
|------|------|------|
| 报障人 | 提交运维申告，描述问题、选择紧急程度 | 从 AI 问答无法解决时转入人工处理 |
| 报障人 | 查看申告处理进度，补充运维要求的信息 | 配合运维人员快速解决问题 |
| 报障人 | 查看站内消息通知 | 及时响应运维的补充信息请求 |
| 运维人员 | 查看申告列表，接单处理 | 高效分配和跟踪运维工作 |
| 运维人员 | 更新申告状态（处理中→请求补充→已解决→关闭） | 标准化申告处理流程 |
| 运维人员 | 将申告经验沉淀为知识候选 | 闭环：申告→知识→更好的 AI 回答 |
| 系统 | 自动关闭 7 天无更新的超期申告 | 保持申告列表干净，避免僵尸申告 |

---

## 前端调用链路

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           前端组件 → API 映射                                   │
├────────────────────┬───────────────────────────┬──────────────────────────────┤
│        页面         │          组件              │          API 调用             │
├────────────────────┼───────────────────────────┼──────────────────────────────┤
│ /portal/tickets    │ PortalTicketsPage         │ GET /portal/tickets          │
│ (我的申告)          │  ├─ AppleTable 申告列表    │  → getMyTickets(page)        │
│                    │  │   → apiFetchPage<Ticket>│  → apiFetchPage<Ticket>      │
│                    │  └─ 点击行 → 跳转详情       │                              │
│                    │                           │                              │
│ /portal/tickets    │ PortalTicketNewPage       │ POST /portal/tickets         │
│ /new               │  ├─ AppleInput: title     │  → createTicket(data)        │
│ (提交申告)          │  ├─ AppleTextarea: desc   │   {title, description,       │
│                    │  ├─ urgency 选择器          │    urgency, impact_scope,    │
│                    │  ├─ impact_scope 选择器     │    affected_systems,         │
│                    │  ├─ affected_systems 多选   │    contact_phone,            │
│                    │  ├─ contact_phone/email    │    contact_email,            │
│                    │  ├─ AppleButton 提交        │    chat_context}             │
│                    │  └─ 支持 ?chat_context     │                              │
│                    │     查询参数（从 Chat 跳转） │                              │
│                    │                           │                              │
│ /portal/tickets    │ PortalTicketDetailPage    │ GET /portal/tickets/:id      │
│ /[id]              │  ├─ 申告信息展示            │  → getTicketDetail(id)       │
│ (申告详情)          │  ├─ 处理记录时间线          │                              │
│                    │  ├─ 补充信息表单             │ PATCH /portal/tickets/:id    │
│                    │  │   AppleTextarea + 提交   │   /supplement                │
│                    │  │   → supplementTicket()  │  → supplementTicket(id,text) │
│                    │  └─ (仅 status=3 时显示)    │                              │
│                    │                           │                              │
│ /portal/messages   │ PortalMessagesPage        │ GET /portal/messages         │
│ (站内消息)          │  ├─ AppleTable 消息列表    │  → apiFetchPage<Message>     │
│                    │  ├─ "查看" 按钮 → 跳转申告  │ PUT /portal/messages/:id     │
│                    │  └─ 标记已读               │   /read                      │
│                    │                           │                              │
├────────────────────┼───────────────────────────┼──────────────────────────────┤
│ /admin/tickets     │ AdminTicketsPage          │ GET /admin/tickets           │
│ (申告管理)          │  ├─ 状态筛选 pill 按钮      │  → listAllTickets(           │
│                    │  │   全部/待处理/处理中/...  │     page, status, urgency)   │
│                    │  ├─ AppleTable 申告列表    │  → apiFetchPage<Ticket>      │
│                    │  │   含提交人/紧急度/状态    │                              │
│                    │  └─ 点击行 → 跳转详情       │                              │
│                    │                           │                              │
│ /admin/tickets     │ AdminTicketDetailPage     │ GET /admin/tickets/:id       │
│ /[id]              │  ├─ 申告详情 + 状态徽章     │  → getAdminTicketDetail(id)  │
│ (申告处理)          │  ├─ 操作按钮组              │ PATCH /admin/tickets/:id     │
│                    │  │   "接单" → start        │   /status                    │
│                    │  │   "请求补充" → req_info  │  → updateTicketStatus(       │
│                    │  │   "已解决" → resolve     │     id, action, result)      │
│                    │  │   "关闭" → close        │                              │
│                    │  ├─ 处理记录表单             │ POST /admin/tickets/:id      │
│                    │  │   AppleTextarea + 提交   │   /records                   │
│                    │  │   → addTicketRecord()   │  → addTicketRecord(          │
│                    │  │                          │     id, action, content)     │
│                    │  └─ 知识候选生成             │ POST /admin/tickets/:id      │
│                    │      KB 下拉 + 生成按钮      │   /knowledge-candidate       │
│                    │      → createKnowledge     │  → createKnowledgeCandidate( │
│                    │        Candidate()         │     id, kb_id)               │
└────────────────────┴───────────────────────────┴──────────────────────────────┘
```

---

## 输入
```
POST /api/v1/portal/tickets
Authorization: Bearer <user_jwt>
{
  "title": "数据库连接超时",
  "description": "生产环境 MySQL 频繁超时，ping 正常但业务查询间歇性失败",
  "urgency": 2,
  "impact_scope": 2,
  "affected_systems": ["mysql", "app-server"],
  "contact_phone": "13800138000",
  "contact_email": "user@example.com",
  "chat_context": { "session_id": 100, "kb_id": 1 }
}
```

---

## 分层数据流

### 0. 路由 & 中间件

1. `router.Setup()` → `registerPortalRoutes()` 注册 `POST /tickets` → `TicketHandler.CreateTicket`
2. `middleware.JWTAuth(userCache, jwtSecret)` — JWT 认证

### 接入层 — Handler

3. 经由 `TicketHandler.CreateTicket(c)` 处理：
   - `c.ShouldBindJSON(&req)` → `request.CreateTicketRequest`
   - `getCurrentUserID(c)` — 提取申告人 ID
   - `h.svc.CreateTicket(c.Request.Context(), req, userID)`

### 业务层 — Service

4. 经由 `TicketService.CreateTicket(ctx, req, userID)` 处理：
   - 参数校验：
     - `strings.TrimSpace(req.Title) == ""` → "标题不能为空"
     - `strings.TrimSpace(req.Description) == ""` → "描述不能为空"
     - `strings.TrimSpace(req.ContactPhone) == ""` → "联系电话不能为空"
     - `req.Urgency < 1 || req.Urgency > 3` → "紧急程度必须为 1-3"
   - `generateTicketNo()` — 生成唯一工单编号 `TK-YYYYMMDD-NNNNNN`
     - `crypto/rand.Int(rand.Reader, big.NewInt(1000000))` 生成 6 位随机数
     - `fmt.Sprintf("TK-%s-%06d", time.Now().Format("20060102"), n)` 拼接
   - `marshalTicketTags(req.AffectedSystems)` — JSON 序列化受影响的系统
   - `json.Marshal(req.ChatContext)` — 序列化对话上下文
   - 设置 `ticket.Status = TicketStatusPending(1)`, `ticket.Source = TicketSourcePortal(1)`

### 数据层

5. `TicketRepo.Create(ctx, ticket)` — INSERT INTO tickets

### 输出
```json
{ "code": 0, "message": "success", "data": null }
```

---

## 申告状态机全路径

```
┌──────────────────────────────────────────────────────────────────┐
│  状态码：1=待处理  2=处理中  3=需补充信息  4=已解决  5=已关闭    │
│  角色： 报障人(portal) / 运维人员(admin) / 调度器(system)         │
└──────────────────────────────────────────────────────────────────┘

                    ┌── 创建申告 ──→ Pending(1)
                    │               │
                    │               ├─ start ──→ Processing(2)
                    │               │            │  │
                    │               │            │  ├─ request_info ──→ NeedSupplement(3)
                    │               │            │  │  (supplement_count < 3)
                    │               │            │  │                    │
                    │               │            │  │←── supplement ─────┘
                    │               │            │  │     (CAS: status=3→2)
                    │               │            │  │
                    │               │            │  ├─ resolve ──→ Resolved(4)
                    │               │            │  │
                    │               │            │  └─ close ──→ Closed(5)
                    │               │            │     (不可从 Resolved 关闭)
                    │               │            │
                    │               │            └─ close ──→ Closed(5)
                    │               │
                    │               └─ close ──→ Closed(5)

Pending/Processing/NeedSupplement ── 超过 7 天 ──→ Scheduler.AutoClose → Closed(5)
```

---

## 操作流：补充信息 (NeedSupplement → Processing)

### 输入
```
PATCH /api/v1/portal/tickets/:id/supplement
{ "content": "报错日志见附件，错误码 111：connect timeout after 30s" }
```

### 分层数据流

1. `TicketHandler.SupplementTicket(c)`
2. `TicketService.SupplementTicket(ctx, id, userID, req)`
   - `repo.FindByID(ctx, id)` — 加载申告
   - `ticket.UserID != userID` — 仅申告人可补充
   - `ticket.Status != TicketStatusNeedSupplement(3)` — 状态校验
   - `txManager.Transaction(ctx, func(tx) { ... })` — 事务内原子操作：
     - `repository.NewTicketRepo(tx)` — 在事务内创建 Repo
     - `txRepo.CreateRecord(ctx, &TicketRecord{Action:"supplement", Content:req.Content})` — 写入记录
     - `txRepo.UpdateStatus(ctx, id, oldStatus=3, newStatus=2)` — CAS 更新（WHERE id=? AND status=?）
     - `rows == 0` → "申告状态已变更，请刷新后重试"（并发冲突提示）

### 输出
```json
{ "code": 0, "message": "success", "data": null }
```

---

## 操作流：状态转换 (Admin)

### 输入
```
PATCH /api/v1/admin/tickets/:id/status
{ "action": "start", "result": "已接单，开始排查" }
```
> action 取值：start / request_info / resolve / close

### 分层数据流

1. `TicketHandler.UpdateStatus(c)` → `TicketService.UpdateStatus(ctx, id, operatorID, req)`
   - `repo.FindByID(ctx, id)` — 加载申告
   - switch-case 状态机校验（见下方表）
   - `txManager.Transaction(ctx, func(tx) { ... })` — 事务内原子操作：
     - `txRepo.UpdateStatus(ctx, id, oldStatus, newStatus)` — CAS 更新
     - `txRepo.CreateRecord(ctx, &TicketRecord{Action, Content:req.Result})` — 写入时间线
     - `repository.NewAuditRepo(tx).Create(ctx, &AuditLog{...})` — 审计日志

   **特殊逻辑 — request_info**：
   - 事务外：`repo.IncrementSupplementCount(ctx, id)` 原子自增
     - `UPDATE tickets SET supplement_count = supplement_count + 1 WHERE id=? AND supplement_count < 3`
     - `ok == false` → "补充信息次数已达上限（3次）"
   - 事务完成后：`msgSvc.NotifySupplement(ctx, ticketID, ticketUserID, ticketTitle)` 站内消息通知

### 状态转换矩阵

| action | 前置状态 | 后置状态 | 说明 |
|--------|----------|----------|------|
| `start` | Pending(1) | Processing(2) | 运维接单 |
| `request_info` | Processing(2) | NeedSupplement(3) | 请求补充（需 supplement_count < 3） |
| `resolve` | Processing(2) | Resolved(4) | 已解决 |
| `close` | Pending(1)/Processing(2)/NeedSupplement(3) | Closed(5) | 关闭（不可从 Resolved 关闭） |

### 输出
```json
{ "code": 0, "message": "success", "data": null }
```

---

## 操作流：补充信息通知

### 触发条件
`UpdateStatus` 中 action=`request_info` 成功且 `msgSvc != nil`

### 数据流
1. `MessageService.NotifySupplement(ctx, ticketID, userID, ticketTitle)`
2. 构建站内消息：`"您的申告「{title}」需要补充信息，请前往申告详情页查看并补充。"`
3. `MessageRepo.Create(ctx, &Message{UserID, Title, Content, Type:"ticket_supplement"})`
   - INSERT INTO messages

---

## 定时任务：自动关闭超期申告

### 触发
`Scheduler.Start(ctx)` 在 `app.run()` 中启动，周期性调用

### 数据流
1. `Scheduler.Start(ctx)` — 启动 goroutine + ticker
2. 每次 tick 调用 `TicketService.AutoClose(ctx, olderThan=7天前)`
3. `txManager.Transaction(ctx, func(tx) { ... })` — 事务内原子操作：
   - `txRepo.AutoCloseTickets(ctx, olderThan)` — UPDATE tickets SET status=5 WHERE status IN (1,2,3) AND created_at < ?
   - 遍历 closed IDs：创建 `TicketRecord{Action:"auto_close", OperatorID:0}`
   - `repository.NewAuditRepo(tx).Create(ctx, &AuditLog{Action:"ticket.auto_close", OperatorID:0})`
4. 返回 `closedCount`

---

## 操作流：从申告创建知识候选

### 输入
```
POST /api/v1/admin/tickets/:id/knowledge-candidate
{ "kb_id": 1 }
```

### 数据流
1. `TicketHandler.CreateKnowledgeCandidate(c)` → `TicketService.CreateKnowledgeCandidate(ctx, id, kbID, userID)`
2. `GetDetail(ctx, id, 0)` — 管理员模式不限制所有权
3. 拼接内容：`"问题描述：{title}\n\n解决方案：{description}"`
4. `kbSvc.CreateArticle(ctx, CreateArticleRequest{kbID, "申告经验 - {title}", answer}, userID)`
   - 与手动创建文章相同路径（草稿状态）

---

## 操作流：添加处理记录

### 输入
```
POST /api/v1/admin/tickets/:id/records
{ "action": "note", "content": "已联系用户确认影响范围", "detail": "{...}" }
```

### 数据流
1. `TicketHandler.AddRecord(c)` → `TicketService.AddRecord(ctx, id, operatorID, req)`
2. `isValidRecordAction(req.Action)` — 白名单校验（note/callback/escalate）
3. `repo.FindByID(ctx, id)` — 校验申告存在
4. `isValidJSON(req.Detail)` — JSON 合法性校验
5. `repo.CreateRecord(ctx, &TicketRecord{...})` — 写入 ticket_records（不更新状态）

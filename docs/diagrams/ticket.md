# 申告管理

> 覆盖申告创建、状态机转换、补充信息、自动关闭、编号生成全生命周期。

---

## 1. 端到端生命周期

```mermaid
flowchart TB
    subgraph Create["1. 创建申告"]
        C1["TicketHandler.CreateTicket<br/>handler/ticket.go<br/>c.ShouldBindJSON → getCurrentUserID"]
        C2["TicketService.CreateTicket(req, userID)<br/>└─ 必填校验: title/description/contact_phone<br/>└─ urgency ∈ [1,3]<br/>└─ ticket_no = 'TK-YYYYMMDD-XXXX'<br/>    TicketRepo.CountTodayTickets + fmt.Sprintf('%04d', count+1)<br/>└─ TicketRepo.Create(&Ticket{Status:Pending(1), Source:Portal(1)})"]
        C3[("INSERT INTO tickets<br/>status=1")]
    end

    subgraph Process["2. 处理阶段 — TicketService.UpdateStatus"]
        P1["PATCH /admin/tickets/:id/status {action}"]
        P2["TicketService.UpdateStatus(id, operatorID, req)<br/>└─ TicketRepo.FindByID → 当前状态<br/>└─ switch req.Action → 状态机校验"]
        P3["GormTxManager.Transaction:<br/>UpdateStatus + CreateRecord"]
    end

    subgraph Actions["状态动作"]
        A1["start: Pending(1)→Processing(2)"]
        A2["request_info: Processing(2)→NeedSupplement(3)<br/>└─ supplement_count < 3 校验<br/>└─ IncrementSupplementCount<br/>└─ MessageService.NotifySupplement → INSERT messages"]
        A3["resolve: Processing(2)→Resolved(4)<br/>└─ 含 detail + 可选 knowledge_candidate"]
        A4["close: 任意→Closed(5)"]
    end

    subgraph Supplement["3. 补充信息 — TicketService.SupplementTicket"]
        S1["PATCH /portal/tickets/:id/supplement"]
        S2["SupplementTicket(id, userID, req)<br/>└─ 校验: userID==owner + status==NeedSupplement(3)<br/>└─ IncrementSupplementCount — UPDATE count+1 WHERE count<3<br/>└─ UpdateStatus(id, Processing=2)<br/>└─ CreateRecord(action='supplement')"]
    end

    subgraph AutoClose["4. 自动关闭 — Scheduler.runAutoCloseLoop"]
        AC1["每小时: time.NewTicker(1*Hour) + 启动立即执行"]
        AC2["TicketService.AutoClose(now - 7天)<br/>└─ TicketRepo.AutoCloseTickets(olderThan)<br/>    SELECT id WHERE status IN(1,2,3) AND created_at<?<br/>    UPDATE tickets SET status=5 WHERE id IN(...)<br/>└─ GormTxManager.Transaction → 批量 CreateRecord('auto_close')"]
    end

    C1 --> C2 --> C3
    C3 --> P1 --> P2 --> P3
    P3 --> A1
    P3 --> A2
    P3 --> A3
    P3 --> A4
    A2 --> S1 --> S2
    S2 -.->|退回 Processing(2)| P2
    A1 --> AC1
    A2 --> AC1
    A3 --> AC1
    A4 --> AC1
    AC1 --> AC2

    style Create fill:#3b82f610,stroke:#3b82f6
    style Process fill:#f59e0b10,stroke:#f59e0b
    style Supplement fill:#5e6ad210,stroke:#5e6ad2
    style AutoClose fill:#ef444410,stroke:#ef4444
```

---

## 2. 状态机

```mermaid
stateDiagram-v2
    [*] --> Pending : CreateTicket()

    Pending --> Processing : start
    Processing --> NeedSupplement : request_info<br/>supplement_count < 3
    NeedSupplement --> Processing : supplement<br/>（报障人补充后自动退回）
    Processing --> Resolved : resolve
    Processing --> Closed : close
    NeedSupplement --> Closed : close
    Pending --> Closed : close / auto_close

    Resolved --> [*]
    Closed --> [*]

    note right of NeedSupplement
        supplement_count +1
        > 3次 → 拒绝 request_info
    end note

    note left of Closed
        Scheduler 每小时检查:
        status ∈ {1,2,3}
        AND created_at < NOW() - 7天
    end note
```

---

## 3. 状态转换函数调用链

```mermaid
flowchart LR
    subgraph Handler["TicketHandler.UpdateStatus"]
        H["parseID → c.ShouldBindJSON → getCurrentUserID"]
    end

    subgraph Service["TicketService.UpdateStatus"]
        S1["TicketRepo.FindByID(id) → *Ticket{Status}"]
        S2["switch req.Action"]
        S3["start: Pending(1)→Processing(2)"]
        S4["request_info: Processing(2)→NeedSupplement(3)<br/>+ supplement_count<3"]
        S5["resolve: Processing(2)→Resolved(4)"]
        S6["close: 任意→Closed(5)"]
        S7["GormTxManager.Transaction:<br/>UpdateStatus + CreateRecord"]
    end

    subgraph Repo["TicketRepo"]
        R1["FindByID: SELECT * FROM tickets WHERE id=?"]
        R2["UpdateStatus: UPDATE tickets SET status=?"]
        R3["CreateRecord: INSERT INTO ticket_records"]
        R4["IncrementSupplementCount:<br/>UPDATE supplement_count+1 WHERE count<3"]
    end

    H --> S1 --> S2
    S2 --> S3
    S2 --> S4
    S2 --> S5
    S2 --> S6
    S3 --> S7
    S4 --> S7
    S5 --> S7
    S6 --> S7
    S7 --> R2
    S7 --> R3
    S4 --> R4

    style Service fill:#f59e0b10,stroke:#f59e0b
    style Repo fill:#22c55e10,stroke:#22c55e
```

---

## 4. 状态转换时序详解

```mermaid
sequenceDiagram
    actor Op as 运维人员
    participant H as TicketHandler
    participant S as TicketService
    participant R as TicketRepo
    participant MSG as MessageService
    participant DB as PostgreSQL

    Op->>H: PATCH /admin/tickets/:id/status {action:"resolve", result:"..."}
    H->>H: parseID → c.ShouldBindJSON → getCurrentUserID
    H->>S: UpdateStatus(ticketID, operatorID, req)

    S->>R: FindByID(ticketID)
    R->>DB: SELECT * FROM tickets WHERE id=?
    DB-->>R: *Ticket{Status}
    R-->>S: *Ticket

    S->>S: switch req.Action — 状态机校验

    alt action = "start"
        S->>S: 校验 Status==Pending(1)
        S->>R: UpdateStatus(id, Processing=2)
    else action = "request_info"
        S->>S: 校验 Status==Processing(2) + SupplementCount<3
        S->>R: IncrementSupplementCount(id)
        S->>R: UpdateStatus(id, NeedSupplement=3)
        S->>MSG: NotifySupplement(ticketID, userID)
        MSG->>DB: INSERT INTO messages
    else action = "resolve"
        S->>S: 校验 Status==Processing(2)
        S->>R: UpdateStatus(id, Resolved=4)
    else action = "close"
        S->>S: 允许从任意状态关闭
        S->>R: UpdateStatus(id, Closed=5)
    end

    S->>R: CreateRecord(&TicketRecord{action, content, detail})
    R->>DB: INSERT INTO ticket_records
    S-->>H: nil
    H-->>Op: 200
```

---

## 5. 自动关闭调度器

```mermaid
flowchart TB
    subgraph Start["启动"]
        ST["Scheduler.Start(ctx)<br/>service/scheduler.go:35<br/>└─ sync.Once.Do → go runAutoCloseLoop"]
    end

    subgraph Loop["主循环"]
        L1["启动立即执行: doAutoClose()"]
        L2["time.NewTicker(1 * time.Hour)"]
        L3["for { select {<br/>  case <-ctx.Done(): return<br/>  case <-ticker.C: doAutoClose()<br/>} }"]
    end

    subgraph Work["执行"]
        W1["ctx, cancel := context.WithTimeout(30s)"]
        W2["olderThan := now.Add(-7*24*Hour)"]
        W3["TicketService.AutoClose(ctx, olderThan)<br/>→ TicketRepo.AutoCloseTickets<br/>→ SELECT id WHERE status IN(1,2,3) AND created_at<?<br/>→ UPDATE SET status=5 WHERE id IN(...)<br/>→ GormTxManager.Transaction → 批量 CreateRecord('auto_close')"]
    end

    ST --> L1 --> L2 --> L3
    L3 --> W1 --> W2 --> W3
    W3 -.-> L3

    style Start fill:#22c55e10,stroke:#22c55e
    style Loop fill:#5e6ad210,stroke:#5e6ad2
    style Work fill:#f59e0b10,stroke:#f59e0b
```

---

## 6. 申告编号生成算法

```mermaid
flowchart LR
    A["CreateTicket"] --> B["now := time.Now()<br/>dateStr := now.Format('20060102')"]
    B --> C["TicketRepo.CountTodayTickets(dateStr)<br/>SELECT COUNT(*) FROM tickets<br/>WHERE ticket_no LIKE 'TK-'||dateStr||'-%'"]
    C --> D["seq := count + 1"]
    D --> E["seqStr := fmt.Sprintf('%04d', seq)"]
    E --> F["ticket_no := 'TK-' + dateStr + '-' + seqStr<br/>例: TK-20260618-0001"]
```

---

## 7. 补充信息流程

```mermaid
sequenceDiagram
    actor R as 报障人
    participant H as TicketHandler
    participant S as TicketService
    participant Repo as TicketRepo
    participant DB as PostgreSQL

    R->>H: PATCH /portal/tickets/:id/supplement {content:"..."}
    H->>H: c.ShouldBindJSON(&SupplementTicketRequest)

    H->>S: SupplementTicket(ticketID, userID, req)
    S->>Repo: FindByID(ticketID)
    Repo->>DB: SELECT * FROM tickets WHERE id=?
    DB-->>Repo: *Ticket
    Repo-->>S: *Ticket

    S->>S: 校验: ticket.UserID == userID (仅申告人)
    S->>S: 校验: ticket.Status == NeedSupplement(3)
    S->>S: 校验: ticket.SupplementCount < 3

    S->>Repo: CreateRecord(action:'supplement', content)
    Repo->>DB: INSERT INTO ticket_records

    S->>Repo: IncrementSupplementCount(id)
    Repo->>DB: UPDATE supplement_count+1 WHERE count<3

    S->>Repo: UpdateStatus(id, Processing=2)
    Repo->>DB: UPDATE tickets SET status=2

    S-->>H: nil
    H-->>R: 200
```

---

## 8. 数据形态变化追踪

| 阶段 | 输入 | 输出 | 关键函数 |
|------|------|------|---------|
| 请求解析 | JSON `{title, description, urgency}` | `CreateTicketRequest` | `c.ShouldBindJSON` |
| 编号生成 | `dateStr + count` | `TK-YYYYMMDD-XXXX` | `CountTodayTickets` + `fmt.Sprintf` |
| 入库 | `*Ticket` | `ticket_id` | `TicketRepo.Create` |
| 状态变换 | `action + *Ticket` | 新 `status + *TicketRecord` | `UpdateStatus` → `GormTxManager.Transaction` |
| 补充通知 | `ticketID + userID` | `INSERT messages` | `MessageService.NotifySupplement` |
| 补充回退 | `content + status=3` | `status=2 + supplement_count+1` | `SupplementTicket` + `IncrementSupplementCount` |
| 自动关闭 | `ticker(1h)` | `[]int64 closedIDs` | `AutoCloseTickets` + 批量 `CreateRecord` |

---

> 相关文件：`server/internal/handler/ticket.go` / `server/internal/service/ticket_service.go` / `server/internal/service/scheduler.go` / `server/internal/service/tx_manager.go` / `server/internal/repository/ticket_repo.go` / `server/internal/model/ticket.go`

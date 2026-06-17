# 数据看板与审计日志流程 (Dashboard & Audit Flow)

> **涉及文件：** `handler/dashboard.go` → `service/dashboard_service.go` (原生 SQL)
> **审计：** `handler/audit.go` → `repository/audit_repo.go` (分散在各 Service 中写入)
> **配置：** `handler/config.go` → `service/config_service.go` → `repository/config_repo.go`

---

## 1. 数据看板统计流程

```mermaid
sequenceDiagram
    actor O as 运维人员/管理员
    participant DH as DashboardHandler<br/>handler/dashboard.go
    participant DS as DashboardService<br/>service/dashboard_service.go
    participant DB as PostgreSQL

    Note over O,DB: ===== 看板概览统计 =====
    O->>DH: GET /api/v1/admin/dashboard/stats
    DH->>DS: s.DashboardService.GetStats()
    
    par 并行执行 7 条聚合 SQL
        DS->>DB: SELECT COUNT(*) FROM tickets<br/>WHERE created_at::date = CURRENT_DATE
        DB-->>DS: TodayTickets
    and
        DS->>DB: SELECT COUNT(*) FROM tickets WHERE status = 1
        DB-->>DS: PendingTickets
    and
        DS->>DB: SELECT COUNT(*) FROM tickets WHERE status = 2
        DB-->>DS: ProcessingTickets
    and
        DS->>DB: SELECT COUNT(*) FROM tickets WHERE status = 4
        DB-->>DS: ResolvedTickets
    and
        DS->>DB: SELECT COUNT(*) FROM chat_sessions<br/>WHERE created_at::date = CURRENT_DATE
        DB-->>DS: TodayChats
    and
        DS->>DB: SELECT COALESCE(AVG(confidence), 0)<br/>FROM chat_sessions<br/>WHERE created_at::date = CURRENT_DATE
        DB-->>DS: AvgConfidence
    and
        DS->>DB: SELECT COUNT(*) FROM knowledge_articles
        DB-->>DS: KnowledgeCount
    end
    
    DS-->>DH: *StatsResponse{<br/>  TodayTickets, PendingTickets, ProcessingTickets,<br/>  ResolvedTickets, TodayChats, AvgConfidence, KnowledgeCount<br/>}
    DH-->>O: {"code": 0, "data": {stats}}

    Note over O,DB: ===== 趋势数据 =====
    O->>DH: GET /api/v1/admin/dashboard/trends<br/>?start_date=2026-06-01&end_date=2026-06-10&granularity=day
    DH->>DS: s.DashboardService.GetTrends(req)
    
    DS->>DS: 生成日期序列（从 startDate 到 endDate 逐日填充）
    
    DS->>DS: 初始化 DataPoints[全部填0]<br/>确保零值日期也有数据点
    
    DS->>DB: SELECT TO_CHAR(created_at::date,'YYYY-MM-DD') AS date,<br/>       COUNT(*) AS count<br/>FROM tickets<br/>WHERE created_at::date >= ?::date<br/>  AND created_at::date <= ?::date<br/>GROUP BY created_at::date
    DB-->>DS: []dailyCount (每日申告数)
    
    DS->>DB: SELECT TO_CHAR(created_at::date,'YYYY-MM-DD') AS date,<br/>       COUNT(*) AS count<br/>FROM chat_sessions<br/>WHERE created_at::date >= ?::date<br/>  AND created_at::date <= ?::date<br/>GROUP BY created_at::date
    DB-->>DS: []dailyCount (每日问答数)
    
    DS->>DS: 合并 ticket + chat 数据到 DataPoints<br/>按日期匹配填充
    
    DS-->>DH: *TrendResponse{DataPoints: []{Date, TicketCount, ChatCount}}
    DH-->>O: {"code": 0, "data": {data_points: [...]}}
```

---

## 2. 看板统计指标计算公式

```mermaid
flowchart LR
    subgraph DB["数据源"]
        T["tickets 表"]
        CS["chat_sessions 表"]
        KA["knowledge_articles 表"]
    end
    
    subgraph SQL["聚合查询 (DashboardService.GetStats)"]
        S1["COUNT(*) FILTER created_at::date=CURRENT_DATE → TodayTickets"]
        S2["COUNT(*) FILTER status=1 → PendingTickets"]
        S3["COUNT(*) FILTER status=2 → ProcessingTickets"]
        S4["COUNT(*) FILTER status=4 → ResolvedTickets"]
        S5["COUNT(*) FILTER created_at::date=CURRENT_DATE → TodayChats"]
        S6["COALESCE(AVG(confidence), 0) FILTER today → AvgConfidence"]
        S7["COUNT(*) → KnowledgeCount"]
    end
    
    subgraph UI["Dashboard.vue 展示"]
        C1["📋 今日申告: TodayTickets"]
        C2["⏳ 待处理: PendingTickets"]
        C3["🔄 处理中: ProcessingTickets"]
        C4["✅ 已解决: ResolvedTickets"]
        C5["💬 今日问答: TodayChats"]
        C6["📊 平均置信度: AvgConfidence"]
        C7["📚 知识条目: KnowledgeCount"]
    end
    
    T --> S1
    T --> S2
    T --> S3
    T --> S4
    CS --> S5
    CS --> S6
    KA --> S7
    
    S1 --> C1
    S2 --> C2
    S3 --> C3
    S4 --> C4
    S5 --> C5
    S6 --> C6
    S7 --> C7
```

---

## 3. 审计日志写入流程（分散在各 Service 中触发）

```mermaid
flowchart TD
    subgraph Triggers["审计触发点 (各 Service 层)"]
        T1["UserService.Freeze / Restore<br/>→ AuditRepo.Create"]
        T2["KnowledgeService.Publish / Disable<br/>→ AuditRepo.Create"]
        T3["TicketService.UpdateStatus<br/>→ AuditRepo.Create"]
        T4["RoleService.Update / Delete<br/>→ AuditRepo.Create"]
        T5["ConfigService.UpdateConfig<br/>→ AuditRepo.Create"]
        T6["Scheduler.AutoCloseJob<br/>→ AuditRepo.Create"]
    end
    
    subgraph Repo["AuditRepo<br/>repository/audit_repo.go"]
        Create["Create(&AuditLog{<br/>  OperatorID, Action,<br/>  TargetType, TargetID,<br/>  Detail(JSONB), IPAddress<br/>})"]
    end
    
    subgraph DB["PostgreSQL"]
        Table["audit_logs 表"]
    end
    
    T1 --> Create
    T2 --> Create
    T3 --> Create
    T4 --> Create
    T5 --> Create
    T6 --> Create
    Create --> Table
```

---

## 4. 审计日志查询流程

```mermaid
sequenceDiagram
    actor A as 系统管理员
    participant AH as AuditHandler<br/>handler/audit.go
    participant AR as AuditRepo<br/>repository/audit_repo.go
    participant DB as PostgreSQL

    A->>AH: GET /api/v1/admin/audit-logs<br/>?operator_id=1&action=user.*&target_type=ticket&date_from=2026-06-01
    AH->>AH: c.Get("currentUser") → 仅系统管理员可访问<br/>(中间件: RequirePermission "audit:read")
    
    AH->>AR: List(AuditFilter{OperatorID, Action, TargetType, TargetID, DateFrom, DateTo, Page, PageSize})
    AR->>DB: SELECT al.*, COALESCE(u.real_name,'') AS operator_name<br/>FROM audit_logs al<br/>LEFT JOIN users u ON al.operator_id = u.id<br/>WHERE al.operator_id = ?<br/>  AND al.action LIKE ?  (action 以 * 结尾时)<br/>  AND al.target_type = ?<br/>  AND al.created_at >= ?::date<br/>ORDER BY al.created_at DESC<br/>LIMIT ? OFFSET ?
    
    DB-->>AR: []AuditLogRow (含 operator_name), total
    AR-->>AH: ([]AuditLogRow, total)
    AH-->>A: {"code": 0, "data": {items, total, page, page_size}}
```

---

## 5. 系统配置读写流程

```mermaid
sequenceDiagram
    actor A as 系统管理员
    participant CH as ConfigHandler<br/>handler/config.go
    participant CS as ConfigService<br/>service/config_service.go
    participant CR as ConfigRepo<br/>repository/config_repo.go
    participant DB as PostgreSQL

    Note over A,DB: ===== 读取配置 =====
    A->>CH: GET /api/v1/admin/configs/ai.default_top_k
    CH->>CS: s.ConfigService.GetConfig("ai.default_top_k")
    CS->>CR: GetByKey("ai.default_top_k")
    CR->>DB: SELECT * FROM system_configs WHERE key = ?
    DB-->>CR: *SystemConfig{Key, Value(JSONB)}
    CR-->>CS: *SystemConfig
    CS->>CS: json.Unmarshal(config.Value, &result)
    CS-->>CH: interface{} (实际值)
    CH-->>A: {"code": 0, "data": {"key": "ai.default_top_k", "value": 5}}

    Note over A,DB: ===== 更新配置 =====
    A->>CH: PUT /api/v1/admin/configs/ai.confidence_threshold<br/>{value: 0.7}
    CH->>CS: s.ConfigService.UpdateConfig("ai.confidence_threshold", 0.7, userID)
    
    CS->>CS: json.Marshal(value) → valueJSON
    
    alt value 为 nil
        CS-->>CH: AppError{10003, "配置值不能为 nil"}
    end
    
    CS->>CR: Upsert("ai.confidence_threshold", valueJSON, userID)
    CR->>DB: INSERT INTO system_configs ...<br/>ON CONFLICT (key) DO UPDATE SET value=?, updated_by=?, updated_at=NOW()
    
    CS-->>CH: nil
    CH-->>A: {"code": 0}
```

---

## 6. 完整数据流总览

```mermaid
flowchart TD
    subgraph Frontend["Vue 3 前端"]
        Dashboard["Dashboard.vue<br/>7 统计卡片 + 趋势图"]
        AuditLog["AuditLog.vue<br/>操作日志表格"]
        ModelConfig["ModelConfig.vue<br/>Top K + 置信度阈值"]
    end
    
    subgraph API["Gin REST API"]
        DS2["GET /api/v1/admin/dashboard/stats"]
        DT["GET /api/v1/admin/dashboard/trends"]
        AL2["GET /api/v1/admin/audit-logs"]
        CFG["GET/PUT /api/v1/admin/configs/:key"]
    end
    
    subgraph Service["Service 层"]
        GetStats["DashboardService.GetStats()<br/>7 条原生 SQL"]
        GetTrends["DashboardService.GetTrends()<br/>日期序列 + LEFT JOIN"]
        ListAudit["AuditRepo.List()<br/>筛选 + 分页"]
        ConfigRW["ConfigService.GetConfig/UpdateConfig<br/>Upsert 操作"]
    end
    
    subgraph DB["PostgreSQL"]
        Tickets["tickets 表"]
        Chat["chat_sessions 表"]
        Articles["knowledge_articles 表"]
        Audit["audit_logs 表"]
        Config["system_configs 表"]
    end
    
    Dashboard --> DS2
    Dashboard --> DT
    AuditLog --> AL2
    ModelConfig --> CFG
    
    DS2 --> GetStats
    DT --> GetTrends
    AL2 --> ListAudit
    CFG --> ConfigRW
    
    GetStats --> Tickets
    GetStats --> Chat
    GetStats --> Articles
    GetTrends --> Tickets
    GetTrends --> Chat
    ListAudit --> Audit
    ConfigRW --> Config
```

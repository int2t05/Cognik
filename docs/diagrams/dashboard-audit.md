# 看板统计与审计日志

> 覆盖 Dashboard 统计查询、趋势分析、审计日志、系统配置管理。

---

## 1. Dashboard 统计流程

```mermaid
flowchart TB
    subgraph Input["输入"]
        I1["GET /admin/dashboard/stats"]
    end

    subgraph Handler["DashboardHandler.GetStats — handler/dashboard.go"]
        H1["→ DashboardService.GetStats()"]
    end

    subgraph Service["DashboardService.GetStats — service/dashboard_service.go"]
        S1["并行执行 7 条原生 SQL 聚合查询"]
        S1 --> S1a["COUNT(*) FROM tickets WHERE created_at::date=CURRENT_DATE → TodayTickets"]
        S1 --> S1b["COUNT(*) FROM tickets WHERE status=1 → PendingTickets"]
        S1 --> S1c["COUNT(*) FROM tickets WHERE status=2 → ProcessingTickets"]
        S1 --> S1d["COUNT(*) FROM tickets WHERE status=4 → ResolvedTickets"]
        S1 --> S1e["COUNT(*) FROM chat_sessions WHERE created_at::date=CURRENT_DATE → TodayChats"]
        S1 --> S1f["COALESCE(AVG(confidence),0) FROM chat_sessions today → AvgConfidence"]
        S1 --> S1g["COUNT(*) FROM knowledge_articles → KnowledgeCount"]
    end

    subgraph Output["输出"]
        O1["200 {today_tickets, pending_tickets, processing_tickets,<br/>resolved_tickets, today_chats, avg_confidence, knowledge_count}"]
    end

    I1 --> H1 --> S1 --> O1

    style Input fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Output fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 2. Dashboard 趋势分析流程

```mermaid
flowchart TB
    subgraph Input["输入"]
        I1["GET /admin/dashboard/trends<br/>?start_date=2026-06-01&end_date=2026-06-18"]
    end

    subgraph Handler["DashboardHandler.GetTrends"]
        H1["→ DashboardService.GetTrends(req)"]
    end

    subgraph Service["DashboardService.GetTrends"]
        S1["生成日期序列: start→end 逐日<br/>初始化 DataPoints[全0]"]
        S2["TO_CHAR(created_at,'YYYY-MM-DD') + COUNT<br/>GROUP BY date FROM tickets<br/>→ 每日申告数"]
        S3["TO_CHAR(created_at,'YYYY-MM-DD') + COUNT<br/>GROUP BY date FROM chat_sessions<br/>→ 每日问答数"]
        S4["合并 ticket+chat 数据到 DataPoints<br/>按日期匹配（不在日期范围的忽略）"]
    end

    subgraph Output["输出"]
        O1["200 {data_points: [{date, ticket_count, chat_count}, ...]}"]
    end

    I1 --> H1 --> S1
    S1 --> S2
    S1 --> S3
    S2 --> S4
    S3 --> S4
    S4 --> O1

    style Input fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Output fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 3. 审计日志查询

```mermaid
flowchart LR
    IN["GET /admin/audit-logs<br/>?operator_id=&action=&page=&page_size="] --> H["AuditHandler.List"]
    H --> AR["AuditRepo.List(operatorID, action, page, pageSize)"]
    AR --> DB["SELECT al.*, u.real_name<br/>FROM audit_logs al LEFT JOIN users u<br/>ON al.operator_id = u.id<br/>ORDER BY al.created_at DESC<br/>LIMIT ? OFFSET ?"]
    DB --> OUT["200 {list: [{id, operator_id, real_name, action,<br/>resource_type, resource_id, detail, created_at}], total}"]

    style IN fill:#1e293b,stroke:#334155,color:#e2e8f0
    style OUT fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 4. 系统配置读写

```mermaid
flowchart TB
    subgraph Read["读取配置"]
        R1["GET /admin/configs/:key"]
        R2["ConfigHandler.Get → ConfigService.GetConfig(key)"]
        R3["ConfigRepo.GetByKey(key)<br/>SELECT * FROM system_configs WHERE key=?"]
        R4["json.Unmarshal(config.Value, &result)"]
        R5["200 {value} — 任意 JSON 类型"]
    end

    subgraph Write["写入配置"]
        W1["PUT /admin/configs/:key {value}"]
        W2["ConfigHandler.Update → ConfigService.UpdateConfig(key, value, userID)"]
        W3["json.Marshal(value) → valueJSON"]
        W4["ConfigRepo.Upsert(key, valueJSON, userID)<br/>INSERT ... ON CONFLICT (key) DO UPDATE"]
        W5["200"]
    end

    R1 --> R2 --> R3 --> R4 --> R5
    W1 --> W2 --> W3 --> W4 --> W5

    style Read fill:#5e6ad210,stroke:#5e6ad2
    style Write fill:#f59e0b10,stroke:#f59e0b
```

---

## 5. 跨模块事件驱动关系总览

```mermaid
flowchart LR
    subgraph Inputs["用户操作事件"]
        I1["登录/刷新<br/>AuthHandler.Login/Refresh"]
        I2["发消息<br/>ChatHandler.StreamChatMessage"]
        I3["上传文件<br/>KnowledgeHandler.UploadDocuments"]
        I4["提交申告<br/>TicketHandler.CreateTicket"]
        I5["修改状态<br/>TicketHandler.UpdateStatus"]
        I6["发布知识<br/>KnowledgeHandler.Publish"]
        I7["管理用户<br/>UserHandler.Create/Freeze"]
        I8["配置 LLM<br/>LLMConfigHandler.CreateConfig"]
    end

    subgraph Processing["核心处理"]
        P1["JWT 生成/校验<br/>pkg/jwt + middleware"]
        P2["RAG Pipeline<br/>Pipeline.Execute"]
        P3["异步 Processor<br/>Processor.worker"]
        P4["状态机<br/>TicketService.UpdateStatus"]
        P5["PGVector 写入<br/>VectorStore.BatchInsert"]
        P6["atomic.Value<br/>LLMConfigManager"]
        P7["TxManager<br/>GormTxManager.Transaction"]
    end

    subgraph Outputs["副作用/输出"]
        O1["JWT Token Pair"]
        O2["SSE 流式答案 + 会话持久化"]
        O3["知识分块 + 向量写入"]
        O4["申告编号 + 状态变更"]
        O5["站内消息<br/>MessageService.NotifySupplement"]
        O6["审计日志<br/>AuditRepo.Create"]
        O7["LLM 配置即时更新"]
    end

    I1 --> P1 --> O1
    I2 --> P2 --> O2
    I3 --> P3 --> O3
    I4 --> P4 --> O4
    I5 --> P4 --> O5
    I5 --> P7
    I6 --> P5 --> O3
    I5 --> O6
    I6 --> O6
    I7 --> O6
    I8 --> P6 --> O7

    style I2 fill:#5e6ad220,stroke:#5e6ad2
    style P2 fill:#5e6ad230,stroke:#5e6ad2
    style O2 fill:#5e6ad240,stroke:#5e6ad2
```

---

> 相关文件：`server/internal/handler/dashboard.go` / `server/internal/handler/audit.go` / `server/internal/handler/config.go` / `server/internal/service/dashboard_service.go` / `server/internal/service/audit_service.go` / `server/internal/repository/dashboard_repo.go` / `server/internal/repository/audit_repo.go`

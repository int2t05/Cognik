# 数据模型

> 覆盖核心业务表 ER 关系、索引策略、域划分。

---

## 1. 核心表 ER 关系

```mermaid
erDiagram
    users ||--o{ user_roles : "has roles"
    users ||--o{ chat_sessions : "creates"
    users ||--o{ tickets : "submits"
    users ||--o{ messages : "receives"
    users ||--o{ audit_logs : "triggers"

    roles ||--o{ user_roles : "assigned to"
    roles ||--o{ role_menus : "has menus"

    menus ||--o{ role_menus : "bound to"
    menus ||--o{ menus : "parent_id self-ref"

    knowledge_bases ||--o{ knowledge_articles : "contains"
    knowledge_bases ||--o{ knowledge_chunks : "owns"
    knowledge_bases ||--o{ chat_sessions : "scoped to"

    knowledge_articles ||--o{ knowledge_chunks : "split into"

    tickets ||--o{ ticket_records : "has history"

    chat_sessions ||--o{ chat_messages : "contains"

    users {
        bigint id PK
        varchar username UK
        varchar password_hash
        varchar real_name
        varchar email
        varchar phone
        smallint status
        boolean first_login
        timestamp last_login_at
        timestamp created_at
        timestamp updated_at
    }

    roles {
        bigint id PK
        varchar name UK
        varchar description
        jsonb permissions
        timestamp created_at
        timestamp updated_at
    }

    menus {
        bigint id PK
        bigint parent_id FK
        varchar name
        varchar path
        varchar icon
        varchar permission
        int sort_order
        timestamp created_at
    }

    user_roles {
        bigint user_id FK
        bigint role_id FK
    }

    role_menus {
        bigint role_id FK
        bigint menu_id FK
    }

    knowledge_bases {
        bigint id PK
        varchar name
        varchar description
        varchar embedding_model
        int vector_dimension
        bigint created_by FK
        timestamp created_at
        timestamp updated_at
    }

    knowledge_articles {
        bigint id PK
        bigint kb_id FK
        varchar question
        text answer
        text content
        varchar category
        smallint status
        varchar process_status
        varchar source_type
        int word_count
        int chunk_count
        bigint created_by FK
        bigint reviewed_by FK
        timestamp reviewed_at
        timestamp created_at
        timestamp updated_at
    }

    knowledge_chunks {
        bigint id PK
        bigint article_id FK
        bigint kb_id FK
        text content
        int chunk_index
        halfvec embedding
        varchar embedding_model
        int vector_dimension
        timestamp created_at
    }

    tickets {
        bigint id PK
        varchar ticket_no UK
        bigint user_id FK
        varchar title
        text description
        smallint urgency
        smallint impact_scope
        jsonb affected_systems
        varchar contact_phone
        varchar contact_email
        smallint status
        smallint supplement_count
        jsonb chat_context
        smallint source
        timestamp created_at
        timestamp updated_at
    }

    ticket_records {
        bigint id PK
        bigint ticket_id FK
        bigint operator_id FK
        varchar action
        text content
        jsonb detail
        timestamp created_at
    }

    chat_sessions {
        bigint id PK
        bigint user_id FK
        bigint kb_id FK
        varchar question
        text answer
        jsonb sources
        float confidence
        int duration_ms
        timestamp created_at
        timestamp updated_at
    }

    chat_messages {
        bigint id PK
        bigint session_id FK
        varchar role
        text content
        jsonb metadata
        timestamp created_at
    }

    messages {
        bigint id PK
        bigint user_id FK
        varchar type
        varchar title
        text content
        bigint reference_id
        boolean is_read
        timestamp created_at
    }

    audit_logs {
        bigint id PK
        bigint operator_id FK
        varchar action
        varchar resource_type
        bigint resource_id
        jsonb detail
        varchar ip_address
        timestamp created_at
    }

    system_configs {
        varchar key PK
        jsonb value
        bigint updated_by FK
        timestamp updated_at
    }

    llm_configs {
        bigint id PK
        varchar name
        varchar provider_type
        varchar base_url
        varchar api_key
        varchar llm_model
        varchar embedding_model
        int max_tokens
        float temperature
        boolean is_default
        timestamp created_at
        timestamp updated_at
    }
```

---

## 2. 关键索引策略

| 表 | 索引类型 | 索引列 | 用途 |
|----|---------|--------|------|
| `knowledge_chunks` | HNSW | `embedding halfvec_ip_ops` | 向量相似度检索（`<=>` 算子） |
| `knowledge_chunks` | B-tree | `kb_id` | 按知识库过滤/删除 |
| `knowledge_chunks` | B-tree | `article_id` | 按文章删除/重索引 |
| `users` | UNIQUE B-tree | `username` | 登录查找 |
| `tickets` | UNIQUE B-tree | `ticket_no` | 编号唯一 |
| `tickets` | B-tree | `user_id, status, created_at` | 列表查询 + AutoClose |
| `chat_sessions` | B-tree | `user_id, created_at` | 会话列表查询 |
| `audit_logs` | B-tree | `operator_id, action, created_at` | 审计过滤 |
| `messages` | B-tree | `user_id, is_read` | 未读消息计数 |

---

## 3. 业务域划分

```mermaid
flowchart LR
    subgraph Auth["认证域"]
        U["users"] --> UR["user_roles"] --> R["roles"]
        R --> RM["role_menus"] --> M["menus"]
    end

    subgraph Knowledge["知识域"]
        KB["knowledge_bases"] --> A["knowledge_articles"]
        A --> KC["knowledge_chunks<br/>(pgvector halfvec + HNSW)"]
    end

    subgraph Chat["问答域"]
        CS["chat_sessions"] --> CM["chat_messages"]
        CS --> KB
    end

    subgraph Ticket["申告域"]
        T["tickets"] --> TR["ticket_records"]
    end

    subgraph System["系统域"]
        AL["audit_logs"]
        MSG["messages"]
        SC["system_configs"]
        LC["llm_configs"]
    end

    U --> CS
    U --> T
    U --> MSG
    U --> AL
    U --> KB
    U --> A

    style Auth fill:#22c55e15,stroke:#22c55e
    style Knowledge fill:#5e6ad215,stroke:#5e6ad2
    style Chat fill:#f59e0b15,stroke:#f59e0b
    style Ticket fill:#ef444415,stroke:#ef4444
    style System fill:#33415515,stroke:#334155
```

---

> 表结构定义见 `server/internal/model/`，迁移脚本见 `server/migrations/`。

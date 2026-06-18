# LLM 配置与模型调用

> 覆盖配置 CRUD、atomic.Value 热替换、LLM/Embedding/pgvector 调用链、连接测试。

---

## 1. 配置 CRUD + 热替换

```mermaid
flowchart TB
    subgraph CRUD["配置管理"]
        C1["POST /admin/llm-configs<br/>LLMConfigHandler.CreateConfig<br/>{name, provider_type, base_url, api_key,<br/> llm_model, embedding_model, max_tokens,<br/> temperature, is_default}"]
        C2["LLMConfigService.CreateConfig<br/>└─ isDefault=true?<br/>    Transaction: ClearDefault() + Create(config)<br/>    → manager.cfg.Store(newConfig)<br/>       atomic.Value 热替换"]
        C3[("INSERT INTO llm_configs")]
        C4["PUT /admin/llm-configs/:id<br/>LLMConfigHandler.UpdateConfig<br/>同 Create 热替换逻辑"]
        C5["DELETE /admin/llm-configs/:id<br/>LLMConfigHandler.DeleteConfig"]
        C6["GET /admin/llm-configs<br/>LLMConfigHandler.ListConfigs<br/>└─ MarshalJSON 自动脱敏: 'sk-****cret'"]
    end

    subgraph HotReload["热替换机制 — atomic.Value"]
        H1["LLMConfigManager<br/>└─ cfg atomic.Value<br/>└─ Store(newConfig) → 即时对所有 goroutine 可见"]
        H2["getModelConfig() — 每次请求调用<br/>└─ cfg, ok := m.cfg.Load().(*LlmConfig)<br/>    atomic.Value.Load → 无锁读取<br/>└─ 返回: model, maxTokens, temperature,<br/>    llmClient, embeddingClient<br/>    (baseURL/apiKey 变化时重建客户端)"]
    end

    subgraph TestConn["连接测试"]
        T1["POST /admin/llm-configs/:id/test<br/>ChatRequest{Messages:[{role:'user', content:'ping'}],<br/>MaxTokens:1, Temperature:0}"]
        T2["OpenAIClient.ChatCompletion(ctx, req)"]
        T3{"响应?"}
        T3 -->|成功| T4["200 {success:true, latency_ms, model}"]
        T3 -->|失败| T5["200 {success:false, error}"]
    end

    C1 --> C2 --> C3
    C2 --> H1
    C4 --> H1
    H1 --> H2
    T1 --> T2 --> T3

    style CRUD fill:#5e6ad210,stroke:#5e6ad2
    style HotReload fill:#f59e0b10,stroke:#f59e0b
    style TestConn fill:#22c55e10,stroke:#22c55e
```

---

## 2. LLM 调用全链路

```mermaid
flowchart TB
    subgraph Callers["调用方"]
        CA1["LLMService.StreamChat — 流式问答"]
        CA2["Pipeline.QueryRewrite — 查询改写"]
        CA3["Pipeline.MultiRoute — 多路路由"]
        CA4["Pipeline.Rerank — 重排序"]
        CA5["LLMConfigHandler.TestConnection — 连接测试"]
    end

    subgraph Interface["LLMClient 接口 — adapter/llm_client.go"]
        IF1["ChatCompletion(ctx, ChatRequest) → (*ChatResponse, error)<br/>同步调用，RAG 管道步骤"]
        IF2["ChatCompletionStream(ctx, ChatRequest) → (<-chan StreamChunk, error)<br/>流式调用，最终生成"]
    end

    subgraph Impl["OpenAIClient 实现"]
        IM1["doRequest(path, body)<br/>└─ json.Marshal<br/>└─ 指数退避重试 (maxRetries=3)<br/>    429/503 → retryableError<br/>    其他 → 直接返回 error<br/>└─ POST {baseURL}/v1/chat/completions"]
        IM2["ChatCompletionStream 额外:<br/>└─ Accept: text/event-stream<br/>└─ go readSSEStream(ctx, resp, ch)<br/>    └─ bufio.Scanner → 逐行 SSE parse<br/>    └─ data: [DONE] → return<br/>    └─ ch ← StreamChunk{Content, FinishReason}"]
    end

    subgraph LLM["LLM 服务"]
        L1["llama.cpp server<br/>或 OpenAI-compatible API<br/>POST /v1/chat/completions"]
    end

    CA1 --> IF1
    CA1 --> IF2
    CA2 --> IF1
    CA3 --> IF1
    CA4 --> IF1
    CA5 --> IF1
    IF1 --> IM1
    IF2 --> IM1 --> IM2
    IM1 --> L1
    IM2 --> L1

    style Callers fill:#5e6ad210,stroke:#5e6ad2
    style Interface fill:#f59e0b10,stroke:#f59e0b
    style Impl fill:#22c55e10,stroke:#22c55e
    style LLM fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 3. Embedding 调用全链路

```mermaid
flowchart TB
    subgraph Callers["调用方"]
        EC1["KnowledgeService.Publish — 文章发布"]
        EC2["Processor.processTask — 文档异步"]
        EC3["Pipeline 向量检索 — 通过 Embedder"]
    end

    subgraph Embedder["Embedder.Embed — rag/embedder.go"]
        E1["Embed(ctx, chunks []string)<br/>└─ batchSize=32 分批<br/>└─ EmbeddingClient.CreateEmbeddings(ctx, req)<br/>    {Model, Input}<br/>└─ 合并结果: [][]float32"]
    end

    subgraph Client["OpenAIEmbeddingClient — adapter/embedding_client.go"]
        C1["CreateEmbeddings(ctx, EmbeddingRequest)<br/>└─ marshalRequest: DashScope 自动 encoding_format='float'<br/>└─ doRequest: 指数退避重试 (maxRetries=3)<br/>    POST {baseURL}/v1/embeddings<br/>└─ parseResponse: index 越界/重复检测"]
    end

    subgraph EMB["Embedding 服务"]
        EM["llama.cpp server<br/>或 OpenAI-compatible<br/>POST /v1/embeddings"]
    end

    EC1 --> E1
    EC2 --> E1
    EC3 --> E1
    E1 --> C1 --> EM

    style Embedder fill:#f59e0b10,stroke:#f59e0b
    style Client fill:#22c55e10,stroke:#22c55e
    style EMB fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 4. pgvector 向量存储调用链路

```mermaid
flowchart TB
    subgraph Callers["调用方"]
        V1["KnowledgeService.Publish<br/>BatchInsert + DeleteByArticle"]
        V2["Processor.processTask<br/>BatchInsert"]
        V3["KnowledgeService.DeleteKB<br/>DeleteByKB"]
        V4["Pipeline 向量检索<br/>CosineSearch"]
    end

    subgraph Interface["VectorStore 接口 — adapter/vector_store.go"]
        VI1["BatchInsert(ctx, []VectorChunk) → error<br/>└─ 维度一致性校验<br/>└─ float32ToPgVector → halfvec"]
        VI2["CosineSearch(ctx, kbID, embedding, topK) → []SearchResult<br/>└─ pgvector <=> 余弦距离<br/>└─ topK 钳位 [1,100]"]
        VI3["DeleteByArticle(ctx, articleID) → error"]
        VI4["DeleteByKB(ctx, kbID) → error"]
    end

    subgraph Impl["PgvectorStore 实现"]
        IM1["复用 GORM *sql.DB 连接池"]
        IM2["BatchInsert SQL:<br/>INSERT INTO knowledge_chunks<br/>(article_id, kb_id, content, chunk_index,<br/> embedding, embedding_model,<br/> vector_dimension, created_at)<br/>VALUES ($1..$7, $8::halfvec, NOW())"]
        IM3["CosineSearch SQL:<br/>SELECT id, article_id, content, chunk_index,<br/>  1 - (embedding <=> $1::halfvec) AS score<br/>FROM knowledge_chunks<br/>WHERE kb_id = $2<br/>ORDER BY embedding <=> $1::halfvec<br/>LIMIT $3"]
    end

    subgraph DB["PostgreSQL 18 + pgvector"]
        D1[("knowledge_chunks<br/>HNSW 索引<br/>halfvec 半精度")]
    end

    V1 --> VI1
    V1 --> VI2
    V1 --> VI3
    V1 --> VI4
    V2 --> VI1
    V2 --> VI2
    V2 --> VI3
    V2 --> VI4
    V3 --> VI1
    V3 --> VI2
    V3 --> VI3
    V3 --> VI4
    V4 --> VI1
    V4 --> VI2
    V4 --> VI3
    V4 --> VI4
    VI1 --> IM1
    VI2 --> IM1
    VI3 --> IM1
    VI4 --> IM1
    IM1 --> IM2
    IM1 --> IM3
    IM2 --> D1
    IM3 --> D1

    style Interface fill:#f59e0b10,stroke:#f59e0b
    style Impl fill:#22c55e10,stroke:#22c55e
    style DB fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 5. atomic.Value 热替换原理

```mermaid
flowchart LR
    subgraph Write["配置写入"]
        W1["CreateConfig(isDefault=true)<br/>或 UpdateConfig(isDefault=true)<br/>或 DeleteConfig(被删的是default)"]
        W2["GormTxManager.Transaction<br/>ClearDefault() + Save(config)"]
        W3["manager.cfg.Store(newConfig)<br/>atomic.Value 写 — 即时对所有 goroutine 可见"]
    end

    subgraph Read["配置读取（每次请求）"]
        R1["LLMService.StreamChat → getModelConfig()"]
        R2["cfg, ok := m.cfg.Load().(*LlmConfig)<br/>atomic.Value 读 — 无锁"]
        R3["返回: model, maxTokens, temperature,<br/>llmClient, embeddingClient"]
    end

    W1 --> W2 --> W3
    R1 --> R2 --> R3
    W3 -.->|即时生效| R2

    style Write fill:#f59e0b10,stroke:#f59e0b
    style Read fill:#5e6ad210,stroke:#5e6ad2
```

---

> 相关文件：`server/internal/handler/llm_config.go` / `server/internal/service/llm_config_service.go` / `server/internal/adapter/llm_client.go` / `server/internal/adapter/embedding_client.go` / `server/internal/adapter/vector_store.go`

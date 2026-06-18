# 智能问答 RAG 管道

> 覆盖会话创建、RAG 管道全链路、SSE 流式输出、降级矩阵。

---

## 1. 端到端数据流（输入 → RAG 管道 → SSE 输出）

```mermaid
flowchart TB
    subgraph Input["输入层"]
        I1["POST /portal/chat-sessions/:id/stream<br/><b>question</b> : string<br/><b>Authorization</b> : Bearer access_token"]
    end

    subgraph Handler["ChatHandler.StreamChatMessage — handler/chat.go"]
        H1["parseID('id') → sessionID<br/>c.ShouldBindJSON(&SendMessageRequest)<br/>getCurrentUserID(c) → userID<br/>Set SSE headers<br/>c.Status(200)"]
    end

    subgraph ChatService["ChatService.StreamChat — service/chat_service.go"]
        C1["strings.TrimSpace(question)<br/>ChatRepo.FindByID(sessionID) → 归属校验<br/>ChatRepo.FindMessagesBySession → 历史消息"]
        C2["writeSSEEvent(w, StreamEvent)<br/>json.Marshal + fmt.Fprintf<br/>flusher.Flush() — 立即推送<br/>rc.SetWriteDeadline(now + 30s)"]
        C3["done 事件 → 持久化<br/>ChatRepo.UpdateSession(answer, sources, confidence, duration_ms)<br/>ChatRepo.CreateBatch([user_msg, assistant_msg])"]
    end

    subgraph LLMService["LLMService.StreamChat — service/llm_service.go"]
        L1["eventCh ← StreamEvent{Type:'step'}<br/>Pipeline.Execute(ctx, question, kbID, opts, onStep)<br/>buildMessages(chunks, question, history)<br/>getModelConfig() → model + maxTokens"]
        L2["OpenAIClient.ChatCompletionStream(ctx, ChatRequest)<br/>逐 token → eventCh ← {Type:'token', Content}<br/>extractSources → maxConfidence → 兜底判断<br/>eventCh ← {Type:'done', Metadata}"]
    end

    subgraph Pipeline["Pipeline.Execute — rag/pipeline.go:75"]
        P0["opts.Normalize() — 零值填充默认值"]
        P1["1. QueryRewrite — rag/query_rewrite.go<br/>   LLMClient.ChatCompletion(systemPrompt+history)<br/>   输入: 口语化 query → 输出: 规范化 rewrittenQuery"]
        P2["2. MultiRoute — rag/multi_route.go<br/>   LLMClient.ChatCompletion(routingPrompt)<br/>   输入: rewrittenQuery → 输出: []routes (2-4 路)"]
        P3["3. HybridRetrieve<br/>   3a. VectorStore.CosineSearch(kbID, embedding, topK)<br/>       pgvector <=> 余弦距离<br/>   3b. BM25Retriever.Retrieve(kbID, query)<br/>       gse 分词 → Okapi BM25(k1=1.5, b=0.75)<br/>   3c. HybridFuse(vectorResults, bm25Results)<br/>       RRF(k=60): score = Σ 1/(60+rank_i)<br/>   输出: []RetrievalResult{ChunkID, Content, Score}"]
        P4["4. Rerank — rag/rerank.go<br/>   Reranker.Rerank(query, candidates[:RerankCount])<br/>   输出: reordered []RetrievalResult"]
        P5["5. TopK 截断: allChunks[:opts.TopK]<br/>   输出: *RAGResult{Chunks, Metrics}"]
    end

    subgraph Output["SSE Stream 输出"]
        O1["data: {'type':'step','id':'query_rewrite','label':'查询改写'}"]
        O2["data: {'type':'step','id':'vector_retrieve','label':'向量检索'}"]
        O3["data: {'type':'token','content':'...'} — 逐 token"]
        O4["data: {'type':'done','metadata':{answer,sources,confidence,duration_ms,pipeline}}"]
    end

    I1 --> H1 --> C1 --> L1 --> P0
    P0 --> P1 --> P2 --> P3 --> P4 --> P5
    P5 --> L1 --> L2 --> C2
    C2 --> O1
    C2 --> O2
    C2 --> O3
    C2 --> O4
    C2 --> C3

    style Input fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Output fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Pipeline fill:#5e6ad215,stroke:#5e6ad2
```

---

## 2. 会话创建（容器先行模式）

```mermaid
flowchart LR
    IN["POST /portal/chat-sessions<br/><b>kb_id</b> : int64<br/><b>title</b>? : string"] --> H["ChatHandler.CreateChatSession<br/>handler/chat.go"]
    H --> S1["ChatService.CreateSession(req, userID)<br/>service/chat_service.go"]
    S1 --> S2["KnowledgeRepo.FindKBByID(kbID) — 校验 KB 存在"]
    S2 --> S3["title = '' → '新会话'"]
    S3 --> S4["ChatRepo.Create(&ChatSession{UserID, KBID, Question:title})<br/>INSERT INTO chat_sessions"]
    S4 --> OUT["200 {session_id, kb_id, question, created_at}"]

    style IN fill:#1e293b,stroke:#334155,color:#e2e8f0
    style OUT fill:#1e293b,stroke:#334155,color:#e2e8f0
```

---

## 3. SSE 流式问答 — 完整时序

```mermaid
sequenceDiagram
    actor U as 用户
    participant CH as ChatHandler.StreamChatMessage
    participant CS as ChatService.StreamChat
    participant LS as LLMService.StreamChat
    participant Pipe as Pipeline.Execute
    participant VR as VectorStore.CosineSearch
    participant B5 as BM25Retriever.Retrieve
    participant LLM as OpenAIClient.ChatCompletionStream
    participant DB as PostgreSQL+pgvector

    U->>CH: POST /chat-sessions/:id/stream {question}
    CH->>CH: parseID → c.ShouldBindJSON → getCurrentUserID → SSE headers
    CH->>CS: StreamChat(ctx, sessionID, question, userID)

    CS->>CS: TrimSpace + FindByID(sessionID) → 归属校验
    CS->>CS: FindMessagesBySession → 历史消息
    CS->>LS: StreamChat(ctx, question, kbID, opts, history)

    Note over LS,Pipe: RAG 检索
    LS->>Pipe: Execute(ctx, question, kbID, opts, onStep)

    opt QueryRewrite
        Pipe->>LLM: ChatCompletion(systemPrompt+question)
        LLM-->>Pipe: rewrittenQuery
    end
    opt MultiRoute
        Pipe->>LLM: ChatCompletion(routingPrompt)
        LLM-->>Pipe: []subQueries
    end

    par 向量检索
        Pipe->>VR: CosineSearch(kbID, embedding, topK)
        VR->>DB: SELECT * FROM knowledge_chunks ORDER BY embedding <=> $1 LIMIT $2
        DB-->>VR: []SearchResult
    and BM25 检索
        Pipe->>B5: Retrieve(kbID, query)
        B5->>B5: gse分词 → Okapi BM25(k1=1.5,b=0.75)
        B5-->>Pipe: []bm25Result
    end

    Pipe->>Pipe: HybridFuse(vectorResults, bm25Results) → RRF(k=60)
    opt Rerank
        Pipe->>LLM: ChatCompletion(rerankPrompt)
        LLM-->>Pipe: rerankOrder
    end

    Pipe-->>LS: *RAGResult{Chunks, Metrics}

    Note over LS,LLM: LLM 流式生成
    LS->>LS: buildMessages(chunks, question, history)
    LS->>LLM: ChatCompletionStream(ctx, ChatRequest{Stream:true})

    loop 逐 token
        LLM-->>LS: StreamChunk{Content}
        LS->>LS: eventCh ← {Type:"token", Content}
    end
    LS->>LS: extractSources → maxConfidence → 兜底
    LS->>LS: eventCh ← {Type:"done", Metadata}

    loop 逐事件
        LS-->>CS: StreamEvent (via channel)
        CS->>CH: writeSSEEvent(w, evt) → flusher.Flush()
        CH-->>U: data: {"type":"token","content":"..."}
    end

    CS->>DB: UPDATE chat_sessions + INSERT chat_messages
    CH-->>U: data: {"type":"done","metadata":{...}}
```

---

## 4. 管道降级矩阵

```mermaid
flowchart TD
    Start(["Pipeline.Execute()"]) --> QR{"QueryRewrite?"}
    QR -->|true| QR_OK["QueryRewrite → LLMClient.ChatCompletion"]
    QR -->|false| MR
    QR_OK -->|成功| MR{"MultiRoute?"}
    QR_OK -->|失败| QR_DG["降级: 使用原始 query"]
    QR_DG --> MR

    MR -->|true| MR_OK["MultiRoute → LLMClient.ChatCompletion"]
    MR -->|false| VR
    MR_OK -->|成功| VR["VectorStore.CosineSearch<br/>pgvector <=> 余弦距离"]
    MR_OK -->|失败| VR_DG["降级: 单路检索"]
    VR_DG --> VR

    VR -->|成功| BM{"Hybrid?"}
    VR -->|失败 ❌| VRFail["返回 code=20002 ErrRAGUnavailable"]

    BM -->|true| BM25["BM25Retriever.Retrieve"]
    BM -->|false| RR
    BM25 -->|成功| Fuse["HybridFuse: RRF(k=60)"]
    BM25 -->|失败| BM_DG["降级: 仅向量结果"]
    BM_DG --> RR
    Fuse --> RR{"Rerank?"}

    RR -->|true| RR_OK["Reranker.Rerank(query, candidates)"]
    RR -->|false| LLM
    RR_OK -->|成功| LLM["LLMClient.ChatCompletionStream"]
    RR_OK -->|失败| RR_DG["降级: RRF 排序结果"]
    RR_DG --> LLM

    LLM -->|成功| Done(["返回 SSE Stream"])
    LLM -->|失败 ❌| LLMFail["返回 code=20001 ErrAIUnavailable"]

    style VRFail fill:#ef444420,stroke:#ef4444
    style LLMFail fill:#ef444420,stroke:#ef4444
    style QR_DG fill:#f59e0b15,stroke:#f59e0b
    style VR_DG fill:#f59e0b15,stroke:#f59e0b
    style BM_DG fill:#f59e0b15,stroke:#f59e0b
    style RR_DG fill:#f59e0b15,stroke:#f59e0b
```

---

## 5. SSE 事件协议

| 事件 Type | ID | 触发位置 | 数据 |
|-----------|-----|---------|------|
| `step` | `query_rewrite` | `Pipeline.Execute` track() | `{type, id, label}` |
| `step` | `multi_route` | `Pipeline.Execute` track() | `{type, id, label}` |
| `step` | `vector_retrieve` | `Pipeline.Execute` track() | `{type, id, label}` |
| `step` | `bm25_retrieve` | `Pipeline.Execute` track() | `{type, id, label}` |
| `step` | `hybrid_fuse` | `Pipeline.Execute` track() | `{type, id, label}` |
| `step` | `rerank` | `Pipeline.Execute` track() | `{type, id, label}` |
| `step` | `llm_generate` | `LLMService.StreamChat` | `{type, id, label}` |
| `token` | — | `OpenAIClient.readSSEStream` | `{type, content}` |
| `error` | — | 任意步骤失败 | `{type, message}` |
| `done` | — | 流式完成 | `{type, metadata: {answer, sources, confidence, duration_ms, pipeline}}` |

---

## 6. 数据形态变化追踪

| 阶段 | 输入 | 输出 | 关键函数 |
|------|------|------|---------|
| 请求解析 | JSON `{question}` | `SendMessageRequest` | `c.ShouldBindJSON` |
| 会话加载 | `sessionID` | `*ChatSession + []ChatMessage` | `ChatRepo.FindByID` + `FindMessagesBySession` |
| 查询改写 | `string query` | `string rewrittenQuery` | `QueryRewrite → LLMClient.ChatCompletion` |
| 多路检索 | `string` | `[]string routes` | `MultiRoute → LLMClient.ChatCompletion` |
| 向量检索 | `[]float32 embedding` | `[]SearchResult{ChunkID, Score}` | `VectorStore.CosineSearch` |
| BM25 检索 | `string query` | `[]RetrievalResult` | `BM25Retriever.Retrieve → gse + Okapi` |
| RRF 融合 | 两路结果 | 融合排序结果 | `HybridFuse(k=60)` |
| 重排序 | `query + candidates` | 重排结果 | `Reranker.Rerank` |
| 上下文构建 | `[]RetrievalResult` | `[]ChatMessage` | `buildMessages` |
| LLM 流式 | `ChatRequest{Stream:true}` | `chan StreamChunk` | `OpenAIClient.ChatCompletionStream` |
| SSE 推送 | `StreamEvent` | `data: {...}\n\n` | `writeSSEEvent → flusher.Flush` |
| 持久化 | answer + sources | DB 写入 | `ChatRepo.UpdateSession` + `CreateBatch` |

---

> 相关文件：`server/internal/handler/chat.go` / `server/internal/service/chat_service.go` / `server/internal/service/llm_service.go` / `server/internal/rag/pipeline.go` / `server/internal/adapter/llm_client.go`

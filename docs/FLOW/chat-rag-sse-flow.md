# Chat RAG SSE 数据流 — 每个 API 端点

> 涉及文件: `handler/chat.go`, `service/chat_service.go`, `service/llm_service.go`, `repository/chat_repo.go`, `rag/pipeline.go`, `rag/query_rewrite.go`, `rag/multi_route.go`, `rag/hybrid.go`, `rag/bm25.go`, `rag/rrerank.go`, `rag/embedder.go`, `rag/retriever.go`, `rag/types.go`, `adapter/llm_client.go`, `adapter/embedding_client.go`, `adapter/vector_store.go`, `model/chat.go`

---

## POST /api/v1/portal/chat-sessions &emsp; 创建会话容器

**输入** `{"kb_id":1, "title":"VPN问题"}`

```
1. ChatHandler.CreateChatSession (handler/chat.go:38)

2. ChatService.CreateSession (service/chat_service.go:87)
   ├─ KnowledgeRepo.FindKBByID (repository/knowledge_repo.go:33)
   │   → SELECT * FROM knowledge_bases WHERE id = ?
   └─ ChatRepo.Create (repository/chat_repo.go:26)
       → INSERT INTO chat_sessions (user_id, kb_id, question)
```

**输出** `{session_id, kb_id, question, created_at}`

---

## POST /api/v1/portal/chat-sessions/:id/stream &emsp; SSE 流式对话

**输入** `{"question":"数据库超时怎么排查","route_count":3,"rerank_count":20}`

### 阶段 1 — Handler: SSE 连接建立

```
ChatHandler.StreamChatMessage (handler/chat.go:162)
  ├─ strconv.ParseInt(idStr) → sessionID
  ├─ c.ShouldBindJSON → request.SendMessageRequest
  ├─ getCurrentUserID → userID
  ├─ c.Writer.(http.Flusher) → 校验 SSE 支持
  ├─ 写 SSE 响应头: Content-Type text/event-stream, Cache-Control no-cache
  └─ ChatService.StreamChat → 获取 <-chan StreamEvent
      └─ for evt := range eventCh { writeSSEEvent → Flush → SetWriteDeadline(30s) }
```

### 阶段 2 — ChatService: 会话校验 + 历史加载

```
ChatService.StreamChat (service/chat_service.go:121)
  ├─ ChatRepo.FindByID (repository/chat_repo.go:30)
  │   → SELECT * FROM chat_sessions WHERE id = ?
  │   → session.UserID != userID → ErrForbidden
  ├─ ChatRepo.FindMessagesBySession (repository/chat_repo.go:74)
  │   → SELECT * FROM chat_messages WHERE session_id=? ORDER BY created_at ASC LIMIT 50
  │   → 失败 → slog.Warn 降级为单轮对话
  ├─ 构建 rag.RAGOptions{TopK,QueryRewrite,MultiRoute,Hybrid,Rerank,RouteCount,RerankCount,History}
  └─ LLMService.StreamChat (service/llm_service.go:188) → 代理 goroutine:
        done 事件时:
          ├─ ChatRepo.UpdateSession (repository/chat_repo.go:82)
          │   → UPDATE chat_sessions SET answer,sources,confidence,duration_ms
          └─ ChatRepo.CreateBatch (repository/chat_repo.go:67)
              → INSERT INTO chat_messages (user + assistant)
```

### 阶段 3 — LLMService: RAG 管道 + LLM 生成

```
LLMService.StreamChat (service/llm_service.go:188) — goroutine:
  ├─ executeRAG (service/llm_service.go 内部)
  │   └─ Pipeline.Execute (rag/pipeline.go:75)
  │       ├─ Step 1: QueryRewrite (rag/query_rewrite.go:26)
  │       │    LLM.ChatCompletion(t=0.1,max_tokens=256)
  │       │    → rewrittenQuery; 失败 → 降级用原始 query
  │       ├─ Step 2: MultiRoute (rag/multi_route.go:31)
  │       │    LLM.ChatCompletion(t=0.3,max_tokens=512)
  │       │    → routes[2-4]; 失败 → [rewrittenQuery]
  │       ├─ Step 3: 检索
  │       │    ├─ 混合模式: VectorRetriever.Retrieve (rag/retriever.go:29)
  │       │    │   ├─ Embedder.Embed (rag/embedder.go:57)
  │       │    │   │   → EmbeddingClient.CreateEmbeddings → POST /v1/embeddings
  │       │    │   └─ PgvectorStore.CosineSearch (adapter/vector_store.go:179)
  │       │    │       → SELECT ... ORDER BY embedding <=> $1::halfvec LIMIT N
  │       │    ├─ BM25Retriever.Retrieve (rag/bm25.go:292)
  │       │    │   → GseSegmenter.Segment → BM25 评分 (k1=1.5,b=0.75)
  │       │    └─ HybridFuse (rag/hybrid.go:35)
  │       │        → RRF 融合 (k=60) → 降序排列 → 去重
  │       └─ Step 4: Rerank (rag/rrerank.go:35)
  │            → Reranker.Rerank (adapter/rerank_client.go:200)
  │            → cross-encoder 重排; 失败 → 保持原序
  │
  ├─ buildMessages (service/llm_service.go 内部)
  │   → system prompt(DB热配置或默认) + 滑动窗口历史(10条) + 知识库上下文
  ├─ getModelConfig → model + maxTokens(DB > config.yaml > 默认)
  │
  └─ llmClient.ChatCompletionStream (adapter/llm_client.go:158)
      → POST /chat/completions (stream:true; 429/503 重试3次)
      → readSSEStream → 逐 token 读 SSE → ch ← StreamChunk{Content}
```

**输出** SSE 事件流 `{type, content/id/label/metadata}`

### 阶段 4 — 事件类型与降级

| 事件 | 触发 | 内容 |
|------|------|------|
| `step` | 每管道步骤开始 | `{id:"query_rewrite", label:"查询改写"}` |
| `token` | LLM 逐 token | `{content:"数据库超时"}` |
| `done` | 流结束 | `{answer, sources, confidence, pipeline}` |
| `error` | 管道失败/LLM 失败 | `{error:"..."}` |

降级策略:
- 查询改写失败 → 用原始 query
- 多路检索失败 → routes=[rewrittenQuery]
- 向量检索失败 → 终止（核心路径）
- BM25 失败 → 仅用向量结果
- 重排序失败 → 保持原序
- LLM 不可用 → sendSimulated 模拟输出检索摘要

---

## 其它会话操作

### GET /api/v1/portal/chat-sessions &emsp; 会话列表

```
ChatHandler.ListSessions → ChatService.ListSessions (service/chat_service.go:318)
  ├─ ChatRepo.ListByUser → SELECT ... WHERE user_id=? ORDER BY created_at DESC
  └─ ChatRepo.CountMessagesBySessions (repository/chat_repo.go:104)
      → SELECT session_id, COUNT(*) ... GROUP BY session_id (批量, 消除N+1)
```

### GET /api/v1/portal/chat-sessions/:id &emsp; 会话详情

```
ChatHandler.GetChatDetail → ChatService.GetChatDetail (service/chat_service.go:257)
  ├─ ChatRepo.FindByID → 归属校验 (session.UserID != userID → 403)
  ├─ json.Unmarshal(session.Sources) → []SourceItem
  └─ ChatRepo.FindMessagesBySession → 最多 50 条, CanSubmitTicket=conf<0.6
```

### POST /api/v1/portal/chat-sessions/:id/feedback &emsp; 反馈

```
ChatHandler.SubmitFeedback → ChatService.SubmitFeedback (service/chat_service.go:231)
  ├─ feedback ∈ [1,2] (禁止0覆盖)
  └─ ChatRepo.UpdateFeedback → UPDATE chat_sessions SET feedback=?
```

### DELETE /api/v1/portal/chat-sessions/:id &emsp; 删除会话

```
ChatHandler.DeleteSession → ChatService.DeleteSession (service/chat_service.go:354)
  └─ ChatRepo.DeleteSession → DELETE chat_messages + DELETE chat_sessions (级联, 含 user_id 校验)
```

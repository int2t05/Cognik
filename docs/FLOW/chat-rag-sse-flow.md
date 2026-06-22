# 智能问答 SSE 流式数据流 — 后端全路径

> **聚焦：后端数据逻辑。不含前端调用链和用户故事。**

---

## 1. 路由注册

```
router.Setup() → registerPortalRoutes():
  POST   /api/v1/portal/chat-sessions          → ChatHandler.CreateChatSession
  GET    /api/v1/portal/chat-sessions          → ChatHandler.ListSessions
  GET    /api/v1/portal/chat-sessions/:id      → ChatHandler.GetChatDetail
  DELETE /api/v1/portal/chat-sessions/:id      → ChatHandler.DeleteSession
  POST   /api/v1/portal/chat-sessions/:id/stream → ChatHandler.StreamChatMessage
  POST   /api/v1/portal/chat-sessions/:id/feedback → ChatHandler.SubmitFeedback

JWT 认证: middleware.JWTAuth(userCache, jwtSecret) — TokenType=="access" + 冻结检查
```

---

## 2. 创建会话 — POST /api/v1/portal/chat-sessions

### Handler: `ChatHandler.CreateChatSession(c)`

```
1. c.ShouldBindJSON(&request.CreateSessionRequest{KBID, Title})
2. getCurrentUserID(c) → userID
3. h.svc.CreateSession(ctx, req, userID)
```

### Service: `ChatService.CreateSession(ctx, req, userID)`

```
1. knowledgeRepo.FindKBByID(ctx, req.KBID)
   → SQL: SELECT * FROM knowledge_bases WHERE id = ?
   → 未找到 → AppError{10004, "知识库不存在"}

2. title = strings.TrimSpace(req.Title)
   → title == "" → title = "新会话"

3. chatRepo.Create(ctx, &ChatSession{UserID, KBID, Question: title})
   → SQL: INSERT INTO chat_sessions (user_id, kb_id, question) VALUES (?, ?, ?)
```

---

## 3. SSE 流式对话 — POST /api/v1/portal/chat-sessions/:id/stream

### 3.1 Handler: `ChatHandler.StreamChatMessage(c)`

```
1. strconv.ParseInt(idStr) → sessionID
2. c.ShouldBindJSON(&request.SendMessageRequest{Question, RouteCount, RerankCount})
3. getCurrentUserID(c) → userID

4. c.Writer.(http.Flusher) → 校验 SSE 支持

5. 设置 SSE 响应头:
   Content-Type: text/event-stream
   Cache-Control: no-cache
   Connection: keep-alive
   X-Accel-Buffering: no
   HTTP 200（先发状态码再流式 body）

6. h.svc.StreamChat(ctx, sessionID, question, userID, routeCount, rerankCount)
   → 返回 <-chan StreamEvent

7. SSE 事件代理循环:
   for evt := range eventCh {
       writeSSEEvent(c.Writer, evt)  // json.Marshal + fmt.Fprintf(w, "data: %s\n\n", data)
       flusher.Flush()
       rc.SetWriteDeadline(time.Now().Add(30s))  // 每次 flush 后延长写超时
   }
```

### 3.2 Service: `ChatService.StreamChat(ctx, sessionID, question, userID, ...) → <-chan StreamEvent`

```
1. strings.TrimSpace(question) == "" → AppError{10003, "问题不能为空"}

2. chatRepo.FindByID(ctx, sessionID)
   → SQL: SELECT * FROM chat_sessions WHERE id = ?
   → session.UserID != userID → AppError{10002, "无权访问该会话"}

3. chatRepo.FindMessagesBySession(ctx, sessionID) — 加载历史消息
   → SQL: SELECT * FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC LIMIT 50
   → 转换为 []adapter.ChatMessage（LLM 上下文注入）
   → 转换为 []map[string]string（RAG 查询改写消歧）
   → 失败 → slog.Warn 降级为单轮对话

4. 构建 rag.RAGOptions{TopK, QueryRewrite, MultiRoute, Hybrid, Rerank, RouteCount, RerankCount, History}

5. llmService.StreamChat(ctx, question, kbID, opts, history) → <-chan StreamEvent

6. 代理 goroutine（监听 llmEvents，done 时持久化）:
   for evt := range llmEvents {
       if evt.Type == "done":
           // 更新会话摘要
           chatRepo.UpdateSession(ctx, &ChatSession{
               ID: sessionID, Answer: answer, Sources: json(sources),
               Confidence: confidence, DurationMs: durationMS,
           })
           → SQL: UPDATE chat_sessions SET answer=?, sources=?, confidence=?, duration_ms=? WHERE id=?

           // 持久化一轮消息
           chatRepo.CreateBatch(ctx, []ChatMessage{
               {Role:"user", Content:question, SessionID},
               {Role:"assistant", Content:answer, SessionID, Sources, Confidence, PipelineMetrics},
           })
           → SQL: INSERT INTO chat_messages (...) VALUES (...), (...)

       sendOrCancel(ctx, outCh, evt)
   }
```

### 3.3 LLMService: `LLMService.StreamChat(ctx, question, kbID, opts, history) → <-chan StreamEvent`

```
goroutine 内:
  1. executeRAG(ctx, question, kbID, opts, onStep) → chunks, pipeMeta, err
     └─ onStep 回调: 每步骤向 eventCh 发送 {type:"step", id, label}

  2. len(chunks) == 0:
     → eventCh ← {type:"done", metadata:{answer:"暂未找到...", confidence:0, canSubmitTicket:true}}

  3. llmClient == nil:
     → sendSimulated(eventCh, 检索摘要, ...) — 逐 5 字符模拟流式输出

  4. llmClient != nil:
     a. eventCh ← {type:"step", id:"llm_generate", label:"LLM 生成"}

     b. buildMessages(chunks, question, history) → []adapter.ChatMessage
        ├─ system: configMgr.GetConfig().SystemPrompt（空则用默认提示词）
        ├─ history: maxHistoryMessages=10 滑动窗口截断
        └─ user: "知识库内容：\n{chunks}\n\n用户问题：{question}"

     c. getModelConfig() → model, maxTokens
        └─ 优先级: DB 热配置 (LLMConfigManager.GetConfig) > config.yaml 默认值

     d. llmClient.ChatCompletionStream(ctx, ChatRequest{Model, Messages, MaxTokens, Temperature:0.3})
        → <-chan StreamChunk

     e. 逐 token 循环:
        for chunk := range tokenCh {
            eventCh ← {type:"token", content: chunk.Content}
            answerBuf += chunk.Content
            if chunk.FinishReason != "" → break
        }

     f. eventCh ← {type:"done", metadata:{
          answer, sources(extractSources), confidence(maxConfidence 钳位[0,1]),
          canSubmitTicket(confidence<0.6), durationMS, pipeline(pipeMeta)
        }}
```

### 3.4 RAG 管道: `Pipeline.Execute(ctx, query, kbID, opts, onStep) → *RAGResult`

```
opts.Normalize() — 零值字段补默认值

Step 1 — 查询改写 (opts.QueryRewrite && llmClient != nil):
  track("query_rewrite", "查询改写", func() {
    rag.QueryRewrite(ctx, llmClient, query, history)
      └─ LLM ChatCompletion(temperature=0.1, maxTokens=256)
      └─ prompt: system="你是查询改写助手..." + 最近3轮历史 + user="原始查询：{query}"
      └─ 失败 → 降级: rewrittenQuery = query
  })

Step 2 — 多路检索 (opts.MultiRoute && routeCount>1 && llmClient != nil):
  track("multi_route", "多路检索", func() {
    rag.MultiRoute(ctx, llmClient, rewrittenQuery, count 钳位[2,4])
      └─ LLM ChatCompletion(temperature=0.3, maxTokens=512)
      └─ prompt: "从不同角度扩展为 N 个子查询。输出 JSON 字符串数组。"
      └─ 解析: tryParseJSONArray → 截取首个[...]再解析 → 去重
      └─ 失败 → 降级: routes = [rewrittenQuery]
  })

Step 3 — 检索:
  ┌─ 混合模式 (opts.Hybrid && bm25Retriever != nil):
  │
  │ Step 3a — 向量检索（核心路径——失败直接返回 error）:
  │   track("vector_retrieve", "向量检索", func() {
  │     for route := range routes:
  │       VectorRetriever.Retrieve(ctx, route, kbID, topK)
  │         ├─ embedder.Embed(ctx, [route])
  │         │   └─ EmbeddingClient.CreateEmbeddings(ctx, {Input: [route]})
  │         │       → HTTP POST /v1/embeddings（重试3次，429/503 指数退避）
  │         └─ store.CosineSearch(ctx, kbID, vectors[0], topK)
  │             → SQL: SELECT ... FROM knowledge_chunks
  │               WHERE kb_id=? ORDER BY embedding <=> $1::halfvec LIMIT ?
  │             → HNSW 索引加速
  │   })
  │
  │ Step 3b — BM25 检索（降级——失败不阻塞）:
  │   track("bm25_retrieve", "BM25 检索", func() {
  │     for route := range routes:
  │       BM25Retriever.Retrieve(ctx, route, kbID, topK)
  │         ├─ GseSegmenter.Segment(text) — gse 中文分词
  │         ├─ BM25 评分: IDF * TF_norm (k1=1.5, b=0.75)
  │         ├─ TTL 过期 → 从 knowledge_chunks 表懒加载重建索引
  │         └─ 排序 → topK 截取
  │   })
  │
  │ Step 3c — RRF 融合:
  │   track("hybrid_fuse", "混合融合", func() {
  │     HybridFuse(vectorResults, bm25Results, rerankCount)
  │       ├─ RRF 分数 = Σ 1/(k+rank)   (k=60)
  │       ├─ 两路都有结果 → 按 ChunkID 融合 → 降序排序 → 截断
  │       ├─ 仅单路有结果 → 直接截断
  │       └─ 两路都为空 → 返回 error
  │           上层处理: 若某路有结果 → dedupChunks() 回退单路
  │   })
  │
  └─ 纯向量模式:
      track("vector_retrieve", "向量检索", func() { ... })
      └─ dedupChunks(allChunks) — 按 ChunkID 去重

Step 4 — 重排序 (opts.Rerank && len(chunks)>1 && reranker != nil):
  track("rerank", "重排序", func() {
    candidates = allChunks[:rerankCount]  — 截取候选池
    rag.Rerank(ctx, reranker, query, candidates)
      ├─ SubprocessReranker.Rerank(query, passages)
      │   └─ Python 子进程 stdin/stdout JSON Lines 协议（cross-encoder）
      │   └─ 线程安全: mutex 保护 stdin 写入 + channel 路由响应
      └─ 失败 → 降级: 保持原序
  })

Step 5 — 截取 TopK:
  if len(allChunks) > opts.TopK { allChunks = allChunks[:opts.TopK] }

返回 RAGResult{Chunks, Metrics{Steps, TotalDurationMS}}
```

---

## 4. 其他会话操作

### 4.1 会话列表 — `ChatService.ListSessions(ctx, userID, page, pageSize)`

```
1. chatRepo.ListByUser(ctx, userID, page, pageSize)
   → SQL: SELECT * FROM chat_sessions WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?

2. chatRepo.CountMessagesBySessions(ctx, sessionIDs)
   → SQL: SELECT session_id, COUNT(*) FROM chat_messages WHERE session_id IN ? GROUP BY session_id
   → 批量查询消除 N+1

3. 组装 []SessionListItem{ID, Question, LastAnswer(截断100字符), MessageCount}
```

### 4.2 会话详情 — `ChatService.GetChatDetail(ctx, sessionID, userID)`

```
1. chatRepo.FindByID → 校验归属 (session.UserID != userID → ErrForbidden)
2. json.Unmarshal(session.Sources) → []SourceItem
3. chatRepo.FindMessagesBySession → 加载消息历史（最多50条，按 created_at ASC）
4. 组装 ChatSessionResponse{..., CanSubmitTicket: confidence < 0.6, Messages}
```

### 4.3 提交反馈 — `ChatService.SubmitFeedback(ctx, sessionID, userID, feedback)`

```
1. 校验: feedback ∈ [1,2]（禁止用 0 覆盖已有反馈）
2. chatRepo.FindByID → 校验归属
3. chatRepo.UpdateFeedback(ctx, sessionID, feedback)
   → SQL: UPDATE chat_sessions SET feedback = ? WHERE id = ?
```

### 4.4 删除会话 — `ChatService.DeleteSession(ctx, sessionID, userID)`

```
1. chatRepo.FindByID → 校验归属
2. chatRepo.DeleteSession — 级联删除
   → SQL: DELETE FROM chat_messages WHERE session_id = ?
   → SQL: DELETE FROM chat_sessions WHERE id = ? AND user_id = ?
```

---

## 5. Adapter 层核心调用

### LLM 流式: `OpenAIClient.ChatCompletionStream(ctx, req) → <-chan StreamChunk`

```
1. json.Marshal(body{model, messages, max_tokens, temperature, stream:true})
2. HTTP POST → baseURL + "/chat/completions"
3. 重试: 429/503 → 最多3次，指数退避 500ms*2^attempt（上限8s）
4. readSSEStream(ctx, resp, ch):
   ├─ bufio.Scanner 逐行读取（1MB 缓冲区）
   ├─ 跳过空行、':' 注释行
   ├─ line == "data: [DONE]" → 关闭 channel
   ├─ json.Unmarshal(line[6:]) → openAIStreamChunk
   │   └─ Qwen3 兼容: 优先 choices[0].delta.content，空时回退 reasoning_content
   └─ ch ← StreamChunk{Content, FinishReason}
```

### Embedding: `OpenAIEmbeddingClient.CreateEmbeddings(ctx, req) → *EmbeddingResponse`

```
1. HTTP POST → baseURL + "/embeddings"
2. DashScope 兼容: baseURL 含 "dashscope" → 自动附加 encoding_format:"float"
3. 重试: 同 LLM（3次，429/503 指数退避）
4. parseResponse: 校验索引无越界/重复，提取 [][]float32 + dimension + tokens_used
```

### pgvector 检索: `PgvectorStore.CosineSearch(ctx, kbID, embedding, topK)`

```sql
SELECT c.id AS chunk_id, c.article_id, c.content, c.chunk_index,
       1 - (c.embedding <=> $1::halfvec) AS score
FROM knowledge_chunks c
WHERE c.kb_id = $2
ORDER BY c.embedding <=> $1::halfvec
LIMIT $3
```

### pgvector 批量写入: `PgvectorStore.BatchInsert(ctx, chunks)`

```sql
INSERT INTO knowledge_chunks
  (article_id, kb_id, content, chunk_index, embedding, embedding_model, vector_dimension, created_at)
VALUES
  ($1,$2,$3,$4,$5::halfvec,$6,$7,NOW()),
  ...
-- 前置校验: 所有 chunk 维度一致; NaN/Inf → 0.0 降级
```

---

## 6. 降级矩阵

| 步骤 | 失败条件 | 降级行为 |
|------|---------|---------|
| 查询改写 | LLM 超时/不可达/返回空 | 使用原始 query |
| 多路检索 | LLM 超时/不可达/解析失败 | routes = [rewrittenQuery] |
| **向量检索** | pgvector 不可达/查询错误 | **返回 error（核心路径，不可降级）** |
| BM25 检索 | 索引损坏/分词失败 | 仅用向量结果 |
| RRF 融合 | 两路都为空 | 某路有结果则回退单路，否则返回 error |
| 重排序 | cross-encoder 不可用 | 保持原始排序 |
| LLM 生成 | API 不可达 | 输出检索摘要（无 LLM）或模拟流式 |
| Embedding | API 不可达 | fail-fast 返回 error |

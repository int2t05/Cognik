# 智能问答 SSE 流式数据流 — 从输入到输出的完整路径

---

## 用户故事

| 角色 | 目标 | 价值 |
|------|------|------|
| 报障人 | 在门户中用自然语言提问运维问题，获得 AI 即时解答 | 自助解决问题，减少人工申告 |
| 报障人 | 查看历史会话，继续之前的对话 | 上下文连续，不用重复描述 |
| 报障人 | 对回答质量反馈赞/踩 | 帮助改进知识库质量 |
| 系统 | 低置信度回答引导用户提交申告 | AI 兜底，不遗漏真正问题 |

---

## 前端调用链路

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           前端组件 → API 映射                                   │
├────────────────────┬───────────────────────────┬──────────────────────────────┤
│        页面         │          组件              │          API 调用             │
├────────────────────┼───────────────────────────┼──────────────────────────────┤
│ /portal/chat       │ ChatPage                  │                              │
│                    │  ├─ 侧栏: 会话列表          │ GET /portal/chat-sessions    │
│                    │  │   → getSessionList()    │  → apiFetchPage<ChatSession> │
│                    │  │                         │                              │
│                    │  ├─ 侧栏: 删除会话按钮       │ DELETE /portal/chat-sessions │
│                    │  │   → deleteSession()     │   /:id                       │
│                    │  │                         │                              │
│                    │  ├─ 侧栏: 点击会话           │ GET /portal/chat-sessions    │
│                    │  │   → getChatDetail()     │   /:id → 加载历史消息         │
│                    │  │                         │                              │
│                    │  ├─ 顶部: KB 选择器          │ GET /portal/knowledge-bases  │
│                    │  │   → getPortalKBList()   │  → apiFetch<KB[]>            │
│                    │  │                         │                              │
│                    │  ├─ 中部: 消息列表           │ (无直接 API — 消息由         │
│                    │  │   ChatMessage 气泡       │  SSE 流式推送填充)            │
│                    │  │   ChatPipeline 管道步骤   │                              │
│                    │  │                         │                              │
│                    │  ├─ 底部: ChatInput 输入框   │ POST /portal/chat-sessions   │
│                    │  │   → useChatStream.send() │   /:id/stream (SSE)          │
│                    │  │     ├─ 无 sessionId 时   │   + 前置: POST /portal/      │
│                    │  │     │  先 createSession  │   chat-sessions (创建容器)    │
│                    │  │     └─ 然后 fetch() SSE  │   → fetch() 逐行解析         │
│                    │  │       直连后端 :8080      │   data: {type, content}      │
│                    │  │                         │                              │
│                    │  └─ ChatMessage 反馈按钮     │ POST /portal/chat-sessions   │
│                    │      → submitFeedback()     │   /:id/feedback              │
│                    │      赞(1) / 踩(2) / 取消(0)│                              │
└────────────────────┴───────────────────────────┴──────────────────────────────┘
```

> **关键架构细节：** SSE 流式请求绕过 Next.js rewrite，直连后端 `http://localhost:8080`。
> `useChatStream` hook 封装了 AbortController 中止、120s 超时、SSE 逐行解析、
> 管道步骤追踪（step/token/done/error 四种事件类型）。
> 会话历史通过 `loadMessages()` 回填到消息列表，无需额外 API。

---

## 输入
```
POST /api/v1/portal/chat-sessions/:id/stream
{
  "question": "数据库连接超时怎么排查？",
  "route_count": 3,
  "rerank_count": 20
}
```
> 前置条件：已通过 `POST /api/v1/portal/chat-sessions` 创建会话容器。

---

## 分层数据流

### 0. 路由 & 中间件（接入层）

- `router.Setup()` → `registerPortalRoutes()` 注册 `POST /chat-sessions/:id/stream` → `ChatHandler.StreamChatMessage`
- `middleware.JWTAuth(userCache, jwtSecret)` — 解析 Bearer token，将 `CurrentUser` 写入 `gin.Context`
- 用户冻结校验：`cache.UserStatusCache.GetStatus(ctx, userID)` 优先内存缓存，未命中回退 DB

### 1. Handler 层 — 参数解析 + SSE 响应头

1. 经由 `ChatHandler.StreamChatMessage(c)` 处理：
   - `strconv.ParseInt(idStr)` 解析 sessionID
   - `c.ShouldBindJSON(&req)` 反序列化 `request.SendMessageRequest`
   - `getCurrentUserID(c)` 从 context 提取 userID
   - `c.Writer.(http.Flusher)` 校验 SSE 支持
   - 设置响应头：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`
   - `h.svc.StreamChat(ctx, sessionID, question, userID, routeCount, rerankCount)` 获取事件通道 `<-chan StreamEvent`

2. 事件代理循环（对前端隐藏内部实现）：
   ```
   for evt := range eventCh {
       writeSSEEvent(c.Writer, evt)   // JSON 序列化 + SSE data: 帧
       flusher.Flush()                 // 逐帧推送给客户端
       rc.SetWriteDeadline(30s)        // 每次刷新后延长写超时
   }
   ```
   - `writeSSEEvent(w, evt)` — `json.Marshal` + `fmt.Fprintf(w, "data: %s\n\n", data)`

### 2. Service 层 — 会话校验 + 历史加载

3. 经由 `ChatService.StreamChat(ctx, sessionID, question, userID, routeCount, rerankCount)` 处理：
   - 参数校验：`strings.TrimSpace(question) == ""`
   - `chatRepo.FindByID(ctx, sessionID)` — 加载会话
   - `session.UserID != userID` — 归属校验
   - `chatRepo.FindMessagesBySession(ctx, sessionID)` — 加载历史消息
     - 转换为 `[]adapter.ChatMessage` 格式（LLM 上下文注入用）
     - 转换为 `[]map[string]string` 格式（RAG 查询改写用）
   - 构建 `rag.RAGOptions`（TopK/QueryRewrite/MultiRoute/Hybrid/Rerank/RouteCount/RerankCount/History）
   - `s.llmService.StreamChat(ctx, question, kbID, opts, history)` — 核心编排入口

4. 代理 goroutine — `done` 事件到达时自动持久化：
   - `chatRepo.UpdateSession(ctx, &ChatSession{...})` — 更新会话摘要（answer/sources/confidence/duration_ms）
   - `chatRepo.CreateBatch(ctx, []ChatMessage{user, assistant})` — 持久化一轮对话

### 3. LLMService 层 — RAG 管道编排 + LLM 生成

5. 经由 `LLMService.StreamChat(ctx, question, kbID, opts, history)` 处理（在独立 goroutine 中运行）：

   **Step 5a — RAG 管道执行**
   6. `LLMService.executeRAG(ctx, question, kbID, opts, onStep)` — 内部调用 `pipeline.Execute()`
      - `onStep` 回调逐步骤向 eventCh 发送 `{type:"step", id:"xxx", label:"xxx"}` 事件

   **Step 5b — LLM 流式生成**
   7. 无检索结果 → 直接发送 `{type:"done", metadata:{answer:"暂未找到...", confidence:0}}`
   8. `llmClient == nil` → `sendSimulated(ctx, eventCh, answer, sources, confidence, durationMS)` 模拟输出
   9. 正常路径：
      - `sendOrCancel(ctx, eventCh, {type:"step", id:"llm_generate", label:"LLM 生成"})`
      - `buildMessages(chunks, question, history)` — 构建 system + 历史 + RAG 上下文 + user 消息列表
        - 从 `LLMConfigManager.GetConfig()` 读取热配置（SystemPrompt/Model/MaxTokens）
      - `getModelConfig()` — 优先级：DB 热配置 > config.yaml 默认值
      - `llmClient.ChatCompletionStream(ctx, ChatRequest{Model, Messages, MaxTokens, Temperature:0.3})`
        - 返回 `<-chan StreamChunk`
      - 逐 token 循环 → `sendOrCancel(eventCh, {type:"token", content:chunkContent})`
      - `type:"done"` 时发送 `StreamDoneMeta{answer, sources, confidence, canSubmitTicket, durationMS, pipeline}`
      - 管道耗时与 LLM 生成耗时会并到 `PipelineMeta.TotalDurationMS`

### 4. RAG 引擎层 — Pipeline.Execute

6. 经由 `rag.Pipeline.Execute(ctx, query, kbID, opts, onStep)` 处理：

   **Step 6.1 — 查询改写** (opts.QueryRewrite && llmClient != nil)
   - `rag.QueryRewrite(ctx, llmClient, query, history)` — LLM 改写口语化查询
     - 构建改写 prompt → `llmClient.ChatCompletion()` 同步调用
     - 失败降级：`rewrittenQuery = query`（原始查询继续）

   **Step 6.2 — 多路检索** (opts.MultiRoute && routeCount > 1 && llmClient != nil)
   - `rag.MultiRoute(ctx, llmClient, rewrittenQuery, routeCount)` — LLM 生成多路子查询
     - 返回 `[]string` 多个路由查询
     - 失败降级：`routes = []string{rewrittenQuery}`

   **Step 6.3 — 检索**（向量 + 可选 BM25）
   - **混合模式** (opts.Hybrid && bm25Retriever != nil)：

     **6.3a 向量检索**（核心路径——失败直接返回错误）：
     - 对每个 route 调用 `VectorRetriever.Retrieve(ctx, route, kbID, topK)`：
       - `Embedder.Embed(ctx, []string{route})` — 文本向量化
         - `EmbeddingClient.Embeddings(ctx, EmbeddingRequest{Input: texts, Model})` → HTTP POST `/v1/embeddings`
       - `PgvectorStore.CosineSearch(ctx, kbID, vectors[0], topK)` — pgvector <=> 算子检索
         - SQL: `SELECT ... WHERE kb_id=? ORDER BY embedding <=> $1::halfvec LIMIT topK`

     **6.3b BM25 检索**（降级——失败不阻塞）：
     - 对每个 route 调用 `BM25Retriever.Retrieve(ctx, route, kbID, topK)`：
       - `GseSegmenter.Segment(text)` — gse 中文分词
       - BM25 评分计算（TF-IDF 变体，带 TTL 懒加载缓存）
       - TTL 过期 → 从 `knowledge_chunks` 表重建索引

     **6.3c RRF 融合**：
     - `rag.HybridFuse(vectorResults, bm25Results, rerankCount)` — Reciprocal Rank Fusion
     - 融合后为空 + 两路都为空 → 返回错误
     - 融合后为空 + 某路有结果 → `dedupChunks()` 回退单路
     - 多路检索去重：`dedupChunks(chunks)` 按 ChunkID 去重

   - **纯向量模式** (Hybrid=false 或 bm25Retriever==nil)：直接执行 6.3a

   **Step 6.4 — 重排序** (opts.Rerank && len(chunks) > 1 && reranker != nil)
   - 按 `RerankCount` 截断候选池
   - `rag.Rerank(ctx, reranker, originalQuery, candidates)` — 使用原始 query
     - `SubprocessReranker.Rerank(query, docs)` — Python 子进程 cross-encoder
     - 失败降级：保持原序（不阻塞管道）
   - 重排序后按 `TopK` 截断

### 5. Adapter 层 — 外部服务调用

7. **LLM 流式调用**：`OpenAIClient.ChatCompletionStream(ctx, req)`
   - `json.Marshal(body)` 序列化请求
   - `http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", ...)`
   - `setHeaders(req)` — Content-Type + Authorization Bearer
   - 429/503 重试（最多 3 次，指数退避 500ms→1s→2s→...直到 8s）
   - `readSSEStream(ctx, resp, ch)` — `bufio.Scanner` 逐行解析 `data: {...}` 帧
     - 跳过空行、`:` 注释行、`[DONE]` 终止符
     - `json.Unmarshal` 解析 `openAIStreamChunk`
     - 通过 channel 发送 `StreamChunk{Content, FinishReason}`
   - 发送 `StreamChunk{Error}` 后关闭 channel

8. **Embedding 调用**：`OpenAIEmbeddingClient.Embeddings(ctx, req)`
   - 同 OpenAIClient 的重试机制（429/503 最多 3 次）
   - HTTP POST → `/v1/embeddings`
   - 解析 `{data: [{embedding: [0.123, ...]}]}` 返回 `[][]float32`

9. **pgvector 检索**：`PgvectorStore.CosineSearch(ctx, kbID, embedding, topK)`
   - `float32ToPgVector(v)` 将 `[]float32` 转换为 pgvector 兼容的数组字面量
     - NaN/Inf → 0.0 降级
   - SQL: `SELECT ... FROM knowledge_chunks WHERE kb_id=? ORDER BY embedding <=> $1::halfvec LIMIT ?`
   - 返回 `[]SearchResult{ChunkID, ArticleID, Content, ChunkIndex, Score}`

---

## 输出（SSE 事件流）

```
data: {"type":"step","id":"query_rewrite","label":"查询改写"}

data: {"type":"step","id":"multi_route","label":"多路检索"}

data: {"type":"step","id":"vector_retrieve","label":"向量检索"}

data: {"type":"step","id":"bm25_retrieve","label":"BM25 检索"}

data: {"type":"step","id":"hybrid_fuse","label":"混合融合"}

data: {"type":"step","id":"rerank","label":"重排序"}

data: {"type":"step","id":"llm_generate","label":"LLM 生成"}

data: {"type":"token","content":"数据"}

data: {"type":"token","content":"库"}

data: {"type":"token","content":"连接"}

data: {"type":"token","content":"超时"}

...（逐 token 流式输出）

data: {"type":"done","metadata":{
  "session_id": 1,
  "question": "数据库连接超时怎么排查？",
  "answer": "数据库连接超时可以从以下几个方面排查：\n1. 检查网络连通性...",
  "sources": [{"doc_name":"chunk_42","chunk_content":"...","confidence":0.92}],
  "confidence": 0.92,
  "can_submit_ticket": false,
  "duration_ms": 2340,
  "pipeline": {
    "steps": [
      {"id":"query_rewrite","label":"查询改写","duration_ms":450},
      {"id":"vector_retrieve","label":"向量检索","duration_ms":120},
      {"id":"bm25_retrieve","label":"BM25 检索","duration_ms":35},
      {"id":"hybrid_fuse","label":"混合融合","duration_ms":2},
      {"id":"llm_generate","label":"LLM 生成","duration_ms":1730}
    ],
    "total_duration_ms": 2340
  }
}}
```

## 关键分支

| 分支 | 条件 | 行为 |
|------|------|------|
| 查询改写失败 | LLM 超时/不可达 | 降级：使用原始 query 继续 |
| 多路检索失败 | LLM 超时/不可达 | 降级：routes = [rewrittenQuery] |
| **向量检索失败** | pgvector 不可用 | **中断：返回 "RAG 服务不可用"** (ErrRAGUnavailable) |
| BM25 检索失败 | BM25 索引损坏 | 降级：仅用向量结果 |
| 重排序失败 | cross-encoder 不可用 | 降级：保持原始顺序 |
| LLM 生成失败 | API 不可达 | 输出检索摘要列表（无 LLM 时） |
| 无检索结果 | 两路检索都为空 | done 事件：confidence=0，引导提交申告 |
| 历史消息加载失败 | DB 查询错误 | 降级为单轮对话 |

## 会话创建分支（非流式）

`POST /api/v1/portal/chat-sessions` → `ChatHandler.CreateChatSession`

1. `ChatService.CreateSession(ctx, req, userID)`
   - `knowledgeRepo.FindKBByID(ctx, req.KBID)` — 校验知识库存在
   - `chatRepo.Create(ctx, &ChatSession{UserID, KBID, Question})` — 写入 chat_sessions 表
2. 返回 `{session_id, kb_id, question, created_at}`

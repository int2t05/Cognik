# 智能问答接口

> **Base URL:** `/api/v1/portal` | **Auth:** JWT | **Module:** Chat & RAG Pipeline

## RAG 管道概述

问答请求经过以下 RAG 管道步骤：

```
用户问题
  → 查询改写 (可选)    — LLM 消除指代歧义（利用会话历史上下文）
  → 多路检索 (可选)    — LLM 生成 2-4 个子查询
  → 向量检索          — pgvector cosine 相似度
  → BM25 检索 (可选)  — 稀疏检索 + RRF 融合
  → 重排序   (可选)    — cross-encoder 重新评分候选分块（使用原始 query，非改写查询）
  → LLM 生成          — 带上下文生成答案 (SSE 流式)
```

**管道入口规范化：** Pipeline.Execute 入口自动调用 `RAGOptions.Normalize()`，
将零值字段（TopK/RouteCount/RerankCount）填充为默认值，避免 LIMIT 0 等异常行为。

**reranker 守卫：** 重排序步骤在 cross-encoder 子进程（reranker）不可用时静默跳过，降级为原始排序。

## 1. 创建会话

```http
POST /api/v1/portal/chat-sessions
Authorization: Bearer <token>
Content-Type: application/json
```

> 仅创建会话容器，不触发 LLM 调用。创建后通过「发送消息（流式）」端点发送首条消息。

**请求体：**

```json
{
  "kb_id": 1,
  "title": "VPN 密码问题"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| kb_id | int64 | ✓ | 目标知识库 ID |
| title | string | | 会话标题（可选，默认"新会话"） |

**成功响应 (200)：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "session_id": 42,
    "kb_id": 1,
    "question": "VPN 密码问题",
    "created_at": "2026-06-16 10:30:00"
  }
}
```

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 请求体格式错误或 KB ID 未提供 |
| 10004 | 404 | 知识库不存在 |
| 99999 | 500 | 服务未初始化或创建失败 |

---

## 2. 发送消息（SSE 流式）

```http
POST /api/v1/portal/chat-sessions/:id/stream
Authorization: Bearer <token>
Content-Type: application/json
```

> 在已有会话中发送消息，SSE 流式返回 AI 答案。支持多轮对话——历史消息自动注入 LLM 上下文（滑动窗口上限 10 条，约 5 轮 Q&A，可通过 `OPSMIND_AI_MAX_HISTORY_MESSAGES` 调整）。

**请求体：**

```json
{
  "question": "如何重置 VPN 密码？",
  "route_count": 3,
  "rerank_count": 5
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| question | string | ✓ | 用户问题（max 2000 字符） |
| route_count | int | | 多路检索子查询数（0=使用默认值 3） |
| rerank_count | int | | 重排序截断数（0=使用默认值 5） |

**SSE 事件流：**

响应类型为 `text/event-stream`，包含以下事件类型：

### step 事件 — 管道步骤进度

```
data: {"type":"step","id":"query_rewrite","label":"查询改写"}

data: {"type":"step","id":"multi_route","label":"多路检索"}

data: {"type":"step","id":"vector_retrieve","label":"向量检索"}

data: {"type":"step","id":"bm25_retrieve","label":"BM25检索"}

data: {"type":"step","id":"hybrid_fuse","label":"结果融合"}

data: {"type":"step","id":"rerank","label":"重排序"}

data: {"type":"step","id":"llm_generate","label":"LLM 生成"}
```

| step id | label | 说明 |
|---------|-------|------|
| `query_rewrite` | 查询改写 | LLM 消除对话指代歧义 |
| `multi_route` | 多路检索 | LLM 生成子查询 |
| `vector_retrieve` | 向量检索 | pgvector cosine 相似度检索 |
| `bm25_retrieve` | BM25检索 | 倒排索引稀疏检索 |
| `hybrid_fuse` | 结果融合 | RRF 融合排序 |
| `rerank` | 重排序 | cross-encoder 重排候选分块 |
| `llm_generate` | LLM 生成 | 调用 LLM 生成答案 |

> step 事件在管道执行期间**实时**发送，无需等待完整答案生成。

### error 事件 — 流式过程错误

```
data: {"type":"error","error":"LLM 生成中断: context deadline exceeded"}
```

> 当 RAG 检索或 LLM 流式生成中途失败时发送此事件。前端应在接收到 error 事件后停止流式渲染并提示用户。
>
> error 事件可能包含的消息类型：
> - `"LLM 生成中断: ..."` — LLM 调用超时或失败
> - `"LLM 流式调用失败: ..."` — LLM 流式响应中断
> - `"知识检索失败: ..."` — 向量或 BM25 检索失败
> - 其他运行时错误

### token 事件 — 逐 token 流式发送

```
data: {"type":"token","content":"VPN 密码"}
data: {"type":"token","content":"重置步骤"}
data: {"type":"token","content":"如下：\n1."}
```

> LLM 服务返回的 token 实时转发，输出速率取决于 LLM 推理性能。

### done 事件 — 流式结束，含完整元数据

```
data: {"type":"done","metadata":{"session_id":42,"question":"如何重置 VPN 密码？","answer":"VPN 密码重置步骤：1. 登录自助平台...","sources":[{"doc_name":"VPN 密码重置 FAQ","chunk_content":"...","confidence":0.85}],"confidence":0.85,"can_submit_ticket":false,"duration_ms":3200,"feedback":0,"created_at":"2026-06-16 10:30:05","pipeline":{"steps":[{"id":"query_rewrite","label":"查询改写","duration_ms":120},{"id":"vector_retrieve","label":"向量检索","duration_ms":45},{"id":"hybrid_fuse","label":"结果融合","duration_ms":2},{"id":"rerank","label":"重排序","duration_ms":180},{"id":"llm_generate","label":"LLM 生成","duration_ms":2800}],"total_duration_ms":3185}}}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| metadata.session_id | int64 | 会话 ID |
| metadata.question | string | 用户问题 |
| metadata.answer | string | AI 完整答案（与流式 token 拼接结果一致） |
| metadata.sources | array | 知识来源列表 |
| metadata.sources[].doc_name | string | 来源文档名称 |
| metadata.sources[].chunk_content | string | 匹配的切片内容 |
| metadata.sources[].confidence | float | 该来源置信度 (0-1) |
| metadata.confidence | float | 整体置信度（取 sources 最高分） |
| metadata.can_submit_ticket | bool | 是否建议转人工申告（confidence < 0.6） |
| metadata.duration_ms | int | 总耗时（毫秒） |
| metadata.feedback | int16 | 用户反馈：0=未评价 1=赞 2=踩 |
| metadata.created_at | string | 会话创建时间 |
| metadata.pipeline | object | 管道执行耗时明细 |
| metadata.pipeline.steps | array | 各步骤耗时 |
| metadata.pipeline.total_duration_ms | int | 管道总耗时 |

**错误降级（非 SSE）：**

当会话不存在或无权访问时，直接返回 JSON 错误：

```json
{
  "code": 10004,
  "message": "会话不存在",
  "data": null
}
```

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 无效的会话 ID（无法解析为数字） |
| 99999 | 500 | SSE 不被服务器支持 |

**前端消费示例：**

```typescript
import { createSession, streamChatMessage } from '@/api/chat'

// 1. 创建会话
const { data } = await createSession(1, '如何重置 VPN 密码？')
const sessionId = data.session_id

// 2. 发送首条消息（SSE 流式）
await streamChatMessage(sessionId, '如何重置 VPN 密码？', {
  onStep(step) {
    currentStep.value = step.label
  },
  onToken(content) {
    assistantMessage.content += content
  },
  onDone(session) {
    currentSession.value = session
  },
  onError(error) {
    showError(error)
  }
})

// 3. 多轮追问（同一会话）
await streamChatMessage(sessionId, '第二步具体怎么做？', { ... })
```

---

## 3. 查询会话列表

```http
GET /api/v1/portal/chat-sessions?page=1&page_size=10
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 42,
      "question": "VPN 密码问题",
      "last_answer": "VPN 密码重置步骤：1. 登录自助平台...",
      "message_count": 4,
      "created_at": "2026-06-16 10:30:00",
      "updated_at": "2026-06-16 10:31:03"
    }
  ],
  "total": 15,
  "page": 1,
  "page_size": 10
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int64 | 会话 ID |
| question | string | 会话标题 |
| last_answer | string | 最后一条回复摘要（截断 100 字） |
| message_count | int64 | 消息总数 |
| created_at | string | 创建时间 |
| updated_at | string | 最后更新时间 |

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 99999 | 500 | 服务未初始化 |

---

## 4. 删除会话

```http
DELETE /api/v1/portal/chat-sessions/:id
Authorization: Bearer <token>
```

> 删除会话及其全部消息。仅允许删除自己的会话（归属校验）。

**成功响应：** `{"code":0,"message":"success","data":null}`

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10002 | 403 | 非会话所有者，无权删除 |
| 10003 | 400 | 无效的会话 ID |
| 10004 | 404 | 会话不存在 |
| 99999 | 500 | 服务未初始化 |

---

## 5. 查询会话详情

```http
GET /api/v1/portal/chat-sessions/:id
Authorization: Bearer <token>
```

> 含归属校验：仅允许查看自己的会话，非会话属主返回 `code=10002`（无权查看该会话）。

**响应：** 含 `messages` 字段（多轮对话历史）及 `pipeline` 步骤指标：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "session_id": 42,
    "question": "VPN 密码问题",
    "answer": "VPN 密码重置步骤：1. 登录自助平台...",
    "sources": [{"doc_name": "VPN FAQ", "chunk_content": "...", "confidence": 0.85}],
    "confidence": 0.85,
    "can_submit_ticket": false,
    "duration_ms": 3200,
    "feedback": 0,
    "created_at": "2026-06-16 10:30:00",
    "pipeline": [
      {"step_id": "query_rewrite", "label": "查询改写", "duration_ms": 120, "success": true},
      {"step_id": "vector_retrieve", "label": "向量检索", "duration_ms": 45, "success": true}
    ],
    "messages": [
      {"id": 1, "role": "user", "content": "如何重置 VPN 密码？", "confidence": 0, "created_at": "2026-06-16 10:30:00"},
      {"id": 2, "role": "assistant", "content": "VPN 密码重置步骤：...", "sources": [...], "confidence": 0.85, "created_at": "2026-06-16 10:30:05"},
      {"id": 3, "role": "user", "content": "第二步具体怎么做？", "confidence": 0, "created_at": "2026-06-16 10:31:00"},
      {"id": 4, "role": "assistant", "content": "第二步需要...", "sources": [...], "confidence": 0.92, "created_at": "2026-06-16 10:31:03"}
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| messages | array | 多轮对话消息历史（按时间正序） |
| messages[].id | int64 | 消息 ID |
| messages[].role | string | `user` 或 `assistant` |
| messages[].content | string | 消息正文 |
| messages[].sources | array | 知识来源（仅 assistant 消息） |
| messages[].confidence | float64 | 置信度（仅 assistant 消息） |
| messages[].created_at | string | 消息创建时间 |
| pipeline | array | RAG 管道步骤指标（可选，含 step_id/label/duration_ms/success/error） |

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 无效的会话 ID |
| 10004 | 404 | 会话不存在 |
| 99999 | 500 | 服务未初始化 |

---

## 6. 提交反馈

```http
POST /api/v1/portal/chat-sessions/:id/feedback
Authorization: Bearer <token>
```

> 含归属校验：仅允许反馈本人所属会话。校验规则在 Service 层集中管理。

**请求体：**

```json
{
  "feedback": 1
}
```

| 值 | 说明 |
|----|------|
| 1 | 已解决 |
| 2 | 未解决 |

> 反馈值仅接受 1（已解决）或 2（未解决）。0 为默认初始值，不可通过 API 提交。

**错误响应：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 反馈值无效（非 1/2） |
| 10002 | 403 | 无权操作该会话（非属主） |
| 10004 | 404 | 会话不存在 |
| 99999 | 500 | 服务未初始化 |

> feedback 字段可选，默认为 0。

---

## 降级规则

RAG 管道的降级策略：单步骤失败不阻塞后续步骤：

| 步骤 | 失败行为 | 降级结果 |
|------|----------|----------|
| 查询改写 | 降级 | 使用原始 question |
| 多路检索 | 降级 | 使用单路检索 |
| 向量检索 | **阻塞** | 返回 `code=20002`（核心路径） |
| BM25 检索 | 降级 | BM25 结果为空，仅用向量结果 |
| RRF 融合 | 降级 | 使用单路结果（哪个有结果用哪个） |
| 重排序 | 降级 | 使用 RRF 排序结果 |
| LLM 生成 | **阻塞** | 返回 `code=20001`（核心路径） |

| 最终场景 | 行为 |
|----------|------|
| LLM 服务不可达 | 返回 `code=20001`，提示 AI 不可用 |
| pgvector 检索失败 | 返回 `code=20002`，提示 RAG 服务不可用 |
| 检索结果为空 | 返回兜底答案 + `can_submit_ticket=true` |
| 置信度 < 阈值（默认 0.6） | 返回兜底答案 + `can_submit_ticket=true` |

**兜底文本：**
- AI 服务不可用：「当前 AI 服务暂不可用，请提交申告由人工处理」
- 低置信度/无结果：「暂未找到足够匹配的知识，建议提交申告由运维人员人工处理」

## 置信度阈值

默认阈值 `0.6`，可通过系统配置 API 修改：

```http
PUT /api/v1/admin/configs/ai.threshold
{"value": 0.7}
```

RAG 默认检索 Top K 也可通过系统配置调整：

```http
PUT /api/v1/admin/configs/ai.top_k
{"value": 5}
```

# 智能问答接口

> **Base URL:** `/api/v1/portal` | **Auth:** JWT | **Module:** Chat & RAG Pipeline

## 问答架构

问答由 Agent ReAct 循环驱动——Agent 自主决策何时检索知识库、何时搜索网络、何时写入新知识：

```
用户问题
  → memory(recall)       — 检索会话/全局记忆
  → kb(search)           — RAG 检索（CRAG 充分性评估：strong/ambiguous/weak）
  → [weak] web_search    — 知识库不足时补搜网络（Exa→Tavily→DuckDuckGo）
  → [weak] web_fetch     — 页面提取（Firecrawl→本地）
  → LLM 生成             — 带上下文生成答案（SSE 流式，token + tool_call + tool_result 事件）
  → kb(create)           — [可选] 将新发现写入知识库闭环
```

**检索优先级**：memory(recall, session) → memory(recall, global) → kb(search) → web_search → kb(create)

**CRAG 充分性评估**：每次 kb(search) 返回 verdict（strong/ambiguous/weak），Agent 据 verdict 决定是否补搜。

**reranker 守卫**：cross-encoder 子进程不可用时静默跳过，降级为原始排序。

## 1. 创建会话

```http
POST /api/v1/portal/threads
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
POST /api/v1/portal/threads/:id/stream
Authorization: Bearer <token>
Content-Type: application/json
```

> 在已有会话中发送消息，SSE 流式返回 AI 答案。支持多轮对话——历史消息自动注入 LLM 上下文（滑动窗口上限 10 条，约 5 轮 Q&A，可通过 `COGNOS_AI_MAX_HISTORY_MESSAGES` 调整）。

**请求体：**

```json
{
  "question": "如何重置 VPN 密码？"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| question | string | ✓ | 用户问题（max 2000 字符） |

**SSE 事件流：**

响应类型为 `text/event-stream`，Agent ReAct 循环驱动，包含以下事件类型：

### reasoning 事件 — Agent 思考

```
data: {"type":"reasoning","content":"需要检索知识库..."}
```

Agent 的思考过程，展示决策意图（是否检索、是否补搜）。

### tool_call 事件 — 工具调用

```
data: {"type":"tool_call","name":"kb","input":{"action":"search","query":"VPN 密码重置","kb_id":1,"limit":5}}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 工具名：kb / memory / web_search / web_fetch / bash 等 |
| input | object | 工具参数（按工具不同） |

### tool_result 事件 — 工具返回

```
data: {"type":"tool_result","name":"kb","output":"[sufficiency: strong | confidence=0.82] 检索充分..."}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 工具名 |
| output | string | 工具结果（kb(search) 含 `[sufficiency: level]` preamble） |

> Agent ReAct 循环可能多轮 tool_call/tool_result（多跳推理）。weak verdict 时 Agent 自主触发 web_search。

### token 事件 — 逐 token 流式发送

```
data: {"type":"token","content":"VPN 密码"}
data: {"type":"token","content":"重置步骤"}
```

LLM 生成答案的 token 实时转发。

### error 事件 — 流式过程错误

```
data: {"type":"error","error":"LLM 生成中断: context deadline exceeded"}
```

### done 事件 — 流式结束，含完整元数据

```
data: {"type":"done","metadata":{"session_id":42,"question":"如何重置 VPN 密码？","answer":"VPN 密码重置步骤：1. 登录自助平台...","sources":[{"doc_name":"VPN 密码重置 FAQ","chunk_content":"...","confidence":0.85}],"confidence":0.76,"confidence_raw":0.76,"confidence_level":"high","can_submit_ticket":false,"duration_ms":3200,"created_at":"2026-06-16 10:30:05"}}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| metadata.session_id | int64 | 会话 ID |
| metadata.question | string | 用户问题 |
| metadata.answer | string | AI 完整答案（与流式 token 拼接结果一致） |
| metadata.sources | array | 知识来源列表（`confidence_level=low` 时为空） |
| metadata.sources[].doc_name | string | 来源文档名称 |
| metadata.sources[].chunk_content | string | 匹配的切片内容 |
| metadata.sources[].confidence | float | 该来源 chunk 展示分 (0-1) |
| metadata.confidence | float | 同 confidence_raw（别名） |
| metadata.confidence_raw | float | 原始综合置信度 Conf_raw [0,1] |
| metadata.confidence_level | string | 置信度等级：`"high"` / `"medium"` / `"low"` |
| metadata.can_submit_ticket | bool | 是否建议转人工申告（`confidence_level != "high"`） |
| metadata.duration_ms | int | 总耗时（毫秒） |
| metadata.created_at | string | 会话创建时间 |

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
import { createSession, streamUrl } from '@/lib/api/chat'

// 1. 创建会话
const res = await createSession(1, '如何重置 VPN 密码？')
const sessionId = res.session_id

// 2. 发送问题并消费 SSE 流（fetch POST + ReadableStream 解析）
const resp = await fetch(streamUrl(sessionId), {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
  body: JSON.stringify({ question: '如何重置 VPN 密码？' }),
})
const reader = resp.body!.getReader()
const decoder = new TextDecoder()
let buffer = ''
while (true) {
  const { done, value } = await reader.read()
  if (done) break
  buffer += decoder.decode(value, { stream: true })
  for (const line of buffer.split('\n')) {
    if (!line.startsWith('data: ')) continue
    const evt = JSON.parse(line.slice(6))
    switch (evt.type) {
      case 'reasoning': setReasoning(prev => prev + evt.content); break
      case 'tool_call': /* evt.name + evt.input: 工具调用 */ break
      case 'tool_result': /* evt.name + evt.output: 工具返回（含 CRAG verdict） */ break
      case 'token':  setAssistant(prev => prev + evt.content); break
      case 'done':   /* evt.metadata: 会话元数据 */ break
      case 'error':  throw new Error(evt.error)
    }
  }
}

// 3. 多轮追问：复用同一 sessionId，重复步骤 2（body 改为新问题）
```

---

## 3. 查询会话列表

```http
GET /api/v1/portal/threads?page=1&page_size=10
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
DELETE /api/v1/portal/threads/:id
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
GET /api/v1/portal/threads/:id
Authorization: Bearer <token>
```

> 含归属校验：仅允许查看自己的会话，非会话属主返回 `code=10002`（无权查看该会话）。

**响应：** 含 `messages` 字段（多轮对话历史）：

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
    "created_at": "2026-06-16 10:30:00",
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

**错误：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 无效的会话 ID |
| 10004 | 404 | 会话不存在 |
| 99999 | 500 | 服务未初始化 |

---

## 6. 取消生成

```http
POST /api/v1/portal/threads/:id/cancel
Authorization: Bearer <token>
```

取消当前会话中正在进行的 SSE 生成任务。请求体为空。

**错误响应：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10004 | 404 | 会话不存在 |
| 99999 | 500 | 服务未初始化 |

---

## 7. 恢复流式输出

```http
GET /api/v1/portal/threads/:id/stream
Authorization: Bearer <token>
```

恢复被中断的 SSE 流式输出，返回格式同 [§2](#2-发送消息sse-流式)。

---

## 8. 更新会话元信息

```http
PATCH /api/v1/portal/threads/:id
Authorization: Bearer <token>
```

**请求体：**

```json
{
  "title": "更新后的标题",
  "kb_id": 1
}
```

**错误响应：**

| code | HTTP 状态 | 说明 |
|------|-----------|------|
| 10003 | 400 | 参数校验失败 |
| 10004 | 404 | 会话不存在 |
| 99999 | 500 | 服务未初始化 |

---

## 降级规则

Agent ReAct 检索的降级策略：单路失败不阻塞另一路，核心路径失败返回错误码：

| 失败点 | 行为 | 降级结果 |
|--------|------|----------|
| 向量检索 | Warn 日志 | BM25-only 继续；双路全失败返回 `code=20002` |
| BM25 检索 | Warn 日志 | 向量-only 继续 |
| rerank | Warn 日志 | 使用 RRF 排序结果 |
| CRAG LLM 评估 | 降级阈值 | 永不阻塞检索返回 |
| LLM 生成 | **阻塞** | 返回 `code=20001`（核心路径） |

| 最终场景 | 行为 |
|----------|------|
| LLM 服务不可达 | 返回 `code=20001`，提示 AI 不可用 |
| 检索双路全失败 | 返回 `code=20002`，提示 RAG 服务不可用 |
| CRAG weak verdict | Agent 自主触发 web_search 补搜 |
| 检索结果为空 | 返回兜底答案 + `can_submit_ticket=true` |

**兜底文本：**
- AI 服务不可用：「当前 AI 服务暂不可用，请提交申告由人工处理」
- 低置信度/无结果：「暂未找到足够匹配的知识，建议提交申告由人工处理」

## 置信度等级

三级体系：`high`（高）/ `medium`（中）/ `low`（低），由两个阈值配置决定：

```http
PUT /api/v1/admin/configs/ai.confidence_threshold_low  {"value": 0.40}
PUT /api/v1/admin/configs/ai.confidence_threshold_high {"value": 0.70}
```

- Conf_raw ≥ T_high → high
- T_low ≤ Conf_raw < T_high → medium
- Conf_raw < T_low → low

### 分位数自动计算

```http
POST /api/v1/admin/confidence/compute-thresholds
{"days": 7}
```

从近 N 天对话数据中计算 P30、P70 分位数，管理员可一键应用到阈值配置。

### 等级话术

| 等级 | 前端展示 |
|------|----------|
| high | 正常输出 + 来源引用 |
| medium | 答案前提示「匹配资料有限，内容仅供参考」 |
| low | 警告条 + 不展示来源 + 引导提交申告 |

RAG 默认检索 Top K 也可通过系统配置调整：

```http
PUT /api/v1/admin/configs/ai.top_k
{"value": 5}
```

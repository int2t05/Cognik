# 可续传的智能问答流式对话 — 设计文档

> 日期：2026-06-24 ｜ 适用：OpsMind 门户端智能问答（RAG 流式对话）

## 1. 背景与目标

### 问题
当前在门户端发起提问、LLM 正在流式回复的过程中，用户若**切换会话**或**进入其他功能页面**，回来时**自己发送的消息**和 **AI 已输出到一半的内容**都会消失。

根因有两层：

1. **前端**：流式状态保存在 `useChatStream` 这个 hook 里，而 hook 挂在聊天页面组件内。导航离开 → 页面卸载 → `useEffect` 清理函数 `abort()` 掉流，所有内存中的消息随之丢失。
2. **后端**（`server/internal/service/chat_service.go`）：生成在一个绑定**请求 context** 的 goroutine 中进行，且 user+assistant 消息**仅在 `done` 事件时**才持久化。客户端断开 → `ctx.Done()` 触发 → 生成停止 → 什么都没存下来。

### 目标（已与用户确认）
- **持久化强度：扛住页面刷新 / 重开标签**（不只是 SPA 内导航）。
- **续生成体验：真实断点续传** —— 刷新后重连到仍在进行的生成，token 继续实时蹦出。
- 支持**多会话并行**生成，互不干扰。
- 对话失败也要**保留用户消息**。
- "停止生成"按钮要**真正停止后端生成**，而非仅断开本地连接。

### 关键假设
- **单实例部署**：进行中的生成放在后端**内存**管理，不引入 Redis 等外部中间件。未来若需多副本，再替换为 Redis Streams（不在本设计范围）。
- **生成超时**：后端独立 cap，沿用现有 120s。
- 每个会话**同一时刻至多 1 个进行中的生成**（符合一问一答语义）；"多会话并行"指不同 session 各有各的生成。

## 2. 总体架构

核心转变：生成的「所有权」从 HTTP 请求转移到后端的 `GenerationHub`。HTTP 请求降级为「订阅者」，断开不影响生成。

| # | 部件 | 位置 | 职责 |
|---|---|---|---|
| 1 | `GenerationHub` | `server/internal/service/generation_hub.go`（新） | 内存管理所有「进行中」的生成：按 sessionID 索引，每个生成持有带序号 token 缓冲 + 订阅者集合 + 状态 + 取消函数 |
| 2 | `ChatMessage.Status` | `server/internal/model/chat.go`（改） | 新增字段 `generating` / `completed` / `failed`，让刷新后能判断是否还在生成 |
| 3 | StreamChat 改造 | `server/internal/service/chat_service.go`（改） | 立即持久化用户消息 → 建 assistant 消息(generating) → 在 `context.Background()` 跑生成 → 完成时落库标记 completed |
| 4 | 续传 + 取消接口 | `server/internal/router/portal.go` + `handler/chat.go`（改） | `GET .../stream?since=<seq>` 续传；`POST .../cancel` 真正取消 |
| 5 | `ChatStreamProvider` | `web/src/app/portal/layout.tsx` 内（新） | 全局 store：跨页面保活前台流；进入会话时若有 active 生成则自动续传 |

**数据真相源**：进行中 → Hub 缓冲；已完成 → DB。前端进入会话先 GET 详情（拿已完成消息 + 是否有 generating），再决定是否开续传流。

## 3. GenerationHub 内部结构与并发模型

```go
// 一次进行中的生成
type generation struct {
    sessionID int64
    msgID     int64          // 对应 assistant 消息行 ID
    mu        sync.Mutex
    buffer    []StreamEvent  // 有序事件缓冲，下标即 seq
    status    string         // generating | completed | failed
    final     *StreamEvent   // done/error 事件（含 sources/confidence）
    subs      map[int]chan StreamEvent // 订阅者：订阅ID → 通道
    nextSubID int
    cancel    context.CancelFunc // 真正取消生成
    doneAt    time.Time          // 完成时间，用于宽限期清理
}

type GenerationHub struct {
    mu  sync.RWMutex
    gen map[int64]*generation // sessionID → 进行中的生成（每会话至多 1 个）
}
```

**核心方法：**

| 方法 | 作用 |
|---|---|
| `Start(sessionID, msgID, cancel)` | 登记新生成；若该会话已有进行中的生成则拒绝（防同会话并发重复提问） |
| `Publish(sessionID, evt)` | 生成 goroutine 调用：加锁追加到 buffer，扇出给所有 subs |
| `Subscribe(sessionID, since)` | 订阅者调用：加锁，先把 `buffer[since:]` 回放，再注册新 channel 接后续；返回 `(replay, ch, subID)` |
| `Unsubscribe(sessionID, subID)` | 客户端断开时移除其 channel（**不影响生成**） |
| `Finish(sessionID, final)` | 置 status、存 final、记 doneAt、关闭所有订阅 channel；启动宽限期（30s）后从 map 删除 |
| `Cancel(sessionID)` | 调用 `cancel()` 停 goroutine |

**并发安全要点：**
- 两层锁：Hub 级 `RWMutex` 保 `gen` map；每个 `generation` 自带 `mu` 保自己的 buffer/subs。避免一个慢订阅者卡住整个 Hub。
- `Publish` 向订阅 channel 用**非阻塞 + 带缓冲**（cap=256）发送；若某订阅者消费太慢导致满，丢弃该订阅（它可凭 `since` 重连补回），保证生成本身永不被订阅者阻塞。
- **先回放后注册必须在同一把 `generation.mu` 锁内完成**，否则回放与实时之间会漏事件。这是 seq 去重的根：订阅者只认 seq 递增，重连用最后看到的 seq 当 `since`。

## 4. 端到端数据流（四条路径）

### 路径 ①：正常发送（首次）
```
前端 handleSend → POST /chat-sessions/:id/stream {question}
后端 handler:
 1. 解析 → 调 svc.StreamChat(sessionID, question, userID)
 2. svc: 立即落库 user 消息；建 assistant 消息(status=generating, 空内容)；拿到 msgID
 3. svc: gctx, cancel := context.WithTimeout(context.Background(), 120s)
         hub.Start(sessionID, msgID, cancel); go runGeneration(gctx, ...)  // 脱离请求 ctx
 4. handler: replay, ch, subID := hub.Subscribe(sessionID, 0)
         先写 replay，再 for evt := range ch { writeSSE(evt); flush() }   // 请求只做转发
runGeneration goroutine:
 - 跑 RAG 管道 → 每个 step/token 调 hub.Publish
 - done: 落库 assistant 最终内容+sources+confidence, status=completed; hub.Finish
```
关键：第 2 步**立即落库 user 消息**（解决"我发的消息也没了"）；生成在 background ctx（解决"断开就停"）。

### 路径 ②：导航离开 / 切会话（SPA 内，不刷新）
- 全局 `ChatStreamProvider` 持有订阅，**不随页面卸载**。切到别的功能页 → 订阅照常在 store 里收事件；生成在后端继续。切回来 → store 里已有最新内容，直接渲染，**全程无网络重连**。
- 切到另一个会话：对新会话开它自己的订阅；旧会话订阅保留在 store（后台继续收），实现多会话并行。

### 路径 ③：刷新 / 重开标签（真实续传）
```
页面重新挂载 → 进入会话:
 1. GET /chat-sessions/:id → 返回已落库消息；最后一条 assistant 若 status=generating，
    带上 msgID（其 content 此时为空——partial token 只在 Hub 缓冲，不落库）
 2. 前端 GET /chat-sessions/:id/stream?since=0（续传，纯订阅，从头回放整个缓冲）
 3. 后端 hub.Subscribe(sessionID, 0) → 回放 buffer[0:] 全量 + 接实时 → token 从头补齐并继续蹦
若生成已在宽限期内结束：续传请求拿到完整 buffer + done；若已过宽限期或 Hub 无此生成：
GET 详情里该 assistant 已是 completed 完整消息，前端不必开流。
```

> 说明：partial token 不写库（避免高频 DB 写）。刷新续传一律 `since=0` 全量回放缓冲；
> `since` 参数主要服务于「同一前端会话内的网络抖动重连」——前端用它已消费到的 `lastSeq` 避免重复。

### 路径 ④：停止生成
```
前端「停止」按钮 → POST /chat-sessions/:id/cancel
后端 → hub.Cancel(sessionID) → cancel() → goroutine 收到 gctx.Done()
 → 落库 assistant 当前已生成内容, status=failed; hub.Finish
```
与旧 `abort()`（只断本地 fetch）不同——现在真停后端。

## 5. 前端状态模型与接口契约

### 全局 store 形状（放在 portal 布局内，跨 chat/tickets/messages 保活）
```ts
interface SessionStream {
  sessionId: number;
  messages: ChatMessage[];      // 该会话当前消息（含正在生成的 assistant）
  status: 'idle' | 'streaming' | 'error';
  lastSeq: number;              // 已消费到的事件序号，断点续传用
  pipelineSteps: PipelineStep[];
  currentStep: string | null;
  controller: AbortController;  // 仅用于「主动断开本地订阅」，不取消后端
}
interface ChatStreamStore {
  streams: Record<number, SessionStream>;  // sessionId → 流状态（多会话并行）
  send(sessionId, kbId, question): Promise<number>;
  resume(sessionId, since): void;          // 路径③
  cancel(sessionId): Promise<void>;        // 路径④，调后端
  getStream(sessionId): SessionStream | undefined;
}
```
- `useChatStream` 从「页面内 hook」重构为「读全局 store 的 selector」。聊天页面只读 `getStream(currentSessionId)`，自身不再持有流状态 → 卸载不丢。
- 进入会话时：先 `getChatDetail` 填 messages；若最后一条 assistant `status=generating`，自动 `resume(sessionId, seq)`。

### SSE 事件契约（新增 `seq`）
```
data: {"type":"step","seq":0,"id":"vector_retrieve","label":"向量检索"}
data: {"type":"token","seq":12,"content":"VPN"}
data: {"type":"done","seq":88,"metadata":{...}}     // 含 answer/sources/confidence
data: {"type":"error","seq":88,"error":"..."}
```
`seq` 在整个生成内单调递增（step 与 token 共用一个序列）；前端只接受 `seq > lastSeq` 的事件 → 续传回放天然去重。

### 后端接口契约
| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/chat-sessions/:id/stream` | 不变：发起新问题（body 带 question）。响应 SSE 带 seq |
| GET | `/chat-sessions/:id/stream?since=<seq>` | **新**：续传——不带 question，纯订阅，从 since 回放 |
| POST | `/chat-sessions/:id/cancel` | **新**：真正取消后端生成 |
| GET | `/chat-sessions/:id` | 改：assistant 消息返回 `status`（generating 时 content 为空，前端据此开续传流 `since=0`） |

**鉴权**：三个流/取消接口都校验 `session.user_id == 当前用户`，防止越权订阅他人会话。

## 6. 错误处理矩阵

| 场景 | 后端行为 | 前端表现 |
|---|---|---|
| LLM 生成失败(20001) | Publish `error`；落库 assistant `status=failed`（保留已生成部分）；`hub.Finish` | toast 报错，**保留用户消息** + 半截内容；可重发 |
| 向量检索失败(20002) | 同上 | 同上 |
| 生成超时(120s) | `gctx` 超时 → goroutine 退出 → 落库 failed | 续传/前台流收到 error |
| 客户端断开（导航/刷新/网络） | **无操作**——生成照常，Finish 时落库 | 重连后凭 `since` 续上 |
| 续传时生成已结束且过宽限期 | Subscribe 返回「无活跃生成」 | 退回用 GET 详情的完整消息，不开流 |
| 同会话重复发起 | `hub.Start` 拒绝 | 提示"该会话正在生成中" |
| 越权订阅他人会话 | 鉴权失败 10002 | 报错 |
| **服务重启（内存 Hub 丢失）** | 进行中生成丢失；已落库完整消息仍在 | **启动时扫一遍把残留 `generating` 标记为 `failed`**，避免前端永久卡"生成中" |

## 7. 测试策略

遵循项目规范：测试在 `server/tests/` 外部 `_test` 包、不 mock、依赖真实模块；集成测试用 `-tags=integration`。

- **Hub 单元/并发测试**（`tests/service/`）：
  - 先回放后注册不漏事件：Publish 若干 → Subscribe(since=N) → 校验回放+实时连续无缺号。
  - 慢订阅者不阻塞生成：一个不消费的订阅者 + 正常订阅者，生成照常完成。
  - 多订阅者同一生成都收到全量；Unsubscribe 不影响生成。
  - Cancel 真正停止 goroutine。
- **集成测试**（`tests/service/` + 真实 PG/embedding）：
  - StreamChat 立即落库 user 消息（断开后查 DB 有 user 消息）。
  - 客户端断开后生成仍完成并落库 completed。
  - 启动清理把残留 generating → failed。
- **前端 E2E**（Playwright）：
  - 发问 → 切到"我的申告"页 → 切回：内容还在/继续。
  - 发问 → 刷新页面：看到"生成中"并 token 继续蹦，最终完整。
  - 两个会话并行生成互不干扰。
  - 停止按钮真正停止（刷新后不再继续）。

## 8. 影响面与迁移

- **数据库**：`chat_messages` 新增 `status` 列（GORM AutoMigrate 自动补；默认值 `completed` 兼容历史数据）。
- **启动钩子**：新增「残留 generating → failed」清理，放在 AutoMigrate 之后。
- **不破坏**现有 `POST .../stream` 契约（仅在事件里加 `seq` 字段，旧前端忽略即可）。
- **降级一致性**：沿用 `docs/API/chat.md` 的降级码（20001/20002）。

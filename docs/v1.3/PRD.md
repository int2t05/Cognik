# V1.3 — Agent 基座（PRD）

## 1. 背景与目标

### 1.1 现状

V1.2 的 AI 问答是固定 7 步线性 RAG 管道（改写→路由→检索→融合→重排→去重→置信度）喂给手写 `OpenAIClient`。存在三个核心缺陷：

- `ChatRequest` 无 tool calling 字段（Agent Loop 致命阻断项）
- `readSSEStream` 用 `bufio.Scanner` 有 1MB buffer 限制
- 手写 `OpenAIClient` HTTP + 自建 `GenerationHub` pub/sub 无参考来源

### 1.2 目标

用 Eino 框架替换自建 AI 组件，搭建 Agent Loop 基座；交付层重构为订阅渠道制网关（对齐 LangGraph Server / Mastra Durable / OpenAI background）。

- Eino `ReactAgent` + `eino-ext/openai` ChatModel 替代手写 `OpenAIClient` + 线性 RAG 管道
- `runtime.Gateway[E]` 网关替代 `GenerationHub`（runID 通用 key + 环形缓冲 + 游标重放）
- 9 基础 OS 工具（bash / async_bash / read_file / write_file / edit_file / list_dir / glob / grep / mkdir）
- SubAgent（research 只读 + coder 读写，`adk.NewAgentTool` 包装为工具，父 Agent 委托子 Agent）
- 异步任务（Task 模型 + TaskManager 后台执行 + SQLite 持久化）
- Provider 热切换（LLMConfigManager.OnChange → ChatModel 重建）
- 前端 parts 数组模型（对标 AI SDK UIMessage.parts），流式渲染解耦，tool_call/tool_result/reasoning 全事件渲染

### 1.3 非目标

- 业务工具 `search_knowledge_base` / `ticket_lookup`（V2.0，`rag/` 引擎保留作后端）
- MCP 工具（V1.4，SearXNG 部署后）
- Eino DeepAgent / interrupt/resume HITL（V2.0+）

## 2. 功能需求

### 2.1 Agent 领域包（`server/internal/agent/`）

- `provider.go`：Eino ChatModel 工厂 + `atomic.Value` 热切换
- `agent.go`：ReactAgent 构造 + SubAgent（research + coder，`adk.NewAgentTool`）+ `StreamToolCallChecker`（llama.cpp 兼容）
- `runner.go`：`AgentRunner.Stream` → `<-chan AgentEvent`（事件生产者）
- `types.go`：AgentEvent 事件类型 + 常量
- `store/`：ChatStore 接口 + Thread/Message 模型 + SQLite 实现（Agent 数据隔离）
- `task/`：Task 模型 + TaskStore + TaskManager（异步任务后台执行）
- `tools/`：bash / async_bash（StreamableTool）/ read_file / write_file / edit_file / list_dir / glob / grep / mkdir

### 2.2 网关引擎（`server/internal/infra/runtime/gateway.go`）

`Gateway[E]` 订阅渠道制网关：
- runID（string）通用 key，不绑 chat session
- 泛型事件类型 E
- Broadcaster（实时 fan-out，非阻塞）+ EventStore（有界环形缓冲，seq 游标重放）职责分离
- `Start` / `Publish` / `Subscribe(since)` / `Finish`（30s grace）/ `Cancel` / `Active`

### 2.3 SSE 事件

| 事件类型 | 字段 | 来源 |
|---------|------|------|
| `reasoning` | content | Eino MessageFuture ReasoningContent |
| `token` | content | Eino StreamReader Content |
| `tool_call` | id/label/content | Eino MessageFuture ToolCalls |
| `tool_result` | content | Eino MessageFuture Role==Tool |
| `done` | metadata.answer | Stream EOF |
| `error` | error | 异常 |

SSE 帧格式：`id: {seq}\ndata: {json}\n\n`（`id:` 供 Last-Event-ID 重连，前端兼容不破坏）。

### 2.4 SubAgent

两个内置子 Agent（对标 Claude Code Explore / general-purpose），通过 `adk.NewAgentTool` 包装为工具，父 Agent 可委托任务给子 Agent（上下文隔离）：

| 子 Agent | 工具集 | 对标 |
|---------|--------|------|
| research | read_file / glob / grep / list_dir（只读探查） | Claude Code Explore |
| coder | bash / async_bash / edit_file / write_file / mkdir（读写操作） | Claude Code general-purpose |

### 2.5 异步任务

`Task` 模型（thread_id / type / status / input / output）+ `TaskManager` 后台执行（detached goroutine + 300s 超时），SQLite 持久化。状态机：pending → running → completed / failed / cancelled。

### 2.6 热切换

`LLMConfigManager.OnChange` → `agentModelFactory.OnConfigChange()` 重建 Eino ChatModel；与 Embedding 热切换共用同一回调。

## 3. 非功能需求

- **安全**：工具 workDir sandbox（path traversal 防护）、timeout、maxBytes 截断
- **降级**：ChatModel 未初始化 → Agent 503（safeHandler placeholder）
- **可靠性**：detached ctx 生成 goroutine，客户端断开不停止生成/落库（对齐 LangGraph `on_disconnect=continue`）
- **断线重连**：`GET ?since=N` 游标重放（网关 Subscribe(since)）

## 4. 验收标准

| 验收项 | 标准 |
|--------|------|
| Agent Loop 跑通 | ReactAgent `Stream()` 返回；前端显示回答 |
| Tool calling | bash 工具被自主调用；tool_call/tool_result 事件发出 |
| 并行工具 | 多个 tool_call 独立 part 渲染（不合并 args） |
| 异步工具 | async_bash 流式输出 stdout/stderr（不阻塞 SSE 流） |
| SubAgent | 复杂任务委托子 Agent（research/coder）→ SubAgentPart 渲染 |
| 异步任务 | Task 后台执行，TaskCard 状态展示（pending→running→completed） |
| 前端渲染 | parts 数组模型：text/reasoning/tool_call/tool_result 全事件渲染 |
| 流式 | token-by-token SSE 输出 |
| 推理 | reasoning 事件（llama.cpp thinking mode） |
| 断线重连 | `GET ?since=N` 续传剩余事件 → done |
| 落库 | 客户端断开后生成跑完，详情含完整 answer |
| 多订阅者 | 同 runID 两个连接都收到完整事件流 |
| 热切换 | 配置变更后新对话用新 ChatModel |
| 降级 | Agent 异常 → error 事件，不崩溃进程 |
| 网关单元测试 | Start/Publish/Subscribe replay/Finish grace/Cancel/non-blocking drop 全通过 |

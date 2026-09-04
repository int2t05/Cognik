# OpsMind 产品技术路线图

> 从 V1.0 到 V2.0 的版本规划。代码级改进清单见 [`TODO.md`](TODO.md)，技术架构详见 [`TECH.md`](TECH.md)。

## 1. 项目愿景

OpsMind 是面向企业 IT 运维的**私有部署 AI 数字员工系统**。核心目标：让运维团队从重复性咨询中解放，让知识沉淀为可复用资产，让 AI 成为运维流程的第一响应者。

**设计原则**：私有部署优先（数据不出域）、自建 RAG 引擎（全链路可控可审计）、单体分层架构（简洁可维护）。

---

## 2. 版本总览

```mermaid
flowchart LR
    V1["V1.0<br/>固定管道 RAG<br/>(已交付)"] --> V11["V1.1<br/>存储简化"]
    V11 --> V12["V1.2<br/>业务完善"]
    V12 --> V13["V1.3<br/>Agent 基座"]
    V13 --> V14["V1.4<br/>深度搜索"]
    V14 --> V2["V2.0<br/>Agentic RAG<br/>(终点)"]
```

| 版本 | 主题 | 核心交付 | 状态 |
|------|------|---------|------|
| V1.0 | 固定管道 RAG | 7 步 RAG 管道 + 申告状态机 + 知识库 CRUD + RBAC + SSE 流式 | ✅ 已交付 |
| V1.1 | 存储简化 | MinIO→本地 FS；配置体系统一 | 📋 规划中 |
| V1.2 | 业务完善 | 知识库与申告增强；Markdown 富文本；看板增强；前端体验优化 | ✅ 已交付 |
| V1.3 | Agent 基座 | Eino ReactAgent + 订阅渠道网关(Gateway)；9 OS 工具(bash/async_bash/read/write/edit/list/glob/grep/mkdir)；SubAgent(research+coder)；异步任务；SQLite 隔离；threads API；parts 数组模型前端渲染 | ✅ 已交付 |
| V1.4 | 深度搜索 | SearXNG 部署；网络搜索工具；深度研究架构（规划器→搜索器→反思器→合成器）；搜索 API 集成 | 📋 规划中 |
| V2.0 | Agentic RAG | Agent ReAct 循环替代固定管道；Agent 事件 UI；多步推理；事件知识自进化 | 📋 规划中 |

---

## 3. V1.0 — 固定管道 RAG（已交付）

### 3.1 已交付能力

```mermaid
mindmap
  root((OpsMind V1.0))
    智能问答
      自建 7 步 RAG 管道
      BM25 + 向量混合 + RRF
      cross-encoder 重排序
      SSE token 级流式
      多轮对话 + 历史管理
      置信度评分与降级
    知识库管理
      统一文章模型
      草稿→审核→发布→停用
      文档上传 PDF/DOCX/MD/TXT
      异步处理 parse→chunk→embed
      pgvector halfvec + HNSW
      BM25 索引自动重建
    申告管理
      完整状态机
      7 天自动关闭
      CAS 并发防护
      知识候选沉淀
      站内消息通知
    运营后台
      RBAC 四角色
      JWT 双令牌
      动态菜单渲染
      数据看板 + 趋势
      审计日志
      LLM 配置热替换
    基础设施
      Docker Compose 一键部署
      llama.cpp 本地推理
      OpenAI-compatible 多 Provider
      Apple Design 双主题
```

### 3.2 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go + Gin + GORM |
| 数据库 | PostgreSQL + pgvector (halfvec + HNSW) |
| 对象存储 | MinIO (S3-compatible) |
| RAG | 自建 Go 引擎 — BM25 (gse) / 向量 (pgvector) / RRF / cross-encoder |
| LLM | llama.cpp server 或 OpenAI-compatible API |
| 前端 | Next.js + React + TypeScript + Radix UI + SWR + Tailwind CSS |
| 部署 | Docker Compose |

---

## 4. V1.1 — 存储简化

**目标**：移除 MinIO 依赖，降低部署复杂度和资源占用。

### 4.1 MinIO → 本地文件系统

MinIO 在单实例部署中增加 200-500MB RAM + HTTP 延迟层。同类系统（Dify / Open WebUI / AnythingLLM）均默认本地 FS。

```mermaid
flowchart LR
    subgraph 当前["V1.0"]
        S1["MinIO 容器"] -->|"HTTP 1-5ms"| APP1["opsmind-server"]
    end
    subgraph 目标["V1.1"]
        S2["本地 FS volume"] -->|"syscall <0.1ms"| APP2["opsmind-server"]
    end
    当前 -->|迁移| 目标
```

### 4.2 交付项

| 项 | 说明 | 验收标准 |
|----|------|---------|
| `LocalFSClient` | 本地 FS 适配器，原子写（temp+rename），UUID 文件名 | `StorageClient` 接口不变；MinIO 数据迁移脚本可用 |
| 后端代理下载 | `GET /api/storage/:bucket/:key` 替代 Presigned URL | 认证后 `http.ServeFile`；权限校验通过 |
| Docker 简化 | 移除 minio 服务，加 `storage_data` volume | `docker compose up` 仅 3 必须服务 |
| 配置统一 | `storage.type` (local/minio) + `storage.root_path` | 切换配置即可换后端，无需改代码 |
| MinIO 惰性下载修复 | 本地 FS 直接 `os.Open`，无惰性问题 | `defer Close` 检查错误 |

### 4.3 保留决策

| 组件 | 决策 | 依据 |
|------|------|------|
| pgvector | 保留 | halfvec(FP16) + HNSW 不可替代；增长最快 PG 扩展 |
| PostgreSQL | 保留 | 12+ JSONB 列依赖 GIN 索引；跨表事务一致性 |
| `MinIOClient` | 保留代码 | 作为 escape hatch，多实例时恢复使用 |

---

## 5. V1.2 — 业务完善

**目标**：完善知识库、申告、看板等业务功能与前端体验，为 Agent 基座夯实业务基础。

### 5.1 知识库上传增强

| 项 | 说明 | 验收标准 |
|----|------|---------|
| 上传上限配置化 | 50MB 硬编码改为按 KB 粒度配置 | `kb.max_upload_size` 可配置 |
| 批量上传 | 支持多文件拖拽批量上传 | 前端 react-dropzone 拖拽；后端并发处理 |
| 上传进度 | 前端显示上传进度条 | XMLHttpRequest upload progress 实时反馈 |
| 文件类型校验 | 前端+后端双重校验文件类型 | 非 PDF/DOCX/XLSX/PPTX/MD/TXT 拒绝 |

### 5.2 申告管理增强

| 项 | 说明 | 验收标准 |
|----|------|---------|
| 申告标签 | 申告支持标签分类（复用 Tags 字段） | TagsInput 组件；回车/逗号/粘贴自动分割 |
| 批量操作 | 申告批量删除/关闭 | 多选 + 批量操作确认 |
| 处理时限 | 申告可设处理时限，超时预警 | 配置时限；超时站内消息通知 |

### 5.3 看板增强

| 项 | 说明 | 验收标准 |
|----|------|---------|
| 自定义日期范围 | 看板支持自定义起止日期 | 日期选择器；趋势图按范围刷新 |
| 数据导出 | 看板数据 CSV 导出 | PapaParse 生成 CSV + BOM 头防 Excel 乱码 |
| `granularity` 生效 | 趋势查询 `granularity` 参数实际生效 | day/week 切换正常 |

### 5.4 前端体验优化

| 项 | 说明 | 验收标准 |
|----|------|---------|
| 代码分割 | `next/dynamic` 按路由懒加载重组件 | TrendChart / ChatPipeline / Markdown 组件按需加载 |
| 虚拟列表高度 | 变长消息 `estimateSize` 动态测量 | 按内容长度估算；滚动位置准确 |
| 表单 required 标记 | 前端表单必填字段标记 `*` | Field 组件 required prop 全覆盖 |
| 搜索空状态 | 列表搜索无结果显示 EmptyState | 4 个列表页空状态统一 |
| 组件一致性 | 全项目统一使用设计系统组件 | raw `<label>`/`<a>`/`<h1>` 替换为 Field/Link/PageTitle |

### 5.5 Markdown 富文本编辑

| 项 | 说明 | 验收标准 |
|----|------|---------|
| MarkdownViewer | react-markdown + remark-gfm + remark-math + rehype-katex | 标题/列表/表格/代码块/公式正确渲染 |
| 代码高亮 | Shiki 异步高亮，主题感知 | 12+ 语言高亮；dark/light 自动切换 |
| Mermaid 图表 | ```mermaid 代码块渲染为 SVG | flowchart/sequence/class 等正确渲染；主题感知 |
| MarkdownEditor | @uiw/react-md-editor 分屏编辑 | 工具栏操作 + 实时预览 |
| 图片粘贴上传 | 粘贴/拖拽图片自动上传 + 插入 Markdown | 调用存储上传 API |
| 模式切换确认 | 未保存内容弹确认框 | diff 检测 + ConfirmDialog |

---

## 6. V1.3 — Agent 基座

**目标**：铺设原生 Go Agent Loop 基础设施，实现 ReAct 循环 + Tool Calling，不改变用户可见行为。

**选型决策**：全部使用外部库，不自建。Eino（LLM Provider + Agent Loop + Stream Handling）+ modelcontextprotocol/go-sdk（MCP 工具）。详见 [`docs/design/agent-loop.md`](design/agent-loop.md)。

### 6.1 原生 Go Agent Loop

```mermaid
flowchart TB
    subgraph Go["Go Backend"]
        H["Handler"] --> S["Service"] --> R["Repository"]
        S --> RAG["RAG Engine (保留)"]
        S --> AG["agent/ 领域 (新增)"]
    end
    subgraph AgentLoop["Agent Loop (~40 行)"]
        AG -->|"同进程"| LLM["LLMClient<br/>(已有 adapter)"]
        AG -->|"ReAct 循环"| TOOLS["Tool Registry"]
        TOOLS -->|"同进程"| RAG
        TOOLS -->|"同进程"| TK["Ticket Service"]
        TOOLS -->|"同进程"| SQL["SQL Query"]
        AG -->|"事件流"| SSE["GenerationHub → SSE → 前端"]
    end
```

| 项 | 说明 | 验收标准 |
|----|------|---------|
| `server/internal/agent/` | Agent 领域包：provider 构造 + tool 注册 + handler | Eino ReactAgent `Stream()` 跑通 |
| Eino ChatModel | eino-ext/openai 接入 llama.cpp | OpenAI 兼容流式 + tool calling 可用 |
| Eino ReactAgent | ReAct 循环 + typed tools + parallel execution | 基本对话通过 Agent 走通；可中断 |
| Gin SSE bridge | `fmt.Fprintf + Flush` 输出 SSE 事件 | 前端 `ChatStreamProvider` 消费成功 |
| 前端 | 保留现有 `ChatStreamProvider` | 现有 SSE 事件类型无需修改 |
| Provider 热切换 | `LLMConfigManager.OnChange` → 替换 Eino ChatModel | 配置热替换生效，无需重启 |

### 6.2 MCP 工具集成

通过官方 Go MCP SDK（`modelcontextprotocol/go-sdk` v1.6.0）消费第三方工具，无需手写集成。V1.3 先集成知识库 / 申告 / SQL 等同进程工具；网络搜索工具在 V1.4 接入。

| 工具 | 来源 | 调用方式 | 版本 |
|------|------|----------|------|
| `search_knowledge_base` | Go RAG Engine | 同进程 `rag.Pipeline.Search()` | V1.3 |
| `ticket_lookup` | Go Ticket Service | 同进程 `ticket.Service.Query()` | V1.3 |
| `sql_query` | Go SQL 执行 | 同进程 `db.Raw()`（只读） | V1.3 |
| `web_search` | SearXNG MCP | MCP Client → HTTP → SearXNG | V1.4 |
| `web_fetch` | Firecrawl / Exa MCP | MCP Client → HTTP | V1.4 |

### 6.3 SSE 事件扩展

当前 SSE 事件：`step` / `chunks` / `token` / `done` / `error`。V1.3 扩展 Agent 事件类型。

| 新增事件类型 | 说明 |
|-------------|------|
| `thinking` | Agent 推理过程（V2.0 启用） |
| `tool_call` | Agent 工具调用详情（V2.0 启用） |
| `tool_result` | 工具返回结果（V2.0 启用） |
| `answer` | Agent 最终回答（V2.0 启用） |

V1.3 阶段 `GenerationHub` 的 `StreamEvent` 类型扩展，但前端不渲染（V2.0 启用）。

---

## 6.5. V1.4 — 深度搜索

**目标**：为 Agent 接入网络搜索与深度研究能力，实现自主决策何时搜网络、从哪个源搜、是否多轮搜索。

### 6.5.1 SearXNG 自托管搜索

| 项 | 说明 | 验收标准 |
|----|------|---------|
| Docker 部署 | `searxng` 服务加入 docker-compose | JSON 输出启用；`it`/`science` 类别 |
| MCP 集成 | 通过官方 Go MCP SDK 消费 SearXNG MCP 服务器 | `search` / `fetch_url` 工具注册到 Tool Registry |
| Ops 域配置 | 预配置技术搜索引擎优先 | StackOverflow/GitHub/厂商域名 |
| 私有搜索验证 | 查询不出域，无 API 密钥 | 日志确认无第三方数据发送 |

### 6.5.2 搜索 API 集成

| API | 自托管 | LLM 优化 | 用途 |
|-----|:---:|:---:|------|
| **SearXNG** | ✅ | ❌ | 主力：私有部署元搜索 |
| **Exa** | ❌ | ✅ | 高亮模式 10x token 效率 |
| **Firecrawl** | ❌ | ✅ | 深度研究 + 单页提取 |

### 6.5.3 深度研究架构

```mermaid
flowchart TD
    QUERY["用户复杂查询"] --> PL["规划器<br/>分解为 3-7 个子问题"]
    PL --> SA["搜索子代理<br/>并行运行, 独立上下文"]
    SA -->|"广度搜索"| SR["SearXNG / Exa"]
    SA -->|"深度阅读"| WF["WebFetch / Firecrawl"]
    SA --> RF["反思器<br/>对比发现与计划"]
    RF -->|"不足"| PL
    RF -->|"充足"| SY["合成器<br/>引用注册表 → 最终报告"]
```

| 项 | 说明 | 验收标准 |
|----|------|---------|
| 规划器 | 分解复杂查询为 3-7 个子问题 | 每个子问题含搜索策略 + 成功标准 |
| 搜索子代理 | 并行运行，每个独立上下文窗口 | 广度搜索 → 深度阅读有潜力 URL |
| 反思器 | 对抗性评论员识别弱点/矛盾 | 硬性上限防止无限循环 |
| 合成器 | 引用注册表防合成幻觉 | 压缩发现 + 引用 → 最终报告 |

### 6.5.4 Ops 域搜索场景

| 场景 | 查询示例 | 来源 |
|------|---------|------|
| 错误代码查找 | "ORA-00942 error" | Stack Overflow、厂商文档 |
| CVE 查询 | "CVE-2025-XXXX affected versions" | NVD、厂商安全公告 |
| 软件版本兼容 | "PostgreSQL 17 pgvector compatibility" | 厂商文档、GitHub releases |
| 内部 KB 未命中 | 内部无文档的问题 | 回退到网络搜索 |

---

## 7. V2.0 — Agentic RAG（终点）

**目标**：Agent ReAct 循环替代固定 7 步管道，实现自主检索决策、网络搜索、多步推理。

### 7.1 核心变化

固定 7 步线性管道 → Agent ReAct 循环自主决策。

```mermaid
flowchart TD
    subgraph V1["固定管道"]
        Q1["用户提问"] --> R1["改写"] --> R2["路由"] --> R3["混合检索"] --> R4["重排"] --> G1["生成"]
    end
    subgraph V2["Agentic RAG"]
        Q2["用户提问"] --> AG["Agent Loop (Go 原生)"]
        AG -->|"think"| T1{"需要什么信息?"}
        T1 -->|"内部知识"| KB["search_knowledge_base"]
        T1 -->|"实时信息"| WS["web_search (SearXNG)"]
        T1 -->|"结构化数据"| SQL["sql_query"]
        T1 -->|"申告历史"| TK["ticket_lookup"]
        KB & WS & SQL & TK --> AG
        AG -->|"observe"| T2{"够了吗?"}
        T2 -->|"不够"| T1
        T2 -->|"够了"| ANS["生成回答"]
    end
    V1 -->|演进| V2
```

### 7.2 Agent 基座：Eino 全栈

| 维度 | Eino 全栈 |
|------|---------------------|
| Agent Loop | Eino ReactAgent；ReAct + Graph 编排 + DeepAgent + HITL interrupt/resume |
| LLM Provider | eino-ext/openai；OpenAI 兼容（llama.cpp / DeepSeek / OpenAI） |
| Stream Handling | Eino 框架内自动流拼接/合并/复制 |
| SSE 输出 | Gin `fmt.Fprintf + Flush` ~20 行（标准库） |
| 前端消费 | 保留现有 `ChatStreamProvider`（rAF 批处理 + 纯函数 reducer + 单测） |
| 工具生态 | modelcontextprotocol/go-sdk v1.6.0；MCP client 消费第三方工具 |
| 部署 | 单二进制；Eino 为 Go 库，无额外运行时 |
| 成熟度 | 12k star，字节生产级（千级 QPS），Apache-2.0 |

### 7.3 网络搜索与深度搜索

V1.4 已部署 SearXNG + 深度研究架构。V2.0 启用 Agent 自主调用网络搜索工具。

```mermaid
flowchart LR
    AG["Agent Loop"] -->|"快速查询"| EXA["Exa MCP (高亮模式)"]
    AG -->|"深度研究"| FC["Firecrawl Agent MCP"]
    AG -->|"私有搜索"| SX["SearXNG (V1.4 部署)"]
    AG -->|"页面提取"| WF["WebFetch MCP / firecrawl_scrape"]
```

| 能力 | 工具 | 说明 |
|------|------|------|
| 快速网络搜索 | Exa MCP / Firecrawl MCP | 高亮模式 10x token 效率 |
| 深度研究 | `firecrawl_agent` MCP / GPT-Researcher MCP | 多轮迭代搜索→阅读→综合 |
| 自托管搜索 | SearXNG + MCP（V1.4 部署） | 聚合 130+ 引擎，无 API 密钥，私有部署 |
| Ops 域过滤 | SearXNG `it`/`science` 类别 | 优先 StackOverflow / GitHub / 厂商域名 |

### 7.4 Agent 场景

| 场景 | Agent 模式 | 工具 |
|------|-----------|------|
| 智能问答 | ReAct + Corrective RAG | RAG 检索、网络搜索、申告历史 |
| 根因分析 | Plan-then-Execute | 日志查询、拓扑探索、指标查询、知识检索 |
| 自助修复 | ReAct + Tool Use | API 调用、脚本执行（需人工审批门） |

### 7.5 目标架构

```mermaid
flowchart TB
    FE["Frontend (Next.js)<br/>ChatStreamProvider (保留)"]
    FE -->|"POST /api/chat"| SSE["Gin SSE bridge<br/>fmt.Fprintf + Flush"]
    SSE --> AGENT["agent/ 领域<br/>Eino ReactAgent"]
    AGENT -->|"eino-ext/openai"| LLM["Eino ChatModel<br/>llama.cpp / DeepSeek / OpenAI"]
    AGENT -->|"同进程"| RAG["RAG Engine (保留)<br/>BM25 + pgvector + RRF + rerank"]
    AGENT -->|"MCP Client"| MCP["modelcontextprotocol/go-sdk<br/>web_search / web_fetch"]
    MCP -->|"HTTP"| SX["SearXNG (V1.4 部署)"]
    SSE -->|"SSE stream"| FE
    RAG --> PG[("PostgreSQL + pgvector")]
```

### 7.6 废弃与新增

| 废弃（固定管道） | 替代（Agentic） |
|-----------|------------|
| `ai.rag_query_rewrite` 开关 | Agent 自主决策（工具参数） |
| `ai.rag_multi_route` 开关 | Agent 循环多次调用 |
| `ai.rag_hybrid` 开关 | 工具参数 `strategy=hybrid` |
| `ai.rag_rerank` 开关 | 工具参数 `rerank=true` |
| 固定 `Pipeline.Execute()` | `agent.Agent.Run()` ReAct 循环 |
| `LLMConfigManager` 单默认 | Agent 重建 + 多 Provider |

| 保留 | 原因 |
|------|------|
| RAG 引擎（BM25 + vector + RRF + rerank） | Go 引擎作为工具后端 |
| pgvector + PostgreSQL | 向量存储 + 业务数据不变 |
| Document Processor | 文档处理管道不变 |
| SSE 流式 + GenerationHub | 扩展事件类型（V1.3 预留） |
| Auth/RBAC/Ticket/Knowledge | 领域逻辑不变 |

| 新增 | 说明 |
|------|------|
| `agent/` 领域 | Agent struct + Tool 接口 + ToolRegistry + Agent Loop |
| MCP Client | 官方 Go MCP SDK，消费第三方工具 |
| 前端 Agent 事件 UI | thinking / tool_call / tool_result / answer 渲染 |
| 人工审批门（HITL） | 敏感操作（自助修复）必须人工确认 |
| 事件知识自进化 | 已解决申告生成知识条目 → 嵌入 pgvector → RAG 检索历史经验 |

### 7.7 验收标准

| 验收项 | 标准 |
|--------|------|
| Agent 对话端到端 | 用户提问 → Agent 自主检索（≥1 轮）→ 带引用回答 |
| 网络搜索 | 内部 KB 无结果时 Agent 自主触发 SearXNG 搜索 |
| 深度搜索 | 复杂问题 Agent 多轮搜索（≥2 轮）并综合 |
| Agent 事件 UI | thinking / tool_call / tool_result 实时渲染；用户可见推理过程 |
| 降级 | Agent Loop 异常时回退固定管道；SearXNG 不可用时跳过网络搜索 |
| 审计 | Agent 轨迹（工具调用、检索查询、推理步骤）写入审计日志 |

---

## 8. 里程碑

```mermaid
gantt
    title OpsMind V1.0 → V2.0 里程碑
    dateFormat YYYY-MM-DD
    axisFormat %Y-%m

    section V1.0 已交付
    固定管道 RAG         :done, v1, 2026-06-01, 2026-09-01

    section V1.1 存储简化
    MinIO→本地 FS        :v11a, 2026-09-15, 14d
    配置体系统一          :v11b, 2026-09-15, 7d

    section V1.2 业务完善
    知识库上传增强        :v12a, 2026-10-01, 14d
    申告管理增强          :v12b, 2026-10-01, 14d
    看板增强+导出         :v12c, 2026-10-15, 14d
    Markdown 富文本       :v12d, 2026-10-15, 21d
    前端体验优化          :v12e, 2026-11-01, 14d

    section V1.3 Agent 基座
    agent/ 领域 + Tool 接口 :v13a, 2026-12-01, 14d
    LLMClient tool calling :v13b, 2026-12-01, 10d
    Agent Loop + 并行工具  :v13c, 2026-12-15, 14d
    MCP SDK 集成          :v13d, 2026-12-15, 10d
    SSE 事件扩展          :v13e, 2026-12-25, 7d

    section V1.4 深度搜索
    SearXNG 部署+MCP     :v14a, 2027-01-15, 14d
    网络搜索工具注册      :v14b, 2027-01-15, 7d
    深度研究架构          :v14c, 2027-02-01, 21d
    Ops 域搜索场景验证    :v14d, 2027-02-01, 14d

    section V2.0 Agentic RAG
    Agent 固定管道替代    :v20a, 2027-03-01, 14d
    前端 Agent 事件 UI    :v20b, 2027-03-01, 21d
    端到端集成+降级       :v20c, 2027-03-15, 14d
```

---

## 9. 技术决策记录

| 决策 | 选择 | 依据 |
|------|------|------|
| 文档存储 | 本地 FS（移除 MinIO） | 单实例下本地 FS 足够；Dify/Open WebUI/AnythingLLM 均默认本地 |
| 向量数据库 | 保留 pgvector | halfvec+HNSW 不可替代；增长最快 PG 扩展；sqlite-vec 无 ANN |
| 业务数据库 | 保留 PostgreSQL | JSONB GIN 索引；pgvector 依赖；跨表事务；GORM 迁移安全 |
| Agent 数据 | 未来 SQLite | ReAct 高频读写时拆分；当前非瓶颈 |
| Agent 基座 | Eino (ByteDance) | 12k star 字节生产级；唯一同时覆盖 LLM Provider + Agent Loop + Stream Handling 的 Go 框架；sashabaranov/go-openai 不含 agent loop |
| LLM Provider | eino-ext/openai | Eino 配套；OpenAI 兼容 → llama.cpp；含 tool calling + streaming |
| SSE 输出 | Gin 标准库 `fmt.Fprintf + Flush` | 标准 SSE 模式（headers + http.Flusher + data: %s\n\n） |
| 工具生态 | 官方 Go MCP SDK | `modelcontextprotocol/go-sdk` v1.6.0，与 Google 共维护 |
| 网络搜索 | SearXNG（自托管）+ Exa/Firecrawl | 私有部署无 API 密钥；深度研究用 Firecrawl Agent |
| Agent 模式 | ReAct + Corrective RAG | 运维问答需要多步推理 + 检索质量保证 |
| LLM Provider 热切换 | `LLMConfigManager.OnChange` → Eino ChatModel 替换 | `atomic.Value` 存储 ChatModel 实例 |
| 前端 SSE | 保留现有 `ChatStreamProvider` | rAF 批处理 + 纯函数 reducer + 单测，已足够好 |
| 版本终点 | V2.0 | 不规划 V2.x，业务提升全部归入 V1.1-V1.4 |

---

## 10. 关联文档

| 文档 | 用途 |
|------|------|
| [`PRD.md`](PRD.md) | 产品需求 — 功能定义、业务规则 |
| [`TECH.md`](TECH.md) | 技术架构 — 分层设计、DDL、ADR |
| [`API/README.md`](API/README.md) | API 文档索引 |
| [`FLOW/README.md`](FLOW/README.md) | 业务流程图 |
| [`TODO.md`](TODO.md) | 代码级改进清单与优先级 |

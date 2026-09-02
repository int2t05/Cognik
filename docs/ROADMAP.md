# OpsMind 产品技术路线图

> 从 V1.0 到 V2.0 的版本规划。代码级改进清单见 [`TODO.md`](TODO.md)，技术架构详见 [`TECH.md`](TECH.md)。

## 1. 项目愿景

OpsMind 是面向企业 IT 运维的**私有部署 AI 数字员工系统**。核心目标：让运维团队从重复性咨询中解放，让知识沉淀为可复用资产，让 AI 成为运维流程的第一响应者。

**设计原则**：私有部署优先（数据不出域）、自建 RAG 引擎（全链路可控可审计）、单体分层架构（简洁可维护）。

---

## 2. 版本总览

```mermaid
flowchart LR
    V1["V1.0<br/>固定管道 RAG<br/>(已交付)"] --> V11["V1.1<br/>存储简化与文档处理"]
    V11 --> V12["V1.2<br/>知识库与申告业务完善"]
    V12 --> V13["V1.3<br/>RAG 引擎增强"]
    V13 --> V14["V1.4<br/>Agent 基础设施"]
    V14 --> V2["V2.0<br/>Agentic RAG<br/>(终点)"]
```

| 版本 | 主题 | 核心交付 | 状态 |
|------|------|---------|------|
| V1.0 | 固定管道 RAG | 7 步 RAG 管道 + 申告状态机 + 知识库 CRUD + RBAC + SSE 流式 | ✅ 已交付 |
| V1.1 | 存储简化与文档处理 | MinIO→本地 FS；DOCX 分割文档；流式解析防 OOM；上传上限配置化 | 📋 规划中 |
| V1.2 | 知识库与申告业务完善 | 批量上传+进度；申告满意度+标签+时限+批量；看板增强；前端体验；Markdown 富文本编辑 | 📋 规划中 |
| V1.3 | RAG 引擎增强 | BM25 增量更新；文档处理重试+死信；历史截断 token 化 | 📋 规划中 |
| V1.4 | Agent 基础设施 | Pi Agent 桥接；SearXNG 部署；Go Internal API；SSE 事件扩展 | 📋 规划中 |
| V2.0 | Agentic RAG | Agent ReAct 循环替代固定管道；网络搜索+深度搜索；Agent 事件 UI | 📋 规划中 |

**版本策略**：V1.1–V1.2 聚焦基础业务完善（存储、文档处理、知识库、申告、看板、前端），V1.3–V1.4 为 RAG 与 Agent 增强铺垫，V2.0 为 Agentic RAG 终态。

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

## 4. V1.1 — 存储简化与文档处理

**目标**：移除 MinIO 依赖降低部署复杂度，修复文档处理的稳定性缺陷（OOM、格式覆盖不全、硬编码限制）。

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

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| `LocalFSClient` | 新增本地 FS 适配器，原子写（temp+rename），UUID 文件名 | `StorageClient` 接口不变；MinIO 数据迁移脚本可用 | — |
| 后端代理下载 | `GET /api/storage/:bucket/:key` 替代 Presigned URL | 认证后 `http.ServeFile`；权限校验通过 | — |
| Docker 简化 | 移除 minio 服务，加 `storage_data` volume | `docker compose up` 仅 3 必须服务 | — |
| 配置统一 | `storage.type` (local/minio) + `storage.root_path` | 切换配置即可换后端，无需改代码 | — |
| MinIO 惰性下载修复 | 本地 FS 直接 `os.Open`，无惰性问题；`defer Close` 检查错误 | `GetObject` 等价路径无数据可读时返回错误 | TODO §后端2 |
| 上传上限配置化 | 50MB 硬编码改为按 KB 粒度配置 | `kb.max_upload_size` 可配置 | TODO §后端2 |
| DOCX 分割文档 | 支持 `word/document2.xml` 分割文档解析 | 多部分 DOCX 正确解析 | TODO §后端2 |
| PDF/DOCX 流式解析 | 逐页/流式读取，不全量 `io.ReadAll` | 50MB PDF/DOCX 内存峰值 < 20MB | TODO §后端2 |

### 4.3 保留决策

| 组件 | 决策 | 依据 |
|------|------|------|
| pgvector | 保留 | halfvec(FP16) + HNSW 不可替代；增长最快 PG 扩展 |
| PostgreSQL | 保留 | 12+ JSONB 列依赖 GIN 索引；跨表事务一致性 |
| `MinIOClient` | 保留代码 | 作为 escape hatch，多实例时恢复使用 |

---

## 5. V1.2 — 知识库与申告业务完善

**目标**：完善知识库上传、申告管理、看板数据、前端体验和 Markdown 富文本编辑，形成完整可用的业务闭环。

### 5.1 知识库上传增强

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| 批量上传 | 支持多文件批量上传 | 前端批量选择；后端并发处理 | — |
| 上传进度 | 前端显示上传进度条 | 实时进度反馈 | — |
| 文件类型校验 | 前端+后端双重校验文件类型 | 非 PDF/DOCX/MD/TXT 拒绝 | — |

### 5.2 申告管理增强

| 项 | 说明 | 验收标准 |
|----|------|---------|
| 满意度评价 | 申告解决后用户评价（满意/不满意+反馈） | 状态机增加评价环节；看板统计满意度 |
| 申告标签 | 申告支持标签分类（复用 Tags 字段） | 前端标签选择；按标签筛选 |
| 处理时限 | 申告可设处理时限，超时预警 | 配置时限；超时站内消息通知 |
| 批量操作 | 申告批量关闭/转派 | 多选+批量操作确认 |

### 5.3 看板增强

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| 自定义日期范围 | 看板支持自定义起止日期 | 日期选择器；趋势图按范围刷新 | — |
| 数据导出 | 看板数据 CSV 导出 | 导出当前筛选条件下的数据 | — |
| 满意度统计 | 看板新增满意度统计卡片 | 满意率趋势；按运维人员分组 | — |
| `granularity` 生效 | 趋势查询 `granularity` 参数实际生效 | day/week 切换正常 | — |
| 趋势窗口配置化 | 趋势查询窗口从硬编码改为可配置 | 管理员可配置默认时间范围 | TODO §后端3 |

### 5.4 前端体验优化

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| 代码分割 | `next/dynamic` 按路由懒加载 | 首屏加载体积减少 50%+ | TODO §前端5 |
| 虚拟列表高度 | 变长消息 `estimateSize` 动态测量 | 滚动位置准确 | TODO §前端1 |
| 表单 required 标记 | 前端表单必填字段标记 | 视觉标识 + 校验提示 | TODO §前端2 |
| 搜索无结果提示 | 用户/申告搜索无结果时提示 | 空状态 UI | TODO §前端3 |

### 5.5 知识库文章 Markdown 富文本编辑

**目标**：知识库文章支持 Markdown 在线查看、阅读、编辑，包含 LaTeX 公式渲染与图片上传。

```mermaid
flowchart LR
    subgraph 阅读模式["阅读模式"]
        RM["react-markdown<br/>渲染 Markdown"] --> KX["rehype-katex<br/>LaTeX 公式"]
        RM --> SH["shiki<br/>代码高亮"]
        RM --> IMG["图片懒加载"]
    end
    subgraph 编辑模式["编辑模式"]
        ED["react-md-editor<br/>分屏编辑"] --> PREV["实时预览"]
        ED --> UPS["图片上传<br/>粘贴/拖拽"]
    end
    阅读模式 <-->|"切换"| 编辑模式
```

| 项 | 说明 | 验收标准 |
|----|------|---------|
| Markdown 阅读模式 | `react-markdown` + `remark-gfm` + `remark-math` + `rehype-katex` 渲染 | GFM 表格/任务列表/删除线正常；`$...$` 行内公式 + `$$...$$` 块级公式正常渲染 |
| 代码高亮 | `shiki` 服务端渲染或 `react-syntax-highlighter` 客户端渲染 | 200+ 语法支持；主题与设计系统亮暗一致 |
| 图片展示 | 文章内图片懒加载 + 点击放大 | `loading="lazy"`；点击触发 lightbox |
| Markdown 编辑模式 | `react-md-editor` 分屏编辑（编辑 + 预览） | 工具栏（加粗/斜体/标题/链接/图片/代码/表格）；`Ctrl+S` 保存 |
| LaTeX 公式编辑 | 编辑模式预览实时渲染 KaTeX | 输入 `$E=mc^2$` 即时渲染；语法错误显示红色提示 |
| 图片上传 | 粘贴剪贴板图片 + 拖拽上传 + 工具栏按钮 | 粘贴自动上传并插入 `![](url)`；上传进度条；上传后光标定位到图片标记 |
| 模式切换 | 阅读模式 ↔ 编辑模式一键切换 | 阅读模式无工具栏；切换不丢失未保存内容（确认弹窗） |
| 图片存储 | 图片通过 V1.1 `LocalFSClient` 存储，`/api/storage` 代理下载 | 与文档附件复用存储路径；UUID 文件名 |

---

## 6. V1.3 — RAG 引擎增强

**目标**：提升 RAG 管道的可靠性、性能和检索质量。

### 6.1 BM25 索引增量更新

当前每次刷新全量重建 BM25 索引，大知识库耗时数秒。

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| 增量索引 | 文章发布/更新时增量更新 BM25 倒排索引，非全量重建 | 10 万 chunk KB 增量更新 < 500ms | TODO §后端1 |
| 并发安全 | 索引读写不阻塞检索请求 | RWMutex；检索不等待索引写 | — |

### 6.2 文档处理重试与死信

当前 embedding API 瞬时失败直接中止整个文档处理。

```mermaid
flowchart TD
    P["parse"] --> C["chunk"]
    C --> E["embed (batch)"]
    E -->|"瞬时失败"| R{"重试 ≤3 次?"}
    R -->|"是"| E
    R -->|"否"| DL["死信队列<br/>process_status=failed"]
    DL -->|"手动重试"| P
    E -->|"成功"| I["index → pgvector"]
```

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| 阶段内重试 | embed/chunk 阶段瞬时失败自动重试（指数退避，max 3） | 429/503 自动重试；非瞬时错误跳过 | TODO §后端1 |
| 死信队列 | 超过重试次数标记 `failed`，记录 `process_error` | 前端显示失败原因；支持手动重试 | TODO §后端1 |
| 重试不重复 | 重试基于 chunk hash 跳过已成功的 chunk | 增量重试，非从头开始 | — |

### 6.3 RAG 历史截断优化

当前按消息条数截断，改为按 token 数。

| 项 | 说明 | 验收标准 | TODO 来源 |
|----|------|---------|-----------|
| Token 计数 | 使用 tokenizer 精确计数 | 超出 `max_history_tokens` 时从最早开始截断 | TODO §后端1 |
| 配置化 | `ai.max_history_tokens` 替代 `ai.max_history_messages` | 兼容旧配置 | — |

---

## 7. V1.4 — Agent 基础设施

**目标**：为 V2.0 Agentic RAG 铺设基础设施，不改变用户可见行为。

### 7.1 Pi Agent 桥接层

```mermaid
flowchart TB
    subgraph Go["Go Backend (不变)"]
        H["Handler"] --> S["Service"] --> R["Repository"]
        S --> RAG["RAG Engine"]
    end
    subgraph Bridge["Agent Bridge (新增)"]
        AB["AgentBridge<br/>管理 Pi 子进程"]
        AB -->|"stdin: JSON 命令"| PI["Pi RPC<br/>--mode rpc"]
        PI -->|"stdout: JSON 事件"| AB
    end
    subgraph Internal["Internal API (新增)"]
        IA["/api/v1/internal/rag/search"]
        IB["/api/v1/internal/tickets"]
        IC["/api/v1/internal/sql"]
    end
    S --> AB
    AB --> IA
```

| 项 | 说明 | 验收标准 |
|----|------|---------|
| `server/internal/agent/` | Agent Bridge 包：管理 Pi RPC 子进程生命周期 | spawn/monitor/restart；超时取消 |
| Pi RPC 集成 | `pi --mode rpc` JSONL over stdin/stdout | 基本对话通过 Pi 走通 |
| Pi Provider 配置 | Pi `--provider` / `--model` 参数从 DB 读取 | 配置热替换生效 |
| 兜底方案验证 | 验证自建 Go Agent Loop（~500 行）可行性 | RPC 不可用时可回退 |

### 7.2 SearXNG 自托管搜索

| 项 | 说明 | 验收标准 |
|----|------|---------|
| Docker 部署 | `searxng` 服务加入 docker-compose | JSON 输出启用；`it`/`science` 类别 |
| SearXNG MCP | MCP 服务器封装，供 Pi Agent 调用 | `search`/`fetch_url` 工具可用 |
| Ops 域配置 | 预配置技术搜索引擎优先 | StackOverflow/GitHub/厂商域名 |
| 私有搜索验证 | 查询不出域，无 API 密钥 | 日志确认无第三方数据发送 |

### 7.3 Go Internal API

供 Pi Agent 工具回调的内部端点，RBAC 内部令牌认证。

| 端点 | 方法 | 说明 | 验收标准 |
|------|------|------|---------|
| `/api/v1/internal/rag/search` | POST | 调用 RAG 引擎检索 | 返回 chunks + scores |
| `/api/v1/internal/tickets` | GET | 查询申告列表 | 支持 status/keyword 筛选 |
| `/api/v1/internal/sql` | POST | 受限 SQL 查询 | 白名单表；只读；行数限制 |
| `/api/v1/internal/escalate` | POST | 触发申告升级 | 创建申告 + 通知 |

### 7.4 SSE 事件扩展

当前 SSE 事件：`step` / `chunks` / `token` / `done` / `error`。V1.4 预留 Agent 事件类型。

| 新增事件类型 | 说明 |
|-------------|------|
| `thinking` | Agent 推理过程（V2.0 启用） |
| `action` | Agent 工具调用（V2.0 启用） |
| `observation` | 工具返回结果（V2.0 启用） |
| `tool_call` | 工具调用详情（V2.0 启用） |
| `tool_result` | 工具返回摘要（V2.0 启用） |

V1.4 阶段 `GenerationHub` 的 `StreamEvent` 类型扩展，但前端不渲染（V2.0 启用）。

---

## 8. V2.0 — Agentic RAG（终点）

**目标**：Agent ReAct 循环替代固定 7 步管道，实现自主检索决策、网络搜索、多步推理。

### 8.1 核心变化

V1 的 RAG 是**固定 7 步线性管道**，V2.0 引入 **Agent 基座**让 AI 自主决策。

```mermaid
flowchart TD
    subgraph V1["V1 固定管道"]
        Q1["用户提问"] --> R1["改写"] --> R2["路由"] --> R3["混合检索"] --> R4["重排"] --> G1["生成"]
    end
    subgraph V2["V2.0 Agentic RAG"]
        Q2["用户提问"] --> AG["Agent 循环"]
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

### 8.2 Agent 基座：Pi Agent

| 维度 | Pi (earendil-works/pi) |
|------|----------------------|
| 成熟度 | ~10 万 star，265 万周下载，257 releases，MIT |
| Agent 循环 | ReAct 循环 + tool calling + steering + compaction |
| 多 Provider | 30+（llama.cpp / DeepSeek / OpenAI / Anthropic） |
| RAG 能力 | 无内置（OpsMind Go 引擎互补作为工具后端） |

**桥接**：V1.4 已铺设 RPC 子进程桥接层，V2.0 切换为 HTTP Sidecar（`createAgentSession` SDK）用于生产。

### 8.3 网络搜索与深度搜索

```mermaid
flowchart LR
    AG["Pi Agent"] -->|"快速查询"| EXA["Exa (高亮模式)"]
    AG -->|"深度研究"| FC["Firecrawl Agent"]
    AG -->|"私有搜索"| SX["SearXNG (自托管, V1.4 部署)"]
    AG -->|"页面提取"| WF["WebFetch / firecrawl_scrape"]
```

| 能力 | 工具 | 说明 |
|------|------|------|
| 快速网络搜索 | Exa MCP / Firecrawl | 高亮模式 10x token 效率 |
| 深度研究 | `firecrawl_agent` / GPT-Researcher MCP | 多轮迭代搜索→阅读→综合 |
| 自托管搜索 | SearXNG + MCP（V1.4 部署） | 聚合 130+ 引擎，无 API 密钥，私有部署 |
| Ops 域过滤 | SearXNG `it`/`science` 类别 | 优先 StackOverflow / GitHub / 厂商域名 |

### 8.4 Agent 场景

| 场景 | Agent 模式 | 工具 |
|------|-----------|------|
| 智能问答 | ReAct + Corrective RAG | RAG 检索、网络搜索、申告历史 |
| 根因分析 | Plan-then-Execute | 日志查询、拓扑探索、指标查询、知识检索 |
| 自助修复 | ReAct + Tool Use | API 调用、脚本执行（需人工审批门） |

### 8.5 目标架构

```mermaid
flowchart TB
    FE["Frontend (Next.js)<br/>Agent 事件 UI"]
    FE -->|"SSE"| GO["Go Backend (Gin)<br/>Auth/RBAC + Ticket + Knowledge"]
    GO --> RAG["RAG Engine (保留)<br/>BM25 + pgvector + RRF + rerank"]
    GO --> IA["Internal API (V1.4 铺设)<br/>/api/v1/internal/rag/search<br/>/api/v1/internal/tickets<br/>/api/v1/internal/sql"]
    IA -->|"HTTP 回调"| PI["Pi Agent (Node.js)<br/>ReAct 循环 + Tool Registry"]
    PI -->|"web_search"| SX["SearXNG (V1.4 部署)"]
    PI -->|"LLM"| LLM["llama.cpp / DeepSeek / OpenAI"]
    PI -->|"SSE 事件"| GO
    RAG --> PG[("PostgreSQL + pgvector")]
```

### 8.6 废弃与新增

| 废弃（V1） | 替代（V2.0） |
|-----------|------------|
| `ai.rag_query_rewrite` 开关 | Agent 自主决策（工具参数） |
| `ai.rag_multi_route` 开关 | Agent 循环多次调用 |
| `ai.rag_hybrid` 开关 | 工具参数 `strategy=hybrid` |
| `ai.rag_rerank` 开关 | 工具参数 `rerank=true` |
| 固定 `Pipeline.Execute()` | Pi `createAgentSession()` |
| `LLMConfigManager` 单默认 | Pi 多 Provider + session 级配置 |

| 保留 | 原因 |
|------|------|
| RAG 引擎（BM25 + vector + RRF + rerank） | Pi 无 RAG，Go 引擎作为工具后端 |
| pgvector + PostgreSQL | 向量存储 + 业务数据不变 |
| Document Processor | 文档处理管道不变 |
| SSE 流式 + GenerationHub | 扩展事件类型（V1.4 预留） |
| Auth/RBAC/Ticket/Knowledge | 领域逻辑不变 |

| 新增 | 说明 |
|------|------|
| Pi HTTP Sidecar | 生产级 Agent 运行时（V1.4 RPC 升级） |
| Pi Extension（TS） | 自定义 tools：search_kb / ticket_lookup / sql_query / web_search |
| 前端 Agent 事件 UI | thinking/action/observation/tool_call/tool_result 渲染 |
| 人工审批门（HITL） | 敏感操作（自助修复）必须人工确认 |
| 事件知识自进化 | 已解决申告生成知识条目 → 嵌入 pgvector → RAG 检索历史经验 |

### 8.7 验收标准

| 验收项 | 标准 |
|--------|------|
| Agent 对话端到端 | 用户提问 → Agent 自主检索（≥1 轮）→ 带引用回答 |
| 网络搜索 | 内部 KB 无结果时 Agent 自主触发 SearXNG 搜索 |
| 深度搜索 | 复杂问题 Agent 多轮搜索（≥2 轮）并综合 |
| Agent 事件 UI | thinking/action/observation 实时渲染；用户可见推理过程 |
| 降级 | Pi Agent 不可用时回退 V1 固定管道；SearXNG 不可用时跳过网络搜索 |
| 审计 | Agent 轨迹（工具调用、检索查询、推理步骤）写入审计日志 |

---

## 9. 里程碑

```mermaid
gantt
    title OpsMind V1.0 → V2.0 里程碑
    dateFormat YYYY-MM-DD
    axisFormat %Y-%m

    section V1.0 已交付
    固定管道 RAG              :done, v1, 2026-06-01, 2026-09-01

    section V1.1 存储与文档处理
    MinIO→本地 FS             :v11a, 2026-09-15, 14d
    配置体系统一               :v11b, 2026-09-15, 7d
    DOCX 分割+流式解析         :v11c, 2026-09-22, 14d
    上传配置化+惰性修复        :v11d, 2026-09-22, 7d

    section V1.2 业务完善
    知识库批量上传+进度        :v12a, 2026-10-13, 14d
    申告满意度+标签+时限       :v12b, 2026-10-13, 21d
    看板增强+导出              :v12c, 2026-10-27, 14d
    前端体验优化               :v12d, 2026-10-27, 14d
    Markdown 富文本编辑        :v12e, 2026-10-27, 21d

    section V1.3 RAG 引擎增强
    BM25 增量更新             :v13a, 2026-11-24, 21d
    文档处理重试+死信          :v13b, 2026-11-24, 14d
    RAG 历史截断优化           :v13c, 2026-12-08, 7d

    section V1.4 Agent 基础设施
    Pi Agent 桥接层           :v14a, 2027-01-05, 21d
    SearXNG 部署+MCP          :v14b, 2027-01-05, 14d
    Go Internal API           :v14c, 2027-01-19, 14d
    SSE 事件扩展              :v14d, 2027-01-19, 7d

    section V2.0 Agentic RAG
    Pi HTTP Sidecar           :v20a, 2027-02-16, 14d
    Agent 工具注册             :v20b, 2027-02-16, 14d
    前端 Agent 事件 UI        :v20c, 2027-03-02, 21d
    网络搜索+深度搜索          :v20d, 2027-03-02, 14d
    端到端集成+降级            :v20e, 2027-03-16, 14d
```

---

## 10. 技术决策记录

| 决策 | 选择 | 依据 |
|------|------|------|
| 版本优先级 | 基础业务优先于 RAG/Agent 增强 | 业务闭环可用是前提；RAG 增强和 Agent 基座在业务完善后推进 |
| 文档存储 | 本地 FS（移除 MinIO） | 单实例下本地 FS 足够；Dify/Open WebUI/AnythingLLM 均默认本地 |
| 向量数据库 | 保留 pgvector | halfvec+HNSW 不可替代；增长最快 PG 扩展；sqlite-vec 无 ANN |
| 业务数据库 | 保留 PostgreSQL | JSONB GIN 索引；pgvector 依赖；跨表事务；GORM 迁移安全 |
| Agent 数据 | 未来 SQLite | ReAct 高频读写时拆分；当前非瓶颈 |
| Agent 基座 | Pi Agent（TS） | 10 万星验证、30+ Provider、成熟 agent loop；Go 桥接 |
| 网络搜索 | SearXNG（自托管）+ Exa/Firecrawl | 私有部署无 API 密钥；深度研究用 Firecrawl Agent |
| Agent 模式 | ReAct + Corrective RAG | 运维问答需要多步推理 + 检索质量保证 |
| LLM Provider | 多 Provider（llama.cpp/DeepSeek/OpenAI） | Pi 管理 Provider 路由，保留本地部署能力 |
| 桥接策略 | V1.4 RPC 子进程 → V2.0 HTTP Sidecar | MVP 用 RPC 快速验证，生产用 Sidecar 稳定 |
| 版本终点 | V2.0 | 不规划 V2.x，业务提升全部归入 V1.1-V1.4 |

---

## 11. TODO 覆盖矩阵

TODO.md 中的每一项在版本中的落点：

| TODO 项 | 优先级 | 落入版本 | 交付项 |
|---------|--------|---------|--------|
| BM25 索引无增量更新 | 🟡 P2 | V1.3 §6.1 | 增量索引 |
| 文档处理器无阶段内重试 | 🟡 P2 | V1.3 §6.2 | 阶段内重试 + 死信队列 |
| 文档处理缺自动重试队列 | 🟢 P3 | V1.3 §6.2 | 死信队列 + 手动重试 |
| RAG 历史截断按消息条数 | 🟢 P3 | V1.3 §6.3 | Token 计数截断 |
| DOCX 仅读 document.xml | 🟡 P2 | V1.1 §4.2 | DOCX 分割文档 |
| PDF/DOCX 全量读入内存 | 🟡 P2 | V1.1 §4.2 | 流式解析 |
| 50MB 上传上限硬编码 | 🟡 P2 | V1.1 §4.2 | 上传上限配置化 |
| MinIO 惰性下载 | 🟡 P2 | V1.1 §4.2 | 本地 FS 直接 `os.Open` |
| 趋势查询窗口硬编码 | 🟢 P3 | V1.2 §5.3 | 趋势窗口配置化 |
| 虚拟列表 estimateSize 常量 | 🟢 P3 | V1.2 §5.4 | 动态高度测量 |
| 表单缺 required 标记 | 🟢 P3 | V1.2 §5.4 | 必填字段标记 |
| 用户搜索无结果提示 | 🟢 P3 | V1.2 §5.4 | 空状态 UI |
| 零代码分割 | 🟡 P2 | V1.2 §5.4 | `next/dynamic` 按路由懒加载 |
| StatusBadge 状态映射硬编码 | 🟢 P3 | 不纳入 | `statusText` prop 已提供逃生舱 |
| 审计页输入框样式重复 | 🟢 P3 | 不纳入 | 审计页布局特殊，不适合统一组件 |
| 全局 ErrorBoundary 仅顶层 | 🟢 P3 | 不纳入 | `SectionErrorBoundary` 已覆盖内容区 |

---

## 12. 关联文档

| 文档 | 用途 |
|------|------|
| [`PRD.md`](PRD.md) | 产品需求 — 功能定义、业务规则 |
| [`TECH.md`](TECH.md) | 技术架构 — 分层设计、DDL、ADR |
| [`API/README.md`](API/README.md) | API 文档索引 |
| [`FLOW/README.md`](FLOW/README.md) | 业务流程图 |
| [`TODO.md`](TODO.md) | 代码级改进清单与优先级 |

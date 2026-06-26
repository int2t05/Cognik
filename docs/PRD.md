# OpsMind — 产品需求文档

> 私有部署的 AI 运维数字员工系统。关联文档：[TECH](TECH.md) · [API](API/README.md) · [FLOW](FLOW/README.md)

## 1. 产品定位

面向企业运维场景的 AI 数字员工，通过私有知识库 + 自建 RAG 引擎 + 申告全流程管理，替代或辅助人工完成咨询、自助处理、工单流转和知识沉淀。数据不出域，全部存储在自有 PostgreSQL + pgvector。

## 2. 系统上下文

```mermaid
flowchart LR
    subgraph Actors["用户角色"]
        Reporter["报障人<br/>智能问答 / 提交申告"]
        Operator["运维人员<br/>处理申告 / 维护知识"]
        Admin["系统管理员<br/>用户权限 / 系统配置"]
    end

    subgraph System["OpsMind"]
        Web["Next.js 前端<br/>门户端 + 管理后台"]
        Server["Go 后端 :8080<br/>Handler → Service → Repository"]
        RAG["RAG 引擎<br/>BM25 + 向量 + RRF + 重排序"]
    end

    subgraph Infra["基础设施"]
        PG[("PostgreSQL + pgvector<br/>业务数据 + 向量存储")]
        MinIO[("MinIO<br/>文档 / 附件")]
        LLM["LLM 服务<br/>llama.cpp 或 OpenAI-compatible"]
    end

    Reporter --> Web
    Operator --> Web
    Admin --> Web
    Web --> Server
    Server --> RAG
    Server --> PG
    Server --> MinIO
    RAG --> LLM
    RAG --> PG

    style RAG fill:#5e6ad220,stroke:#5e6ad2
```

## 3. 部署拓扑

```mermaid
flowchart TD
    Client["浏览器"] --> Web["opsmind-web :3000<br/>Next.js standalone"]
    Web --> Server["opsmind-server :8080<br/>Go Gin"]
    Server --> Postgres[("postgres :5432<br/>pgvector + HNSW")]
    Server --> Minio[("minio :9000/:9001<br/>S3-compatible")]
    Server -.->|ai-local profile| LlamaCpp["llama-cpp :8080/v1<br/>可选"]

    style LlamaCpp stroke-dasharray: 5 5
```

## 4. 核心功能

### 4.1 智能问答

```mermaid
flowchart LR
    Q["用户提问"] --> QR["查询改写<br/>LLM 规范化口语表达"]
    QR --> MR["多路检索<br/>LLM 生成 2-4 路子查询"]
    MR --> VR["向量检索<br/>pgvector cosine <=>"]
    MR --> BM["BM25 检索<br/>gse 分词 + Okapi"]
    VR --> FUSE["RRF 融合<br/>score = Σ 1/(60+rank)"]
    BM --> FUSE
    FUSE --> RR["重排序<br/>Cross-Encoder"]
    RR --> GEN["LLM 生成<br/>SSE 逐 token 流式"]
    GEN --> OUT["答案 + 来源 + 置信度"]

    style RR fill:#f59e0b20,stroke:#f59e0b
    style GEN fill:#22c55e20,stroke:#22c55e
```

- 全管道 7 步骤可独立开关，非核心步骤失败降级不阻塞
- 向量检索与 LLM 生成失败返回明确错误码（20002 / 20001）
- 置信度三级：高（≥0.8）/ 中（≥0.6）/ 低（<0.6），低置信度引导提交申告
- SSE 事件类型：step / token / error / done

### 4.2 知识库管理

```mermaid
stateDiagram-v2
    [*] --> Draft: 创建文章
    Draft --> Reviewing: 提交审核
    Reviewing --> Approved: 审核通过（审核人≠创建人）
    Reviewing --> Rejected: 审核驳回
    Approved --> Published: 发布 → 分块 → embedding → pgvector
    Published --> Disabled: 停用 → 删除向量
    Disabled --> Published: 启用 → 重跑发布管道
```

- 文档上传支持 PDF/DOCX/MD/TXT（上限 50MB），异步解析入库
- 发布管道：Chunker(1000/200) → Embedder(batch=32) → pgvector halfvec → 先写后删替换旧向量
- 删除知识库级联清理文章和向量

### 4.3 申告管理

```mermaid
stateDiagram-v2
    [*] --> Pending: 报障人提交
    Pending --> Processing: 运维接单 start
    Processing --> Resolved: 标记解决 resolve
    Processing --> NeedSupplement: 索要补充 request_info
    NeedSupplement --> Processing: 报障人补充后自动退回
    Pending --> Closed: 关闭 close
    Processing --> Closed: 关闭 close
    NeedSupplement --> Closed: 关闭 close
```

- 状态机显式校验前置状态，补充信息上限 3 次
- 调度器每小时扫描，自动关闭超过 7 天的未完结申告
- CAS 防并发：`UPDATE WHERE id=? AND status=?`
- 编号格式：TK-YYYYMMDD-XXXX

### 4.4 用户与权限

```mermaid
flowchart TD
    SA["系统管理员 — 全部权限"]
    OP["运维人员 — 申告处理 / 知识候选"]
    KM["知识库管理员 — 知识 CRUD / 审核发布"]
    RP["报障人 — 门户端问答与申告"]

    JWT["JWT 双令牌<br/>access 2h / refresh 7d"] --> RBAC["RBAC 中间件<br/>角色 → 权限 → 路由"]

    RP --> JWT
    SA --> RBAC
    OP --> RBAC
    KM --> RBAC

    style SA fill:#ef444420,stroke:#ef4444
    style OP fill:#f59e0b20,stroke:#f59e0b
    style KM fill:#5e6ad220,stroke:#5e6ad2
    style RP fill:#22c55e20,stroke:#22c55e
```

- 密码策略：`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`，bcrypt cost=10
- 菜单-权限-路由三级绑定，`/admin/*` 强制 RBAC 校验

### 4.5 LLM 配置

- 双模式：llama.cpp 本地推理 / OpenAI-compatible 远程 API
- `atomic.Value` 热替换：修改默认配置即时生效，无需重启
- API Key AES-GCM 加密存储，JSON 序列化自动脱敏（`sk-****cret`）

### 4.6 数据看板与审计

- 实时统计 7 项指标：今日申告 / 待处理 / 处理中 / 已解决 / 今日问答 / 平均置信度 / 知识条目
- 趋势图：日粒度申告量 + 问答量
- 审计日志：按操作人 / 操作类型 / 日期筛选，记录所有管理操作

## 5. 技术选型

| 层 | 技术 |
|----|------|
| 后端框架 | Go 1.26 + Gin + GORM |
| 数据库 | PostgreSQL 18 + pgvector (halfvec + HNSW) |
| 对象存储 | MinIO (S3-compatible) |
| 前端 | Next.js 16 + React 19 + TypeScript + Tailwind CSS 4 |
| UI | Radix UI + Lucide Icons + SWR |
| LLM/Embedding | llama.cpp server 或 OpenAI-compatible API |
| 中文分词 | gse（纯 Go，无 CGO） |
| 部署 | Docker Compose（4 必须服务 + 1 可选 ai-local profile） |

## 6. API 概览

统一响应信封：`{"code":0,"message":"success","data":{}}`

| 路由组 | 前缀 | 认证 |
|--------|------|------|
| 公开 | `/api/v1/auth` | 无 |
| 门户 | `/api/v1/portal` | JWT |
| 管理 | `/api/v1/admin` | JWT + RBAC |

> 详细接口定义见 [API 文档](API/README.md)，业务数据流见 [FLOW 文档](FLOW/README.md)。

# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟠 功能缺陷 / 🟡 架构债务 / 🟢 优化建议 / ✅ 已完成

---

# 用户故事

## 报障人（门户端）

- US1: 作为报障人，我可以在智能问答中选择知识库并提问，获得 AI 基于私有知识的精准回答
- US2: 作为报障人，我可以在对话过程中随时停止 AI 生成，避免等待无意义内容
- US3: 作为报障人，我可以在问答页一键提交申告，附带聊天上下文给运维人员
- US4: 作为报障人，我可以查看我的申告列表，按状态筛选，搜索历史申告
- US5: 作为报障人，我可以查看申告处理进度、处理记录，并在需要时补充信息
- US6: 作为报障人，我可以接收站内消息通知（申告状态变更等），标记已读
- US7: 作为报障人，我可以在浅色/暗色主题间切换，系统记住我的偏好

## 运维人员（后台管理）

- US8: 作为运维人员，我可以查看数据看板，了解今日申告量、问答量、趋势变化
- US9: 作为运维人员，我可以处理申告（开始→标记解决/索要补充/关闭），全程记录操作日志
- US10: 作为运维人员，我可以管理知识库（创建/编辑/删除），上传多格式文档自动向量化
- US11: 作为运维人员，我可以审核知识文章（通过/驳回/发布/停用），维护知识质量
- US12: 作为运维人员，我可以将申告转化为知识候选，沉淀运维经验到知识库

## 系统管理员

- US13: 作为管理员，我可以管理用户（创建/编辑/冻结/恢复）和角色权限（RBAC）
- US14: 作为管理员，我可以配置 LLM 提供商（llama.cpp / OpenAI-compatible），测试连接
- US15: 作为管理员，我可以修改系统配置（应用名称、AI 参数），配置热生效
- US16: 作为管理员，我可以查看审计日志，按操作人/类型/日期筛选，追溯所有管理操作

---

# 后端

## 1. 智能问答与 RAG 管道

### 🔴 生产隐患

- 🔴 `nil VectorRetriever` 赋值给接口后调用 `Retrieve` 可能 panic——pgvector 初始化失败后向量检索触发服务崩溃（`retriever.go:30`, `main.go:222`）
- 🔴 `tryRefreshIndex` panic 无 recovery——`buildIndex()` 崩溃时 building 标志未清除 + 写锁未释放，BM25 索引永久死锁（`bm25.go:378`）
- 🔴 DB 宕机时 `chat_service` 的 `FindByID` 失败全部返回 `"会话不存在"(10004)`——用户无法区分"DB 故障"和"真的不存在"（`chat_service.go:133`）

### 🟠 功能缺陷

- 🟠 流式中断丢弃已累积答案——`answerBuf` 在 SSE error 事件时被丢弃，前端收不到部分回答（`llm_service.go:256`）
- 🟠 多路检索失败返回 nil error——LLM 调用失败时 `MultiRoute` 返回 nil 而非 error，管道日志显示 `Success=true`（`multi_route.go:57`）
- 🟠 `llmClient` 指针替换无同步——`SetLLMClient` 写与 `StreamChat` 读无 mutex 保护，存在数据竞争（`llm_service.go:104`）

### 🟡 架构债务

- 🟡 BM25 索引无增量更新，每次刷新全量重建——需算法重构
- 🟡 文档处理器无阶段内重试机制，embedding API 瞬时失败直接中止——需架构变更
- 🟡 错误事件未使用 `sendOrCancel` 保护——消费者退出时 goroutine 可能永久阻塞（`llm_service.go:201`）
- 🟡 Embedding 连接/超时不重试——与 LLM 流式调用的重试策略不对称（`embedding_client.go:150`）
- 🟡 重排序子进程崩溃后永不重启——恢复需重启服务（`rerank_client.go:178`）

### 🟢 优化建议

- 🟢 RAG 历史截断按消息条数而非 token 数——设计权衡，非阻塞
- 🟢 无向量检索模式开关——`RAGOptions` 缺 `DisableRetrieval`，无法做纯 LLM 对话（`rag/types.go:40`）
- 🟢 Sync/Stream 两个路径的 AI 不可用兜底文本不一致（`llm_service.go:151` vs `chat_service.go:24`）
- 🟢 重排序无内部超时，仅依赖调用者 context（`rerank_client.go:247`）

## 2. 知识库管理

### 🟠 功能缺陷

- 🟠 发布失败后向量丢失窗口——旧向量删除成功但新向量写入失败时，文章暂时无向量（`knowledge_service.go:433`）

### 🟡 架构债务

- 🟡 DOCX 解析仅读取 `word/document.xml`，不处理 `word/document2.xml` 分割文档——需解析器改进
- 🟡 PDF/DOCX 解析前全量读入内存（`io.ReadAll`），大文件 OOM 风险——需流式解析重构
- 🟡 50MB 上传上限硬编码，不支持按 KB 粒度配置——低优先级配置化
- 🟡 MinIO 惰性下载——`GetObject` 成功不代表数据可读，`defer reader.Close()` 不检查错误（`processor.go:201`）

### 🟢 优化建议

- 🟢 文档处理缺自动重试队列和死信队列——当前仅手动 `RetryDocument`

## 3. 申告管理

### 🟠 功能缺陷

- 🟠 自动关闭事务中的一条 Record 插入失败会阻塞全量工单关闭（`ticket_service.go:515`）

### 🟢 优化建议

- 🟢 `TicketRecord.OperatorID` 系统自动操作时设为 0，无 FK 约束——模型字段变更
- 🟢 消息通知失败静默吞没（仅 `slog.Warn`），调用方无感知——设计决策（通知是尽力而为）

## 4. 数据看板

- 🟢 趋势查询窗口硬编码，不可配置——低优先级

## 5. 系统配置

- 🟡 config_service 仅白名单 `app_name` 一个 key，扩展性受限——需架构改进
- 🟡 config.yaml / config.go 未暴露 MinIO bucket 名、上传大小上限、BM25 TTL 等——低优先级配置化

## 6. 基础设施

### 🟠 功能缺陷

- 🟠 `autoClose` 调度器退出时不等待进行中的任务完成——`ctx.Done()` 直接退出 goroutine（`scheduler.go:61`）

### 🟡 架构债务

- 🟡 `database/migrate.go` 每次启动重建全部索引（含 `IF NOT EXISTS`）——风险较高，需慎重评估
- 🟡 Router 中 ~150 行 handler nil-check 样板代码——需大规模重构
- 🟡 `ListSessions` 泄露原始 DB 错误——未包装为 AppError，可能暴露 SQL/连接信息（`chat_service.go:322`）

### 🟢 优化建议

- 🟢 无 DB / LLM / Embedding 健康检查端点或熔断器——下游依赖故障时无快速失败机制
- 🟢 默认 LLM 配置缺失时静默降级为硬编码 fallback，无日志警告（`llm_service.go:336`）

---

# 前端

## 1. 智能问答

- 🟢 虚拟列表 `estimateSize: () => 80` 常量估算，变长消息滚动位置不准——需消息高度测量，非阻塞

## 2. 表单与交互

- 🟢 表单缺 required 标记——非阻塞
- 🟢 用户搜索无结果提示——非阻塞

## 3. 组件架构

- 🟢 StatusBadge 领域状态映射硬编码在组件内，后端新增状态时前端需同步更新——`statusText` prop 为已提供的逃生舱

## 4. 设计系统

- 🟢 审计页输入框样式在 5 处重复——审计页布局特殊，不适合直接 `AppleInput`

## 5. 基础设施

- 🟡 零代码分割，全量打包——需 `next/dynamic` 架构变更
- 🟢 全局 ErrorBoundary 仅顶层一个——`SectionErrorBoundary` 已覆盖内容区

---

## 统计

| | 🔴 P0 | 🟠 P1 | 🟡 P2 | 🟢 P3 |
|---|---|---|---|---|
| 后端 | 3 | 6 | 8 | 9 |
| 前端 | 0 | 0 | 1 | 5 |
| **合计** | **3** | **6** | **9** | **14** |

> 来源：`docs/degradation-matrix.md` 全链路降级审计 v1.0（2026-06-23）

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

- ✅ `nil VectorRetriever` panic — `Retrieve` 增加 nil receiver 检查，静默降级返回空结果（`retriever.go:30`）
- ✅ `tryRefreshIndex` panic 无 recovery — 增加 `defer recover()` 保护，panic 时清除 building 标志（`bm25.go:378`）
- ✅ DB 宕机伪装为"会话不存在" — `FindByID` 增加 `errors.Is(err, gorm.ErrRecordNotFound)` 区分 DB 故障（`chat_service.go:133`）

### 🟠 功能缺陷

- ✅ 流式中断丢弃已累积答案 — 已有 `answerBuf` 时发送 `done`（含部分回答），无内容时发送 `error`（`llm_service.go:256`）
- ✅ 多路检索失败返回 nil error — 改为返回 `fmt.Errorf("多路检索 LLM 调用失败，降级为单路: %w", err)`（`multi_route.go:57`）
- ✅ `llmClient` 指针替换数据竞争 — 添加 `sync.Mutex` 保护读写，通过 `getLLMClient()` 访问（`llm_service.go:104`）

### 🟡 架构债务

- 🟡 BM25 索引无增量更新，每次刷新全量重建——需算法重构
- 🟡 文档处理器无阶段内重试机制，embedding API 瞬时失败直接中止——需架构变更
- ✅ 错误事件改用 sendOrCancel 保护（llm_service.go:201）
- ✅ Embedding 连接/超时改为始终重试（embedding_client.go:150）
- ✅ 重排序子进程崩溃后 3s 自动重启（rerank_client.go:178）

### 🟢 优化建议

- 🟢 RAG 历史截断按消息条数而非 token 数——设计权衡，非阻塞
- ✅ Admin 可运行时配置 RAG 各步骤开关 + TopK + 阈值，ChatService 请求时读取最新值
- ✅ Sync/Stream 兜底文本统一为 chat_service.go 常量（llm_service.go:151）
- ✅ 重排序增加内部 30s 超时保护（rerank_client.go:247）

## 2. 知识库管理

### 🟠 功能缺陷

- ✅ 发布向量替换改为事务内操作 ReplaceVectors（vector_store.go）

### 🟡 架构债务

- 🟡 DOCX 解析仅读取 `word/document.xml`，不处理 `word/document2.xml` 分割文档——需解析器改进
- 🟡 PDF/DOCX 解析前全量读入内存（`io.ReadAll`），大文件 OOM 风险——需流式解析重构
- 🟡 50MB 上传上限硬编码，不支持按 KB 粒度配置——低优先级配置化
- 🟡 MinIO 惰性下载——`GetObject` 成功不代表数据可读，`defer reader.Close()` 不检查错误（`processor.go:201`）

### 🟢 优化建议

- 🟢 文档处理缺自动重试队列和死信队列——当前仅手动 `RetryDocument`

## 3. 申告管理

### 🟠 功能缺陷

- ✅ 自动关闭单条失败改为 continue 跳过，不阻塞其余工单（ticket_service.go:515）

### 🟢 优化建议

- ✅ OperatorID=0 为系统操作哨兵值，无 FK 是设计决策
- ✅ 通知是尽力而为副作用，静默吞没是正确设计

## 4. 数据看板

- 🟢 趋势查询窗口硬编码，不可配置——低优先级

## 5. 系统配置

- ✅ config_service 白名单扩展至 8 个 key，含全部 RAG 开关
- ✅ 菜单去冗余：「LLM 配置」已合并入「模型配置」，路径对齐前端路由 (`/admin/config/llm`, `/admin/config/system`)，8 项菜单

## 6. 基础设施

### 🟠 功能缺陷

- ✅ `autoClose` 调度器退出时不等待进行中的任务完成 — 添加 WaitGroup + Stop 等待进行中任务（`scheduler.go:61`）

### 🟡 架构债务

- ✅ `database/migrate.go` 每次启动重建全部索引 — 改用 `CREATE INDEX IF NOT EXISTS`，去 DROP（`migrate.go`）
- ✅ Router 中 ~150 行 handler nil-check 样板代码 — safeHandler 辅助函数消除 if/else（`helpers.go` + `portal.go` + `admin.go`）
- ✅ `ListSessions` 泄露原始 DB 错误 — 包装为 `fmt.Errorf("查询会话列表失败: %w", err)`（`chat_service.go:327`）

### 🟢 优化建议

- ✅ 无 DB 健康检查端点 — `/readyz` 增加 DB Ping 探测（`router.go` + `main.go`）
- ✅ 默认 LLM 配置缺失时静默降级 — `getModelConfig` 增加 `slog.Warn` 降级日志（`llm_service.go:404`）

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
| 后端 | 0 | 0 | 6 | 3 |
| 前端 | 0 | 0 | 1 | 6 |
| **合计** | **0** | **0** | **7** | **9** |

> 最近修复（2026-06-23）：6 项（1 🟠 + 3 🟡 + 2 🟢），降级矩阵已并入 [TECH.md §5](TECH.md#5-可靠性设计)

---

# 未来方向

## 产品路线图

### 2026 H2

- 知识库模板系统：常见运维场景（新员工入职、设备申领）一键创建知识库结构
- 看板增强：自定义日期范围、数据导出
- 申告满意度评价：闭环反馈机制

### 2027

- 外部 ITSM 对接（Jira Service Management、ServiceNow）
- 自然语言转 SQL：非技术人员直接查询运维数据
- 知识库覆盖度分析：识别高频未命中问题，指导知识补充

## 技术演进

- BM25 索引增量更新替代全量重建
- 文档处理器增加阶段内重试 + 死信队列
- DOCX 解析支持 `document2.xml` 分割文档
- PDF/DOCX 流式解析替代全量读入内存
- 前端代码分割：`next/dynamic` 按路由懒加载
- 50MB 上传上限配置化

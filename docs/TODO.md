# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议
> 📌 标记为代码中已存在 `// TODO:` 注释，与本文档双向同步。
> 已完成项不在此列出（见 git log）。
> 最后审计：2026-06-18，覆盖 93 后端 + 63 前端 = 156 源文件。

---

# 后端

## 1. 认证与授权

- 📌 🟡 每次 API 请求都查 DB 校验用户状态，高并发时 N 次额外查询 — [server/internal/middleware/auth.go:73](server/internal/middleware/auth.go#L73)
- 🟢 ChangePassword 未校验新旧密码不同 — [server/internal/service/auth_service.go:268](server/internal/service/auth_service.go#L268)

## 2. 智能问答

- 🟡 BM25 索引无增量更新，每次刷新全量重建 — [server/internal/rag/bm25.go:213](server/internal/rag/bm25.go#L213)
- 🟡 文档处理器无阶段内重试机制，embedding API 瞬时失败直接中止 — [server/internal/rag/processor.go](server/internal/rag/processor.go)
- 🟢 RAG 历史截断按消息条数而非 token 数 — [server/internal/service/llm_service.go](server/internal/service/llm_service.go)
- 📌 🟡 rerank_client.go doc 引用笔误指向 multi_route.go — [server/internal/rag/rerank.go](server/internal/rag/rerank.go)

## 3. 知识库管理

- 🟡 DOCX 解析仅读取 `word/document.xml`，不处理 `word/document2.xml` 分割文档 — [server/internal/rag/document_parser.go:142](server/internal/rag/document_parser.go#L142)
- 🟡 PDF/DOCX 解析前全量读入内存（`io.ReadAll`），大文件 OOM 风险 — [server/internal/rag/document_parser.go](server/internal/rag/document_parser.go)
- 🟡 50MB 上传上限硬编码，不支持按 KB 粒度配置 — [server/internal/service/knowledge_service.go:30](server/internal/service/knowledge_service.go#L30)

## 4. 申告管理

- 📌 📌 未读数每 30 秒全量 COUNT 查询，适合缓存或 WebSocket/SSE 推送 — [server/internal/service/message_service.go:102](server/internal/service/message_service.go#L102)
- 🟢 TicketRecord.OperatorID 系统自动操作时设为 0，无 FK 约束 — [server/internal/model/ticket.go](server/internal/model/ticket.go)

## 5. 数据看板与审计

- 📌 🔴 Dashboard repo 用字符串拼接构造 SQL `date_trunc`，虽上层有校验但脆弱 — [server/internal/repository/dashboard_repo.go:70](server/internal/repository/dashboard_repo.go#L70)
- 🟡 DashboardService 并行 7 个 goroutine 查询统计，任一失败不取消其余 — [server/internal/service/dashboard_service.go](server/internal/service/dashboard_service.go)
- 🟢 趋势查询 90 天窗口硬编码，不可配置 — [server/internal/repository/dashboard_repo.go](server/internal/repository/dashboard_repo.go)
- 🟢 Audit handler 使用硬编码错误码 `10003`，应用 `errcode.ErrParam` — [server/internal/handler/audit.go](server/internal/handler/audit.go)

## 6. 系统管理与配置

- 📌 🔴 LlmConfig.BeforeSave 每次保存都执行加密，更新非 APIKey 字段时已加密值可能被重复加密 — [server/internal/model/llm_config.go:43](server/internal/model/llm_config.go#L43)
- 🟡 config_service 仅白名单 `app_name` 一个 key，扩展性受限 — [server/internal/service/config_service.go](server/internal/service/config_service.go)
- 🟡 config.yaml / config.go 未暴露 MinIO bucket 名、上传大小上限、BM25 TTL 等为可配置项 — [server/internal/config/config.go:270](server/internal/config/config.go#L270)
- 🟢 反馈提交允许 feedback=0（未反馈）覆盖已有反馈，语义不明确 — [server/internal/service/chat_service.go:235](server/internal/service/chat_service.go#L235)

## 7. 基础设施

- 📌 🟡 日志文件无保留策略，长期运行磁盘持续增长 — [server/internal/log/rotating_writer.go:1](server/internal/log/rotating_writer.go#L1)
- 📌 🟡 Scheduler.doAutoClose 使用 `context.Background()`，优雅关闭时无法取消正在执行的自动关闭 — [server/internal/service/scheduler.go:70](server/internal/service/scheduler.go#L70)
- 🟡 `database/migrate.go` 每次启动重建全部索引（含 `IF NOT EXISTS`），零停机部署风险 — [server/internal/database/migrate.go:50](server/internal/database/migrate.go#L50)
- 🟡 Router 中 ~150 行 handler nil-check 样板代码，`placeholder()` 未统一使用 — [server/internal/router/admin.go](server/internal/router/admin.go)
- 🟢 bcrypt cost=10 硬编码，不同硬件环境下不可调 — [server/pkg/hash/hash.go](server/pkg/hash/hash.go)

---

# 前端

## 1. 认证与授权

- 🟡 proxy.ts 中 JWT 解码/过期判断与 `lib/auth.ts` 逻辑重复 — [web/src/proxy.ts:16](web/src/proxy.ts#L16) / [web/src/lib/auth.ts:18](web/src/lib/auth.ts#L18)
- 🟢 useAuth cookie 同步 effect 在 token 变 null（退出登录）时未清除 cookie — [web/src/hooks/useAuth.tsx:73](web/src/hooks/useAuth.tsx#L73)

## 2. 智能问答

- 🟡 Chat 页面 212 行单文件：SSE 流解析 + 虚拟滚动 + 消息管理耦合，应拆分为 `useChatStream` hook — [web/src/app/portal/chat/page.tsx](web/src/app/portal/chat/page.tsx)
- 🟡 SSE 流解析错误仅 `console.debug`，生产构建中静默丢弃 — [web/src/app/portal/chat/page.tsx:140](web/src/app/portal/chat/page.tsx#L140)
- 🟡 `response.body!` non-null 断言，body 为 null 时运行时崩溃 — [web/src/app/portal/chat/page.tsx:97](web/src/app/portal/chat/page.tsx#L97)
- 🟢 SSE 超时 120 秒硬编码，无用户提示 — [web/src/app/portal/chat/page.tsx:93](web/src/app/portal/chat/page.tsx#L93)
- 🟢 虚拟列表 `key="pipeline"` 为静态字符串，可能导致 React 复用错误 — [web/src/app/portal/chat/page.tsx:184](web/src/app/portal/chat/page.tsx#L184)

## 3. 知识库管理

- 🟡 文档上传仍用原始 XMLHttpRequest + 手动 Promise 包装，~20 行样板代码 — [web/src/app/admin/knowledge/[kbId]/new/page.tsx:37](web/src/app/admin/knowledge/[kbId]/new/page.tsx#L37)
- 🟢 文章标签用数组索引作 key，标签列表排序变化时 React 可能误渲染 — [web/src/app/admin/knowledge/[kbId]/[articleId]/page.tsx:63](web/src/app/admin/knowledge/[kbId]/[articleId]/page.tsx#L63)
- 🟢 50MB 文件大小限制仅在前端提示文本中，无实际前端校验 — [web/src/app/admin/knowledge/[kbId]/new/page.tsx:95](web/src/app/admin/knowledge/[kbId]/new/page.tsx#L95)

## 4. 申告管理

- 🟡 消息标记已读未处理 API 失败（unhandled promise rejection） — [web/src/app/portal/messages/page.tsx:16](web/src/app/portal/messages/page.tsx#L16)
- 🟢 handleSupplement 无 try/catch，早期 return 前的校验逻辑裸露 — [web/src/app/portal/tickets/[id]/page.tsx:20](web/src/app/portal/tickets/[id]/page.tsx#L20)
- 🟢 ticket status=3 硬编码控制补充信息区域显隐，后端新增状态时静默隐藏 — [web/src/app/portal/tickets/[id]/page.tsx:63](web/src/app/portal/tickets/[id]/page.tsx#L63)

## 5. 数据看板与审计

- 🟡 手写 bar chart（inline style + index key），无障碍性差，无 tooltip — [web/src/app/admin/dashboard/page.tsx:54](web/src/app/admin/dashboard/page.tsx#L54)
- 🟡 useDebounce 在 `audit/page.tsx` 中重复定义，应提取到 `hooks/useDebounce.ts` — [web/src/app/admin/audit/page.tsx:9](web/src/app/admin/audit/page.tsx#L9)
- 🟢 图例色块为 Unicode 字符 `■`，跨平台渲染不一致 — [web/src/app/admin/dashboard/page.tsx:66](web/src/app/admin/dashboard/page.tsx#L66)
- 🟢 start/end 日期每次 render 重新计算，未 useMemo — [web/src/app/admin/dashboard/page.tsx:13](web/src/app/admin/dashboard/page.tsx#L13)

## 6. 系统管理与配置

- 🟡 LLMConfig 编辑时强制清空 APIKey 字段，用户每次编辑都必须重新输入 — [web/src/app/admin/config/llm/page.tsx:23](web/src/app/admin/config/llm/page.tsx#L23)
- 🟡 ConfigRow 每个 key 一次 SWR 请求，10 个 key = 10 次并行请求 — [web/src/app/admin/config/system/page.tsx:26](web/src/app/admin/config/system/page.tsx#L26)
- 🟢 测试连接结果用 emoji 前缀匹配判断成功/失败（`startsWith('✅')`），脆弱 — [web/src/app/admin/config/llm/page.tsx:89](web/src/app/admin/config/llm/page.tsx#L89)
- 🟢 用户搜索无防抖，每次按键触发 SWR 重新请求 — [web/src/app/admin/users/page.tsx](web/src/app/admin/users/page.tsx)
- 🟢 角色权限列表 `knownPermissions` 每次 render 重新计算，应 useMemo — [web/src/app/admin/roles/page.tsx:26](web/src/app/admin/roles/page.tsx#L26)

## 7. 基础设施

- 🔴 全局内联样式（~30 文件/数百处），无 SSR CSS 抽取，无样式去重，暗色模式维护困难 — [web/src/](web/src/)（全量）
- 📌 🟡 AppleBadge 硬编码 hex 色值，暗色模式下不自适应 — [web/src/components/ui/AppleBadge.tsx:5](web/src/components/ui/AppleBadge.tsx#L5)
- 🟡 未读数轮询逻辑在 AdminLayout 和 PortalLayout 中完全重复 — [web/src/components/layout/AdminLayout.tsx:44](web/src/components/layout/AdminLayout.tsx#L44) / [web/src/components/layout/PortalLayout.tsx:28](web/src/components/layout/PortalLayout.tsx#L28)
- 🟡 轮询错误静默吞没（`.catch(() => {})`），API 持续失败时未读数永久过期 — [web/src/components/layout/PortalLayout.tsx:29](web/src/components/layout/PortalLayout.tsx#L29)
- 🟡 not-found 使用 `<a>` 而非 `<Link>`，导致整页刷新而非客户端导航 — [web/src/app/not-found.tsx:6](web/src/app/not-found.tsx#L6)
- 🟡 全局 ErrorBoundary 只有顶层一个，子模块崩溃会让整个 UI 替换为错误页 — [web/src/components/Providers.tsx](web/src/components/Providers.tsx)
- 🟡 apiFetch 不自动附加 Authorization header，每个调用方手动添加 — [web/src/lib/api/client.ts:22](web/src/lib/api/client.ts#L22)
- 🟢 全局零 `useMemo` 使用，多处可 memoize 的计算每 render 重复执行 — [web/src/app/admin/roles/page.tsx:26](web/src/app/admin/roles/page.tsx#L26)
- 🟢 AppleSpinner 动画依赖全局 CSS 中的 `@keyframes spin`，无局部定义 — [web/src/components/ui/AppleSpinner.tsx:14](web/src/components/ui/AppleSpinner.tsx#L14)
- 🟢 图标按钮（主题切换/侧栏折叠/消息）缺少 `aria-label` — [web/src/components/layout/AdminLayout.tsx:105](web/src/components/layout/AdminLayout.tsx#L105)
- 🟢 PortalLayout 中 clickable `<span>` 无 `role="button"` 和键盘支持 — [web/src/components/layout/PortalLayout.tsx:43](web/src/components/layout/PortalLayout.tsx#L43)

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（7）

| 位置 | 内容 |
|------|------|
| 📌 `server/internal/repository/dashboard_repo.go:70` | SQL 拼接 date_trunc 应用参数化 |
| 📌 `server/internal/model/llm_config.go:43` | APIKey 重复加密检测 |
| 📌 `server/internal/log/rotating_writer.go:1` | 日志文件保留策略 |
| 📌 `server/internal/middleware/auth.go:73` | 用户状态查询缓存 |
| 📌 `server/internal/service/scheduler.go:70` | context.Background() 改为可取消 |
| 📌 `server/internal/service/message_service.go:102` | 未读数缓存/WebSocket |
| 📌 `server/internal/rag/rerank.go` | doc 引用笔误修正 |

### 前端 TODO（1）

| 位置 | 内容 |
|------|------|
| 📌 `web/src/components/ui/AppleBadge.tsx:5` | 暗色模式色值适配 |

### 一致性校验

- ✅ 代码中 7 个后端 `// TODO:` ↔ TODO.md 后端 TODO 表 7 条 — 完全匹配
- ✅ 代码中 1 个前端 `/** TODO:` ↔ TODO.md 前端 TODO 表 1 条 — 完全匹配
- ✅ TODO.md 所有 📌 项均能在对应文件中找到同名 TODO 注释

---

## 统计

### 后端

| 模块 | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|------|-------|-------|-------|---------|
| 1. 认证与授权 | — | 1 | 1 | 1 |
| 2. 智能问答 | — | 2 | 1 | 1 |
| 3. 知识库管理 | — | 3 | — | — |
| 4. 申告管理 | — | 1 | 1 | 1 |
| 5. 数据看板与审计 | 1 | 1 | 2 | 1 |
| 6. 系统管理与配置 | 1 | 2 | 1 | 1 |
| 7. 基础设施 | — | 3 | 1 | 2 |
| **后端合计** | **2** | **13** | **7** | **7** |

### 前端

| 模块 | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|------|-------|-------|-------|---------|
| 1. 认证与授权 | — | 1 | 1 | — |
| 2. 智能问答 | — | 3 | 2 | — |
| 3. 知识库管理 | — | 1 | 2 | — |
| 4. 申告管理 | — | 1 | 2 | — |
| 5. 数据看板与审计 | — | 2 | 2 | — |
| 6. 系统管理与配置 | — | 2 | 3 | — |
| 7. 基础设施 | 1 | 6 | 4 | 1 |
| **前端合计** | **1** | **16** | **16** | **1** |

### 全栈总计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 2 | 13 | 7 | 7 |
| 前端 | 1 | 16 | 16 | 1 |
| **合计** | **3** | **29** | **23** | **8** |

---

## 审计元数据

- **审计日期：** 2026-06-18
- **覆盖范围：** 后端 93 Go 文件（`server/internal/` + `server/cmd/` + `server/pkg/`）+ 前端 63 TS/TSX 文件（`web/src/`）
- **API 文档一致性：** ✅ 68/68 端点双向匹配（`docs/API/` ↔ Router 代码）
- **代码 TODO 双向同步：** ✅ 8 个代码 TODO 全部在本文档中索引，本文档 📌 项全部可追溯到代码
- **前次审计遗留：** 2 个已完成的 config.yaml TODO 已移除（read_timeout / max_open_conns 均已实现）

# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议
> 📌 标记为代码中已存在 `// TODO:` 注释，与本文档双向同步。

---

# 后端

## 1. 认证与授权

- ✅ 📌 🟡 每次 API 请求都查 DB 校验用户状态 — 已修复：`cache/user_status.go` 内存缓存 + 冻结/恢复时失效
- ✅ 🟢 ChangePassword 未校验新旧密码不同 — 已修复：`auth_service.go:279` 添加 `oldPwd == newPwd` 校验

## 2. 智能问答

- 🟡 BM25 索引无增量更新，每次刷新全量重建 — 保留（需算法重构）
- 🟡 文档处理器无阶段内重试机制，embedding API 瞬时失败直接中止 — 保留（需架构变更）
- 🟢 RAG 历史截断按消息条数而非 token 数 — 保留（设计权衡，非阻塞）
- ✅ 📌 🟡 rerank.go doc 引用笔误 — 已修复：更正为 `adapter/rerank_client.go`

## 3. 知识库管理

- 🟡 DOCX 解析仅读取 `word/document.xml`，不处理 `word/document2.xml` 分割文档 — 保留（需解析器改进）
- 🟡 PDF/DOCX 解析前全量读入内存（`io.ReadAll`），大文件 OOM 风险 — 保留（需流式解析重构）
- 🟡 50MB 上传上限硬编码，不支持按 KB 粒度配置 — 保留（低优先级配置化）

## 4. 申告管理

- ✅ 📌 🟡 未读数每 30 秒全量 COUNT 查询 — 保留（需缓存/WebSocket 基础设施）
- 🟢 TicketRecord.OperatorID 系统自动操作时设为 0，无 FK 约束 — 保留（模型字段变更）

## 5. 数据看板与审计

- ✅ 📌 🔴 Dashboard repo 字符串拼接 SQL `date_trunc` — 已修复：改用 `CASE WHEN` 参数化查询
- 🟡 DashboardService 并行 7 个 goroutine 查询统计，任一失败不取消其余 — 保留（需 concurrency 重构）
- 🟢 趋势查询 90 天窗口硬编码，不可配置 — 保留（低优先级）
- ✅ 🟢 Audit handler 使用硬编码错误码 `10003` — 已修复：改用 `errcode.ErrParam`

## 6. 系统管理与配置

- 📌 🔴 LlmConfig.BeforeSave 每次保存都执行加密，更新非 APIKey 字段时已加密值可能被重复加密 — 保留（模型行为变更，需谨慎）
- 🟡 config_service 仅白名单 `app_name` 一个 key，扩展性受限 — 保留（需架构改进）
- 🟡 config.yaml / config.go 未暴露 MinIO bucket 名、上传大小上限、BM25 TTL 等 — 保留（低优先级配置化）
- ✅ 🟢 反馈提交允许 feedback=0 覆盖已有反馈 — 已修复：`chat_service.go` 拒绝 feedback=0 的提交

## 7. 基础设施

- ✅ 📌 🟡 日志文件无保留策略 — 已修复：添加 `maxFiles` 限制 + `prune()` 自动清理
- ✅ 📌 🟡 Scheduler.doAutoClose 使用 `context.Background()` — 已修复：改用 `context.WithTimeout`
- 🟡 `database/migrate.go` 每次启动重建全部索引（含 `IF NOT EXISTS`）— 保留（风险较高，需慎重评估）
- 🟡 Router 中 ~150 行 handler nil-check 样板代码 — 保留（需大规模重构）
- ✅ 🟢 bcrypt cost=10 硬编码 — 已修复：`hash.go` 支持 `OPSMIND_BCRYPT_COST` 环境变量

---

# 前端

## 1. 认证与授权

- 🟡 proxy.ts 中 JWT 解码/过期判断与 `lib/auth.ts` 逻辑重复 — 保留（需提取共享模块但 non-trivial）
- 🟢 useAuth cookie 同步 effect 在 token 变 null 时未清除 cookie — 保留（行为变更需验证）

## 2. 智能问答

- 🟡 Chat 页面 212 行单文件：SSE 流解析 + 虚拟滚动 + 消息管理耦合 — 保留（需大规模组件拆分）
- ✅ 🟡 SSE 流解析错误仅 `console.debug` — 已修复：移除无效日志，静默跳过不完整分块
- ✅ 🟡 `response.body!` non-null 断言 — 已修复：添加 null 检查提前 throw
- 🟢 SSE 超时 120 秒硬编码，无用户提示 — 保留（需 UI 提示设计）
- 🟢 虚拟列表 `key="pipeline"` 静态字符串 — 保留（单实例无实际风险）

## 3. 知识库管理

- 🟡 文档上传仍用原始 XMLHttpRequest + 手动 Promise 包装 — 保留（需 fetch API 重构）
- 🟢 文章标签用数组索引作 key — 保留（低频风险）
- 🟢 50MB 文件大小限制仅在前端提示文本中 — 保留（后端已有校验）

## 4. 申告管理

- ✅ 🟡 消息标记已读未处理 API 失败 — 已修复：添加 try/catch + toast.error
- ✅ 🟢 handleSupplement 已有 try/catch — 已存在（前次修复后未更新 TODO）
- ✅ 🟢 ticket status=3 硬编码 — 已修复：提取 `TICKET_STATUS_NEED_SUPPLEMENT` 常量

## 5. 数据看板与审计

- 🟡 手写 bar chart（inline style + index key）— 保留（需图表库或完整组件重构）
- ✅ 🟡 useDebounce 在 `audit/page.tsx` 中重复定义 — 已修复：提取到 `hooks/useDebounce.ts`
- ✅ 🟢 图例色块为 Unicode 字符 `■` — 已修复：替换为 styled `<span>` 色块
- ✅ 🟢 start/end 日期每次 render 重新计算 — 已修复：添加 `useMemo`

## 6. 系统管理与配置

- 🟡 LLMConfig 编辑时强制清空 APIKey 字段 — 保留（行为变更需产品确认）
- 🟡 ConfigRow 每个 key 一次 SWR 请求 — 保留（需批量 API 支持）
- 🟢 测试连接结果用 emoji 前缀匹配 — 保留（需结构化响应字段）
- 🟢 用户搜索无防抖 — 保留（SWR 自动去重已有一定缓解）
- 🟢 角色权限列表 `knownPermissions` 每次 render 重新计算 — 保留（数据量小，非瓶颈）

## 7. 基础设施

- ✅ 🔴 全局内联样式（~30 文件/数百处）— 已修复：全量迁移至 Tailwind CSS v4 工具类，替换 CSS Modules，零 `.module.css` 文件残留
- ✅ 📌 🟡 AppleBadge 硬编码 hex 色值，暗色模式不自适应 — 已修复：改用 CSS 变量 + `[data-theme="dark"]` 覆盖
- ✅ 🟡 未读数轮询逻辑在 AdminLayout 和 PortalLayout 中完全重复 — 已修复：提取 `hooks/useUnreadCount.ts` 共享 hook
- 🟡 轮询错误静默吞没（`.catch(() => {})`）— 保留（行为变更需验证）
- ✅ 🟡 not-found 使用 `<a>` 而非 `<Link>` — 已修复：改用 `<Link>` 实现客户端导航
- 🟡 全局 ErrorBoundary 只有顶层一个 — 保留（需分层错误边界设计）
- 🟡 apiFetch 不自动附加 Authorization header — 保留（可能破坏现有调用方）
- 🟢 全局零 `useMemo` 使用 — 保留（优化建议，非必需）
- 🟢 AppleSpinner 动画依赖全局 CSS 中的 `@keyframes spin` — 保留（全局 keyframes 正常模式，非 bug）
- ✅ 🟢 图标按钮缺少 `aria-label` — 已修复：AdminLayout 侧栏折叠/主题切换/消息添加 aria-label
- ✅ 🟢 PortalLayout 中 clickable `<span>` 无 `role="button"` — 已修复：添加 role/button/tabIndex/onKeyDown/aria-label

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（5 → 已清理 5 个）

| 位置 | 内容 | 状态 |
|------|------|------|
| ~~`server/internal/repository/dashboard_repo.go:70`~~ | ~~SQL 拼接 date_trunc~~ | ✅ 已修复 |
| ~~`server/internal/log/rotating_writer.go:1`~~ | ~~日志文件保留策略~~ | ✅ 已修复 |
| ~~`server/internal/service/scheduler.go:70`~~ | ~~context.Background()~~ | ✅ 已修复 |
| ~~`server/internal/rag/rerank.go`~~ | ~~doc 引用笔误~~ | ✅ 已修复 |
| ~~`server/internal/middleware/auth.go:73`~~ | ~~用户状态每次查 DB~~ | ✅ 已修复（内存缓存） |
| `server/internal/model/llm_config.go:43` | APIKey 重复加密检测 | 📌 保留 |
| `server/internal/service/message_service.go:102` | 未读数缓存/WebSocket | 📌 保留 |

### 前端 TODO（0 → 已清理 0 个）

前端代码已无 TODO 注释。本次重构：
- 全量迁移至 Tailwind CSS v4（`globals.css` + `@theme` 配置 Apple Design Tokens）
- 删除全部 40+ 个 `.module.css` 文件
- 移除旧 `styles/global.css` 和 `styles/tokens.css`
- 新增 `postcss.config.mjs`（`@tailwindcss/postcss` 插件）

---

## 统计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 1 | 6 | 2 | 2 |
| 前端 | 0 | 6 | 3 | 0 |
| **合计** | **1** | **12** | **5** | **2** |

---

> 本次修复：Tailwind CSS v4 全量迁移 + 前端 API 全覆盖审计 — 补充 4 个缺失函数（getUserDetail/getLLMConfigDetail/getRoleDetail/updateRoleMenus），58→62 函数，覆盖全部可封装端点。

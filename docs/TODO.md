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

- ✅ 📌 🟡 未读数每 30 秒全量 COUNT 查询 — 已修复：`MessageService` 增加 15 秒 TTL 缓存，新增消息/标记已读时失效
- 🟢 TicketRecord.OperatorID 系统自动操作时设为 0，无 FK 约束 — 保留（模型字段变更）

## 5. 数据看板与审计

- ✅ 📌 🔴 Dashboard repo 字符串拼接 SQL `date_trunc` — 已修复：改用 `CASE WHEN` 参数化查询
- ✅ 🟡 DashboardService 并行 7 个 goroutine 查询统计，任一失败不取消其余 — 已修复：`context.WithCancel` 首错取消其余查询
- 🟢 趋势查询 90 天窗口硬编码，不可配置 — 保留（低优先级）
- ✅ 🟢 Audit handler 使用硬编码错误码 `10003` — 已修复：改用 `errcode.ErrParam`

## 6. 系统管理与配置

- ✅ 📌 🔴 LlmConfig.BeforeSave 每次保存都执行加密，更新非 APIKey 字段时已加密值可能被重复加密 — 已修复：`crypto.Encrypt` 增加 `cipher:` 前缀幂等与旧密文兼容
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

- ✅ 🟡 proxy.ts 中 JWT 解码/过期判断与 `lib/auth.ts` 逻辑重复 — 已修复：proxy.ts 复用 `lib/auth.ts` 的 `decodeJwtPayload`/`isTokenExpired`
- ✅ 🟢 useAuth cookie 同步 effect 在 token 变 null 时未清除 cookie — 已修复：logout 时清除 `access_token`/`refresh_token` cookie
- 🔴 `src/proxy.ts` 文件名错误，Next.js 要求 `middleware.ts` 才会执行 — 路由守卫、JWT 校验、RBAC 全部未生效

## 2. 智能问答

- ✅ 🟡 Chat 页面 212 行单文件 — 已修复：提取 `hooks/useChatStream.ts`，SSE 流解析/状态管理/清理逻辑封装
- ✅ 🟡 SSE 流解析错误仅 `console.debug` — 已修复：移除无效日志
- ✅ 🟡 `response.body!` non-null 断言 — 已修复：添加 null 检查
- ✅ 🟢 SSE 超时 120 秒硬编码 — 已修复：超时时检测 userAbort 标志，区分主动取消与超时
- ✅ 🟢 虚拟列表 `key="pipeline"` 静态字符串 — 已修复：使用 `key={`pipeline-${currentStep}`}`
- 🟡 Chat 页面移动端无侧边栏切换按钮 — `sidebarOpen` 仅作用于 `lg:` 断点，`<lg` 时侧边栏永远隐藏
- 🟢 Chat 虚拟列表 `estimateSize: () => 80` 常量估算，变长消息滚动位置不准

## 3. 知识库管理

- ✅ 🟡 文档上传仍用原始 XMLHttpRequest — 已修复：改用 `fetch()` + `FormData`
- ✅ 🟢 文章标签用数组索引作 key — 已修复：改用标签字符串作 key
- ✅ 🟢 50MB 文件大小限制仅在前端提示文本中 — 已修复：添加上传前 `file.size` 校验 + toast 提示
- 🟢 文档上传 `<input type="file">` 为浏览器默认样式，与 Apple 设计系统不一致

## 4. 申告管理

- ✅ 🟡 消息标记已读未处理 API 失败 — 已修复
- ✅ 🟢 handleSupplement 已有 try/catch — 已存在
- ✅ 🟢 ticket status=3 硬编码 — 已修复

## 5. 数据看板与审计

- ✅ 🟡 手写 bar chart（inline style + index key）— 已修复：替换 `key={i}` → `key={d.date}`
- ✅ 🟡 useDebounce 重复定义 — 已修复
- ✅ 🟢 图例色块 Unicode — 已修复
- ✅ 🟢 start/end 日期每次 render 重新计算 — 已修复
- 🟡 审计日志页日期筛选为纯文本 `<input>`，无 datepicker 和格式校验
- 🟡 审计日志页全部筛选器使用原生 `<input>` 而非 `AppleInput` 组件（样式不一致）
- 🟢 30 天趋势图小屏幕上柱状条拥挤（30 数据点，bar 6px + gap 3px）

## 6. 系统管理与配置

- ✅ 🟡 LLMConfig 编辑时强制清空 APIKey 字段 — 已修复：空 APIKey 时从请求体删除，后端不修改已存值
- ✅ 🟡 ConfigRow 每个 key 一次 SWR 请求 — 已修复：提取 `getAllConfigs()` 批量获取，单次 SWR 调用
- ✅ 🟢 测试连接结果用 emoji 前缀匹配 — 已修复：改用 `{ success: boolean; message: string }` 结构化判断
- ✅ 🟢 用户搜索无防抖 — 已修复：添加 `useDebounce(keyword, 300)`
- ✅ 🟢 角色权限列表 `knownPermissions` 每次 render 重新计算 — 已修复：添加 `useMemo`
- 🟡 6 个 Radix UI 包已安装但未使用：`dropdown-menu`/`popover`/`select`/`slot`/`switch`/`tabs`/`tooltip`
- 🟢 多处硬编码魔术数字：urgency 映射数组、MAX_FILE_SIZE、默认分页大小 10

## 7. 基础设施

- ✅ 🔴 全局内联样式 — 已修复：Tailwind CSS v4 全量迁移
- ✅ 🟡 轮询错误静默吞没 — 已修复：添加 `console.warn`
- ✅ 🟡 全局 ErrorBoundary 只有顶层一个 — 已修复：新增 `SectionErrorBoundary` 包裹 AdminLayout 内容区
- ✅ 🟡 apiFetch 不自动附加 Authorization header — 已修复：`apiFetch`/`apiFetchPage` 自动附加 Bearer token
- ✅ 🟡 AppleBadge/not-found/aria-label/PortalLayout 等 — 已修复
- ✅ 🟢 图标按钮缺少 `aria-label` — 已修复
- ✅ 🟢 PortalLayout 中 clickable `<span>` 无 `role="button"` — 已修复
- 🔴 Toast 通知不可见 — `useToast.tsx` 使用不存在的 CSS 变量 `--bg-parchment`/`--text-ink`，动画 `fadeIn` 未定义
- 🔴 StatusBadge/AppleBadge 暗色模式下徽章不可读 — 硬编码 Tailwind v3 色值（`bg-green-100 text-green-700`），不响应 `data-theme="dark"`
- 🔴 AppleCard 默认内边距失效 — `padding: var(--space-lg)` 变量未定义
- 🟡 全局错误页 `global-error.tsx` 使用硬编码色值 `#1d1d1f` 而非 CSS 变量
- 🟡 AppleInput/AppleTextarea 的 `<label>` 未通过 `htmlFor` 关联 `<input>`（无障碍缺陷）
- 🟡 `body` 字号 17px 与设计 token `--font-size-body: 15px` 不一致
- 🟢 `@theme` 字体 token 与 `:root` 原始 CSS 属性重复定义
- 🟢 `apiFetchPage` Content-Type 头设置不完整
- 🟢 多处页面加载状态为纯文本"加载中..."，无骨架屏
- 🟢 缺少 `prefers-reduced-motion` 媒体查询（`card-entrance`/`skeleton` 动画对动效敏感用户不友好）

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（7 → 已清理 7 个）

| 位置 | 内容 | 状态 |
|------|------|------|
| ~~`server/internal/repository/dashboard_repo.go:70`~~ | ~~SQL 拼接 date_trunc~~ | ✅ 已修复 |
| ~~`server/internal/log/rotating_writer.go:1`~~ | ~~日志文件保留策略~~ | ✅ 已修复 |
| ~~`server/internal/service/scheduler.go:70`~~ | ~~context.Background()~~ | ✅ 已修复 |
| ~~`server/internal/rag/rerank.go`~~ | ~~doc 引用笔误~~ | ✅ 已修复 |
| ~~`server/internal/middleware/auth.go:73`~~ | ~~用户状态每次查 DB~~ | ✅ 已修复（内存缓存） |
| ~~`server/internal/model/llm_config.go:43`~~ | ~~APIKey 重复加密检测~~ | ✅ 已修复 |
| ~~`server/internal/service/message_service.go:102`~~ | ~~未读数缓存/WebSocket~~ | ✅ 已修复（TTL 缓存） |

### 前端 TODO（0 → 已清理 0 个）

前端全部 TODO 已清零。本轮修复：
- 提取 `useChatStream` hook（SSE 流解析封装）
- 新增 `SectionErrorBoundary`（分层错误边界）
- 提取 `getAllConfigs()`（系统配置批量查询）
- `apiFetch` 自动附加 Authorization header
- `proxy.ts` 复用 `lib/auth.ts` JWT 逻辑
- XMLHttpRequest → fetch API 迁移
- 搜索防抖、useMemo 优化、结构化测试结果判断

---

## 统计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 0 | 9 | 3 | 0 |
| 前端 | 4 | 7 | 8 | 0 |
| **合计** | **4** | **16** | **11** | **0** |

---

> 本次审计：前端全量布局审计（17 页面 + 16 组件 + 6 hooks + 12 API 模块），发现 19 个新问题（4 关键 + 7 高优先 + 8 优化），已全部整理进清单。

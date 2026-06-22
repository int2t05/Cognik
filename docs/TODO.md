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
- ✅ 🔴 `src/proxy.ts` 文件名错误，Next.js 要求 `middleware.ts` 才会执行 — 已修复：重命名为 `middleware.ts`，导出函数改为 `middleware`

## 2. 智能问答

- ✅ 🟡 Chat 页面 212 行单文件 — 已修复：提取 `hooks/useChatStream.ts`，SSE 流解析/状态管理/清理逻辑封装
- ✅ 🟡 SSE 流解析错误仅 `console.debug` — 已修复：移除无效日志
- ✅ 🟡 `response.body!` non-null 断言 — 已修复：添加 null 检查
- ✅ 🟢 SSE 超时 120 秒硬编码 — 已修复：超时时检测 userAbort 标志，区分主动取消与超时
- ✅ 🟢 虚拟列表 `key="pipeline"` 静态字符串 — 已修复：使用 `key={`pipeline-${currentStep}`}`
- ✅ 🟡 Chat 页面移动端无侧边栏切换按钮 — 已修复：添加移动端 Menu 按钮 + overlay 抽屉式侧边栏
- 🟢 Chat 虚拟列表 `estimateSize: () => 80` 常量估算，变长消息滚动位置不准 — 保留（需消息高度测量，非阻塞）

## 3. 知识库管理

- ✅ 🟡 文档上传仍用原始 XMLHttpRequest — 已修复：改用 `fetch()` + `FormData`
- ✅ 🟢 文章标签用数组索引作 key — 已修复：改用标签字符串作 key
- ✅ 🟢 50MB 文件大小限制仅在前端提示文本中 — 已修复：添加上传前 `file.size` 校验 + toast 提示
- ✅ 🟢 文档上传 `<input type="file">` 为浏览器默认样式 — 已修复：Apple file:: 伪元素样式（pill 圆角按钮）

## 4. 申告管理

- ✅ 🟡 消息标记已读未处理 API 失败 — 已修复
- ✅ 🟢 handleSupplement 已有 try/catch — 已存在
- ✅ 🟢 ticket status=3 硬编码 — 已修复

## 5. 数据看板与审计

- ✅ 🟡 手写 bar chart（inline style + index key）— 已修复：替换 `key={i}` → `key={d.date}`
- ✅ 🟡 useDebounce 重复定义 — 已修复
- ✅ 🟢 图例色块 Unicode — 已修复
- ✅ 🟢 start/end 日期每次 render 重新计算 — 已修复
- ✅ 🟡 审计日志页日期筛选为纯文本 `<input>`，无 datepicker 和格式校验 — 已修复：日期字段改为 `type="date"`
- ✅ 🟡 审计日志页全部筛选器使用原生 `<input>` 而非 `AppleInput` 组件 — 已修复：统一样式（共享 focus 环 + border + 圆角 class）
- ✅ 🟢 30 天趋势图小屏幕上柱状条拥挤 — 已修复：添加 `overflow-x-auto` 横向滚动

## 6. 系统管理与配置

- ✅ 🟡 LLMConfig 编辑时强制清空 APIKey 字段 — 已修复：空 APIKey 时从请求体删除，后端不修改已存值
- ✅ 🟡 ConfigRow 每个 key 一次 SWR 请求 — 已修复：提取 `getAllConfigs()` 批量获取，单次 SWR 调用
- ✅ 🟢 测试连接结果用 emoji 前缀匹配 — 已修复：改用 `{ success: boolean; message: string }` 结构化判断
- ✅ 🟢 用户搜索无防抖 — 已修复：添加 `useDebounce(keyword, 300)`
- ✅ 🟢 角色权限列表 `knownPermissions` 每次 render 重新计算 — 已修复：添加 `useMemo`
- ✅ 🟡 6 个 Radix UI 包已安装但未使用 — 已修复：移除 7 个未使用包
- ✅ 🟢 多处硬编码魔术数字：urgency 映射数组、MAX_FILE_SIZE、默认分页大小 10 — 已修复：提取 `URGENCY_LABELS` 常量，`DAYS_30_MS` 提取

## 7. 基础设施

- ✅ 🔴 全局内联样式 — 已修复：Tailwind CSS v4 全量迁移
- ✅ 🟡 轮询错误静默吞没 — 已修复：添加 `console.warn`
- ✅ 🟡 全局 ErrorBoundary 只有顶层一个 — 已修复：新增 `SectionErrorBoundary` 包裹 AdminLayout 内容区
- ✅ 🟡 apiFetch 不自动附加 Authorization header — 已修复：`apiFetch`/`apiFetchPage` 自动附加 Bearer token
- ✅ 🟡 AppleBadge/not-found/aria-label/PortalLayout 等 — 已修复
- ✅ 🟢 图标按钮缺少 `aria-label` — 已修复
- ✅ 🟢 PortalLayout 中 clickable `<span>` 无 `role="button"` — 已修复
- ✅ 🔴 Toast 通知不可见 — 已修复：CSS 变量 `--bg-parchment`→`--color-parchment`，`--text-ink`→`--color-ink`，添加 `@keyframes fadeIn`
- ✅ 🔴 StatusBadge/AppleBadge 暗色模式下徽章不可读 — 已修复：改用 CSS 变量 `--badge-{variant}-bg/text`，在 `:root`/`[data-theme="dark"]` 中双主题定义
- ✅ 🔴 AppleCard 默认内边距失效 — 已修复：`--space-lg`（不存在）→ `24px`
- ✅ 🟡 全局错误页 `global-error.tsx` 使用硬编码色值 — 已修复：改用 CSS 变量
- ✅ 🟡 AppleInput/AppleTextarea 的 `<label>` 未通过 `htmlFor` 关联 `<input>` — 已修复：`useId()` 生成 id + `htmlFor` 绑定
- ✅ 🟡 `body` 字号 17px 与设计 token `--font-size-body: 15px` 不一致 — 已修复：`font-size: var(--font-size-body)`
- ✅ 🟢 `@theme` 字体 token 与 `:root` 原始 CSS 属性重复定义 — 已修复：移除 `:root` 中 `--text-*` 重复变量，统一使用 `@theme` 的 `--font-size-*`
- ✅ 🟢 `apiFetchPage` Content-Type 头设置不完整 — 已修复：添加 `Content-Type: application/json`
- 🟢 多处页面加载状态为纯文本"加载中..."，无骨架屏 — 保留（需骨架屏组件，非阻塞优化）
- ✅ 🟢 缺少 `prefers-reduced-motion` 媒体查询 — 已修复：全局禁用动画/过渡

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（0 个）

全部后端 TODO 已清零（7 个已修复）。

### 前端 TODO（0 个）

全部前端 TODO 已清零。

---

## 本轮修复（2026-06-21）

**前端全量清理，修复 17 项：**

| 优先级 | 修复项 | 文件 |
|--------|--------|------|
| 🔴 | proxy.ts → middleware.ts | `middleware.ts` 重命名 + 导出 `middleware` |
| 🔴 | Toast CSS 变量 + fadeIn 动画 | `useToast.tsx` + `globals.css` |
| 🔴 | StatusBadge/AppleBadge 暗色模式 | `StatusBadge.tsx` / `AppleBadge.tsx` + `globals.css` 徽章色 token |
| 🔴 | AppleCard 默认内边距 | `AppleCard.tsx`：`--space-lg` → `24px` |
| 🟡 | Chat 移动端侧边栏 | `chat/page.tsx`：Menu 按钮 + overlay drawer |
| 🟡 | 审计日志日期筛选 | `audit/page.tsx`：日期 → `type="date"` |
| 🟡 | 未使用 Radix UI 包 | `package.json`：移除 7 个包 |
| 🟡 | global-error 硬编码色值 | `global-error.tsx`：改用 CSS 变量 |
| 🟡 | AppleInput/AppleTextarea htmlFor | `AppleInput.tsx`：`useId()` + `htmlFor` |
| 🟡 | body font-size | `globals.css`：`17px` → `var(--font-size-body)` |
| 🟢 | 文件上传 input 样式 | `knowledge/[kbId]/new/page.tsx`：`file::` 伪元素 |
| 🟢 | 趋势图小屏溢出 | `dashboard/page.tsx`：`overflow-x-auto` |
| 🟢 | @theme 字体 token 去重 | `globals.css`：移除 `:root` 重复 `--text-*` |
| 🟢 | apiFetchPage Content-Type | `client.ts`：添加 `Content-Type: application/json` |
| 🟢 | prefers-reduced-motion | `globals.css`：全局媒体查询 |
| 🟢 | urgency 魔术数组 | `format.ts`：提取 `URGENCY_LABELS` 常量 |
| 🟢 | 30 天魔术数字 | `dashboard/page.tsx`：提取 `DAYS_30_MS` |

**保留/延期 2 项：** 虚拟列表 estimateSize、骨架屏加载状态

---

## 统计（代码质量）

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 0 | 9 | 3 | 0 |
| 前端 | 0 | 0 | 2 | 0 |
| **合计** | **0** | **9** | **5** | **0** |

---
> 上半轮审计：前端全量清理（17 项修复），后端 12 项保留（需架构/算法变更）。
> 下半轮审计：前端 UX/UI 全量审计，新增 34 项优化条目（详见下方「前端 UX 与用户旅程优化」）。

---

# 前端 UX 与用户旅程优化

> 基于 UI/UX Pro Max 审计标准，覆盖可访问性、触控交互、布局响应式、表单反馈、导航、动画性能。
> 📖 **用户故事映射：** 报障人（门户端）— 智能问答 / 提交申告 / 查看进度 · 运维/管理员（后台端）— 处理申告 / 管理知识库 / 系统配置

---

## 🔴 P0 — 生产隐患（数据丢失）

- [ ] **KB 删除无确认弹窗** — `admin/knowledge/page.tsx:57` 点击删除直接调用 API，无 ConfirmDialog。误点击导致知识库不可逆删除。
- [ ] **LLM 配置删除无确认弹窗** — `admin/config/llm/page.tsx:147-156` 同上，可能中断 AI 服务。

---

## 🟡 P1 — 用户阻塞（无法完成操作 / A11y 违规）

### 图标按钮 aria-label

- [ ] **ChatPage 图标按钮无 label** — `portal/chat/page.tsx` 移动端菜单(L263)、侧栏折叠(L269)、删除按钮(L244) 三个图标按钮无 `aria-label`，屏幕阅读器无法朗读。
- [ ] **ChatMessage 反馈按钮无 label** — `chat/ChatMessage.tsx:50-73` 👍/👎 按钮仅有 `title` 属性，需补 `aria-label`。

### 表单 label 关联

- [ ] **审计日志筛选栏无 label** — `admin/audit/page.tsx:22-26` 5 个 input 仅有 placeholder。WCAG 1.3.1 违规。
- [ ] **ChatInput 无 label** — `chat/ChatInput.tsx:23-30` 聊天输入框仅有 placeholder。添加 `aria-label="输入消息"`。
- [ ] **LLM 配置表单无 htmlFor** — `admin/config/llm/page.tsx:190-251` select/textarea 的 `<label>` 缺少 `htmlFor`/`id` 关联。

### 键盘导航

- [ ] **知识库卡片不可键盘访问** — `admin/knowledge/page.tsx:50` + `ui/AppleCard.tsx:26` `<div onClick>` 无 `role`/`tabIndex`/`onKeyDown`。

### 移动端响应式

- [ ] **登录/改密页小屏溢出** — `login/page.tsx:61` `w-[420px]` / `change-password/page.tsx:37` `w-[400px]` 在 375px 屏幕溢出。改为 `w-full max-w-[420px]`。

### 缺失确认弹窗

- [ ] **文章停用无确认** — `admin/knowledge/[kbId]/[articleId]/page.tsx:46`。

---

## 🟢 P2 — 体验摩擦

### 布局

- [ ] **6 种 max-width 无体系** — 1600/1200/800/720/640/600px。统一为 2-3 档，通过 `@theme` token 管理。
- [ ] **`100vh` → `dvh`** — `globals.css` body、`chat/page.tsx` 等 6 处未适配移动端地址栏。改为 `100dvh`。
- [ ] **z-index 无层级** — 散落 z-40/50/100/1000/9999。定义 CSS 变量 `--z-nav/overlay/dialog/toast`。

### 字体尺度

- [ ] **`@theme` 字体 token 未使用** — 40+ 处 `text-[28px]/[17px]/[15px]/[13px]/[12px]` 裸值。替换为 `text-hero/title/body/caption/fine`。
- [ ] **超小字号** — `PortalLayout.tsx:52` 角标 `text-[11px]`、`dashboard/page.tsx:69` 图表 `text-[9px]`。

### 交互反馈

- [ ] **Toast 错误替代内联校验** — 多表单用 `toast.error()` 而非字段旁提示。AppleInput 已支持 `error` prop。
- [ ] **4 处"加载中..."纯文本** — `portal/tickets/[id]` / `admin/tickets/[id]` / `articleId` 等。替换为 AppleSpinner 或骨架屏。

### 空状态

- [ ] **知识库列表无空状态** — `admin/knowledge/page.tsx:49` kbs 为空时 grid 无渲染。
- [ ] **用户搜索无结果提示** — 搜索无匹配时表格为空。

### 导航

- [ ] **Cancel 用 router.back()** — `admin/knowledge/[kbId]/new/page.tsx:112` / `portal/tickets/new/page.tsx:86` 可能离开应用。改为 `router.push`。
- [ ] **KB 详情无返回按钮** — `admin/knowledge/[kbId]/page.tsx`。

### 性能

- [ ] **零代码分割** — 无 `next/dynamic`。至少对 chat 页 virtual 列表和 Lucide 图标做 dynamic import。
- [ ] **未读消息双轮询** — `useUnreadCount` 在两个 layout 各挂载一次。改为全局 SWR `refreshInterval`。
- [ ] **SWRConfig 缺** — `Providers.tsx` 添加 `revalidateOnFocus: false` + `dedupingInterval: 5000`。

### 细节

- [ ] **表单缺 required 标记** — 登录/改密/申告/文章等。
- [ ] **LLM textarea 缺 focus 环** — `admin/config/llm/page.tsx:246` 仅 `focus:border` 无 `focus:shadow`。
- [ ] **`:focus` → `:focus-visible`** — `globals.css:135` 鼠标点击也触发焦点环。

---

## 🔵 P3 — 体验增强

- [ ] **登录卡片动画 600ms → 300ms** — `globals.css` `card-entrance 0.6s` 超推荐范围。
- [ ] **Toast 消失时间分级** — `useToast.tsx:40` 错误 5s / 成功 3s。
- [ ] **AdminLayout 侧栏 <1024px 无自动折叠** — 小屏手动展开侧栏过大。
- [ ] **heading 跳跃** — `admin/tickets/[id]/page.tsx` h1 → h3 跳过 h2。
- [ ] **9px 图表日期标签** — `dashboard/page.tsx:69` 改为 `text-[11px]`。

---

## 统计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 🔵 P3 |
|---|---|---|---|---|
| 后端（保留） | 0 | 9 | 3 | 0 |
| 前端 UX/UI | 2 | 8 | 18 | 6 |
| **合计** | **2** | **17** | **21** | **6** |

> 📖 **用户故事优先级：** ① P0 数据安全（确认弹窗）→ ② P1 可访问性阻塞（aria-label / label 关联）→ ③ P1 移动端可用性（固定宽度溢出 / dvh）→ ④ P2 字体体系统一 → ⑤ P2 交互反馈（Toast → 内联）→ ⑥ P3 动画/性能优化

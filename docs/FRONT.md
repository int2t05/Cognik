# OpsMind 前端架构文档 (FRONT.md)

> **目标读者：** Go + Next.js 全栈开发者
> **最后更新：** 2026-06-22
> **对应分支：** main @ eeac183

---

## 1. 目录结构 → 对应访问路由

### 1.1 文件树与路由映射表

```
web/src/app/                               →  Next.js App Router 路由根
├── layout.tsx                             →  根布局（注入主题/字体/全局 Providers）
├── globals.css                            →  全局样式 + Apple Design 双主题 CSS 变量
├── page.tsx                               →  "/"             → redirect('/portal/chat')
├── not-found.tsx                          →  404 页面        → 所有未匹配路径
├── global-error.tsx                       →  全局错误边界     → 未捕获异常时
│
├── login/
│   ├── layout.tsx                         →  (透传 children，无额外布局)
│   └── page.tsx                           →  "/login"        → 登录页
│
├── change-password/
│   ├── layout.tsx                         →  (透传 children)
│   └── page.tsx                           →  "/change-password" → 修改密码页
│
├── portal/                                →  门户端（报障人/普通用户）
│   ├── layout.tsx                         →  门户端布局壳（顶栏导航 + 内容区）
│   ├── loading.tsx                        →  骨架屏（skeleton）
│   ├── chat/
│   │   └── page.tsx                       →  "/portal/chat"       → 智能问答（SSE 流式）
│   ├── messages/
│   │   └── page.tsx                       →  "/portal/messages"   → 站内消息
│   ├── tickets/
│   │   ├── page.tsx                       →  "/portal/tickets"    → 我的申告列表
│   │   ├── new/
│   │   │   └── page.tsx                   →  "/portal/tickets/new" → 提交申告表单
│   │   └── [id]/
│   │       └── page.tsx                   →  "/portal/tickets/:id" → 申告详情 + 补充信息
│
└── admin/                                 →  后台管理端（管理员/运维人员）
    ├── layout.tsx                         →  后台布局壳（侧栏菜单 + 顶栏 + 内容区）
    ├── loading.tsx                        →  骨架屏
    ├── dashboard/
    │   └── page.tsx                       →  "/admin/dashboard"    → 数据看板
    ├── tickets/
    │   ├── page.tsx                       →  "/admin/tickets"      → 申告管理列表
    │   └── [id]/
    │       └── page.tsx                   →  "/admin/tickets/:id"  → 申告处理详情
    ├── knowledge/
    │   ├── page.tsx                       →  "/admin/knowledge"    → 知识库列表
    │   └── [kbId]/
    │       ├── page.tsx                   →  "/admin/knowledge/:kbId"          → 文章列表
    │       ├── new/
    │       │   └── page.tsx               →  "/admin/knowledge/:kbId/new"      → 新建文章/上传文档
    │       └── [articleId]/
    │           └── page.tsx               →  "/admin/knowledge/:kbId/:articleId" → 文章编辑/审核
    ├── users/
    │   └── page.tsx                       →  "/admin/users"        → 用户管理
    ├── roles/
    │   └── page.tsx                       →  "/admin/roles"        → 角色与权限管理
    ├── audit/
    │   └── page.tsx                       →  "/admin/audit"        → 审计日志
    └── config/
        ├── llm/
        │   └── page.tsx                   →  "/admin/config/llm"   → LLM 配置
        └── system/
            └── page.tsx                   →  "/admin/config/system" → 系统配置
```

### 1.2 路由分组说明

| 路由前缀 | 目标用户 | 布局组件 | 权限控制 |
|---------|---------|---------|---------|
| `/login` | 所有用户 | `LoginLayout` (透传) | 公开（`PUBLIC_PATHS`） |
| `/change-password` | 已登录用户 | `ChangePasswordLayout` (透传) | 需 JWT |
| `/portal/*` | 报障人角色 | `PortalLayout` (`components/layout/PortalLayout.tsx`) | JWT 认证 |
| `/admin/*` | 管理员角色 | `AdminLayout` (`components/layout/AdminLayout.tsx`) | JWT + RBAC（`isAdminRole()`） |
| `/` | — | — | `redirect('/portal/chat')` |

---

## 2. Server / Client 组件划分

### 2.1 划分原则

- **Server Component（默认）**：无 `'use client'` 指令，在服务端渲染，无法使用 hooks/浏览器 API
- **Client Component**：有 `'use client'` 指令，在浏览器端水合（hydrate），可使用 hooks/state/effects

### 2.2 组件分类清单

#### Server Components（无 `'use client'`，服务端渲染）

| 文件 | 组件名 | 说明 |
|------|--------|------|
| `src/app/layout.tsx` | `RootLayout` | 根布局 — 注入 `<html>` + `<body>` + 全局 Providers + FOUC 消除脚本 |
| `src/app/page.tsx` | `Home` | 首页 — 执行 `redirect('/portal/chat')` |
| `src/app/not-found.tsx` | `NotFound` | 404 页面 |
| `src/app/login/layout.tsx` | `LoginLayout` | 登录页布局壳 |
| `src/app/change-password/layout.tsx` | `ChangePasswordLayout` | 修改密码布局壳 |
| `src/app/admin/loading.tsx` | `AdminLoading` | 后台骨架屏 |
| `src/app/portal/loading.tsx` | `PortalLoading` | 门户骨架屏 |

#### Client Components（`'use client'`，浏览器端水合）

| 文件 | 组件名 | 关键 Hooks/依赖 |
|------|--------|----------------|
| `src/app/global-error.tsx` | `GlobalError` | `AppleButton` |
| `src/app/admin/layout.tsx` | `AdminLayout` (壳) | 委托给 `AdminLayoutUI` |
| `src/app/portal/layout.tsx` | `PortalLayout` (壳) | 委托给 `PortalLayoutUI` |
| `src/app/login/page.tsx` | `LoginPage` | `useAuth().login()`, `useToast()`, `isAdminRole()`, `apiFetch()` |
| `src/app/change-password/page.tsx` | `ChangePasswordPage` | `changePassword()`, `useToast()` |
| `src/app/portal/chat/page.tsx` | `ChatPage` | `useChatStream()`, `useVirtualizer()`, `useSWR()`, SSE 流式解析 |
| `src/app/portal/messages/page.tsx` | `MessagesPage` | `useSWR()`, `markAsRead()` |
| `src/app/portal/tickets/page.tsx` | `TicketQueryPage` | `useSWR()`, `getMyTickets()` |
| `src/app/portal/tickets/[id]/page.tsx` | `TicketDetailPage` | `useSWR()`, `supplementTicket()` |
| `src/app/portal/tickets/new/page.tsx` | `TicketSubmitPage` | `createTicket()`, `useSearchParams()` |
| `src/app/admin/dashboard/page.tsx` | `DashboardPage` | `useSWR()`, `getStats()`, `getTrends()`, `TrendChart` |
| `src/app/admin/tickets/page.tsx` | `AdminTicketListPage` | `useSWR()`, `listAllTickets()` |
| `src/app/admin/tickets/[id]/page.tsx` | `AdminTicketDetailPage` | `useSWR()`, `updateTicketStatus()`, `createKnowledgeCandidate()` |
| `src/app/admin/knowledge/page.tsx` | `KnowledgeListPage` | `useSWR()`, `getKBList()`, `createKB()`, `updateKB()`, `deleteKB()` |
| `src/app/admin/knowledge/[kbId]/page.tsx` | `ArticleListPage` | `useSWR()`, `getArticleList()` |
| `src/app/admin/knowledge/[kbId]/[articleId]/page.tsx` | `ArticleEditPage` | `useSWR()`, `getArticle()`, `updateArticle()`, `reviewArticle()`, `publishArticle()` |
| `src/app/admin/knowledge/[kbId]/new/page.tsx` | `NewArticlePage` | `createArticle()`, 原生 `fetch` 上传文档 |
| `src/app/admin/users/page.tsx` | `UserListPage` | `useSWR()`, `getUserList()`, `createUser()`, `updateUser()`, `freezeUser()` |
| `src/app/admin/roles/page.tsx` | `RoleManagePage` | `useSWR()`, `getRoleList()`, `createRole()`, `updateRole()`, `deleteRole()`, `getMenus()` |
| `src/app/admin/audit/page.tsx` | `AuditLogPage` | `useSWR()`, `getAuditLogs()`, `useDebounce()` |
| `src/app/admin/config/llm/page.tsx` | `LLMConfigPage` | `useSWR()`, `getLLMConfigs()`, `createLLMConfig()`, `updateLLMConfig()`, `testLLMConnection()` |
| `src/app/admin/config/system/page.tsx` | `SystemConfigPage` | `useSWR()`, `getAllConfigs()`, `setConfig()`, 内嵌 `ConfigRow` |

#### 公共 Client Components（`components/` 目录）

| 文件 | 组件名 | 关键实现 |
|------|--------|---------|
| `components/Providers.tsx` | `Providers` | 组合 `SWRConfig` → `AuthProvider` → `ThemeProvider` → `ToastProvider` → `ErrorBoundary` |
| `components/ThemeProvider.tsx` | `ThemeProvider` | `useTheme().theme` → `document.documentElement.setAttribute('data-theme', theme)` |
| `components/ErrorBoundary.tsx` | `ErrorBoundary` / `SectionErrorBoundary` / `ErrorFallback` | React Class Component 错误边界 |
| `components/layout/AdminLayout.tsx` | `AdminLayout` | 侧栏菜单树 + 顶栏 + 主题切换 + 折叠/展开 + `FRONTEND_ROUTES` 路径映射 |
| `components/layout/PortalLayout.tsx` | `PortalLayout` | 顶栏导航 + 主题切换 + 后台管理入口 |
| `components/chat/ChatInput.tsx` | `ChatInput` | `forwardRef` + Enter 发送 |
| `components/chat/ChatMessage.tsx` | `ChatMessage` | 用户/AI 气泡 + 来源引用 + 置信度警告 + 反馈按钮 |
| `components/chat/ChatPipeline.tsx` | `ChatPipeline` | RAG 管道步骤可视化 |
| `components/ui/AppleDialog.tsx` | `AppleDialog` | Radix Dialog 封装 |
| `components/ui/AppleTable.tsx` | `AppleTable` | 泛型表格 + loading/empty 状态 |
| `components/ui/ApplePagination.tsx` | `ApplePagination` | 分页 + pageSize 选择器 |
| `components/shared/ConfirmDialog.tsx` | `ConfirmDialog` | 危险操作二次确认 |
| `components/shared/StatusBadge.tsx` | `StatusBadge` | 领域状态码 → 语义标签映射 |
| `components/shared/StatCard.tsx` | `StatCard` | 看板统计卡片 |

#### Server Components（`components/` 目录，无 `'use client'`）

| 文件 | 组件名 | 说明 |
|------|--------|------|
| `components/ui/AppleButton.tsx` | `AppleButton` | `forwardRef`，四种变体（pill/ghost/utility/pearl），纯渲染无副作用 |
| `components/ui/AppleCard.tsx` | `AppleCard` | 白底卡片 + 键盘可访问点击 |
| `components/ui/AppleInput.tsx` | `AppleInput` / `AppleTextarea` | `forwardRef` + `useId()`（React 19 `useId` 可在 RSC 中使用） |
| `components/ui/AppleBadge.tsx` | `AppleBadge` | CSS 变量语义色标签 |
| `components/ui/AppleSpinner.tsx` | `AppleSpinner` | 旋转 loading 指示器 |

> **注意：** `AppleInput` 和 `AppleDialog` 标记了 `'use client'` 是因为它们使用了 `useId()`（涉及客户端水合一致性）和 Radix UI（需要 Portal/事件）。

---

## 3. API 接口数据流向

### 3.1 整体数据流架构

```
┌──────────────────────────────────────────────────────────┐
│  浏览器                                                    │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │ Page 组件    │→│ lib/api/*.ts │→│ lib/api/client.ts │  │
│  │ (useSWR /   │  │ (业务 API)   │  │ (rawApiRequest) │  │
│  │  直接调用)   │  │              │  │                 │  │
│  └─────────────┘  └──────────────┘  └───────┬─────────┘  │
│                                             │            │
│                                    fetch() + Authorization│
│                                             │            │
└─────────────────────────────────────────────┼────────────┘
                                              │
                              开发: localhost:8080 (直连)
                              生产: /api/* → NEXT_PUBLIC_API_URL (rewrite)
                                              │
┌─────────────────────────────────────────────┼────────────┐
│  Next.js Server (middleware.ts)             │            │
│  ┌──────────────────────────────────────────┘            │
│  │  middleware() 拦截所有非 /api 路由:                     │
│  │  1. decodeJwtPayload() 解析 token                       │
│  │  2. isTokenExpired() 检查过期 → refreshAccessToken()   │
│  │  3. isAdminRole() RBAC 校验 → /admin/*                │
│  │  4. 非公开路径 + 无 token → redirect /login             │
│  └───────────────────────────────────────────────────────│
│                                                          │
│  next.config.ts rewrites():                              │
│  /api/:path* → ${NEXT_PUBLIC_API_URL}/api/:path*          │
│  (默认 http://localhost:8080)                             │
└──────────────────────────────────────────────────────────┘
                                              │
┌─────────────────────────────────────────────┼────────────┐
│  Go 后端 (Gin server :8080)                 ▼            │
│  Handler → Service → Repository → PostgreSQL/pgvector    │
└──────────────────────────────────────────────────────────┘
```

### 3.2 认证流程详解

```
LoginPage.handleSubmit()
  │
  ├─ 1. apiFetch<LoginResponse>('/api/v1/auth/login', { POST, body })
  │     └─ rawApiRequest() → fetch() → 解析 JSON
  │        └─ token 通过 _tokenGetter() 获取（登录时无需 token）
  │        └─ 响应: { code, message, data: { access_token, refresh_token, user, roles, permissions, menus } }
  │
  ├─ 2. useAuth().login(access_token, refresh_token, user, roles, permissions, menus)
  │     └─ AuthProvider.login() → setState() + persistAuth(localStorage)
  │     └─ 副作用: useEffect 写 cookie (access_token + refresh_token, path=/, SameSite=Lax, max-age=604800)
  │     └─ 副作用: useLayoutEffect 调用 setTokenGetter() 更新 apiFetch 的 token 获取器
  │
  ├─ 3. isAdminRole(roles) 判断角色
  │     └─ roles.some(r => ['系统管理员','admin','operator','knowledge_manager'].includes(r))
  │
  └─ 4. router.push(isAdmin ? '/admin/dashboard' : '/portal/chat')
```

### 3.3 API 客户端调用链（以用户列表为例）

```
UserListPage (组件)
  │
  ├─ useSWR(`users-${page}-${debouncedKeyword}`, () => getUserList(page, debouncedKeyword))
  │     │
  │     └─ lib/api/user.ts: getUserList(page, keyword)
  │           │
  │           └─ lib/api/client.ts: apiFetchPage<User>(`/api/v1/admin/users?page=...`)
  │                 │
  │                 └─ rawApiRequest(url)
  │                       │
  │                       ├─ 1. 构造 Headers: { Content-Type: application/json, Authorization: Bearer <token> }
  │                       │     └─ token ← _tokenGetter() → AuthProvider state.token
  │                       │
  │                       ├─ 2. fetch(`${BASE_URL}${url}`, { headers })
  │                       │     └─ BASE_URL = NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  │                       │     └─ 开发模式: 直连 localhost:8080（绕过 Next.js rewrite，避免 Turbopack POST 代理 500）
  │                       │     └─ 生产模式: 通过 rewrite 代理
  │                       │
  │                       ├─ 3. safeResJson(res) → 解析 JSON
  │                       │     └─ 空响应 → ApiError("后端服务不可达")
  │                       │     └─ 非 JSON → ApiError("服务器返回非 JSON 响应")
  │                       │
  │                       ├─ 4. 检查 json.code === 0
  │                       │     └─ code !== 0 → throw ApiError(code, message)
  │                       │
  │                       └─ 5. apiFetchPage 解包: { items: data, total, page, pageSize }
  │
  └─ AppleTable<User> 渲染
        └─ columns + data + loading + rowKey
        └─ ApplePagination 分页
```

### 3.4 SSE 流式问答调用链（chat → RAG）

```
ChatPage.handleSend()
  │
  ├─ 1. isTokenExpired(token) 检查
  │
  ├─ 2. useChatStream.send(question, kbId, sessionId)
  │     │
  │     ├─ 2a. 若无 sessionId → createSession(kbId, title) → POST /api/v1/portal/chat-sessions → { session_id }
  │     │
  │     ├─ 2b. fetch(`${NEXT_PUBLIC_API_URL}/api/v1/portal/chat-sessions/${sid}/stream`, { POST, body: { question } })
  │     │     └─ 绕过 Next.js rewrite，直连后端（SSE 需要原始流）
  │     │     └─ signal: AbortSignal.any([controller.signal, AbortSignal.timeout(120_000)])
  │     │
  │     ├─ 2c. ReadableStream 解析循环:
  │     │     └─ reader.read() → decoder.decode() → buffer 分割 '\n'
  │     │     └─ line.startsWith('data: ') → JSON.parse(line.slice(6))
  │     │     └─ 事件类型分发:
  │     │         ├─ type: 'step'    → setCurrentStep(label) + setPipelineSteps([...prev, {id, label}])
  │     │         ├─ type: 'token'   → 累积 assistantContent + setMessages 增量更新
  │     │         ├─ type: 'done'    → setStreaming(false) + 填充 sources/confidence/pipeline
  │     │         └─ type: 'error'   → setStreaming(false) + onError(msg)
  │     │
  │     └─ 2d. Catch 分支:
  │           ├─ userAborted → 静默返回 null
  │           ├─ TimeoutError → onError("请求超时...")
  │           └─ 其他 Error → onError(err.message)
  │
  └─ 3. 消息列表通过 @tanstack/react-virtual useVirtualizer() 虚拟滚动渲染
```

### 3.5 状态管理架构

```
┌─────────────────────────────────────────────┐
│  AuthProvider (React Context)               │
│  ├─ token / refreshToken / user / roles     │
│  ├─ permissions / menus / isLoggedIn        │
│  ├─ login() / logout() / hasPermission()    │
│  └─ 持久化: localStorage('auth') + cookie   │
│     同步: setTokenGetter() → apiFetch       │
├─────────────────────────────────────────────┤
│  ToastProvider (React Context)              │
│  ├─ toasts: Toast[]                         │
│  └─ success() / error() / warning() / info()│
├─────────────────────────────────────────────┤
│  ThemeProvider + useTheme()                 │
│  ├─ theme: 'light' | 'dark'                │
│  ├─ toggleTheme() / setTheme()             │
│  └─ 持久化: cookie + localStorage +         │
│     data-theme 属性                          │
├─────────────────────────────────────────────┤
│  SWR (数据获取 + 缓存)                       │
│  ├─ revalidateOnFocus: false               │
│  ├─ dedupingInterval: 5000ms               │
│  └─ 各页面 useSWR(key, fetcher)             │
├─────────────────────────────────────────────┤
│  useChatStream (Chat Page 专用)             │
│  ├─ messages / streaming / loading          │
│  ├─ pipelineSteps / currentStep             │
│  └─ send() / abort() / clear() /           │
│     loadMessages()                           │
└─────────────────────────────────────────────┘
```

### 3.6 各业务 API 模块调用速查

| API 模块文件 | 导出函数 | 调用的 client 函数 | 后端端点 |
|-------------|---------|-------------------|---------|
| `lib/api/auth.ts` | `login()` | `apiFetch` | `POST /api/v1/auth/login` |
| | `refreshToken()` | `apiFetch` | `POST /api/v1/auth/refresh` |
| | `changePassword()` | `apiFetch` | `POST /api/v1/auth/me/change-password` |
| | `logout()` | `apiFetch` | `POST /api/v1/auth/me/logout` |
| `lib/api/chat.ts` | `createSession()` | `apiFetch` | `POST /api/v1/portal/chat-sessions` |
| | `getSessionList()` | `apiFetchPage` | `GET /api/v1/portal/chat-sessions` |
| | `getChatDetail()` | `apiFetch` | `GET /api/v1/portal/chat-sessions/:id` |
| | `deleteSession()` | `apiFetch` | `DELETE /api/v1/portal/chat-sessions/:id` |
| | `submitFeedback()` | `apiFetch` | `POST /api/v1/portal/chat-sessions/:id/feedback` |
| `lib/api/knowledge.ts` | `getKBList()` | `apiFetch` | `GET /api/v1/admin/knowledge-bases` |
| | `getPortalKBList()` | `apiFetch` | `GET /api/v1/portal/knowledge-bases` |
| | `createKB()` / `updateKB()` / `deleteKB()` | `apiFetch` | `POST/PUT/DELETE /api/v1/admin/knowledge-bases` |
| | `getArticleList()` | `apiFetchPage` | `GET /api/v1/admin/knowledge-bases/:id/articles` |
| | `getArticle()` / `createArticle()` / `updateArticle()` | `apiFetch` | `GET/POST/PUT /api/v1/admin/articles/:id` |
| | `submitReview()` / `reviewArticle()` / `publishArticle()` | `apiFetch` | `POST /api/v1/admin/articles/:id/(submit-review\|review\|publish)` |
| | `disableArticle()` / `enableArticle()` | `apiFetch` | `POST /api/v1/admin/articles/:id/(disable\|enable)` |
| | `uploadDocuments()` | `apiFetch` | `POST /api/v1/admin/knowledge-bases/:id/documents/upload` |
| `lib/api/ticket.ts` | `createTicket()` | `apiFetch` | `POST /api/v1/portal/tickets` |
| | `getMyTickets()` | `apiFetchPage` | `GET /api/v1/portal/tickets` |
| | `getTicketDetail()` | `apiFetch` | `GET /api/v1/portal/tickets/:id` |
| | `supplementTicket()` | `apiFetch` | `PATCH /api/v1/portal/tickets/:id/supplement` |
| | `listAllTickets()` | `apiFetchPage` | `GET /api/v1/admin/tickets` |
| | `getAdminTicketDetail()` | `apiFetch` | `GET /api/v1/admin/tickets/:id` |
| | `updateTicketStatus()` | `apiFetch` | `PATCH /api/v1/admin/tickets/:id/status` |
| | `addTicketRecord()` | `apiFetch` | `POST /api/v1/admin/tickets/:id/records` |
| | `createKnowledgeCandidate()` | `apiFetch` | `POST /api/v1/admin/tickets/:id/knowledge-candidate` |
| `lib/api/user.ts` | `getUserList()` | `apiFetchPage` | `GET /api/v1/admin/users` |
| | `createUser()` / `updateUser()` | `apiFetch` | `POST/PUT /api/v1/admin/users` |
| | `freezeUser()` / `unfreezeUser()` | `apiFetch` | `PATCH /api/v1/admin/users/:id/(freeze\|unfreeze)` |
| | `getUserDetail()` | `apiFetch` | `GET /api/v1/admin/users/:id` |
| `lib/api/role.ts` | `getRoleList()` | `apiFetchPage` | `GET /api/v1/admin/roles` |
| | `createRole()` / `updateRole()` / `deleteRole()` | `apiFetch` | `POST/PUT/DELETE /api/v1/admin/roles` |
| | `getRoleDetail()` | `apiFetch` | `GET /api/v1/admin/roles/:id` |
| | `updateRoleMenus()` | `apiFetch` | `PUT /api/v1/admin/roles/:id/menus` |
| | `getMenus()` | `apiFetch` | `GET /api/v1/admin/menus` |
| `lib/api/audit.ts` | `getAuditLogs()` | `apiFetchPage` | `GET /api/v1/admin/audit-logs` |
| `lib/api/config.ts` | `getConfig()` / `setConfig()` | `apiFetch` | `GET/PUT /api/v1/admin/configs/:key` |
| | `getAllConfigs()` | `getConfig` × N (并行) | 多个 `GET /api/v1/admin/configs/:key` |
| `lib/api/dashboard.ts` | `getStats()` | `apiFetch` | `GET /api/v1/admin/dashboard/stats` |
| | `getTrends()` | `apiFetch` | `GET /api/v1/admin/dashboard/trends` |
| `lib/api/llm_config.ts` | `getLLMConfigs()` / `getLLMConfigDetail()` | `apiFetch` | `GET /api/v1/admin/llm-configs` |
| | `createLLMConfig()` / `updateLLMConfig()` / `deleteLLMConfig()` | `apiFetch` | `POST/PUT/DELETE /api/v1/admin/llm-configs` |
| | `testLLMConnection()` | `apiFetch` | `POST /api/v1/admin/llm-configs/:id/test` |
| `lib/api/message.ts` | `getMessages()` | `apiFetchPage` | `GET /api/v1/portal/messages` |
| | `markAsRead()` | `apiFetch` | `PUT /api/v1/portal/messages/:id/read` |
| | `getUnreadCount()` | `apiFetch` | `GET /api/v1/portal/messages/unread-count` |

### 3.7 API 客户端核心实现

**`lib/api/client.ts`** 是整个前端 API 层的唯一入口，核心函数调用链：

```
apiFetch<T>(url, options?)           apiFetchPage<T>(url)
  │                                    │
  └─ rawApiRequest(url, options)  ←────┘
       │
       ├─ 1. 构造 Headers:
       │     └─ 'Content-Type': 'application/json'
       │     └─ 'Authorization': `Bearer ${_tokenGetter()}`
       │          └─ _tokenGetter 默认从 localStorage('auth') 读取
       │          └─ AuthProvider.useLayoutEffect 调用 setTokenGetter() 覆盖
       │
       ├─ 2. fetch(`${BASE_URL}${url}`, { ...options, headers })
       │     └─ BASE_URL = NEXT_PUBLIC_API_URL || 'http://localhost:8080'
       │     └─ TypeError('Failed to fetch') → "后端服务不可达"
       │
       ├─ 3. safeResJson(res):
       │     └─ res.text() → 空? ApiError("服务器返回空响应")
       │     └─ JSON.parse(text) → 失败? ApiError("服务器返回非 JSON 响应")
       │
       ├─ 4. 检查 json.code !== 0 → throw ApiError(code, message)
       │
       ├─ 5. apiFetch: return json.data as T
       └─ 5. apiFetchPage: return { items: data, total, page, pageSize }
```

**`setTokenGetter()` 竞态修复说明：**
- 模块初始化时设置默认 `_tokenGetter`，直接从 `localStorage('auth')` 读取
- `AuthProvider` 挂载后在 `useLayoutEffect` 中调用 `setTokenGetter()` 覆盖
- 即使 `AuthProvider` 尚未挂载（HMR 模块重置），默认 getter 也能工作，避免 token 丢失

---

## 4. 全局布局 & 公共组件复用

### 4.1 布局嵌套层次

```
RootLayout (src/app/layout.tsx)                   ← Server Component
  ├─ <html lang="zh-CN">
  ├─ <head>: Inter 字体 preconnect + CSS
  ├─ <Script id="theme-fouc">: FOUC 消除脚本（beforeInteractive）
  └─ <body>
       └─ <Providers>                              ← Client Component (src/components/Providers.tsx)
            └─ <SWRConfig>                         ← revalidateOnFocus: false, dedupingInterval: 5000
                 └─ <AuthProvider>                  ← 全局认证状态 (src/hooks/useAuth.tsx)
                      └─ <ThemeProvider>            ← data-theme 注入 (src/components/ThemeProvider.tsx)
                           └─ <ToastProvider>       ← 全局 Toast 通知 (src/hooks/useToast.tsx)
                                └─ <ErrorBoundary>  ← 全局错误边界 (src/components/ErrorBoundary.tsx)
                                     └─ {children}  ← 页面内容
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
              LoginLayout          PortalLayout            AdminLayout
              (透传)               (PortalLayoutUI)       (AdminLayoutUI)
                    │                     │                     │
              LoginPage            Portal Pages           Admin Pages
```

### 4.2 AdminLayout 详解 (`components/layout/AdminLayout.tsx`)

**核心函数和状态：**

| 函数/变量 | 作用 |
|----------|------|
| `menuTree` (useMemo) | 将扁平的 `menus[]` 按 `parent_id` 构建树形结构 |
| `renderMenuItem(m, depth)` | 递归渲染菜单项，支持展开/折叠子菜单 |
| `collapsed` (useState) | 侧栏折叠状态，持久化到 `localStorage('sidebar-collapsed')` |
| `expandedMenus` (useState) | 当前展开的子菜单 ID 集合 |
| `toggleSubmenu(id)` | 切换子菜单展开/折叠 |
| `isActivePath(menuPath, pathname)` | 高亮当前激活菜单项 |
| `sidebarWidth` | `collapsed ? 64 : 220` px |

**关键映射：**

```
ICON_MAP (后端 icon 字段 → Lucide React 图标):
  'dashboard'   → <LayoutDashboard>
  'ticket'      → <Ticket>
  'knowledge'   → <BookOpen>
  'book'        → <BookOpen>
  'users'/'user'→ <Users> / <User>
  'role'/'shield'→ <Shield>
  'config'/'settings' → <Settings>
  'audit'/'file-text' → <ScrollText> / <FileText>
  'message'     → <MessageSquare>
  'cpu'         → <Cpu>

FRONTEND_ROUTES (后端路径 → 前端路由):
  '/admin/audit-logs'     → '/admin/audit'
  '/admin/model-config'   → '/admin/config/llm'
  '/admin/llm-config'     → '/admin/config/llm'
  '/admin/system-config'  → '/admin/config/system'
```

### 4.3 PortalLayout 详解 (`components/layout/PortalLayout.tsx`)

**固定导航项 (`NAV_ITEMS`)：**

| 路径 | 标签 | 图标 | 说明 |
|------|------|------|------|
| `/portal/chat` | 智能问答 | `<Bot>` | 入口页，SSE 流式对话 |
| `/portal/tickets/new` | 提交申告 | `<TicketPlus>` | 申告表单 |
| `/portal/tickets` | 我的申告 | `<ListTodo>` | 申告列表 |
| `/portal/messages` | 消息 | `<MessageSquare>` | 站内消息 + 未读 badge |

**条件渲染：**
- `isAdmin = menus.length > 0` → 显示"后台管理"按钮（`<Shield>` 图标）
- `unreadCount > 0` → 消息导航项显示红色未读 badge（最多显示 "99"）

### 4.4 公共组件复用矩阵

| 组件 | 使用场景 | 被以下页面/组件引用 |
|------|---------|-------------------|
| **`AppleButton`** | 所有按钮（4 变体: pill/ghost/utility/pearl + loading 状态） | 全部页面 + 全部 layout + ConfirmDialog + ErrorBoundary |
| **`AppleInput` / `AppleTextarea`** | 表单输入（pill 搜索框 / 标准输入 / textarea） | login, change-password, users, roles, knowledge, tickets, llm-config, system-config |
| **`AppleCard`** | 内容卡片（白底 + 圆角 + hairline 边框 + 可点击） | dashboard, knowledge, tickets, roles, llm-config, system-config |
| **`AppleTable`** | 数据表格（泛型 `<T>` + loading/empty 状态） | users, roles, audit, knowledge, tickets, messages |
| **`ApplePagination`** | 分页控件 | users, roles, audit, knowledge, tickets, messages |
| **`AppleDialog`** | 模态对话框（Radix UI 封装） | users, roles, knowledge, llm-config |
| **`AppleBadge`** | 语义标签（5 变体: success/warning/error/info/neutral） | StatusBadge (内部使用) |
| **`AppleSpinner`** | Loading 指示器 | AppleTable (loading 态), ChatMessage (流式等待) |
| **`ConfirmDialog`** | 危险操作二次确认 (AppleDialog + AppleButton) | users(freeze), knowledge(delete KB/article), roles(delete), chat(delete session), llm-config(delete) |
| **`StatusBadge`** | 领域状态标签（ticket/user/article/process × 语义色） | tickets, knowledge, users |
| **`StatCard`** | 看板统计卡片 | dashboard |
| **`ErrorBoundary`** | 全局错误边界 (Class Component) | Providers (根级别) |
| **`SectionErrorBoundary`** | 局部错误边界 (Class Component) | AdminLayout 的 `<main>` 内容区 |
| **`ChatInput`** | 问答输入框（forwardRef + Enter 发送） | portal/chat |
| **`ChatMessage`** | 对话气泡（用户/AI 双样式 + 来源引用 + 反馈按钮） | portal/chat |
| **`ChatPipeline`** | RAG 管道步骤可视化 | portal/chat |

### 4.5 自定义 Hooks 复用

| Hook 文件 | 导出函数 | 使用场景 | 内部依赖 |
|----------|---------|---------|---------|
| `hooks/useAuth.tsx` | `AuthProvider` / `useAuth()` | 全局认证状态（token/user/roles/permissions/menus） | React Context + localStorage + cookie |
| `hooks/useTheme.ts` | `useTheme()` | 双主题切换（light/dark） | localStorage + cookie + `data-theme` 属性 + `matchMedia` |
| `hooks/useToast.tsx` | `ToastProvider` / `useToast()` | 全局 Toast 通知（最多 3 条堆叠，分级消失时间） | React Context + setTimeout |
| `hooks/useChatStream.ts` | `useChatStream()` | SSE 流式问答的状态管理 | `createSession()` + fetch + ReadableStream + AbortController |
| `hooks/useDebounce.ts` | `useDebounce<T>()` | 搜索/筛选防抖（300ms 默认） | useState + useEffect + setTimeout |
| `hooks/useUnreadCount.ts` | `useUnreadCount()` | 消息未读数轮询（30s 间隔） | `useSWR('unread-count', getUnreadCount, { refreshInterval: 30000 })` |

### 4.6 工具函数复用

| 文件 | 导出函数 | 说明 |
|------|---------|------|
| `lib/auth.ts` | `decodeJwtPayload(token)` | base64url → JSON 解码 JWT payload（不验证签名） |
| | `isTokenExpired(token)` | 检查 exp + 60s 缓冲 |
| `lib/date.ts` | `formatDate(dateStr)` | ISO → `zh-CN` 日期时间格式化 |
| | `formatDateOnly(dateStr)` | ISO → `zh-CN` 仅日期格式化 |
| `lib/format.ts` | `formatPercent(value)` | null-safe 百分比格式化 |
| | `truncate(text, maxLen)` | 文本截断 + "…" |
| | `URGENCY_LABELS` | `['', '低', '中', '高']` 索引常量 |
| `lib/id.ts` | `generateId()` | `nanoid()` URL-safe 随机 ID |
| `lib/menu.ts` | `isActivePath(menuPath, currentPath)` | 菜单路径匹配（含子路由） |
| `lib/roles.ts` | `isAdminRole(roles)` / `ADMIN_ROLES` | 管理员角色判断（`['系统管理员','admin','operator','knowledge_manager']`） |

---

## 5. 启动命令与依赖配置

### 5.1 核心依赖

```json
{
  "next": "16.2.9",           // React 框架（App Router）
  "react": "19.2.4",          // UI 库
  "react-dom": "19.2.4",      // React DOM 渲染
  "swr": "^2.4.1",            // 数据获取 + 缓存（stale-while-revalidate）
  "tailwindcss": "^4.3.1",    // CSS 工具框架（v4，通过 @tailwindcss/postcss）
  "@radix-ui/react-dialog": "^1.1.17",  // 无障碍对话框
  "@tanstack/react-virtual": "^3.14.3", // 虚拟滚动（聊天消息列表）
  "lucide-react": "^1.20.0",  // 图标库
  "nanoid": "^5.1.11"         // 唯一 ID 生成
}
```

### 5.2 开发依赖

```json
{
  "typescript": "^5",                    // 类型系统
  "@types/react": "^19",                 // React 类型
  "@types/node": "^20",                 // Node.js 类型
  "eslint": "^9",                        // 代码检查
  "eslint-config-next": "16.2.9",       // Next.js ESLint 规则
  "@playwright/test": "^1.61.0",        // E2E 测试框架
  "playwright": "^1.61.0",              // 浏览器自动化
  "vitest": "^4.1.9",                   // 单元测试框架
  "@vitejs/plugin-react": "^6.0.2",    // Vitest React 插件
  "@testing-library/react": "^16.3.2", // React 组件测试
  "@testing-library/jest-dom": "^6.9.1", // DOM 断言扩展
  "@testing-library/user-event": "^14.6.1", // 用户交互模拟
  "jsdom": "^29.1.1"                    // 浏览器环境模拟
}
```

### 5.3 启动命令

```bash
# 进入前端目录
cd web

# ===== 日常开发 =====

# 安装依赖
npm install

# 启动开发服务器（端口 3000，热更新）
npm run dev
#   → next dev
#   → API 请求 /api/* 通过 next.config.ts rewrites 代理到 localhost:8080
#   → 客户端 fetch 直连 localhost:8080（绕过 Turbopack POST 代理 500 问题）

# ===== 质量检查 =====

# TypeScript 类型检查 + ESLint
npm run lint
#   → eslint

# 单元测试（Vitest + jsdom）
npx vitest run
#   → 测试文件: src/**/*.test.{ts,tsx}
#   → 结果输出在终端

# E2E 测试（Playwright）
npx playwright test
#   → api 项目: test/api/*.spec.ts（仅需 Go 后端，串行）
#   → e2e 项目: test/**/*.spec.ts（需前后端同时运行，并行）
#   → 报告: npx playwright show-report

# ===== 生产构建 =====

# 构建（standalone 输出模式）
npm run build
#   → next build
#   → 输出: .next/standalone/（自包含，可直接 node server.js 运行）

# 启动生产服务器（端口 3000）
npm run start
#   → next start

# ===== Docker 构建（多阶段） =====
# 在 web/ 目录下
docker build -t opsmind-web .
#   → Stage 1 (deps): npm ci --omit=dev
#   → Stage 2 (builder): npm ci → npm run build
#   → Stage 3 (runner): node:22-alpine, node server.js
#   → 暴露端口 3000

# 在项目根目录下
docker compose up -d --build
#   → 编排 opsmind-server + opsmind-web + postgres(pgvector) + minio
```

### 5.4 关键配置文件

| 文件 | 关键配置项 | 说明 |
|------|----------|------|
| `next.config.ts` | `output: 'standalone'` | Docker 部署用自包含输出 |
| | `rewrites(): /api/:path* → ${API_URL}/api/:path*` | API 代理（开发模式客户端绕过使用直连） |
| `tsconfig.json` | `paths: { "@/*": ["./src/*"] }` | 路径别名 `@/components/...` |
| | `strict: true` | 严格模式 |
| | `moduleResolution: "bundler"` | 适配 Next.js 打包 |
| `vitest.config.ts` | `environment: 'jsdom'` | 浏览器环境模拟 |
| | `setupFiles: ['./src/__tests__/setup.ts']` | 加载 `@testing-library/jest-dom/vitest` |
| | `include: ['src/**/*.test.{ts,tsx}']` | 仅跑 src 下的测试 |
| `playwright.config.ts` | `testDir: './test'` | 测试目录 |
| | `baseURL: 'http://127.0.0.1:3000'` | 测试基准 URL |
| | `projects: [api (串行), e2e (并行)]` | 双项目分离 |
| `postcss.config.mjs` | `plugins: { '@tailwindcss/postcss': {} }` | Tailwind CSS v4 通过 PostCSS |
| `eslint.config.mjs` | `eslint-config-next/core-web-vitals` + `typescript` | Next.js 推荐规则 |
| `Dockerfile` | 三阶段构建 (deps → builder → runner) | `node:22-alpine`，用户 `nextjs`，端口 3000 |

### 5.5 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `NEXT_PUBLIC_API_URL` | 后端 API 地址 | `http://localhost:8080` |

> **注意：** 客户端 fetch 请求（如 SSE 流式问答）使用 `NEXT_PUBLIC_API_URL` 直连后端，绕过 Next.js rewrite。其他 API 请求的 `BASE_URL` 逻辑同样是优先使用 `NEXT_PUBLIC_API_URL`。

### 5.6 测试架构

```
测试分层:
├── Vitest (单元/组件测试)
│   ├── src/components/ui/__tests__/AppleButton.test.tsx    ← 组件渲染 + 交互 + props
│   ├── src/hooks/__tests__/useTheme.test.ts                ← Hook 逻辑 + localStorage mock
│   ├── src/lib/__tests__/auth.test.ts                      ← JWT 解码/过期判断
│   └── src/lib/api/__tests__/client.test.ts                ← apiFetch/apiFetchPage + ApiError
│
└── Playwright (E2E/集成测试)
    ├── test/api/          ← 仅需 Go 后端（串行）
    │   ├── auth.spec.ts
    │   ├── dashboard-etc.spec.ts
    │   ├── knowledge.spec.ts
    │   ├── roles.spec.ts
    │   ├── tickets.spec.ts
    │   └── users.spec.ts
    │
    └── test/{domain}/     ← 需前后端同时运行（并行）
        ├── auth/login.spec.ts
        ├── chat/chat.spec.ts
        ├── dashboard/dashboard.spec.ts
        ├── knowledge/knowledge.spec.ts
        ├── tickets/tickets.spec.ts
        ├── users/users.spec.ts
        ├── roles/roles.spec.ts
        ├── messages/messages.spec.ts
        ├── llm-config/llm-config.spec.ts
        ├── config/system-config.spec.ts
        └── audit/audit.spec.ts
```

---

## 附录：关键设计决策

### A. 为什么客户端 fetch 直连后端而不是统一走 Next.js rewrite？

`lib/api/client.ts` 中的 `BASE_URL` 在开发模式使用 `http://localhost:8080`，这是**有意为之**：

- Turbopack（Next.js 16 默认）在处理 POST 请求代理时存在已知 500 错误
- SSE 流式端点 (`/api/v1/portal/chat-sessions/:id/stream`) 需要原始 ReadableStream，经过 Next.js rewrite 可能导致缓冲
- 生产环境通过 `NEXT_PUBLIC_API_URL` 环境变量指定后端地址

### B. 为什么 AuthProvider 用 `useLayoutEffect` 设置 token getter？

`setTokenGetter()` 必须在浏览器渲染之前生效，因为 SWR 的首次数据请求可能在 `useEffect` 之前触发。`useLayoutEffect` 在 DOM 变更后、浏览器绘制前同步执行，保证首次 API 请求就能携带正确的 token。

### C. 为什么 ChatPage 使用 `@tanstack/react-virtual` 而不是 CSS overflow？

聊天消息列表可能包含数百条历史消息，虚拟滚动 (`useVirtualizer`) 只渲染可视区域内的消息 DOM 节点，避免大量消息导致页面卡顿。

### D. 为什么用 SWR 而不是 React Query 或 Redux？

- SWR 更轻量（~5KB），API 更简洁
- 项目没有复杂的服务端状态管理需求
- 全局认证状态用 React Context 已足够，无需状态管理库
- `revalidateOnFocus: false` + `dedupingInterval: 5000` 防止重复请求

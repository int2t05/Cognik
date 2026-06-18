# OpsMind 前端重构计划

> 从 Vue 3 (Naive UI + Linear Design) 迁移到 Next.js 15 (Radix UI + Apple Design System)
>
> 日期：2026-06-18 | 关联：[PRD](PRD.md) · [TECH](TECH.md) · [TODO](TODO.md) · [API](API/README.md) · [ui.md](prompts/ui.md)

---

## 1. 目标与范围

### 1.1 核心目标

| 目标 | 说明 |
|------|------|
| 框架迁移 | Vue 3 → Next.js 15 (React 19) App Router |
| 设计重构 | Linear Design 暗色单主题 → Apple Design 浅色/暗色双主题 |
| UI 组件 | Naive UI → Radix UI 无头组件 + 自定义 Apple 样式 |
| 状态管理 | Pinia → React Context + SWR |
| Bug 修复 | 修复 TODO.md 中全部 12 个 P0 和 28 个 P1 前端问题 |
| 代码质量 | 组件拆分（LLMConfig 610→多文件、Chat 381→子组件等） |

### 1.2 不变项

- 后端 API 完全不变（零改动）
- 所有 62 个 API 端点的请求/响应格式不变
- 认证机制不变（JWT access/refresh token）
- RBAC 权限模型不变
- Docker Compose 部署拓扑不变（仅 `opsmind-web` 镜像从 Nginx+Vue 改为 Next.js standalone）

---

## 2. 技术栈对比

| 层级 | 旧 (Vue) | 新 (Next.js) | 理由 |
|------|----------|-------------|------|
| 框架 | Vue 3.4 + Vite 5 | Next.js 15 + React 19 | App Router 原生支持 SSR/SSG/ISR，路由即文件系统 |
| UI 组件 | Naive UI 2.44 | Radix UI + 自定义 Apple 组件 | Radix 无头组件零样式冲突，完全控制 Apple 设计 |
| 状态管理 | Pinia 2.1 | React Context + SWR 2.x | SWR 自动处理缓存/重新验证，Context 覆盖全局状态 |
| 路由 | Vue Router 4.3 | Next.js App Router (文件系统路由) | 嵌套布局、loading/error 边界、Server Components |
| HTTP | Axios 1.7 | 原生 fetch + SWR fetcher | 减少依赖，SWR 内置请求去重和缓存 |
| 图标 | @vicons/ionicons5 | lucide-react | 更轻量、更好的 React 支持 |
| 字体 | Inter Variable (单 weight) | Inter Variable (全 weight 轴) | 匹配 ui.md 的 300/400/600 权重阶梯 |

### 2.1 移除的依赖

| 包 | 大小 | 替换 |
|---|------|------|
| `naive-ui` | ~600KB gzipped | Radix UI + 自定义 Apple 组件 |
| `@vicons/ionicons5` | ~200KB | `lucide-react` (tree-shakeable) |
| `pinia` | ~10KB | React Context (零依赖) |
| `vue-router` | ~30KB | Next.js App Router (内置) |
| `axios` | ~14KB | 原生 `fetch` (零依赖) |
| `vue` / `@vue/*` | ~130KB | `react` / `react-dom` |

**净收益：~1MB 运行时体积减少，依赖数减少 60%。**

---

## 3. Apple Design System

严格对照 `docs/prompts/ui.md` 定义。

### 3.1 设计 Token (tokens.css)

```css
/* ===== 颜色 (40 tokens) ===== */
/* 品牌强调 */
--accent: #0066cc;              /* Action Blue — 唯一交互色 */
--accent-focus: #0071e3;        /* Focus ring */
--accent-on-dark: #2997ff;      /* 暗色面上的链接蓝 */

/* 画布 */
--bg-canvas: #ffffff;           /* 纯白主画布 */
--bg-parchment: #f5f5f7;        /* 羊皮纸（交替区间分隔） */
--bg-pearl: #fafafc;            /* 珍珠按钮填充 */

/* 暗色瓦片 */
--bg-tile-1: #272729;           /* 主暗色瓦片 */
--bg-tile-2: #2a2a2c;           /* 微亮（相邻暗瓦片分隔） */
--bg-tile-3: #252527;           /* 微暗（底部/嵌入框） */
--bg-black: #000000;            /* 纯黑（导航栏、视频播放器） */

/* 文字 */
--text-ink: #1d1d1f;            /* 标题/正文 */
--text-body: #1d1d1f;           /* 同 ink */
--text-on-dark: #ffffff;        /* 暗面文字 */
--text-muted: #cccccc;          /* 暗面次要文字 */
--text-muted-80: #333333;       /* 珍珠按钮文字 */
--text-muted-48: #7a7a7a;       /* 禁用/法律文本 */

/* 分割线 */
--divider-soft: #f0f0f0;        /* 次要按钮环 */
--hairline: #e0e0e0;            /* 卡片细线 */

/* ===== 字体 (15 tokens) ===== */
--font-display: 'Inter Variable', system-ui, -apple-system, sans-serif;
--font-body: 'Inter Variable', system-ui, -apple-system, sans-serif;

--text-hero: 56px/600/1.07/-0.28px;     /* hero-display */
--text-display-lg: 40px/600/1.1/0;       /* display-lg */
--text-display-md: 34px/600/1.47/-0.374px;
--text-lead: 28px/400/1.14/0.196px;
--text-body: 17px/400/1.47/-0.374px;     /* ← Apple 的标志性 17px */
--text-body-strong: 17px/600/1.24/-0.374px;
--text-caption: 14px/400/1.43/-0.224px;
--text-caption-strong: 14px/600/1.29/-0.224px;
--text-fine-print: 12px/400/1.0/-0.12px;
--text-nav: 12px/400/1.0/-0.12px;

/* ===== 间距 (8 tokens) ===== */
--space-xs: 8px;
--space-sm: 12px;
--space-md: 17px;
--space-lg: 24px;
--space-xl: 32px;
--space-xxl: 48px;
--space-section: 80px;

/* ===== 圆角 (7 tokens) ===== */
--radius-none: 0;
--radius-xs: 5px;
--radius-sm: 8px;
--radius-md: 11px;
--radius-lg: 18px;
--radius-pill: 9999px;
--radius-full: 50%;

/* ===== 阴影 (仅 1 个) ===== */
--shadow-product: 3px 5px 30px 0 rgba(0,0,0,0.22);  /* 仅产品图使用 */

/* ===== 动效 ===== */
--ease-out: cubic-bezier(0.16, 1, 0.3, 1);
--duration-fast: 150ms;
--duration-normal: 250ms;
```

### 3.2 暗色主题

```css
[data-theme="dark"] {
  --accent: #2997ff;               /* Sky Link Blue 替代 Action Blue */
  --accent-focus: #40a9ff;
  --bg-canvas: #1d1d1f;            /* ink 变为画布 */
  --bg-parchment: #272729;         /* tile-1 变为羊皮纸 */
  --bg-pearl: #2a2a2c;
  --bg-tile-1: #000000;            /* 纯黑瓦片 */
  --bg-tile-2: #1a1a1c;
  --bg-tile-3: #0d0d0f;
  --bg-black: #000000;
  --text-ink: #f5f5f7;
  --text-body: #f5f5f7;
  --text-on-dark: #ffffff;
  --text-muted: #999999;
  --text-muted-80: #cccccc;
  --text-muted-48: #666666;
  --divider-soft: rgba(255,255,255,0.08);
  --hairline: rgba(255,255,255,0.12);
}
```

### 3.3 主题切换实现

```typescript
// hooks/useTheme.ts
'use client';
import { useState, useEffect, useCallback } from 'react';

type Theme = 'light' | 'dark';

// 模块级变量：初始化默认 dark，SSR 安全
let cachedTheme: Theme = 'dark';

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(cachedTheme);

  useEffect(() => {
    // 客户端：从 localStorage 读取
    const stored = localStorage.getItem('theme-preference') as Theme | null;
    const resolved = stored || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    applyTheme(resolved);
  }, []);

  const applyTheme = useCallback((t: Theme) => {
    cachedTheme = t;
    setTheme(t);
    document.documentElement.setAttribute('data-theme', t);
    localStorage.setItem('theme-preference', t);
  }, []);

  const toggleTheme = useCallback(() => {
    applyTheme(theme === 'light' ? 'dark' : 'light');
  }, [theme, applyTheme]);

  return { theme, toggleTheme, setTheme: applyTheme };
}
```

关键修复：消除旧版 `useTheme.ts` 的模块级 `localStorage` SSR 不安全访问（TODO.md P1 项）。

---

## 4. 组件架构

### 4.1 分层

```
styles/tokens.css           ← 设计 Token，零组件依赖
components/ui/              ← 14 个 Apple 原子组件
components/layout/          ← 2 个布局组件（依赖 ui）
components/shared/          ← 3 个复合组件（依赖 ui）
app/                        ← App Router 页面（依赖一切）
hooks/                      ← 5 个自定义 Hook
lib/api/                    ← 12 个 API 模块
```

### 4.2 组件清单（14 + 2 + 3 = 19 个）

#### UI 组件 (`components/ui/`)

| 组件 | Radix 封装 | 变体 | Props 要点 |
|------|-----------|------|-----------|
| `AppleButton` | — | `pill` / `ghost` / `utility` / `pearl` | `variant, size, loading, disabled` |
| `AppleInput` | — | `pill` / `default` / `textarea` | `label, error, pill, rows` |
| `AppleDialog` | `@radix-ui/react-dialog` | — | `open, title, description, width` |
| `AppleSelect` | `@radix-ui/react-select` | — | `options, placeholder, value, onChange` |
| `AppleDropdown` | `@radix-ui/react-dropdown-menu` | — | `items, align, trigger` |
| `AppleTabs` | `@radix-ui/react-tabs` | — | `tabs, defaultValue, onChange` |
| `AppleSwitch` | `@radix-ui/react-switch` | — | `checked, onChange, label` |
| `ApplePopover` | `@radix-ui/react-popover` | — | `trigger, content, align` |
| `AppleTooltip` | `@radix-ui/react-tooltip` | — | `content, children` |
| `AppleTable` | — | — | `columns, data, loading, rowKey, emptyText` |
| `ApplePagination` | — | — | `page, pageSize, total, onChange` |
| `AppleBadge` | — | `success/warning/error/info/neutral` | `variant, label` |
| `AppleSpinner` | — | — | `size` |
| `AppleCard` | — | — | `padding, hoverable, onClick` |
| `AppleToast` | — (全局单例) | `success/error/warning/info` | 通过 `useToast()` 调用 |
| `AppleFileUpload` | — | — | `accept, maxSize, maxFiles, onChange` |

#### 布局组件 (`components/layout/`)

| 组件 | 说明 |
|------|------|
| `AdminLayout` | 侧栏（可折叠）+ 顶栏（汉堡、主题切换、消息徽章、用户下拉、loading 条） |
| `PortalLayout` | 顶栏（导航链接、消息徽章）+ 内容区（max-width 1200px 居中） |

#### 共享组件 (`components/shared/`)

| 组件 | 说明 |
|------|------|
| `StatCard` | 看板统计卡片（label + value + trend indicator） |
| `StatusBadge` | 业务状态标签（用户冻结/正常、申告 5 态、知识 6 态、处理状态 8 态） |
| `ConfirmDialog` | 通用确认弹窗（危险操作二次确认） |

### 4.3 组件设计原则

1. **无阴影** — Apple 阴影仅用于产品图，UI 组件一律 flat
2. **无边框** — 分隔通过 `--divider-soft` 或表面色差，而非可见边框
3. **Active 态统一** — `transform: scale(0.95)` + 150ms ease-out
4. **Focus 态统一** — `outline: 2px solid var(--accent-focus)` + `outline-offset: 2px`
5. **无 weight 500** — 严格 300/400/600，body 始终 400，strong 始终 600
6. **17px body** — 不跟随 SaaS 常见的 16px 惯例

---

## 5. 路由设计

### 5.1 App Router 文件结构

```
web/src/app/
├── layout.tsx                          # 根布局（ThemeProvider + ToastProvider）
├── page.tsx                            # 重定向到 /portal/chat
├── login/
│   ├── layout.tsx                      # 无导航，纯居中卡片
│   └── page.tsx                        # 登录表单
├── change-password/
│   ├── layout.tsx
│   └── page.tsx                        # 修改密码表单
├── portal/
│   ├── layout.tsx                      # PortalLayout（顶部导航 + message badge 轮询）
│   ├── chat/
│   │   └── page.tsx                    # 智能问答主页
│   ├── tickets/
│   │   ├── page.tsx                    # 我的申告列表
│   │   ├── new/
│   │   │   └── page.tsx                # 提交新申告
│   │   └── [id]/
│   │       └── page.tsx                # 申告详情
│   └── messages/
│       └── page.tsx                    # 站内消息
├── admin/
│   ├── layout.tsx                      # AdminLayout（侧栏 + 顶栏 + RBAC 守卫）
│   ├── dashboard/
│   │   └── page.tsx                    # 数据看板
│   ├── tickets/
│   │   ├── page.tsx                    # 全部申告列表
│   │   └── [id]/
│   │       └── page.tsx                # 申告处理详情
│   ├── knowledge/
│   │   ├── page.tsx                    # 知识库列表
│   │   └── [kbId]/
│   │       ├── page.tsx                # 文章列表
│   │       ├── new/
│   │       │   └── page.tsx            # 新建文章
│   │       └── [articleId]/
│   │           └── page.tsx            # 编辑文章 + 文档上传
│   ├── users/
│   │   └── page.tsx                    # 用户管理
│   ├── roles/
│   │   └── page.tsx                    # 角色管理
│   ├── config/
│   │   ├── llm/
│   │   │   └── page.tsx                # LLM 配置
│   │   └── system/
│   │       └── page.tsx                # 系统配置（合并原 ModelConfig）
│   └── audit/
│       └── page.tsx                    # 审计日志
└── not-found.tsx                       # 404
```

### 5.2 路由守卫

```typescript
// middleware.ts — 请求级守卫（Edge Runtime）
import { NextRequest, NextResponse } from 'next/server';

export function middleware(req: NextRequest) {
  const token = req.cookies.get('access_token')?.value;
  const { pathname } = req.nextUrl;

  // 公开路由
  if (pathname === '/login') {
    if (token) return NextResponse.redirect(new URL('/portal/chat', req.url));
    return NextResponse.next();
  }

  // 认证检查
  if (!token) {
    return NextResponse.redirect(new URL('/login', req.url));
  }

  // Token 过期检查（base64url 兼容解码）
  if (isTokenExpired(token)) {
    // 尝试 refresh（Server Action）
    // 失败则重定向到登录页
  }

  // RBAC：从 token payload 读取 roles，匹配路由前缀
  if (pathname.startsWith('/admin') && !canAccessAdmin(token)) {
    return NextResponse.redirect(new URL('/portal/chat', req.url));
  }

  return NextResponse.next();
}
```

关键修复：旧版 `atob` 不兼容 base64url 导致 JWT 过期检查失效（TODO.md P0-1）。

### 5.3 布局嵌套

```
RootLayout (ThemeProvider)
├── LoginLayout          → 无导航，纯居中登录卡片
├── ChangePasswordLayout → 无导航，纯居中表单
├── PortalLayout         → 顶栏 + 消息 badge + 内容区
│   ├── ChatPage
│   ├── TicketListPage
│   ├── TicketNewPage
│   ├── TicketDetailPage
│   └── MessagesPage
└── AdminLayout          → 侧栏 + 顶栏 + loading 条 + 内容区
    ├── DashboardPage
    ├── TicketListPage
    ├── TicketDetailPage
    ├── KnowledgeListPage
    ├── KnowledgeEditPage
    ├── UserListPage
    ├── RoleManagePage
    ├── LLMConfigPage
    ├── SystemConfigPage
    └── AuditLogPage
```

### 5.4 CMS 菜单路由

后端返回的 `menus` 数据中的 `path` 字段直接映射到 App Router 路径，无需前端硬编码。

```typescript
// lib/menu.ts — 菜单匹配与动态构建
export function isActivePath(menuPath: string, currentPath: string): boolean {
  return currentPath === menuPath || currentPath.startsWith(menuPath + '/');
}
```

---

## 6. 状态管理

### 6.1 方案

| 场景 | 旧 (Pinia) | 新 | 理由 |
|------|-----------|-----|------|
| 用户认证状态 | `stores/auth.ts` | `AuthContext` (React Context) | 全局读写，跨组件树共享 |
| 聊天会话 | `stores/chat.ts` | `useChat` hook (useReducer + SWR) | 复杂状态机（加载/流式/错误/完成） |
| 未读消息数 | `stores/app.ts` | `MessageContext` + SWR `useSWR('/messages/unread-count')` | 自动轮询替代手动 30s 定时器 |
| 侧栏折叠 | `stores/app.ts` | `localStorage` + `useState` | 仅 UI 状态，无需全局 store |
| 主题 | `composables/useTheme.ts` | `useTheme` hook | 模块级缓存 + SSR 安全 |
| Toast | `composables/useToast.ts` | `ToastContext` (全局单例) | 修复多组件 toast 冲突 |
| 服务端数据 | 混杂（API 调用 + 手动缓存） | SWR (`useSWR`) | 自动去重、重新验证、乐观更新 |

### 6.2 AuthContext

```typescript
// hooks/useAuth.tsx
'use client';
import { createContext, useContext, useState, useCallback, useEffect } from 'react';
import type { User, Role, Menu, Permission } from '@/types';

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: User | null;
  roles: string[];
  permissions: string[];
  menus: Menu[];
  isLoggedIn: boolean;
}

// Context 提供 login、logout、refreshToken、hasPermission
```

### 6.3 SWR 数据获取模式

所有服务端数据请求统一通过 SWR：

```typescript
// lib/api/knowledge.ts — SWR hook 工厂
import useSWR from 'swr';
import { apiFetch } from '@/lib/request';

export function useKBList() {
  return useSWR('/api/v1/admin/knowledge-bases', apiFetch);
}

export function useArticleList(kbId: number, page: number, status?: string) {
  return useSWR(
    `/api/v1/admin/knowledge-bases/${kbId}/articles?page=${page}&page_size=10${status ? `&status=${status}` : ''}`,
    apiFetch
  );
}
```

### 6.4 SSE 流式处理（聊天）

```typescript
// hooks/useChat.ts
// 使用 useReducer 管理聊天状态机：
// idle → loading → streaming → done | error
// 通过 AbortController 取消飞行中的请求
// fetch() 替代 axios 以获得原生 ReadableStream 支持

const initialState = {
  sessionId: null,
  messages: [],
  loading: false,
  streaming: false,
  error: null,
  currentStep: null,
  pipelineMetrics: null,
};
```

关键修复：
- 使用 `crypto.randomUUID()` 替代不安全的自定义 `generateId()`，并带 `crypto` 可用性 fallback（TODO.md P0-3）
- 竞态处理：新的 `sendQuestion` 自动 abort 旧的 stream，防止回调覆盖（TODO.md P0-2）
- SSE 流绕过 fetch 拦截后 token 过期的处理（TODO.md P0-4）

---

## 7. P0 Bug 修复方案

对照 `docs/TODO.md` 的 12 个 🔴 项，逐项修复：

| # | 文件 (旧) | 问题 | 新方案 |
|---|----------|------|--------|
| 1 | `router/index.ts` | `atob` 不兼容 base64url | `middleware.ts` 使用 `Buffer.from(token, 'base64url')` 或 polyfill `atob` |
| 2 | `views/auth/Login.vue` | 错误提取用 `err?.message` | `login` Server Action 中统一抛出自定义错误，客户端 `try/catch` 提取 `error.message` |
| 3 | `views/auth/Login.vue` | 路由判断基于 `permissions.length` | Middleware 读取 JWT payload 的 `roles` 字段直接匹配 |
| 4 | `stores/chat.ts` | `crypto.randomUUID()` HTTP 下 undefined | `useChat` hook 内 `const genId = () => crypto?.randomUUID?.() ?? fallback()` |
| 5 | `stores/chat.ts` | re-entrant 竞态 | `useRef<AbortController>` 在每次 `sendQuestion` 前 abort 旧请求 |
| 6 | `api/chat.ts` | SSE 绕过 Axios 拦截器 | SWR Mutation + `fetch()` — token 过期由 middleware 层刷新 |
| 7 | `views/portal/ChatPipelineSteps.vue` | `s.success` 不存在 | SSE `step` 事件中 `success` 字段命名与后端 `done.metadata.pipeline.steps[].success` 对齐 |
| 8 | `views/admin/TicketList.vue` | 响应解包 `res?.data \|\| res?.items` | `useSWR` 直接返回 typed `PageResponse<T>`，统一 `.items` 访问 |
| 9 | `views/admin/TicketDetail.vue` | 按钮缺 loading 守卫 | `AppleButton` 内置 `loading` prop → `disabled` + spinner，防止重复点击 |
| 10 | `views/portal/TicketSubmit.vue` | `submitting` 永不重置 | `finally { setSubmitting(false) }` — try/catch/finally 保证重置 |
| 11 | `views/admin/AuditLog.vue` | 分页 `total` 为当前页条目数 | `useSWR` 从 `PageResponse.total` 直接取，不 fallback 到 `logs.length` |
| 12 | `App.vue` | `NMessageProvider` 死代码 | 移除，AppleToast 全局单例挂载在 RootLayout |

---

## 8. P1 关键改进

| # | 模块 | 改进 | 实现 |
|---|------|------|------|
| 1 | Chat | 组件 >560 行拆分 | `ChatInput` + `ChatMessage` + `ChatPipeline` + `ChatAdvancedPanel` 四个子组件 |
| 2 | Chat | 反馈失败静默 | `AppleToast.error('反馈提交失败，请重试')` 替代 `console.error` |
| 3 | Chat | `clearSession()` 不重置 KB/RAG | `useReducer` 的 `CLEAR` action 重置全部状态字段 |
| 4 | KnowledgeEdit | 组件 >400 行拆分 | `KnowledgeMetaForm` + `DocumentUpload` + `ProcessStatus` + `ReviewPanel` |
| 5 | KnowledgeEdit | `fetch` 失败静默 | SWR 的 `error` 状态渲染 `<AppleToast>` + 重试按钮 |
| 6 | KnowledgeEdit | 使用 `alert()` | 统一迁移到 `useToast()` |
| 7 | KnowledgeList | `description` 静默清空 | 编辑 KB 弹窗预填原值，`undefined` 时保留空字符串而非覆盖 |
| 8 | LLMConfig | 组件 >610 行拆分 | `LLMConfigList` + `LLMConfigDialog` + `ConnectionTestDialog` |
| 9 | LLMConfig | 测试连接崩溃 | `handleTestConnection` 检查 `editingId` 存在性再调用 API |
| 10 | LLMConfig | API Key 脱敏 | 编辑弹窗提示「留空则不修改」，placeholder 显示 `****1234` |
| 11 | ModelConfig + SystemConfig | 重复配置 | 合并为单一 `SystemConfig` 页面，`topK` 和 `threshold` 统一管理 |
| 12 | RoleManage | 权限硬编码 | 从后端 `/api/v1/admin/roles` 返回的 `permissions` 字段动态渲染 |
| 13 | UserList | 冻结/恢复无确认 | `ConfirmDialog` 二次确认，内容为「确定要冻结用户 XXX 吗？」 |
| 14 | request.ts | Token 刷新竞态 | SSR/CSR 分离：服务端用 cookie，客户端用 `onTokenRefreshed` Promise 队列 |
| 15 | request.ts | loadingState 全局计数器 | `useLoading` hook 改为 per-request 模式，不使用全局计数器 |
| 16 | request.ts | baseURL 依赖代理 | `next.config.js` `rewrites` 代理规则 + 客户端 `baseURL` 环境变量 |
| 17 | useAIConfig | loadConfig 吞错误 | SWR 的 `onError` 回调 → `useToast().error()` |
| 18 | useToast | 多 toast 冲突 | `ToastContext` 全局单例，`toasts[]` 状态堆叠 3 条，3s 自动消除 |
| 19 | useTheme | SSR 不兼容 | `useEffect` 延迟读取 `localStorage`，SSR 默认 `'light'`，hydrate 后切换 |
| 20 | AdminLayout | 菜单路径硬编码 | 后端 `menus` 数据的 `path` 字段直接映射 |
| 21 | AdminLayout | `path.startsWith()` 硬编码 | `isActivePath()` 工具函数统一匹配逻辑 |
| 22 | StatusBadge | knowledge 类型未实现 | 完整实现 6 种知识状态 (draft/reviewing/approved/published/rejected/disabled) + 8 种处理状态 |
| 23 | TicketDetail (admin) | 时间显示 ISO 字符串 | `formatDate` 工具函数统一格式化 |
| 24 | TicketDetail (admin) | 知识候选 KB ID 无下拉 | `AppleSelect` 下拉 KB 列表，带搜索过滤 |
| 25 | TicketSubmit (portal) | `chat_context` 无 JSON 校验 | 提交前 `JSON.parse(JSON.stringify(chat_context))` 确保有效 JSON |
| 26 | TicketDetail (portal) | API 失败静默 | SWR `error` → `<AppleToast>` + 空状态提示 |
| 27 | Dashboard | `avg_confidence` null 显示 NaN% | `formatPercent(avgConfidence: number | null)` 安全格式化 |
| 28 | Dashboard | `fetchTrends` 失败静默 | SWR `error` → 趋势图空状态文案 + 重试按钮 |

---

## 9. 文件结构（完整）

```
web/
├── public/
│   ├── favicon.ico
│   └── fonts/
│       └── InterVariable.woff2          # 自托管 Inter（性能优化）
├── src/
│   ├── app/                              # 路由定义（见 §5.1）
│   ├── components/
│   │   ├── ui/                           # 14 个 Apple 原子组件
│   │   │   ├── AppleButton.tsx
│   │   │   ├── AppleInput.tsx
│   │   │   ├── AppleDialog.tsx
│   │   │   ├── AppleSelect.tsx
│   │   │   ├── AppleDropdown.tsx
│   │   │   ├── AppleTabs.tsx
│   │   │   ├── AppleSwitch.tsx
│   │   │   ├── ApplePopover.tsx
│   │   │   ├── AppleTooltip.tsx
│   │   │   ├── AppleTable.tsx
│   │   │   ├── ApplePagination.tsx
│   │   │   ├── AppleBadge.tsx
│   │   │   ├── AppleSpinner.tsx
│   │   │   ├── AppleCard.tsx
│   │   │   ├── AppleToast.tsx
│   │   │   ├── AppleFileUpload.tsx
│   │   │   └── index.ts                 # barrel export
│   │   ├── layout/
│   │   │   ├── AdminLayout.tsx
│   │   │   ├── AdminSidebar.tsx          # ← 从 AdminLayout 拆分
│   │   │   ├── AdminHeader.tsx           # ← 从 AdminLayout 拆分
│   │   │   ├── PortalLayout.tsx
│   │   │   └── PortalHeader.tsx          # ← 从 PortalLayout 拆分
│   │   └── shared/
│   │       ├── StatCard.tsx
│   │       ├── StatusBadge.tsx
│   │       └── ConfirmDialog.tsx
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useTheme.ts
│   │   ├── useToast.ts
│   │   ├── useLoading.ts
│   │   ├── useAIConfig.ts
│   │   └── useChat.ts                   # ← 聊天状态机
│   ├── lib/
│   │   ├── api/
│   │   │   ├── client.ts                # fetch 封装 (base URL, error handling, token)
│   │   │   ├── server.ts                # Server Component 专用 (cookies)
│   │   │   ├── auth.ts
│   │   │   ├── chat.ts
│   │   │   ├── ticket.ts
│   │   │   ├── admin.ts
│   │   │   ├── knowledge.ts
│   │   │   ├── user.ts
│   │   │   ├── role.ts
│   │   │   ├── config.ts
│   │   │   ├── llm_config.ts
│   │   │   ├── message.ts
│   │   │   ├── audit.ts
│   │   │   ├── dashboard.ts
│   │   │   └── types.ts                 # ApiResponse<T>, PageResponse<T>
│   │   ├── format.ts
│   │   ├── date.ts
│   │   ├── auth.ts                      # JWT decode (base64url safe)
│   │   ├── knowledge.ts
│   │   ├── ticket.ts
│   │   ├── id.ts                        # generateId (带 crypto fallback)
│   │   └── menu.ts                      # isActivePath, buildMenuTree
│   ├── styles/
│   │   ├── tokens.css                   # 设计 Token (浅色 + 暗色)
│   │   └── global.css                   # Reset + 全局排版 + 滚动条 + 动效
│   └── types/
│       ├── api.ts
│       ├── knowledge.ts
│       ├── ticket.ts
│       ├── menu.ts
│       └── index.ts
├── middleware.ts                         # 路由守卫
├── next.config.js
├── tsconfig.json
├── package.json
└── Dockerfile                           # Next.js standalone 模式
```

---

## 10. 实施阶段

### Phase 1：基础平台（核心框架 + 主题 + 认证）

**预计工作量：3-4 天**

1. 初始化 Next.js 项目 — `create-next-app` + TypeScript + ESLint
2. 安装依赖 — `react`, `react-dom`, `@radix-ui/*`, `swr`, `lucide-react`, `inter`
3. 创建 `tokens.css` — 完整 Apple 设计 Token（浅色 + 暗色）
4. 创建 `global.css` — Reset + 全局排版 + 滚动条 + 动效
5. 实现 `useTheme` hook — SSR 安全的 dual theme 切换
6. 实现 `ThemeProvider` + `ToastProvider` — 挂载在 RootLayout
7. 实现 `middleware.ts` — JWT 认证 + base64url 兼容解码 + RBAC 路由守卫
8. 实现 `lib/api/client.ts` — fetch 封装（base URL, error handling）
9. 实现 `lib/auth.ts` — JWT decode（base64url safe）
10. 实现 `useAuth` hook — Context + localStorage 持久化
11. 实现 `lib/menu.ts` — `isActivePath` + `buildMenuTree`
12. 实现 `Login` 页面 — Apple 风格居中卡片
13. 实现 `ChangePassword` 页面
14. **验证：** 登录 → 角色路由跳转 → 主题切换 → Token 过期刷新 → 登出

### Phase 2：Apple UI 组件库

**预计工作量：3-4 天**

1. `AppleButton` — 4 种变体 + loading/disabled/focus/active 状态
2. `AppleInput` — text/textarea/password/search + pill 变体
3. `AppleDialog` — Radix Dialog 封装 + title/description/footer slot
4. `AppleSelect` — Radix Select 封装
5. `AppleDropdown` — Radix DropdownMenu 封装
6. `AppleTabs` — Radix Tabs 封装
7. `AppleSwitch` — Radix Switch 封装
8. `ApplePopover` — Radix Popover 封装
9. `AppleTable` — 无边框 + 斑马行 + loading skeleton + empty state
10. `ApplePagination` — 精简紧凑样式
11. `AppleBadge` — 5 种语义变体
12. `AppleSpinner` — 简洁 loading 指示器
13. `AppleCard` — 白底 + hairline 边框 + 18px 圆角
14. `AppleToast` — 全局单例 + 堆叠 3 条 + 3s 自动消失
15. `AppleFileUpload` — 拖放区 + 文件列表 + 进度
16. **验证：** 组件 Storybook/手动测试 — 所有变体、状态、键盘导航、ARIA

### Phase 3：布局 + 共享组件

**预计工作量：1-2 天**

1. `AdminLayout` — 侧栏折叠 + 顶栏（主题切换、消息徽章、用户菜单）+ loading 条
2. `AdminSidebar` — 菜单动态渲染 + `isActivePath` 高亮
3. `PortalLayout` — 顶部导航 + 消息轮询 + 1200px 内容区
4. `StatCard` — 统计卡片（label + value + trend）
5. `StatusBadge` — 完整状态映射（用户/申告/知识/处理）
6. `ConfirmDialog` — 危险操作二次确认
7. **验证：** 布局切换 → 侧栏折叠 → 菜单高亮 → 消息 badge 轮询

### Phase 4：门户端页面

**预计工作量：2-3 天**

1. 聊天主页 — `ChatInput` + `ChatMessageList` + `ChatPipeline` + `ChatAdvancedPanel`
2. 申告提交页面 — 表单 + 标签输入 + chat_context 验证
3. 申告查询页面 — 列表 + 分页
4. 申告详情页面 — 信息展示 + 补充信息表单 + 处理记录时间线
5. 消息页面 — 消息列表 + 已读标记
6. SWR 集成 — 所有门户数据使用 `useSWR` 自动缓存
7. SSE 流式 — `useChat` hook 管理 ReadableStream + pipeline 步骤事件
8. **验证：** 完整用户流程 — 登录 → 问答 → 低置信度转申告 → 查询进度 → 查看消息

### Phase 5：后台管理页面

**预计工作量：3-4 天**

1. 数据看板 — StatCard 网格 + 自建条形图（无图表库）
2. 申告管理 — 列表 + 筛选 + 详情 + 状态操作 + 知识候选
3. 知识库管理 — 左侧 KB 列表 + 右侧文章表格 + 编辑页（元信息 + 文档上传 + 审核发布）
4. 用户管理 — 表格 + 创建/编辑弹窗 + 冻结/恢复确认
5. 角色管理 — 表格 + 权限 checkbox 组（动态从 API 读取）
6. LLM 配置 — 卡片列表 + CRUD 弹窗 + 连接测试
7. 系统配置 — 键值对表 + AI 参数滑块（合并 ModelConfig）
8. 审计日志 — 表格 + 多维筛选
9. **验证：** 全部后台 CRUD 流程 + 权限控制 + Toast 提示

### Phase 6：测试 + 收尾

**预计工作量：2-3 天**

1. 单元测试 — 工具函数（`lib/format.ts`, `lib/date.ts`, `lib/auth.ts`, `lib/id.ts`）
2. 组件测试 — `AppleButton`, `AppleTable`, `AppleDialog`
3. E2E 测试 — Playwright：登录 → 问答 → 提交申告 → 后台处理 → 知识发布
4. Dockerfile — Next.js standalone 模式 + 最小化镜像
5. docker-compose.yml — 更新 `opsmind-web` 服务定义
6. 性能检查 — Lighthouse (LCP, CLS, TBT) + bundle analyzer
7. **验证：** 全部测试通过 + Docker Compose 一键启动 + Lighthouse > 90

---

## 11. 测试策略

### 11.1 测试金字塔

```
        E2E (Playwright) — 3 个核心流程
       /                              \
  集成测试 (fetch mock + SWR) — API 层
 /                                      \
单元测试 (工具函数 + hooks + 组件) — ~30 个
```

### 11.2 关键 E2E 场景

1. **完整问答流程** — 登录 → 创建会话 → SSE 流式 → 反馈 → 转申告
2. **完整申告流程** — 提交 → 后台接单 → 处理中 → 索要补充 → 补充信息 → 已解决
3. **完整知识流程** — 上传文档 → 等待处理 → 提交审核 → 审核通过 → 发布 → 停用

### 11.3 组件测试清单

| 组件 | 测试要点 |
|------|----------|
| `AppleButton` | 4 种变体、loading disabled、click 事件、scale active |
| `AppleInput` | value 绑定、error 显示、pill 样式、textarea rows |
| `AppleDialog` | open/close、title/description、footer slot、escape dismiss |
| `AppleTable` | columns 渲染、data 绑定、loading skeleton、empty state、rowKey |
| `ApplePagination` | page 切换、pageSize 切换、total 为 0 |
| `AppleToast` | success/error/warning 变体、3s 自动消失、堆叠 3 条 |
| `StatusBadge` | 用户（2 态）、申告（5 态）、知识（6 态）、处理（8 态） |

---

## 12. 迁移检查清单

### 文档更新

- [x] `docs/TECH.md` — 架构图 + web 目录结构
- [x] `docs/PRD.md` — 技术选型表
- [x] `CLAUDE.md` — 角色声明 + 命令 + 项目结构 + 开发边界 + 资源表
- [x] `docs/refactorui.md` — 本文档

### 旧代码清理

- [ ] 删除 `web/src/` 全部现有代码（79 个源文件）
- [ ] 删除 `web/nginx.conf`
- [ ] 删除 `web/package.json` 中的旧依赖
- [ ] 删除 `web/theme/index.ts`（Naive UI theme overrides）
- [ ] 删除 `web/src/styles/global.css`（Linear Design tokens）
- [ ] 删除 `web/src/stores/`（Pinia）
- [ ] 删除 `web/src/router/`（Vue Router）
- [ ] 删除 `web/src/composables/`（Vue composables）
- [ ] 删除 `web/src/components/common/`（Vue 通用组件）
- [ ] 删除 `web/index.html`
- [ ] 删除 `web/vite.config.ts`
- [ ] 删除 `web/src/__tests__/`（Vitest 测试 — 用 Playwright + React Testing Library 替代）

### 新增

- [ ] `web/src/app/` — 18 个页面 + 4 个布局
- [ ] `web/src/components/` — 19 个组件
- [ ] `web/src/hooks/` — 6 个 hooks
- [ ] `web/src/lib/` — 18 个模块
- [ ] `web/src/styles/` — tokens.css + global.css
- [ ] `web/middleware.ts`
- [ ] `web/Dockerfile` — Next.js standalone
- [ ] `web/src/__tests__/` — 单元 + 组件测试

### 部署变更

- [ ] `docker-compose.yml` — `opsmind-web` 服务：镜像从 `nginx:alpine` 改为 `node:22-alpine`，端口 3000
- [ ] `web/Dockerfile` — `next build` → `next start`
- [ ] 健康检查端点：`/health` 或 `/api/health`

---

## 13. 风险与缓解

| 风险 | 概率 | 缓解 |
|------|------|------|
| SWR 与 SSE 流式不兼容 | 低 | SSE 使用原生 `fetch` + `ReadableStream`，不经过 SWR |
| Radix UI 某组件不满足需求 | 中 | 14 个 UI 组件仅 7 个依赖 Radix，有回退到原生实现的空间 |
| Inter 字体加载性能 | 低 | 自托管 woff2 + `font-display: swap` + subset 中文 |
| Server Components 与认证 cookies 的交互 | 中 | middleware 层处理 token 注入，Server Component 通过 `cookies()` 读取 |
| Apple 17px body 在小屏上显得过大 | 低 | Responsive typography：小屏降为 16px |
| Radix Select/Dialog 在当前项目中不可用 | 极低 | `@radix-ui/*` 包与框架无关，直接安装使用 |

---

## 附录 A：关键代码片段

### A.1 SSE 流式处理（修复 P0-2, P0-3, P0-4, P0-5）

```typescript
// hooks/useChat.ts
export function useChat() {
  const [state, dispatch] = useReducer(chatReducer, initialState);
  const abortRef = useRef<AbortController | null>(null);

  const sendQuestion = useCallback(async (sessionId: number, question: string) => {
    // 取消旧的飞行中请求（修复竞态）
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    dispatch({ type: 'SEND_START' });

    try {
      const response = await fetch(`/api/v1/portal/chat-sessions/${sessionId}/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${getAccessToken()}`,
        },
        body: JSON.stringify({ question }),
        signal: controller.signal,
      });

      if (!response.ok) throw new ChatError(response.status, 'Failed to connect');

      const reader = response.body!.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const parsed = JSON.parse(line.slice(6));
            dispatch({ type: 'SSE_EVENT', payload: parsed }); // step/token/done/error
          }
        }
      }
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      dispatch({ type: 'STREAM_ERROR', payload: err.message });
    }
  }, []);

  // ID 生成（修复 crypto.randomUUID HTTP 下 undefined — TODO.md P0-3）
  const generateId = useCallback(() => {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
      return crypto.randomUUID();
    }
    // Fallback: timestamp + random
    return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
  }, []);

  return { ...state, sendQuestion, generateId };
}
```

### A.2 响应解包标准化（修复 P0-8, P0-11）

```typescript
// lib/api/client.ts
import type { ApiResponse, PageResponse } from '@/types/api';

const BASE_URL = process.env.NEXT_PUBLIC_API_URL || '';

export async function apiFetch<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!res.ok && res.status === 401) {
    // Token 过期 → 触发 refresh（middleware 层也会处理）
    throw new AuthError('Token expired');
  }

  const json: ApiResponse<T> = await res.json();

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message);
  }

  return json.data;
}

// SWR fetcher
export const swrFetcher = (url: string) => apiFetch(url);

// 分页类型安全访问 — 不再需要 (res as any).data || ...
export async function apiFetchPage<T>(url: string): Promise<PageResponse<T>> {
  const res = await fetch(`${BASE_URL}${url}`);
  const json = await res.json(); // { code, message, data, total, page, page_size }
  if (json.code !== 0) throw new ApiError(json.code, json.message);
  return {
    items: json.data as T[],
    total: json.total,
    page: json.page,
    pageSize: json.page_size,
  };
}
```

### A.3 JWT Base64URL 安全解码（修复 P0-1）

```typescript
// lib/auth.ts
export function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;

    const payload = parts[1];
    // Base64URL → Base64 兼容转换
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const json = atob(base64);
    return JSON.parse(json);
  } catch {
    return null;
  }
}

export function isTokenExpired(token: string): boolean {
  const payload = decodeJwtPayload(token);
  if (!payload?.exp) return true;
  // 60 秒缓冲
  return payload.exp * 1000 < Date.now() + 60_000;
}
```

---

## 附录 B：API 端点映射表

全部 62 个端点 → 新前端调用方式：

| 端点 | 方法 | 新调用 | SWR Key |
|------|------|--------|---------|
| `/api/v1/auth/login` | POST | `loginServerAction()` | — |
| `/api/v1/auth/refresh` | POST | `refreshToken()` | — |
| `/api/v1/auth/me/change-password` | POST | `apiFetch()` | — |
| `/api/v1/auth/me/logout` | POST | `logoutAction()` | — |
| `/api/v1/portal/chat-sessions` | POST | `apiFetch()` | — |
| `/api/v1/portal/chat-sessions/:id/stream` | POST | `useChat().sendQuestion()` | — (SSE) |
| `/api/v1/portal/chat-sessions` | GET | `useSWR()` | `/portal/chat-sessions?page=...` |
| `/api/v1/portal/chat-sessions/:id` | GET/DELETE | `useSWR()` / `apiFetch()` | `/portal/chat-sessions/:id` |
| `/api/v1/portal/chat-sessions/:id/feedback` | POST | `apiFetch()` | — |
| `/api/v1/portal/tickets` | GET/POST | `useSWR()` / `apiFetch()` | `/portal/tickets?page=...` |
| `/api/v1/portal/tickets/:id` | GET | `useSWR()` | `/portal/tickets/:id` |
| `/api/v1/portal/tickets/:id/supplement` | PATCH | `apiFetch()` | — |
| `/api/v1/portal/knowledge-bases` | GET | `useSWR()` | `/portal/knowledge-bases` |
| `/api/v1/portal/messages` | GET | `useSWR()` | `/portal/messages?page=...` |
| `/api/v1/portal/messages/:id/read` | PUT | `apiFetch()` | — |
| `/api/v1/portal/messages/unread-count` | GET | `useSWR()` | `/portal/messages/unread-count` |
| `/api/v1/admin/tickets` | GET | `useSWR()` | `/admin/tickets?page=...&status=...` |
| `/api/v1/admin/tickets/:id` | GET/PATCH | `useSWR()` / `apiFetch()` | `/admin/tickets/:id` |
| `/api/v1/admin/tickets/:id/records` | POST | `apiFetch()` | — |
| `/api/v1/admin/tickets/:id/knowledge-candidate` | POST | `apiFetch()` | — |
| `/api/v1/admin/knowledge-bases` | GET/POST | `useSWR()` / `apiFetch()` | `/admin/knowledge-bases` |
| `/api/v1/admin/knowledge-bases/:id` | PUT/DELETE | `apiFetch()` | — |
| `/api/v1/admin/knowledge-bases/:kbId/articles` | GET/POST | `useSWR()` / `apiFetch()` | `/admin/knowledge-bases/:kbId/articles?...` |
| `/api/v1/admin/articles/:id` | GET/PUT | `useSWR()` / `apiFetch()` | `/admin/articles/:id` |
| `/api/v1/admin/articles/:id/submit-review` | POST | `apiFetch()` | — |
| `/api/v1/admin/articles/:id/review` | POST | `apiFetch()` | — |
| `/api/v1/admin/articles/:id/publish` | POST | `apiFetch()` | — |
| `/api/v1/admin/articles/:id/disable` | POST | `apiFetch()` | — |
| `/api/v1/admin/articles/:id/enable` | POST | `apiFetch()` | — |
| `/api/v1/admin/knowledge-bases/:kbId/documents/upload` | POST | `FormData` + `apiFetch()` | — |
| `/api/v1/admin/knowledge-bases/:kbId/documents/:id/status` | GET | `useSWR({ refreshInterval: 2000 })` | 轮询 2s |
| `/api/v1/admin/knowledge-bases/:kbId/documents/:id/retry` | POST | `apiFetch()` | — |
| `/api/v1/admin/users` | GET/POST | `useSWR()` / `apiFetch()` | `/admin/users?page=...` |
| `/api/v1/admin/users/:id` | GET/PUT | `useSWR()` / `apiFetch()` | `/admin/users/:id` |
| `/api/v1/admin/users/:id/freeze` | PATCH | `apiFetch()` | — |
| `/api/v1/admin/users/:id/unfreeze` | PATCH | `apiFetch()` | — |
| `/api/v1/admin/roles` | GET/POST | `useSWR()` / `apiFetch()` | `/admin/roles?page=...` |
| `/api/v1/admin/roles/:id` | GET/PUT/DELETE | `useSWR()` / `apiFetch()` | `/admin/roles/:id` |
| `/api/v1/admin/roles/:id/menus` | PUT | `apiFetch()` | — |
| `/api/v1/admin/menus` | GET | `useSWR()` | `/admin/menus` |
| `/api/v1/admin/llm-configs` | GET/POST | `useSWR()` / `apiFetch()` | `/admin/llm-configs` |
| `/api/v1/admin/llm-configs/:id` | GET/PUT/DELETE | `useSWR()` / `apiFetch()` | `/admin/llm-configs/:id` |
| `/api/v1/admin/llm-configs/:id/test` | POST | `apiFetch()` | — |
| `/api/v1/admin/configs/:key` | GET/PUT | `useSWR()` / `apiFetch()` | `/admin/configs/:key` |
| `/api/v1/admin/audit-logs` | GET | `useSWR()` | `/admin/audit-logs?page=...&...` |
| `/api/v1/admin/dashboard/stats` | GET | `useSWR()` | `/admin/dashboard/stats` |
| `/api/v1/admin/dashboard/trends` | GET | `useSWR()` | `/admin/dashboard/trends?start=...&end=...` |

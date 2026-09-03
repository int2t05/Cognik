# OpsMind 前端设计系统 — shadcn/ui

> 组件库统一：shadcn/ui（Radix + Tailwind v4）+ 统一 AppShell。UI 层基于 shadcn/ui + 定制 variant（AppShell 内含内联搜索组件）。

## 1. 设计目标

- **组件库统一**：采用 shadcn/ui（组件代码生成进仓库，非黑盒依赖）+ 定制 variant；AppShell 内含内联搜索组件。
- **专业工具风格**：正文 13px 高信息密度、中性灰阶、靛蓝强调色、中性小圆角、克制阴影。
- **统一 Shell**：顶栏（品牌+内联全局搜索+主题+账号）+ 可折叠侧栏（分区 nav）+ main。Portal/Admin 共用单一 `AppShell`。

## 2. Token 映射（shadcn 语义 → 项目 Token）

`globals.css` 中 shadcn 语义 token 映射到项目设计 token，`@theme inline` 暴露为 Tailwind utility：

| shadcn token | 映射到 | 说明 |
|---|---|---|
| `--primary` / `--color-primary` | `var(--color-accent)` #5b5bd6 | 强调色 = shadcn primary |
| `--accent` / `--color-accent` | `var(--color-tile-1)` #f4f4f5 | 静音 hover bg（Radix accent 概念） |
| `--background` | `var(--color-parchment)` #fafafa | 页面画布 |
| `--card` / `--popover` | `var(--color-canvas)` #ffffff | 卡片/弹层白底 |
| `--border` / `--input` | `var(--color-hairline)` #e4e4e7 | 分割线 |
| `--ring` | `var(--color-accent-focus)` | 聚焦环 |
| `--destructive` | `var(--color-error)` #dc2626 | 危险红 |
| `--muted` / `--secondary` | `var(--color-tile-1)` | 静音瓦片 |
| radius | `--radius-lg`=10px / `--radius-md`=8px / `--radius-sm`=6px | 中性小圆角 |

**暗色**：`@custom-variant dark (&:where([data-theme="dark"], [data-theme="dark"] *))`，对齐项目 `data-theme` 属性。主题 cookie 预读在 root layout 服务端（防 FOUC）。

**字体**：`@theme` 块声明 `--font-sans` / `--font-mono`，使 Tailwind `font-sans` / `font-mono` utility 自动映射到项目字体变量。`font-sans` = `system-ui, -apple-system, 'Segoe UI', 'Inter Variable', sans-serif`。

## 3. 组件库

### 基础组件（shadcn 标准生成）
button / card / input / textarea / label / table / dialog / badge / select / dropdown-menu / toggle / skeleton / sonner / checkbox / separator。

### 定制（设计系统配置）
| 组件 | 定制 |
|---|---|
| `badge.tsx` | 扩展 cva 加 5 语义 variant（success/warning/error/info/neutral），映射 `--badge-*-bg/text`，颜色+图标双编码 |
| `toggle.tsx` | 加 `pill` variant + `pill-md`/`pill-sm` size，用于 segmented control / 筛选 |
| `button.tsx` | base `rounded-md` + 加 `menu` variant（导航栏，ink 文字） |
| `skeleton.tsx` | base 用 `skeleton-shimmer`（扫光动画，非默认 pulse） |
| `card.tsx` | Card base：10px radius + hairline border + canvas bg + p-6 |
| `dialog.tsx` | DialogContent `bg-card`，overlay 用 `var(--color-overlay)` |
| `sonner.tsx` | 用 `@/hooks/useTheme`，配置 top-right/richColors/closeButton/visibleToasts=3 |
| `data-table-pagination.tsx` | 居中页码分页：左侧计数+页大小 + 居中页码导航 |

### 定制组件
| 组件 | 说明 |
|---|---|
| `lib/utils.ts` `cn()` | clsx + tailwind-merge，类名合并标准入口 |
| `form-field.tsx` `Field` | Label+children+error 容器，useId+cloneElement 注入 id/aria |
| `data-table.tsx` `DataTable` | shadcn Table + TanStack Table v9，内置 skeleton/empty 态 |
| `data-table-pagination.tsx` | 分页器 + Select 页大小 |

## 4. AppShell

`components/layout/AppShell.tsx`：Portal/Admin 共用。
- **顶栏**：品牌 + 内联全局搜索（输入即时过滤导航+快捷操作，⌘K 聚焦）+ 主题切换 + 跨跳 + AccountSwitcher
- **侧栏**：可折叠（56/240px，localStorage 持久化 + <1024px 自动折叠），nav 嵌套子菜单展开，active 顶层 sibling 检查（避免 /portal/tickets 误匹配 /portal/tickets/new）
- **main**：`flex-1 min-h-0 overflow-hidden`，内容由页面自行管理 padding 和滚动

`PortalLayout`：静态 NAV（用户菜单 + 消息未读 badge）+ 管理员条件渲染管理分区（来自 `useAuth().menus`，`hasAdminAccess` 判断）。
`AdminLayout`：复用 PortalLayout（管理员分区由权限自动渲染）。

## 5. Toast

`sonner` 替换手写 `ToastProvider`。`Toaster` 在 Providers 渲染，调用方 `import { toast } from 'sonner'` 直接调用（success/error/warning/info API 一致）。

## 6. 行为契约（Chat 5 条，严格保留）

1. `ChatStreamProvider` 留在 `app/portal/layout.tsx` 层（跨路由流持久化 + 自动 resume）。
2. rAF token 批处理（`rafRefs`/`reasoningRafRefs` 双槽位）——不可改 per-token setState。
3. seq 去重（`evt.seq > s.lastSeq`）——resume 安全。
4. 条件虚拟化（`messages.length > 50` 阈值）+ 双渲染路径。
5. cookie 预读主题（root layout 服务端，防 FOUC）。

## 7. 组件清单

UI 层组件按功能分类：

### 表单类
- `Input` / `Textarea` + `Field`（Label + error + aria 统一包裹）
- `Select`（Radix，替代原生 select）
- `Checkbox`（Radix，替代原生 checkbox）
- `Label`

### 数据展示类
- `DataTable`（shadcn Table + TanStack Table v9，内置 skeleton/empty 态）
- `DataTablePagination`（居中页码分页 + Select 页大小）
- `Badge` / `StatusBadge`（5 语义 variant，颜色+图标双编码）
- `Card`（10px radius + hairline border + canvas bg）
- `StatCard` / `TrendChart`（指标卡 + 趋势图）
- `FilterBar`（列表筛选栏）

### 反馈类
- `Dialog` / `ConfirmDialog`（compound，Radix 焦点陷阱/escape/scroll-lock）
- `Toaster`（sonner）
- `Skeleton`（shimmer 扫光动画）
- `InlineError`（inline / full-page 双模式，统一错误提示）
- `EmptyState` / `ErrorFallback`
- `ErrorBoundary` / `SectionErrorBoundary`（全局 + 局部错误边界）

### 导航/布局类
- `Button`（default/outline/destructive/ghost/secondary/link/menu variant + default/xs/sm/lg/icon/icon-xs/icon-sm/icon-lg 尺寸）
- `Toggle`（pill variant 用于 segmented control / 筛选）
- `DropdownMenu`（AccountSwitcher / 账号切换）
- `AppShell`（统一 Shell：顶栏+侧栏+main）
- `PageTitle`（页面标题）
- `AccountSwitcher`（账号切换）
- `DynamicTitle`（动态浏览器标题）

### Chat 类
- `ChatInput` / `ChatMessage` / `ChatPipeline`（输入框 / 消息 / 流水线进度）
- `BatchSelectCheckbox`（批量选择）

UI 层由 shadcn/ui + 定制 variant 构成，AppShell 内含内联搜索组件。全部路由 build + tsc 通过。

# OpsMind 前端设计系统 — shadcn/ui × Apple Design（方向 C）

> 重构分支：`v1.0`。将原 11 个手写 `Apple*` 组件全部替换为 shadcn/ui（Radix + Tailwind v4），统一为方向 C AppShell（顶栏品牌+⌘K + 分区侧栏），保留 Apple Design 视觉语言。

## 1. 设计目标

- **组件库统一**：采用 shadcn/ui（组件代码生成进仓库，非黑盒依赖），消除手写 `Apple*` 维护负担。
- **风格不变**：Apple Design token（`#0066cc` 品牌、parchment 画布、17px 正文、亮暗双主题、负 letter-spacing 标题）原样保留。
- **方向 C**：顶栏（品牌+折叠钮+主题+跨跳+账号）+ 可折叠侧栏（分区 nav）+ ⌘K 全局命令面板。Portal/Admin 共用单一 `AppShell`，消除原双 shell 割裂。

## 2. Token 映射（shadcn 语义 → Apple Design）

`globals.css` 中 shadcn 语义 token 映射到 Apple 原始 token，`@theme inline` 暴露为 Tailwind utility：

| shadcn token | 映射到 | 说明 |
|---|---|---|
| `--primary` / `--color-primary` | `var(--color-accent)` #0066cc | 品牌色 = shadcn primary（`bg-primary` 即品牌蓝） |
| `--accent` / `--color-accent` | `var(--color-tile-1)` #f0f0f2 | 静音 hover bg（Radix accent 概念，非品牌） |
| `--background` | `var(--color-parchment)` | 页面画布 |
| `--card` / `--popover` | `var(--color-canvas)` | 卡片/弹层白底 |
| `--border` / `--input` | `var(--color-hairline)` | 发丝边 |
| `--ring` | `var(--color-accent-focus)` | 聚焦环 |
| `--destructive` | `var(--color-error)` | 危险红 |
| `--muted` / `--secondary` | `var(--color-tile-1)` | 静音瓦片 |
| radius | `--radius-lg`=18px / `--radius-md`=11px / `--radius-sm`=8px | 复用 Apple 半径，shadcn `rounded-lg` 自动 18px |

**冲突处理**：shadcn `--accent`（静音）≠ 项目 `--color-accent`（品牌）。`@theme inline` 的 `--color-accent` 被 `:root` 品牌色覆盖，故 `bg-accent` 运行时解析为品牌色（Apple 风格 hover 淡蓝），非 shadcn 默认静音灰。

**暗色**：`@custom-variant dark (&:where([data-theme="dark"], [data-theme="dark"] *))`，对齐项目 `data-theme` 属性（非 shadcn 默认 `.dark` class）。主题 cookie 预读在 root layout 服务端（防 FOUC）。

## 3. 组件库

### 基础组件（shadcn 标准生成）
button / card / input / textarea / label / table / dialog / badge / tabs / select / dropdown-menu / sheet / tooltip / command / pagination / toggle / skeleton / sonner。

### 定制（设计系统配置，非 shim）
| 组件 | 定制 |
|---|---|
| `badge.tsx` | 扩展 cva 加 5 语义 variant（success/warning/error/info/neutral），映射 `--badge-*-bg/text`，颜色+图标双编码 |
| `toggle.tsx` | 加 `pill` variant + `pill-md`/`pill-sm` size，复刻 Apple segmented-control（pill 圆角+border+选中品牌色填充白字） |
| `button.tsx` | base `rounded-full`（Apple 全 pill）+ 加 `menu` variant（导航栏，ink 文字）+ `active:scale-95` |
| `skeleton.tsx` | base 用 `skeleton-shimmer`（Apple 扫光动画，非 shadcn 默认 pulse）+ radius-lg |
| `card.tsx` | Card base 改 Apple 等价（18px radius + hairline border + canvas bg + p-6，去 flex/gap/shadow） |
| `dialog.tsx` | DialogContent `bg-background` → `bg-card`（dialog 内容用 canvas 白） |
| `sonner.tsx` | 用 `@/hooks/useTheme`（非 next-themes），配置 top-right/richColors/closeButton/visibleToasts=3 |

### 新建组件
| 组件 | 说明 |
|---|---|
| `lib/utils.ts` `cn()` | clsx + tailwind-merge，shadcn 标准入口（项目首个 cn，替代手写 filter(Boolean).join） |
| `form-field.tsx` `Field` | Label+children+error 容器，useId+cloneElement 注入 id/aria，保持 label-input-error a11y 关联（替代 AppleInput 内聚） |
| `data-table.tsx` `DataTable` | shadcn Table + TanStack Table v9（useTable+tableFeatures+table.FlexRender），内置 skeleton/empty 态 |
| `data-table-pagination.tsx` | shadcn Pagination + Select 页大小，保留 getVisiblePages 椭圆逻辑 |

## 4. AppShell（方向 C 统一 Shell）

`components/layout/AppShell.tsx`：Portal/Admin 共用。
- **顶栏**：折叠钮 + 主题切换 + 跨跳（Portal→Admin / Admin→Portal）+ AccountSwitcher
- **侧栏**：可折叠（68/240px，localStorage 持久化 + <1024px 自动折叠），nav 嵌套子菜单展开，active 顶层 sibling 检查（避免 /portal/tickets 误匹配 /portal/tickets/new）
- **main**：max-w-wide 居中 + SectionErrorBoundary
- **⌘K**：GlobalCommand（shadcn CommandDialog + cmdk），客户端过滤导航 + 快捷操作（主题/跨跳），AppShell 自动从 nav 构造 groups

`PortalLayout`：静态 NAV（含消息未读 badge）→ AppShell，Portal 首次获得侧栏。
`AdminLayout`：`useAuth().menus` → NavItem（ICON_MAP/FRONTEND_ROUTES 保留，去重）→ AppShell。

## 5. Toast

`sonner` 替换手写 `ToastProvider`（84 行 context+定时器+堆叠）。`Toaster` 在 Providers 渲染，调用方 `import { toast } from 'sonner'` 直接调用（success/error/warning/info API 一致）。

## 6. 行为契约（Chat 5 条，重构中严格保留）

1. `ChatStreamProvider` 留在 `app/portal/layout.tsx` 层（跨路由流持久化 + 自动 resume）。
2. rAF token 批处理（`rafRefs`/`reasoningRafRefs` 双槽位）——不可改 per-token setState。
3. seq 去重（`evt.seq > s.lastSeq`）——resume 安全。
4. 条件虚拟化（`messages.length > 50` 阈值）+ 双渲染路径。
5. cookie 预读主题（root layout 服务端，防 FOUC）。

## 7. 迁移总结

原 11 个手写 `Apple*` 组件全部删除，替换为 shadcn/ui：

| 原 Apple* | 替换为 | 消费方数 |
|---|---|---|
| AppleBadge | Badge（+5 语义 variant） | 1 |
| AppleSkeleton | Skeleton（shimmer） | 2 |
| AppleChip | Toggle（pill variant） | 4 |
| AppleSpinner | lucide Loader2 + animate-spin | 9 |
| AppleDialog | Dialog compound | 5 |
| AppleCard | Card（Apple 等价） | 8 |
| AppleInput/Textarea | Input/Textarea + Field | 12 |
| AppleTable | DataTable（TanStack v9） | 7 |
| ApplePagination | DataTablePagination | 7 |
| AppleButton | Button（+menu variant, rounded-full） | 28 |
| SearchInput | inline Input + icon + clear | 1 |

净删除 ~1000 行手写组件代码。全部路由 build + tsc 通过，零 `Apple*` 引用残留。

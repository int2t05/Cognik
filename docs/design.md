# OpsMind 前端设计审计与优化建议

> 审计日期：2026-06-23
> 审计范围：`web/src/` 全部组件、样式、页面
> 参考标准：Apple Human Interface Guidelines + 用户 UI 理论体系

---

## 目录

1. [总体评价](#1-总体评价)
2. [字体体系审计](#2-字体体系审计)
3. [圆角体系审计](#3-圆角体系审计)
4. [留白与间距审计](#4-留白与间距审计)
5. [色彩体系审计](#5-色彩体系审计)
6. [组件逐项审计](#6-组件逐项审计)
7. [布局与导航审计](#7-布局与导航审计)
8. [交互与动效审计](#8-交互与动效审计)
9. [无障碍审计](#9-无障碍审计)
10. [暗色主题审计](#10-暗色主题审计)
11. [优化优先级矩阵](#11-优化优先级矩阵)

---

## 1. 总体评价

### 做得好的地方

- ✅ CSS 自定义属性体系完整，浅色/暗色双主题 Token 齐全
- ✅ Apple 设计 Token（Action Blue、Parchment、Pearl、Hairline）忠实还原
- ✅ 组件命名统一 `Apple*` 前缀，架构清晰
- ✅ 聚焦环（focus-visible）、active 微交互、reduced-motion 等无障碍基础到位
- ✅ 单一品牌色策略（仅 `--color-accent`）符合 Apple 规范
- ✅ 按钮 4 变体（pill/ghost/utility/pearl）覆盖主要场景
- ✅ 徽章颜色+图标双重编码，兼顾色觉障碍用户

### 核心问题概览

| 问题类别 | 严重程度 | 影响面 |
|----------|---------|--------|
| 字体层级偏离 Apple 标准 | 🔴 高 | 全局 |
| 圆角使用不一致 | 🔴 高 | 全局 |
| 输入框默认圆角错误 | 🟡 中 | 全部表单 |
| 留白密度不符合"留白即设计" | 🟡 中 | 全局 |
| 导航项圆角与用户理论冲突 | 🟡 中 | 侧栏/顶栏 |
| 骨架屏与内容圆角不匹配 | 🟢 低 | 加载态 |
| 缺少负 letter-spacing | 🟢 低 | 标题视觉 |

---

## 2. 字体体系审计

### 2.1 当前状态

```css
/* globals.css — 当前 Token */
--font-size-display: 72px;   /* 未使用 */
--font-size-hero: 28px;      /* PageTitle 使用 */
--font-size-headline: 21px;  /* Portal 标题 */
--font-size-title: 17px;     /* Dialog 标题 */
--font-size-body: 15px;      /* 正文 — ❌ 应该是 17px */
--font-size-caption: 13px;   /* 辅助文字 */
--font-size-fine: 12px;      /* 极小文字 */
```

### 2.2 Apple HIG 对照

| 用途 | Apple 标准 | OpsMind 当前 | 偏差 |
|------|-----------|-------------|------|
| Hero 标题 | 56px / 600 | 28px / 600 | **小 50%** |
| 二级标题 | 40px / 600 | 无对应 | 缺失 |
| 三级标题 | 34px / 600 | 无对应 | 缺失 |
| Tagline | 21px / 600 | 21px / 600（用作标题） | 用法错位 |
| 正文 | 17px / 400 | 15px / 400 | **小 2px** |
| 辅助文字 | 14px / 400 | 13px / 400 | 小 1px |
| 极小文字 | 12px / 400 | 12px / 400 | ✅ |

### 2.3 用户理论对照：字体完全统一

**原则：** 全站使用统一的字体层级，不允许同一语义层级出现多种字号。

**当前违规：**
1. `AppleButton` 硬编码 `font-sans`，而其他组件依赖全局 `body { font-family }` —— 存在字体继承不一致的风险
2. 表格内容使用 `text-caption`（13px），但 Apple HIG 表格内容应为 17px 正文
3. ChatMessage 使用 `text-body leading-relaxed`（15px + relaxed），与其他正文区不一致
4. Dialog 标题使用 `text-title`（17px），但 Apple Dialog 标题应为 17px Semibold——此处巧合正确，但语义混乱（`title` Token 在别处表示页面标题）

### 2.4 优化建议

**建议 1 — 重设字体层级 Token（🔴 高优先级）**

```css
:root {
  /* 标题层级 — 对齐 Apple HIG */
  --font-size-hero: 48px;        /* 首页/登录页大标题，原 28px → 48px */
  --font-size-display: 40px;     /* 页面主标题，原 72px → 40px */
  --font-size-headline: 28px;    /* 区块标题，原 21px → 28px */
  --font-size-title: 21px;       /* 卡片/对话框标题，原 17px → 21px */
  --font-size-body: 17px;        /* 正文，原 15px → 17px（Apple 核心） */
  --font-size-callout: 15px;     /* 次要正文（表单标签、列表摘要） */
  --font-size-caption: 14px;     /* 辅助文字，原 13px → 14px */
  --font-size-fine: 12px;        /* 极小文字 */

  /* 行高 — 严格对齐 Apple */
  --line-height-hero: 1.07;      /* 大标题紧致行高 */
  --line-height-display: 1.10;
  --line-height-headline: 1.14;
  --line-height-title: 1.19;
  --line-height-body: 1.47;      /* 正文舒适阅读行高（Apple 核心） */
  --line-height-callout: 1.4;
  --line-height-caption: 1.29;
}
```

**建议 2 — 标题添加负 letter-spacing（🟡 中优先级）**

Apple 的签名特征之一是标题使用负 letter-spacing。当前完全缺失。

```css
h1, .text-hero { letter-spacing: -0.015em; }    /* Hero/Display */
h2, .text-headline { letter-spacing: -0.01em; }  /* 区块标题 */
.text-title { letter-spacing: -0.008em; }        /* 卡片标题 */
```

**建议 3 — 正文必须是 17px（🔴 高优先级）**

Apple HIG 明确指出 "Body copy at 17px, not 16px"。当前 15px 的正文让 OpsMind 看起来像传统 SaaS 而不是 Apple 风格产品。17px 是实现 "阅读感而非扫描感" 的关键。

**影响面评估：** 改为 17px 会影响所有使用 `text-body` 的元素，卡片内边距、表格行高、输入框高度可能需要联动调整。

**建议 4 — 删除未使用的 `--font-size-display: 72px`（🟢 低优先级）**

如果不需要 72px 标题，应从 Token 中移除，避免后续开发者误用。

---

## 3. 圆角体系审计

### 3.1 当前 Token

```css
--radius-sm: 8px;    /* 侧栏菜单项、输入框默认、骨架屏、日期选择器 */
--radius-md: 11px;   /* 会话列表项、AppleButton utility/pearl */
--radius-lg: 18px;   /* 卡片、对话框、表格容器、聊天气泡、统计卡片 */
--radius-pill: 9999px; /* 主按钮、徽章、搜索输入、反馈按钮、筛选按钮 */
```

### 3.2 用户理论对照

> **胶囊弧度一致：** 圆角矩形四周弧度一定要一致
> **引导用户点击使用胶囊（pill），引导用户输出使用圆角矩阵（rounded rect）**

这是本次审计的核心理论框架。按此理论，UI 元素应分为两类：

| 类型 | 形状 | 圆角 | 用途 |
|------|------|------|------|
| 操作型（可点击） | 胶囊 Pill | `--radius-pill` | 按钮、标签、筛选器、菜单项 |
| 展示型（输出内容） | 圆角矩形 | `--radius-lg` (18px) | 卡片、对话框、气泡、输入框 |

### 3.3 圆角违规清单

以下是违反用户理论的实例：

| 位置 | 当前圆角 | 元素性质 | 应为 |
|------|---------|---------|------|
| `AdminLayout` 侧栏菜单项 | `rounded-sm` (8px) | 可点击导航 | `rounded-pill` |
| `PortalLayout` 顶栏导航 | `rounded-sm` (8px) | 可点击导航 | `rounded-pill` |
| `AppleInput` 默认状态 | `rounded-sm` (8px) | 输入区（展示型） | `rounded-lg` (18px) |
| `AppleInput` pill 变体 | `rounded-pill` | 搜索框（特殊） | `rounded-pill` ✅ |
| `AppleTable` 表头/单元格 | 无圆角 | 展示型 | 内部无需圆角 ✅ |
| `AppleTable` 外层容器 | `rounded-lg` (18px) | 展示型 | `rounded-lg` ✅ |
| `AppleButton utility` | `rounded-md` (11px) | 可点击 | `rounded-pill` |
| `AppleButton pearl` | `rounded-md` (11px) | 可点击 | `rounded-pill` |
| `TrendChart` 日期输入 | `rounded-sm` (8px) | 输入区 | `rounded-lg` |
| `TrendChart` 预设按钮 | `rounded-pill` | 可点击 | `rounded-pill` ✅ |
| `Skeleton` 骨架屏 | `rounded-sm` (8px) | 占位符 | 与所代表内容一致 |
| `ChatInput` 发送按钮 | `rounded-full` | 可点击 | `rounded-pill` ✅ |
| `ChatInput` 停止按钮 | `rounded-full` | 可点击 | `rounded-full` ✅ |
| `ChatMessage` 用户气泡 | `rounded-lg` + `rounded-tr-sm` | 展示型 | 不规整四角违反理论 |
| `ChatMessage` AI 气泡 | `rounded-lg` + `rounded-tl-sm` | 展示型 | 不规整四角违反理论 |
| `ConfirmDialog` 按钮容器 | 无独立圆角 | 可点击 | 由子按钮决定 ✅ |
| 登录页卡片 | `rounded-lg` (18px) | 展示型 | `rounded-lg` ✅ |
| `FilterBar` 选项按钮 | `rounded-pill` | 可点击 | `rounded-pill` ✅ |
| `ApplePagination` 页码 | `rounded-pill` | 可点击 | `rounded-pill` ✅ |
| `ApplePagination` 下拉 | `rounded-sm` (8px) | 输入区 | `rounded-lg` |

### 3.4 优化建议

**建议 5 — 统一操作型元素为 pill（🔴 高优先级）**

所有可点击的交互元素（导航菜单、按钮、筛选器、标签页）必须统一使用 `--radius-pill`。当前最突出的问题是：
- 侧栏和顶栏导航项使用 `rounded-sm` (8px)——这违反了用户理论的核心原则
- `utility` 和 `pearl` 按钮使用 `rounded-md` (11px)——它们仍是可点击按钮

```css
/* 修复示例 — 侧栏菜单项 */
/* 当前：rounded-[var(--radius-sm)] → 应改为：rounded-[var(--radius-pill)] */
```

**建议 6 — 统一展示型元素为 lg（🔴 高优先级）**

所有展示/输入区域（卡片、输入框、对话框、图表容器）必须统一使用 `--radius-lg` (18px)。当前 `AppleInput` 默认使用 `rounded-sm` (8px) 是最大的违规。

```tsx
// AppleInput.tsx — 当前
'w-full h-11 px-4 text-body rounded-[var(--radius-sm)] border ...'
// 应改为
'w-full h-11 px-4 text-body rounded-[var(--radius-lg)] border ...'
```

**建议 7 — 聊天气泡四角弧度一致（🟡 中优先级）**

当前用户气泡 `rounded-tr-sm`（右上小圆角）和 AI 气泡 `rounded-tl-sm`（左上小圆角）模仿了 iMessage 风格，但违反了用户"圆角矩形四周弧度一定要一致"的理论。

选项 A（推荐）：改为标准 `rounded-lg` 四角一致，区分通过颜色（蓝色 vs 白色）和位置（左/右对齐）实现。
选项 B：保留当前不对称设计，在用户理论框架中将其视为"对话气泡"这一特殊类别。

**建议 8 — 删除 `--radius-md` (11px) 的使用（🟡 中优先级）**

按照严格的 pill-vs-lg 二分法，`11px` 作为中间值不应存在。所有当前使用 `--radius-md` 的地方应归类到 pill（可点击）或 lg（展示型）。

但注意：Apple 自身使用 11px 的 Pearl Button。如果要保留这个中间值，需要有明确的语义——仅在 Pearl Button 这种"半交互半展示"的场景中使用。

---

## 4. 留白与间距审计

### 4.1 用户理论：留白才是设计，每一个元素都要有意义

**原则：** 空间本身就是设计元素。空白不是"没东西放"，而是"故意不放"。每个像素的存在都要有理由。

### 4.2 当前问题

**问题 1 — 页面内容区 padding 偏紧**

```tsx
// AdminLayout: <main className="flex-1 p-5 ...">
// PortalLayout: <main className="w-full max-w-wide mx-auto p-5">
```

当前 `p-5` = 20px。Apple 的标准是 24px（`--spacing-lg`）作为内容区 padding。对于数据密集型后台，20px 显得拥挤。

**问题 2 — 卡片内边距不统一**

- `AppleCard` 默认 `padding='20px'` → 应为 24px
- `StatCard` 使用 `p-5` (20px) → 应为 24px
- `TrendChart` 使用 `p-5` (20px) → 应为 24px

**问题 3 — 侧栏宽度偏窄**

```
SIDEBAR_EXPANDED_WIDTH = 220px
```

对于中文菜单（通常 4-6 字），220px 偏紧。Apple 的侧栏通常在 240-260px。当前菜单文字在 220px 内被截断风险高。

**问题 4 — 表格单元格 padding 偏紧**

```tsx
// AppleTable: px-3 py-2.5 (12px / 10px)
```

对于数据密集型后台表格，12px 的水平 padding 显得很紧。Apple HIG 建议表格行高至少 44px，当前 `py-2.5`(10px) + 文字行高难以达到。

**问题 5 — 对话框内容区 padding 不统一**

```tsx
// AppleDialog: px-5 py-4 (内容区 20px/16px) vs px-5 pt-4 pb-0 (标题区)
```

标题区的 `pb-0` 意味着标题直接贴着内容，缺少呼吸感。Apple 的对话框标题与内容之间有明确间距。

**问题 6 — 缺少段落间垂直节奏**

全站没有定义段落间距 Token。多个页面中文本块之间的间距靠 `mb-2`/`mb-3` 等工具类手动维护，容易出现不一致。

### 4.3 优化建议

**建议 9 — 建立统一的间距 Token 体系（🟡 中优先级）**

```css
:root {
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 12px;
  --spacing-lg: 24px;    /* 卡片内边距、内容区 padding */
  --spacing-xl: 32px;
  --spacing-xxl: 48px;
  --spacing-section: 64px; /* 区块间距 */
}
```

**建议 10 — 增加全局内容区 padding（🟡 中优先级）**

```tsx
// 从 p-5 (20px) → p-6 (24px)
<main className="flex-1 p-6 max-w-wide w-full mx-auto">
```

**建议 11 — 卡片内边距统一为 24px（🟡 中优先级）**

- `AppleCard` 默认 `padding` 从 `'20px'` → `'24px'`
- `StatCard` 从 `p-5` → `p-6`
- `TrendChart` 从 `p-5` → `p-6`

**建议 12 — 表格行高增大至 44px 最小触控区域（🟡 中优先级）**

```tsx
// AppleTable: 从 px-3 py-2.5 → px-4 py-3
// 配合 body 从 15px → 17px，增加整体可读性
```

**建议 13 — 侧栏宽度增加至 240px（🟢 低优先级）**

```tsx
const SIDEBAR_EXPANDED_WIDTH = 240; // 从 220 → 240
```

**建议 14 — 对话框标题与内容之间增加间距（🟢 低优先级）**

```tsx
// AppleDialog 标题区: 从 pb-0 → pb-2
<Dialog.Title className="px-6 pt-5 pb-2 ...">
```

---

## 5. 色彩体系审计

### 5.1 Token 审计

| Token | 浅色值 | 暗色值 | 问题 |
|-------|--------|--------|------|
| `--color-accent` | #0066cc | #2997ff | ✅ Apple 标准 |
| `--color-accent-hover` | #0055aa | #40a9ff | ✅ |
| `--color-canvas` | #ffffff | #1d1d1f | ✅ |
| `--color-parchment` | #f5f5f7 | #161618 | ✅ |
| `--color-pearl` | #fafafc | #2a2a2c | ✅ |
| `--color-ink` | #1d1d1f | #f5f5f7 | ✅ |
| `--color-text-muted-48` | #6d6d6d | #999999 | ⚠️ 浅色值由 #7a7a7a 改为 #6d6d6d |
| `--color-error` | #dc2626 | #ff3b30 | ⚠️ 不统一：浅色用 Tailwind red，暗色用 Apple SF red |

### 5.2 问题

**问题 7 — 错误色不一致**

浅色主题错误色 `#dc2626`（Tailwind Red-600），暗色主题 `#ff3b30`（Apple SF Red）。两个色值色相不同（358° vs 354°），饱和度不同。

**问题 8 — `--color-text-muted-48` 偏离 Apple 标准**

Apple 使用 `#7a7a7a`（48% 透明度等效），当前使用 `#6d6d6d`。浅色背景下对比度略高于 Apple 标准（但仍在 WCAG AA 范围内）。

**问题 9 — 缺少 `--color-primary-on-dark` Token**

Apple 在暗色表面上使用 `#2997ff`（Sky Link Blue）作为链接色。当前 `[data-theme="dark"]` 中 `--color-accent` 已设为 `#2997ff`，但缺少独立的"暗色表面链接色"。如果将来有深色卡片嵌套在浅色页面中，会出问题。

### 5.3 优化建议

**建议 15 — 统一错误色为 Apple SF Red（🟢 低优先级）**

```css
--color-error: #ff3b30; /* Apple SF Red — 浅色/暗色统一 */
```

**建议 16 — 增加 `--color-accent-on-dark` 独立 Token（🟢 低优先级）**

```css
--color-accent-on-dark: #2997ff; /* 暗色表面专用链接色 */
```

---

## 6. 组件逐项审计

### 6.1 AppleButton

**当前状态：** 4 种变体（pill/ghost/utility/pearl），使用 CSS 变量。

| 变体 | 圆角 | 问题 |
|------|------|------|
| pill | `rounded-pill` | ✅ |
| ghost | `rounded-pill` | ✅ |
| utility | `rounded-md` (11px) | ❌ 应为 pill |
| pearl | `rounded-md` (11px) | ❌ 应为 pill |

**建议 17 — utility/pearl 按钮改为 pill 圆角（🔴 高优先级）**

依据用户理论，所有可点击按钮统一 pill。Apple 自身的 Pearl Button 使用 11px 是特例——但在 OpsMind 的紧凑后台场景中，统一 pill 更能建立一致的点击暗示。

```tsx
// 修复
utility: '... rounded-[var(--radius-pill)] ...',
pearl: '... rounded-[var(--radius-pill)] ...',
```

**建议 18 — 按钮增加高度变体（🟡 中优先级）**

当前按钮高度依赖于 padding + 文字大小。建议增加显式的高度变体：

```tsx
// 紧凑型（表格操作）：h-8 (32px)
// 标准型（表单按钮）：h-10 (40px)
// 大型（登录页 CTA）：h-12 (48px)
```

### 6.2 AppleInput

**当前状态：** 默认 `rounded-sm` (8px)，pill 变体 `rounded-pill`。

**问题：** 默认输入框使用 8px 圆角，过于方正，不符合用户理论的"展示型使用圆角矩形"。

**建议 19 — 默认输入框改为 lg 圆角（🔴 高优先级）**

```tsx
// 当前
'w-full h-11 px-4 text-body rounded-[var(--radius-sm)] border ...'
// 改为
'w-full h-11 px-4 text-body rounded-[var(--radius-lg)] border ...'
```

**建议 20 — 输入框高度统一为 44px（🟡 中优先级）**

当前 `h-11` = 44px ✅ 正确。但 `ChatInput` 中的输入框同样是 `h-11` 却是 pill 圆角——如果改为 `rounded-lg`，需要确保视觉协调。

### 6.3 AppleCard

**当前状态：** 18px 圆角 + hairline 边框，正确。

**建议 21 — 默认 padding 改为 24px（🟡 中优先级）**

```tsx
// 从 padding = '20px' → '24px'
```

**建议 22 — hover 阴影使用 CSS 变量（🟢 低优先级）**

```tsx
// 当前硬编码
hover:shadow-[0_2px_12px_rgba(0,0,0,0.08)]
// 改为
hover:shadow-[var(--shadow-card-hover)]
```

### 6.4 AppleTable

**当前状态：** 18px 外层容器 + 无边框表头底线 + 行悬浮。

**建议 23 — 表格行高增大（🟡 中优先级）**

配合正文 17px，行 padding 从 `py-2.5` (10px) → `py-3` (12px)。

**建议 24 — 表头文字使用 14px（🟢 低优先级）**

当前表头使用 `text-caption`（13px → 将改为 14px），正确。

### 6.5 AppleDialog

**当前状态：** 18px 圆角、阴影、backdrop-blur。

**建议 25 — 标题字号对齐新 Token（🟡 中优先级）**

当前使用 `text-title`（17px），改为后用 `text-title`（21px）——或保留 17px 作为对话框标题专用大小。

对话框标题 21px 可能过大。建议：
- 对话框标题：17px Semibold（保留）
- 页面标题：28px Semibold（新 headline）
- 卡片标题：21px Semibold（新 title）

### 6.6 ChatMessage（聊天气泡）

**当前状态：** 用户蓝色气泡 + AI 白色气泡，不对称小角。

**建议 26 — 气泡四角统一（🟡 中优先级）**

见建议 7。

**建议 27 — AI 气泡增加 subtle 背景区分（🟢 低优先级）**

当前 AI 气泡使用 `bg-[var(--color-canvas)]`（白色），在 parchment 背景上已足够区分。但如果改为纯白背景页面，建议使用 `bg-[var(--color-pearl)]` 增加微妙区分。

### 6.7 ChatInput

**当前状态：** 居中 pill 输入框 + 发送/停止按钮。

**建议 28 — 发送按钮区分圆形 vs 胶囊（🟢 低优先级）**

当前发送按钮使用 `rounded-full`（正圆），而停止按钮也是正圆。两者形状一致但功能相反（发送 vs 停止），这是合理的——正圆暗示"单一明确操作"，pill 暗示"文字标签操作"。✅ 当前设计正确。

### 6.8 StatCard（统计卡片）

**当前状态：** 18px 圆角 + hairline + hover 微阴影。

**建议 29 — 数值字号对齐 Token（🟡 中优先级）**

当前数值使用 `text-hero font-bold`（28px Bold）。改为后 `text-hero` 变为 48px——对于卡片内数值过大。建议增加专用 Token：

```css
--font-size-metric: 32px; /* 指标卡数值专用 */
```

### 6.9 FilterBar

**当前状态：** Pill 按钮组，激活态蓝色填充。✅ 完全符合用户理论。

### 6.10 ApplePagination

**当前状态：** Pill 页码按钮 + 8px 下拉框。

**建议 30 — 页码下拉框改为 lg 圆角（🟡 中优先级）**

```tsx
// 从 rounded-[var(--radius-sm)] → rounded-[var(--radius-lg)]
```

### 6.11 TrendChart

**当前状态：** Pill 预设按钮 + 8px 日期输入框。

**建议 31 — 日期输入框改为 lg 圆角（🟡 中优先级）**

```tsx
// 从 rounded-[var(--radius-sm)] → rounded-[var(--radius-lg)]
```

### 6.12 AppleBadge / StatusBadge

**当前状态：** Pill 圆角 + 语义色 + 图标。✅ 完全符合用户理论。

### 6.13 Skeleton / AppleSkeleton

**当前状态：** `rounded-sm` (8px)。

**建议 32 — 骨架屏圆角与所代表内容一致（🟢 低优先级）**

```tsx
// 如果骨架屏代表卡片内容 → 使用 rounded-lg
// 如果骨架屏代表按钮 → 使用 rounded-pill
// 不应固定为 rounded-sm
```

---

## 7. 布局与导航审计

### 7.1 侧栏 (AdminLayout)

**当前状态：** 固定侧栏 + 折叠动画 + 嵌套菜单。

**问题：**
- 菜单项 `rounded-sm` (8px) 违反用户 pill 理论
- 侧栏宽度 220px 对中文偏紧
- 折叠态 64px 仅显示图标——正确 ✅

**建议 33 — 菜单项改为 pill 圆角（🔴 高优先级）**

```tsx
// 从 rounded-[var(--radius-sm)] → rounded-[var(--radius-pill)]
```

**建议 34 — 活跃菜单项指示器优化（🟢 低优先级）**

当前活跃菜单项使用 `shadow-[inset_4px_0_0_var(--color-accent)]`（左侧 4px 蓝色竖线）。这在 pill 圆角下会与弧形边缘冲突——pill 形状不适合左侧竖线指示器。

选项 A：移除竖线，活跃态仅靠背景色区分。
选项 B：保留竖线但改为 2px，减小视觉冲突。

### 7.2 顶栏 (AdminLayout)

**当前状态：** 半透明背景 + backdrop-blur + sticky。

与 Apple 的 frosted sub-nav 一致。✅

### 7.3 顶栏 (PortalLayout)

**当前状态：** 导航按钮 `rounded-sm` (8px) 违反理论。

**建议 35 — Portal 导航改为 pill（🔴 高优先级）**

```tsx
// 从 rounded-[var(--radius-sm)] → rounded-[var(--radius-pill)]
```

### 7.4 对话页布局

**当前状态：** 桌面端侧栏 240px + 对话区，移动端 overlay。

侧栏使用 `bg-[var(--color-parchment)]` 与主区域背景一致。如果能用微妙的不同背景色区分（如 `bg-[var(--color-canvas)]`），层次更清。

**建议 36 — 对话侧栏背景与主区域区分（🟢 低优先级）**

---

## 8. 交互与动效审计

### 8.1 当前动效

- ✅ 按钮 active: `transform: scale(0.95)` — Apple 标准
- ✅ 登录卡片入场动画 — 精致
- ✅ toast 入场动画 — fadeIn + translateY
- ✅ 侧栏折叠 transition 250ms — 流畅
- ✅ `prefers-reduced-motion` — 已处理

### 8.2 缺失的动效

**建议 37 — 增加页面切换过渡（🟢 低优先级）**

当前页面间切换是瞬时的——建议增加 subtle fade 过渡（150ms）。

**建议 38 — 增加对话框入场动画（🟢 低优先级）**

当前 `AppleDialog` 无入场动画。建议增加 `@keyframes dialog-enter`：

```css
@keyframes dialog-enter {
  from { opacity: 0; transform: translate(-50%, -48%) scale(0.96); }
  to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
```

**建议 39 — 增加悬浮卡片的 hover 过渡（🟢 低优先级）**

当前 `AppleCard` hover 有 `transition-shadow`，但缺少 `transition-transform`。建议增加 subtle lift：

```css
.card-interactive:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-card-hover);
}
```

### 8.3 用户理论：引导点击用胶囊

**原则解读：** 用户看到胶囊形状就知道"可以点"，看到圆角矩形就知道"这里是内容/输出"。这是一种视觉语言约定。

**当前实现评估：**
- 筛选按钮（FilterBar）✅ 使用 pill
- 主按钮（pill variant）✅ 使用 pill
- 分页按钮 ✅ 使用 pill
- 反馈按钮 ✅ 使用 pill
- 侧栏菜单 ❌ 使用 8px 圆角 → 应改为 pill
- 顶栏导航 ❌ 使用 8px 圆角 → 应改为 pill
- 会话列表项 ❌ 使用 11px 圆角 → 应改为 pill 或 lg

**建议 40 — 建立形状语义约定文档（🟡 中优先级）**

在 `docs/prompts/ui.md` 中明确定义：
- Pill = "我是可以点击的"
- Rounded-lg (18px) = "我是内容/输出区域"
- 不允许出现 sm/md 圆角（在交互元素中）

---

## 9. 无障碍审计

### 9.1 当前做得好的

- ✅ `focus-visible` 全局样式
- ✅ `aria-label` 在无文字按钮上普遍使用
- ✅ `aria-invalid` / `aria-describedby` 在输入框上使用
- ✅ `role="status"` / `role="alert"` 在 spinner 和错误提示上使用
- ✅ `prefers-reduced-motion` 媒体查询
- ✅ 徽章颜色+图标双重编码

### 9.2 问题

**问题 10 — 侧栏活跃菜单项缺少 `aria-current`**

当前活跃菜单项仅靠视觉样式（背景色+竖线），屏幕阅读器无法识别当前位置。

**建议 41 — 添加 `aria-current="page"`（🟡 中优先级）**

```tsx
<button aria-current={active ? 'page' : undefined} ...>
```

**问题 11 — 对话区虚拟列表缺少 live region**

当前消息列表使用 `@tanstack/react-virtual`，新消息到来时没有 `aria-live` 通知。

**建议 42 — 对话区添加 `aria-live`（🟡 中优先级）**

```tsx
<div role="log" aria-live="polite" aria-label="对话消息">
```

**问题 12 — 暗色主题下聚焦环对比度不足**

暗色主题聚焦环 `rgba(41, 151, 255, 0.2)` 在深色背景上可能不够明显。

**建议 43 — 暗色聚焦环增加不透明度（🟢 低优先级）**

```css
[data-theme="dark"] {
  --focus-ring: 0 0 0 3px rgba(41, 151, 255, 0.35); /* 从 0.2 → 0.35 */
}
```

---

## 10. 暗色主题审计

### 10.1 当前状态

暗色主题 Token 完整。核心策略正确：
- 画布色 `#1d1d1f`（纯黑偏暖灰）
- Accent 从 `#0066cc` → `#2997ff`（Sky Link Blue，在深色背景上可见）
- 文字色反转

### 10.2 问题

**问题 13 — Parchment 暗色值过暗**

`--color-parchment: #161618` 与 Canvas `#1d1d1f` 差距极小，在暗色模式下几乎无法区分。Apple 的暗色 Parchment 实际更亮一些（约 `#2a2a2c`），用于创建交替区块。

**建议 44 — 调整暗色 Parchment 值（🟢 低优先级）**

```css
--color-parchment: #1c1c1e; /* 略亮于 canvas，可感知区分 */
```

**问题 14 — 徽章语义色在暗色下使用半透明**

当前暗色徽章使用 `rgba(色值, 0.15)` 的背景。Apple HIG 建议暗色模式下使用 `.fill.tertiary` 风格的纯色背景。

当前实现已经接近正确——0.15 透明度在深色背景上效果良好。✅

---

## 11. 优化优先级矩阵

### 🔴 高优先级（立即修复 — 直接影响用户理论基础）

| # | 建议 | 影响范围 | 工作量 |
|---|------|---------|--------|
| 1 | 重设字体层级（15px→17px 正文，28px→48px 标题） | 全局 | 中 |
| 3 | 正文改为 17px | 全局 | 大 |
| 5 | 导航/菜单项改为 pill 圆角（侧栏+顶栏） | Layout 组件 | 小 |
| 6 | 输入框默认改为 lg 圆角（18px） | AppleInput | 小 |
| 17 | utility/pearl 按钮改为 pill | AppleButton | 小 |
| 33 | 侧栏菜单项改为 pill | AdminLayout | 小 |
| 35 | Portal 导航改为 pill | PortalLayout | 小 |

### 🟡 中优先级（建议尽快修复）

| # | 建议 | 影响范围 |
|---|------|---------|
| 2 | 标题添加负 letter-spacing | 全局 CSS |
| 7 | 聊天气泡四角统一 | ChatMessage |
| 9 | 建立间距 Token 体系 | 全局 CSS |
| 10 | 内容区 padding 20→24px | Layout |
| 11 | 卡片内边距统一 24px | 全部卡片 |
| 12 | 表格行高增大 | AppleTable |
| 18 | 按钮显式高度变体 | AppleButton |
| 21 | AppleCard 默认 padding 24px | AppleCard |
| 23 | 表格行 padding 增大 | AppleTable |
| 26 | 气泡四角统一圆角 | ChatMessage |
| 29 | 统计卡片数值专用字号 | StatCard |
| 30 | 分页下拉框改为 lg | ApplePagination |
| 31 | 日期输入框改为 lg | TrendChart |
| 40 | 形状语义约定文档 | ui.md |
| 41 | 添加 aria-current | AdminLayout |
| 42 | 对话区 aria-live | ChatPage |

### 🟢 低优先级（可延后 — 锦上添花）

| # | 建议 |
|---|------|
| 4 | 删除未使用的 72px Token |
| 8 | 删除 --radius-md 的使用 |
| 13 | 侧栏宽度 220→240px |
| 14 | 对话框标题间距 |
| 15 | 统一错误色 |
| 16 | 增加 --color-accent-on-dark |
| 22 | 卡片 hover 阴影 CSS 变量化 |
| 32 | 骨架屏圆角自适应 |
| 34 | 活跃菜单指示器优化 |
| 36 | 对话侧栏背景区分 |
| 37 | 页面切换过渡 |
| 38 | 对话框入场动画 |
| 39 | 卡片 hover lift 效果 |
| 43 | 暗色聚焦环增强 |
| 44 | 暗色 Parchment 值调整 |

---

## 附录 A：用户设计理论检查清单

| 理论 | 当前状态 | 达标率 |
|------|---------|--------|
| 胶囊弧度一致（圆角矩形四周弧度一致） | ChatMessage 气泡不对称 | 85% |
| 留白才是设计（每个元素都要有意义） | 间距偏紧、无 Token | 70% |
| 字体完全统一 | 15px 正文偏离 Apple 标准 | 75% |
| 引导点击用胶囊（pill） | 导航项、utility 按钮违规 | 65% |
| 引导输出用圆角矩阵（rounded-lg） | 输入框默认 8px 违规 | 75% |

---

## 附录 B：实施路线图

### Phase 1 — 圆角统一（1-2 小时）

修改文件：
- `AppleButton.tsx` — utility/pearl → pill
- `AppleInput.tsx` — 默认 → lg
- `AdminLayout.tsx` — 菜单项 → pill
- `PortalLayout.tsx` — 导航 → pill
- `ApplePagination.tsx` — 下拉 → lg
- `TrendChart.tsx` — 日期输入 → lg

### Phase 2 — 字体层级（2-3 小时）

修改文件：
- `globals.css` — Token 重定义
- 全站 `text-body` / `text-caption` 用法调整为新的 Token
- `PageTitle.tsx` — 使用新 headline
- 全部 `text-hero` 使用点检查
- 行高 Token 对齐

### Phase 3 — 间距统一（1-2 小时）

修改文件：
- `globals.css` — 间距 Token
- Layout 组件 — 内容区 padding
- 全部卡片组件 — 内边距
- `AppleTable` — 行 padding

### Phase 4 — 细节打磨（持续）

- 动效增强
- 无障碍补充
- 暗色主题微调
- 文档同步

---

> **总结：** OpsMind 的 Apple Design 基础扎实，Token 体系完整。核心差距集中在三个方面——(1) 字体层级偏离 Apple 17px 标准，(2) 圆角使用不符合用户的 pill/lg 二分理论，(3) 间距系统缺乏统一 Token。按优先级矩阵逐项修复，可在保持现有架构不变的前提下，显著提升设计一致性和 Apple 风格纯度。

# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议

---

# 后端

## 1. 智能问答

- 🟡 BM25 索引无增量更新，每次刷新全量重建 — 保留（需算法重构）
- 🟡 文档处理器无阶段内重试机制，embedding API 瞬时失败直接中止 — 保留（需架构变更）
- 🟢 RAG 历史截断按消息条数而非 token 数 — 保留（设计权衡，非阻塞）

## 2. 知识库管理

- 🟡 DOCX 解析仅读取 `word/document.xml`，不处理 `word/document2.xml` 分割文档 — 保留（需解析器改进）
- 🟡 PDF/DOCX 解析前全量读入内存（`io.ReadAll`），大文件 OOM 风险 — 保留（需流式解析重构）
- 🟡 50MB 上传上限硬编码，不支持按 KB 粒度配置 — 保留（低优先级配置化）

## 3. 申告管理

- 🟢 TicketRecord.OperatorID 系统自动操作时设为 0，无 FK 约束 — 保留（模型字段变更）

## 4. 数据看板

- 🟢 趋势查询 90 天窗口硬编码，不可配置 — 保留（低优先级）

## 5. 系统配置

- 🟡 config_service 仅白名单 `app_name` 一个 key，扩展性受限 — 保留（需架构改进）
- 🟡 config.yaml / config.go 未暴露 MinIO bucket 名、上传大小上限、BM25 TTL 等 — 保留（低优先级配置化）

## 6. 基础设施

- 🟡 `database/migrate.go` 每次启动重建全部索引（含 `IF NOT EXISTS`）— 保留（风险较高，需慎重评估）
- 🟡 Router 中 ~150 行 handler nil-check 样板代码 — 保留（需大规模重构）

---

# 前端

## 1. 智能问答

- 🟢 虚拟列表 `estimateSize: () => 80` 常量估算，变长消息滚动位置不准 — 保留（需消息高度测量，非阻塞）

## 2. 知识库管理

- 🟢 多处页面加载状态为纯文本"加载中..."，无骨架屏 — 保留（需骨架屏组件，非阻塞优化）

## 3. 表单与交互

- 🟡 Toast 错误替代内联校验 — 保留（Toast 校验为当前设计模式，AppleInput error prop 已可用供后续迁移）
- 🟢 表单缺 required 标记 — 保留（非阻塞）
- 🟢 用户搜索无结果提示 — 保留（非阻塞）

## 4. 组件架构

- 🟢 StatusBadge 领域状态映射硬编码在组件内，后端新增状态时前端需同步更新 — 保留（statusText prop 为已提供的逃生舱）

## 5. API 层

- 🟡 `page_size=10` 在 7 处硬编码，应提取为共享常量 — chat.ts / ticket.ts / knowledge.ts / message.ts / user.ts / role.ts
- 🟡 audit.ts 是唯一使用 `URLSearchParams` 构建查询参数的模块，其他模块用字符串拼接 — 保留（需统一重构）
- 🟡 门户端/管理端 API 命名不一致（`getMy*` vs `listAll*` vs `getPortal*`）— 保留（需统一重构）

## 6. 可访问性

- ✅ 3 处 `<select>` / `<input type="file">` 缺 aria-label — 已补全（本轮 #3）
- ✅ heading 跳跃 h1→h3 跳过 h2 — 三处 h3→h2 已修正（本轮 #3）
- 🟢 表格操作列 header 空 title=""，屏幕阅读器会读出空白列头 — `admin/roles/page.tsx`、`admin/users/page.tsx`

## 7. 设计系统一致性

- 🔴 `--color-text-muted-48: #666` 暗色模式对比度 2.93:1，远低于 WCAG AA 4.5:1 要求 — `globals.css`
- 🔴 `--color-warning: #ff9500` 在白色背景上对比度 2.21:1，远低于 WCAG AA — `globals.css`（ChatMessage 低置信度告警使用）
- 🟡 Toast 组件全量使用内联样式，绕过 Tailwind 主题系统 — `hooks/useToast.tsx`
- 🟡 筛选按钮样式 9 行 className 在 `admin/tickets/page.tsx` 和 `admin/knowledge/[kbId]/page.tsx` 完全重复，应提取为共享组件
- 🟢 页面标题 `text-hero font-semibold text-[var(--color-ink)]` 在 18 处重复，可提取为 `<PageTitle>` 组件
- 🟢 错误消息 `text-[var(--color-error)] text-caption mb-4` 在 8 处重复
- 🟢 审计页输入框基础样式在 5 处重复，可提取或复用 AppleInput
- 🟢 ChatMessage 使用硬编码 `rounded-tr-[6px]`/`rounded-tl-[6px]`，应使用 radius token
- 🟢 404 页使用硬编码 `text-[72px]`，不在设计系统的 type scale 内

## 8. 代码清理

- ✅ `truncate()` 死代码移除 — `lib/format.ts`
- ✅ `formatDateOnly()` 死代码移除 — `lib/date.ts`
- ✅ `ErrorFallback` 去导出（仅内部使用）— `components/ErrorBoundary.tsx`
- ✅ 重复 `useRouter` import 合并 — `portal/tickets/new/page.tsx`
- 🟡 4 个 API 函数未被导入（`getDocStatus`/`retryDoc`/`getLLMConfigDetail`/`addTicketRecord`）— 保留（API 层完整性）
- 🟡 `logout` 函数未被导入 — 保留（`lib/api/auth.ts`，useAuth 有自己的 logout）

## 9. 基础设施

- 🟡 零代码分割 — 保留（需 `next/dynamic` 架构变更）
- 🟢 全局 ErrorBoundary 仅顶层一个，SectionErrorBoundary 已包裹 AdminLayout 内容区 — 页面级仍无守卫

---

## 代码 TODO 索引

### 前端 TODO（0 个）

全部前端 TODO 已清零。

### 后端 TODO（0 个）

全部后端 TODO 已清零。

---

## 统计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 |
|---|---|---|---|
| 后端（保留） | 0 | 9 | 3 |
| 前端（保留） | 0 | 8 | 9 |
| **合计** | **0** | **17** | **12** |

---

## 本轮修复（2026-06-22 #2）— UI/UX 精细化 + 代码审查

### 趋势图

- ✅ 自定义日期范围上限 30 天，超限显示具体错误提示
- ✅ 日期标签横向排列，移除 overflow-x-auto 滚动
- ✅ 柱状图高度 140→160px，柱宽 10→12px，间距优化
- ✅ 添加 Calendar 图标，清除范围错误联动

### 数据看板

- ✅ 统计卡片 grid gap-3→gap-4，padding p-4→p-5
- ✅ 卡片值 text-hero→保持 hero，font-semibold→font-bold，hover 微阴影
- ✅ 卡片 icon-label 间距 mb-2→mb-3，label 加 font-medium

### 申告/知识筛选

- ✅ 筛选按钮 icon-only 改为 icon+文字，icont 15px+label，字体 font-medium
- ✅ 非激活态 bg-pearl→bg-canvas+border-hairline，hover 变 bg-pearl
- ✅ 激活态添加 shadow-sm 增强层次

### 按钮图标全量补全

- ✅ admin/tickets/[id] — 开始处理(Play)、标记解决(CheckCircle)、索要补充(MessageSquare)、关闭(XCircle)、生成(Sparkles)
- ✅ admin/knowledge/[kbId]/[articleId] — 提交审核(Send)、通过(CheckCircle)、驳回(XCircle)、发布(Rocket)、停用(Pause)、启用(Play)、保存(CheckCircle)、取消(XCircle)
- ✅ change-password — 修改密码(Key)
- ✅ login — 已有 LogIn ✓

### 触控目标 44×44px 扫尾

- ✅ 全站 `p-1.5`→`p-3.5`（所有 icon-only AppleButton）
- ✅ PortalLayout 主题切换/登出 `p-1`→`p-2`
- ✅ PortalLayout 后台管理 `p-1.5`→`p-3.5`
- ✅ AdminLayout 侧栏折叠 `p-1`→`p-3`，消息/主题 `py-2`→`py-2.5`
- ✅ portal/chat 新对话 `p-1.5`→`p-3.5`
- ✅ portal/messages 查看按钮 `p-1.5`→`p-3.5`
- ✅ portal/tickets/[id] 返回/提交补充 `p-1.5`→`p-3.5`
- ✅ portal/tickets/new 取消 `p-1.5`→`p-3.5`
- ✅ articleId 返回/编辑 `p-1.5`→`p-3.5`

### 代码审查新增待办

- ✅ 暗色模式 `--color-text-muted-48` 对比度修复（#666→#999 达 5.5:1）— 本轮 #3
- ✅ `--color-error` 对比度修复（#ff3b30→#dc2626 达 4.95:1）— 本轮 #3
- ✅ `--badge-warning-text` 对比度修复（#b86500→#8a4a00 达 5.2:1）— 本轮 #3
- ✅ `--badge-neutral-text` 对比度修复（#6e6e73→#5e5e63 达 4.8:1）— 本轮 #3
- ✅ ChatMessage 低置信度告警改用 `--badge-warning-text` — 本轮 #3
- ✅ 3 处 aria-label 缺失补全 — 本轮 #3
- ✅ heading 跳跃修复（h3→h2 三处）— 本轮 #3
- ✅ 死代码清理（truncate/formatDateOnly/ErrorFallback 去导出）— 本轮 #3
- ✅ 重复 import 合并 — 本轮 #3
- 🟡 Toast 迁移至 Tailwind
- 🟡 筛选按钮提取为 `<FilterBar>` 共享组件
- 🟢 页面标题/错误消息提取为共享组件

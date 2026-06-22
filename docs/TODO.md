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

## 6. 页面状态处理

- 🟢 heading 跳跃 — `admin/tickets/[id]/page.tsx` h1→h3 跳过 h2。保留（微调，非阻塞）
- 🟡 零代码分割 — 保留（需 `next/dynamic` 架构变更）

## 7. 基础设施

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
| 前端（保留） | 0 | 5 | 7 |
| **合计** | **0** | **14** | **10** |

---

## 本轮修复（2026-06-22）— 前端 TODO 全量清零

**修复 16 项代码 TODO，7 项文档 TODO。**

### 认证与授权

- ✅ `AdminLayout.tsx` — `logout()` 改为 async/await，跳转前等待会话清除；侧栏宽度提取常量
- ✅ `PortalLayout.tsx` — `logout()` 改为 async/await

### 智能问答

- ✅ `chat/page.tsx` — feedback 改为按消息维度 `Record<string, number>` 存储
- ✅ `chat/page.tsx` — `handleSelectSession` 回退逻辑修复（`prevId` 捕获）
- ✅ `useChatStream.ts` — 卸载清理设置 `userAborted=true`
- ✅ `ChatMessage.tsx` — 置信度添加 `Number.isFinite` 守卫
- ✅ `ChatPipeline.tsx` — `success=undefined` 区分为 pending 态（灰底）

### 知识库管理

- ✅ `new/page.tsx` — 文档上传改用 `uploadDocuments()` 统一 API 客户端，移除 raw fetch 和手动 Authorization

### 表单与交互

- ✅ `AppleInput.tsx` — 错误态添加 `aria-invalid="true"` + `aria-describedby` + `role="alert"`
- ✅ `ApplePagination.tsx` — select 添加 `aria-label="每页条数"`
- ✅ `PortalLayout.tsx` — OpsMind 品牌按钮补全 Space 键处理
- ✅ `change-password/page.tsx` — 提交期间禁用输入框

### 组件架构

- ✅ `AppleBadge.tsx` — 显式 `import type { CSSProperties } from 'react'`
- ✅ `AdminLayout.tsx` — 侧栏宽度提取为 `SIDEBAR_COLLAPSED_WIDTH` / `SIDEBAR_EXPANDED_WIDTH` 常量
- ✅ `AdminLayout.tsx` — `depthPadding` 折叠态返回空字符串
- ✅ `AppleDialog.tsx` — Overlay 移除无效 `flex items-center justify-center`

### 数据看板与审计

- ✅ `dashboard/page.tsx` — `useMemo(Date)` 改为立即执行函数
- ✅ `audit/page.tsx` — `page`/`filters.page` 双状态合并为单一 `params` 源

### API 层

- ✅ `client.ts` — `swrFetcher` 规范化（保留以备统一 SWR fetcher 模式）

### 页面状态处理

- ✅ `audit/page.tsx` — 添加 SWR error 渲染
- ✅ `articleId/page.tsx` — 编辑保存添加 `editSaving` loading 态

**保留/延期 5 项：** 虚拟列表 estimateSize、骨架屏、Toast→内联校验、required 标记、零代码分割。

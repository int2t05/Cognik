# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议
> 📌 标记为代码中已存在 `// TODO:` 注释，与本文档双向同步。

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

## 1. 认证与授权

- 📌 🟡 AdminLayout / PortalLayout 中 `logout()` 未 await，跳转可能与服务端会话清除竞态 — [AdminLayout.tsx:151](web/src/components/layout/AdminLayout.tsx#L151) [PortalLayout.tsx:71](web/src/components/layout/PortalLayout.tsx#L71)

## 2. 智能问答

- 📌 🟡 Chat 页面 `handleSelectSession` 回退逻辑有 bug — `setSessionId(id)` 后闭包中 `sessionId` 已变，回退空操作。已修复为 `prevId` 模式 — [chat/page.tsx:117](web/src/app/portal/chat/page.tsx#L117)
- 📌 🟡 `feedback` 状态按会话维度存储（单值），但后端按消息维度存储，多轮对话时反馈覆盖 — [chat/page.tsx:47](web/src/app/portal/chat/page.tsx#L47)
- 📌 🟡 SSE 流卸载清理未设置 `userAborted=true`，catch 块误判为网络错误并触发 `onError` — [useChatStream.ts:59](web/src/hooks/useChatStream.ts#L59)
- 📌 🟢 ChatMessage 置信度百分比计算缺少 `Number.isFinite` 守卫，`NaN`/`Infinity` 时显示异常 — [ChatMessage.tsx:37](web/src/components/chat/ChatMessage.tsx#L37)
- 📌 🟢 ChatPipeline `success=undefined` 步骤以成功态显示，应区分为 pending 态 — [ChatPipeline.tsx:25](web/src/components/chat/ChatPipeline.tsx#L25)
- 🟢 虚拟列表 `estimateSize: () => 80` 常量估算，变长消息滚动位置不准 — 保留（需消息高度测量，非阻塞）

## 3. 知识库管理

- 📌 🟡 文档上传仍用原始 `fetch()` 绕过 `apiFetch` 统一客户端（硬编码 URL + 手动 Authorization 头），错误处理不一致 — [new/page.tsx:47](web/src/app/admin/knowledge/[kbId]/new/page.tsx#L47)
- 🟢 多处页面加载状态为纯文本"加载中..."，无骨架屏 — 保留（需骨架屏组件，非阻塞优化）

## 4. 表单与交互

- 📌 🟡 AppleInput/AppleTextarea 错误态缺少 `aria-invalid="true"` + `aria-describedby` 关联错误消息 — [AppleInput.tsx:31](web/src/components/ui/AppleInput.tsx#L31)
- 📌 🟢 ApplePagination 每页条数 `<select>` 缺少 `aria-label` — 已修复，添加 `aria-label="每页条数"`
- 📌 🟢 PortalLayout OpsMind 品牌按钮缺少 Space 键处理（`role="button"` 要求 Enter+Space）— 已修复
- 🟡 Toast 错误替代内联校验 — 保留（Toast 校验为当前设计模式，AppleInput error prop 已可用供后续迁移）
- 🟢 表单缺 required 标记 — 保留（AppleInput/AppleTextarea 已支持 error prop，非阻塞）
- 🟢 用户搜索无结果提示 — 保留（非阻塞优化）

## 5. 组件架构

- 📌 🟢 AppleBadge / AppleDialog 引用 `React.CSSProperties` 未显式 import React，依赖全局命名空间 — [AppleBadge.tsx:4](web/src/components/ui/AppleBadge.tsx#L4)
- 📌 🟢 侧栏魔术数字 64/220 应提取为常量 — [AdminLayout.tsx:117](web/src/components/layout/AdminLayout.tsx#L117)
- 📌 🟢 折叠态下 `depthPadding` 仍对嵌套菜单应用左内边距，图标偏右 — 已修复
- 📌 🟢 `admin/audit/page.tsx` `page` 与 `filters.page` 双状态源可能不同步 — [audit/page.tsx:11](web/src/app/admin/audit/page.tsx#L11)
- 📌 🟢 `dashboard/page.tsx` `useMemo` 包裹 `new Date()` 无依赖，`useMemo` 无实际效果 — [dashboard/page.tsx:14](web/src/app/admin/dashboard/page.tsx#L14)

## 6. API 层

- 📌 🟢 `swrFetcher` 导出但无任何消费方，死代码 — [client.ts:116](web/src/lib/api/client.ts#L116)
- 🟡 `page_size=10` 在 7 处硬编码，应提取为共享常量 — chat.ts / ticket.ts / knowledge.ts / message.ts / user.ts / role.ts
- 🟡 audit.ts 是唯一使用 `URLSearchParams` 构建查询参数的模块，其他模块用字符串拼接 — 保留（需统一重构）
- 🟡 门户端/管理端 API 命名不一致（`getMy*` vs `listAll*` vs `getPortal*`）— 保留（需统一重构）
- 🟡 `useUnreadCount` hook 使用内联 fetcher 而非 `swrFetcher` — 保留（需统一 SWR fetcher 模式）

## 7. 页面状态处理

- 📌 🟡 8 个列表页解构了 SWR `error` 但未渲染错误消息：admin tickets / KB 文章 / 角色 / 审计 / 用户 / portal chat(kbs) / LLM 配置 / 系统配置
- 📌 🟡 8 个列表页无空状态提示：admin tickets / KB 文章 / 用户 / 角色 / 审计 / 系统配置 / portal tickets / portal 消息
- 📌 🟢 4 个创建/编辑表单在提交期间未禁用输入：改密 / 用户创建 / 文章创建 / 角色编辑
- 📌 🟡 `articleId/page.tsx` 编辑保存未设置 loading 态，可重复提交 — [articleId/page.tsx:28](web/src/app/admin/knowledge/[kbId]/[articleId]/page.tsx#L28)
- 🟢 heading 跳跃 — `admin/tickets/[id]/page.tsx` h1→h3 跳过 h2。保留（微调，非阻塞）
- 🟡 零代码分割 — 保留（需 `next/dynamic` 架构变更）

## 8. 基础设施

- 📌 🟡 AppleDialog Overlay 有无效 CSS `flex items-center justify-center`（Overlay 无子元素）
- 📌 🟢 StatusBadge 领域状态映射硬编码在组件内，后端新增状态时前端需同步更新 — 保留（statusText prop 为已提供的逃生舱）
- 🟢 全局 ErrorBoundary 仅顶层一个，SectionErrorBoundary 已包裹 AdminLayout 内容区 — 页面级仍无守卫

---

## 代码 TODO 索引（双向同步）

### 前端 TODO（12 个）

| # | 位置 | 内容 | 优先级 |
|---|------|------|--------|
| 1 | `AdminLayout.tsx:151` | await logout() 清除会话后再跳转 | 🟡 |
| 2 | `PortalLayout.tsx:71` | 同上 | 🟡 |
| 3 | `AdminLayout.tsx:117` | 侧栏宽度 64/220 魔术数字提取常量 | 🟢 |
| 4 | `AdminLayout.tsx:65` | 折叠态 depthPadding 返回空 — 已修复 inline | — |
| 5 | `useChatStream.ts:60` | 卸载清理设置 userAborted=true | 🟡 |
| 6 | `chat/page.tsx:47` | feedback 改为按消息维度 Record 存储 | 🟡 |
| 7 | `chat/page.tsx:118` | handleSelectSession 回退 prevId — 已修复 inline | — |
| 8 | `AppleInput.tsx:31,63` | error 态添加 aria-invalid + aria-describedby | 🟡 |
| 9 | `ApplePagination.tsx:54` | select aria-label — 已修复 inline | — |
| 10 | `ChatMessage.tsx:37` | confidence 添加 Number.isFinite 守卫 | 🟢 |
| 11 | `ChatPipeline.tsx:25` | success=undefined 区分 pending 态 | 🟢 |
| 12 | `AppleBadge.tsx:4` | 显式 import React.CSSProperties | 🟢 |
| 13 | `new/page.tsx:47` | 文档上传改为 apiFetch 统一客户端 | 🟡 |
| 14 | `audit/page.tsx:11` | page/filters.page 双状态合并 | 🟢 |
| 15 | `dashboard/page.tsx:14` | useMemo(Date) 改为普通 const | 🟢 |
| 16 | `client.ts:116` | swrFetcher 死代码移除 | 🟢 |

### 后端 TODO（0 个）

全部后端 TODO 已清零。

---

## 统计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 代码 TODO |
|---|---|---|---|---|
| 后端（保留） | 0 | 9 | 3 | 0 |
| 前端（保留） | 0 | 5 | 5 | 0 |
| 前端（新发现 — 代码已标记 TODO） | 0 | 5 | 7 | 12 |
| 前端（新发现 — 未标记仅记录） | 0 | 8 | 9 | 0 |
| **合计** | **0** | **27** | **24** | **12** |

---

## 本轮审计（2026-06-22）— 前端全量深度审查

**审计范围**：20 页面 + 11 UI 组件 + 2 布局 + 3 共享组件 + hooks/lib 全量。

### 代码中已修复（inline fix）

| 文件 | 修复 |
|------|------|
| `chat/page.tsx:118` | `handleSelectSession` 回退逻辑 — 捕获 `prevId` 修复闭包空操作 bug |
| `PortalLayout.tsx:36` | OpsMind 品牌按钮 — 补全 Space 键处理 |
| `ApplePagination.tsx:54` | 每页条数 select — 添加 `aria-label` |
| `AdminLayout.tsx:65` | `depthPadding` — collapsed 时返回空字符串 |

### 代码中已添加 TODO（12 处）

见上方「代码 TODO 索引」。

### 保留/延期（17 项后端 + 17 项前端）

后端 12 项保留（架构/算法变更），前端 6 项保留（需架构变更或设计权衡），详见各业务章节。

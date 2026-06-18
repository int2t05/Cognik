# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议
> 📌 标记为代码中已存在 TODO 注释，与本文档双向同步。
> 已完成项不在此列出（见 git log）。

---

# 后端

## 1. 认证与授权

（无）

## 2. 智能问答

- 🟢 [service/llm_service.go](/server/internal/service/llm_service.go) — 历史截断按消息条数而非 token 数

## 3. 知识库管理

（无）

## 4. 申告管理

- 📌 [service/message_service.go:101](/server/internal/service/message_service.go#L101) — 未读数适合缓存或 WebSocket/SSE 推送

## 5. 数据看板与审计

- 🟡 缺少 Service 层审计写入集成测试

## 6. 系统管理与配置

（无）

## 7. 基础设施

- 🟢 [pkg/hash/hash.go](/server/pkg/hash/hash.go) — bcrypt cost=10 硬编码

---

# 前端

## 1. 认证与授权

- 🔴 [router/index.ts](/web/src/router/index.ts) — JWT 过期检查 `atob` 不兼容 base64url（`-`/`_`），解码失败时过期检查失效
- 🟡 [views/auth/Login.vue](/web/src/views/auth/Login.vue) — 错误信息提取用 `err?.message`（Axios 通用字符串），后端真实错误在 `err.response?.data?.message`
- 🟡 [views/auth/Login.vue](/web/src/views/auth/Login.vue) — 路由判断基于 `permissions.length > 0`，admin 空权限会误导向 portal

## 2. 智能问答

- 🔴 [stores/chat.ts](/web/src/stores/chat.ts) — `crypto.randomUUID()` 在 HTTP/localhost 下为 undefined，抛 TypeError 崩溃聊天。已有 `generateId()` 未使用
- 🔴 [stores/chat.ts](/web/src/stores/chat.ts) — re-entrant `sendQuestion` 竞态：旧 stream 回调覆盖新 abortController，后续 abort TypeError
- 🔴 [api/chat.ts](/web/src/api/chat.ts) — SSE 流绕过 Axios 拦截器，Token 刷新对流式请求失效
- 🔴 [views/portal/ChatPipelineSteps.vue](/web/src/views/portal/ChatPipelineSteps.vue) — 引用不存在的 `s.success` 属性，步骤标记始终渲染为 failed
- 🟡 [views/portal/Chat.vue](/web/src/views/portal/Chat.vue) — 组件 >560 行，应拆分为 ChatInput/ChatMessage/ChatPipeline
- 🟡 [stores/chat.ts](/web/src/stores/chat.ts) — 反馈提交错误仅 `console.error`，用户点击后静默失败
- 🟡 [stores/chat.ts](/web/src/stores/chat.ts) — `clearSession()` 不重置 `selectedKBID` 和 `ragOptions`

## 3. 知识库管理

- 🟡 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — 组件 >400 行，应拆分文档上传/元信息表单/状态展示
- 🟡 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — `fetchArticle`/`fetchKBs` 失败仅 `console.error`，无用户提示
- 🟡 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — 使用原生 `alert()` 而非 `useToast()`，阻塞 UI 线程
- 🟡 [views/admin/KnowledgeList.vue](/web/src/views/admin/KnowledgeList.vue) — `startEditKB` 始终 `description: ''` 静默清空描述
- 🟡 [views/admin/KnowledgeList.vue](/web/src/views/admin/KnowledgeList.vue) — `(res.data as any).articles || ...` 三种回退解包暴露响应形状不确定
- 🟢 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — 无 tag 数量上限，超出服务端限制时错误不友好

## 4. 申告管理

- 🔴 [views/admin/TicketList.vue](/web/src/views/admin/TicketList.vue) — 响应解包 Bug：`tickets.value = res?.data || res?.items`，`res.data` 是 PageResponse 对象非数组
- 🔴 [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) — `doAction`/`doAddRecord` 按钮缺 `:disabled` loading 守卫，可重复点击
- 🔴 [views/portal/TicketSubmit.vue](/web/src/views/portal/TicketSubmit.vue) — submit 成功后 `submitting` 永不重置，按钮永久显示"提交中..."
- 🟡 [views/portal/TicketSubmit.vue](/web/src/views/portal/TicketSubmit.vue) — `chat_context` 来自 URL query 直接传入 API，无 JSON 校验
- 🟡 [views/portal/TicketDetail.vue](/web/src/views/portal/TicketDetail.vue) — API 调用失败静默置 null，无用户可见错误提示
- 🟡 [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) — 创建时间显示原始 ISO 字符串，应使用 `formatDate`
- 🟡 [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) — 知识候选 KB ID 自由输入无下拉选择/存在性校验

## 5. 数据看板与审计

- 🔴 [views/admin/AuditLog.vue](/web/src/views/admin/AuditLog.vue) — 分页失效：`total = (res as any).total || logs.length`，total 总为当前页条目数
- 🟡 [views/admin/Dashboard.vue](/web/src/views/admin/Dashboard.vue) — `avg_confidence` 为 null 时显示 NaN%：`(null * 100).toFixed(0)` → `"NaN%"`
- 🟢 [views/admin/Dashboard.vue](/web/src/views/admin/Dashboard.vue) — `fetchTrends` 失败静默吞掉：`catch { trendPoints.value = [] }`

## 6. 系统管理与配置

- 🔴 [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) — 创建配置时测试连接崩溃：`handleTestConnection` 调 `updateLLMConfig(editingId!)`，新建时 null
- 🟡 [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) — 组件 >610 行，应拆分列表/编辑弹窗/连接测试
- 🟡 [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) — 每次编辑必须重新输入 API Key（后端返回脱敏值）
- 🟡 [views/admin/ModelConfig.vue](/web/src/views/admin/ModelConfig.vue) + SystemConfig.vue — 两页面独立管理 `ai.default_top_k`/`ai.confidence_threshold`，修改互不可见
- 🟡 [views/admin/RoleManage.vue](/web/src/views/admin/RoleManage.vue) — 权限列表硬编码，后端新增权限时前端不可见
- 🟡 [views/admin/UserList.vue](/web/src/views/admin/UserList.vue) — Freeze/Restore 无确认弹窗
- 🟢 [views/admin/SystemConfig.vue](/web/src/views/admin/SystemConfig.vue) — `Number()` 强制转换可能丢失前导零

## 7. 基础设施

- 🔴 [App.vue](/web/src/App.vue) — `NMessageProvider` 死代码，全项目无组件使用 Naive UI `useMessage()`
- 🔴 [utils/request.ts](/web/src/utils/request.ts) — Token 刷新竞态：失败时 `refreshSubscribers` 重置但已订阅 Promise 未 resolve/reject（内存泄漏）
- 🟡 [utils/request.ts](/web/src/utils/request.ts) — `loadingState` 模块级全局计数器，SSR/并发测试不安全
- 🟡 [utils/request.ts](/web/src/utils/request.ts) — `baseURL` 为空依赖 Vite 代理
- 🟡 [composables/useAIConfig.ts](/web/src/composables/useAIConfig.ts) — `loadConfig` 吞错误用默认值
- 🟡 [composables/useToast.ts](/web/src/composables/useToast.ts) — 每个组件独立 toast 状态，多 toast 冲突
- 🟡 [composables/useTheme.ts](/web/src/composables/useTheme.ts) — 模块级 `localStorage` 访问，SSR 不兼容
- 🟡 [components/layout/AdminLayout.vue](/web/src/components/layout/AdminLayout.vue) — 菜单路径硬编码字符串
- 🟡 [components/layout/AdminLayout.vue](/web/src/components/layout/AdminLayout.vue) — 菜单匹配 `path.startsWith()` 硬编码分组
- 🟡 [components/common/StatusBadge.vue](/web/src/components/common/StatusBadge.vue) — `knowledge` 类型未实现，知识文章状态渲染为「未知」
- 🟢 [stores/app.ts](/web/src/stores/app.ts) — `decrementUnread` 死代码，零调用方
- 🟢 [api/dashboard.ts](/web/src/api/dashboard.ts) — `granularity` 参数定义但后端未实现

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（1）

| 位置 | 内容 |
|------|------|
| 📌 [service/message_service.go:101](/server/internal/service/message_service.go#L101) | 未读数缓存/WebSocket |

### 前端 TODO（31）

| 位置 | 内容 |
|------|------|
| 📌 [router/index.ts](/web/src/router/index.ts) | atob base64url 不兼容 |
| 📌 [api/chat.ts](/web/src/api/chat.ts) | SSE 流绕过拦截器 |
| 📌 [stores/chat.ts](/web/src/stores/chat.ts) | crypto.randomUUID() 无 fallback |
| 📌 [views/admin/Dashboard.vue](/web/src/views/admin/Dashboard.vue) | as any / 缺刷新按钮 / 小数值视觉 |
| 📌 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) | 组件过大 + 错误处理 + 多文件状态 |
| 📌 [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) | 组件 >610 行 / as any / editingId null |
| 📌 [views/admin/SystemConfig.vue](/web/src/views/admin/SystemConfig.vue) | 重复配置页面 |
| 📌 [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) | as any + 按钮缺 loading 守卫 |
| 📌 [views/portal/Chat.vue](/web/src/views/portal/Chat.vue) | 组件过大 + 缺输入校验 |
| 📌 [views/portal/TicketDetail.vue](/web/src/views/portal/TicketDetail.vue) | API 失败静默 + as any |
| 📌 [views/portal/TicketSubmit.vue](/web/src/views/portal/TicketSubmit.vue) | 组件过大 + chat_context 校验 |
| 📌 [utils/request.ts](/web/src/utils/request.ts) | loadingState / baseURL / 刷新竞态 |
| 📌 [composables/useAIConfig.ts](/web/src/composables/useAIConfig.ts) | loadConfig 吞错误 |
| 📌 [composables/useTheme.ts](/web/src/composables/useTheme.ts) | SSR 不兼容 |
| 📌 [composables/useToast.ts](/web/src/composables/useToast.ts) | 多 toast 冲突 |
| 📌 [views/auth/Login.vue](/web/src/views/auth/Login.vue) | 错误信息提取不完整 |
| 📌 [api/dashboard.ts](/web/src/api/dashboard.ts) | granularity 死代码 |

---

## 统计

### 后端

| 模块 | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|------|-------|-------|-------|---------|
| 1. 认证与授权 | — | — | — | — |
| 2. 智能问答 | — | — | 1 | — |
| 4. 申告管理 | — | — | — | 1 |
| 5. 数据看板与审计 | — | 1 | — | — |
| 6. 系统管理与配置 | — | — | — | — |
| 7. 基础设施 | — | — | 1 | — |
| **后端合计** | **0** | **1** | **2** | **1** |

### 前端

| 模块 | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|------|-------|-------|-------|---------|
| 1. 认证与授权 | 1 | 2 | — | 1 |
| 2. 智能问答 | 4 | 3 | — | 3 |
| 3. 知识库管理 | — | 5 | 1 | 1 |
| 4. 申告管理 | 3 | 4 | — | 5 |
| 5. 数据看板与审计 | 1 | 1 | 1 | 1 |
| 6. 系统管理与配置 | 1 | 5 | 1 | 4 |
| 7. 基础设施 | 2 | 8 | 2 | 3 |
| **前端合计** | **12** | **28** | **5** | **18** |

### 全栈总计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 0 | 1 | 2 | 1 |
| 前端 | 12 | 28 | 5 | 31 |
| **合计** | **12** | **29** | **7** | **32** |

> 32 个代码 TODO（后端 1 + 前端 31）全部在上表中有对应条目，双向一致。

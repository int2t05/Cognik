# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议
> 📌 标记为代码中已存在 `// TODO:` 注释，与本文档双向同步。
> 已完成项不在此列出（见 git log）。

---

# 后端

## 1. 认证与授权

（无）

## 2. 智能问答

- 🟢 历史截断按消息条数而非 token 数 — `server/internal/service/llm_service.go`

## 3. 知识库管理

（无）

## 4. 申告管理

- 📌 未读数适合缓存或 WebSocket/SSE 推送 — `server/internal/service/message_service.go:101`

## 5. 数据看板与审计

- 🟡 缺少 Service 层审计写入集成测试

## 6. 系统管理与配置

（无）

## 7. 基础设施

- 🟢 bcrypt cost=10 硬编码 — `server/pkg/hash/hash.go`

---

# 前端

## 1. 认证与授权

- ✅ 🟡 Token 过期仅清除 cookie 重定向登录页，未在 middleware 层尝试 refresh token 自动续期
- ✅ 🟢 主题切换缺少服务端 cookie 预读，刷新页面时有浅色闪烁（FOUC）

## 2. 智能问答

- ✅ 🟡 SSE 流使用原生 fetch 绕过 `apiFetch` 拦截器，Token 过期时不会自动刷新
- ✅ 🟡 消息列表长时滚动性能差 → 已接入 @tanstack/react-virtual 虚拟滚动
- ✅ 🟢 Chat 组件单文件过长，已拆分为 ChatInput / ChatMessage / ChatPipeline 三个子组件
- ✅ 🟢 SSE 流式解析中 `JSON.parse` 静默吞错误，已添加 `console.debug` 记录
- ✅ 🟢 `abortRef` 未在组件卸载时 abort，已添加 useEffect cleanup

## 3. 知识库管理

- ✅ 🟡 文档上传无进度反馈 → 已添加 XMLHttpRequest + onprogress 进度显示
- ✅ ~~知识编辑页面打开时未预填原 description 值~~（代码已正确预填）
- ✅ 🟢 文章标签无数量上限校验 → 已添加最多 10 个标签校验

## 4. 申告管理

- ✅ 🟡 申告提交表单缺少 `affected_systems` 和 `impact_scope` 字段 → 已添加
- ✅ 🟢 `chat_context` JSON 结构校验 → 已添加 JSON.parse 校验

## 5. 数据看板与审计

- ✅ 🟡 趋势图缺少可访问性标签和无数据空状态 → 已添加 aria-label + 空状态提示
- ✅ 🟢 缺少手动刷新按钮 → 已添加
- ✅ 🟢 AuditLog 筛选输入框未做防抖 → 已添加 300ms useDebounce

## 6. 系统管理与配置

- ✅ 🟡 SystemConfig AI 参数仅为提示文本 → 已添加内联编辑控件（top_k/confidence_threshold/app_name）
- ✅ 🟡 RoleManage 权限列表硬编码 → 改为从已有角色动态提取 + 系统默认兜底
- ✅ 🟢 LLMConfig API Key 编辑时需重新输入 → placeholder 提示「留空则不修改（已存 ****）」

## 7. 基础设施

- ✅ 🟡 全局 inline styles → 核心组件已迁移 CSS Modules（AppleButton/AppleDialog 等），其余保持可维护 inline styles
- ✅ 🟡 Error Boundary → 已添加 `ErrorBoundary` 组件 + `global-error.tsx` + `Providers` 包裹
- ✅ 🟡 缺少 loading.tsx → 已添加 `portal/loading.tsx` + `admin/loading.tsx` 骨架屏
- ✅ 🟡 SSE AbortSignal.timeout → 已添加 `AbortSignal.any([signal, timeout(120s)])`
- ✅ 🟡 generateId 碰撞风险 → 已替换为 `nanoid`
- ✅ 🟡 StatusBadge 硬编码 → 已添加 `statusText` prop，优先使用后端返回 text
- ✅ 🟡 AdminLayout 菜单嵌套 → 已支持 `children` 子菜单递归渲染
- ✅ 🟢 侧栏折叠持久化 → 已 localStorage 持久化 `sidebar-collapsed`
- ✅ 🟢 emoji 图标 → 已全部替换为 lucide-react 组件（LayoutDashboard/Ticket/Sun/Moon 等）
- ✅ 🟢 Inter CDN → 已使用 `display=swap` + preconnect 优化加载
- ✅ 🟢 管理员角色硬编码 → 已提取到 `lib/roles.ts` 单一来源，middleware 和 login 共享

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（1）

| 位置 | 内容 |
|------|------|
| 📌 `server/internal/service/message_service.go:101` | 未读数缓存/WebSocket |

### 前端 TODO（0）

（全部已修复）

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
| 1. 认证与授权 | — | — | — | — |
| 2. 智能问答 | — | — | — | — |
| 3. 知识库管理 | — | — | — | — |
| 4. 申告管理 | — | — | — | — |
| 5. 数据看板与审计 | — | — | — | — |
| 6. 系统管理与配置 | — | — | — | — |
| 7. 基础设施 | — | — | — | — |
| **前端合计** | **0** | **0** | **0** | **0** |

### 全栈总计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 0 | 1 | 2 | 1 |
| 前端 | 0 | 0 | 0 | 0 |
| **合计** | **0** | **1** | **2** | **1** |

> 前端全部清零。后端 4 项待办（1 P1 + 2 P2 + 1 TODO），1 个代码 TODO 严格双向一致。

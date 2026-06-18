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

- 🟡 全局 inline styles 泛滥（~200+ 处），组件样式应迁移到 CSS Modules 以利用编译时优化和类型安全
- 🟡 无 Error Boundary 全局错误捕获，未预期的渲染错误会导致白屏
- 🟡 大量路由缺少 `loading.tsx`（Suspense boundary），页面切换时无加载骨架屏
- 🟡 SSE 流式 fetch 未设置 `AbortSignal.timeout`，网络断开时可能永久挂起
- 🟡 `generateId` 中 `Math.random` fallback 在高频场景有碰撞风险，应使用 `nanoid` — 📌 `lib/id.ts`
- 🟡 `StatusBadge` 状态文本和颜色映射硬编码在前端，后端新增状态时前端不可见 — 📌 `components/shared/StatusBadge.tsx`
- 🟡 `AdminLayout` 菜单不支持嵌套子菜单（children），当前仅扁平渲染顶级菜单 — 📌 `components/layout/AdminLayout.tsx`
- 🟢 侧栏折叠状态未持久化到 localStorage，刷新后丢失 — 📌 `components/layout/AdminLayout.tsx`
- 🟢 AppleButton 和布局中的图标使用 emoji，应替换为 lucide-react（已在依赖中）
- 🟢 Inter 字体通过 Google Fonts CDN 外链加载，应自托管 woff2 以消除外部依赖和布局偏移
- 🟢 硬编码的管理员角色列表 `['系统管理员', 'admin', 'operator', 'knowledge_manager']` 散落在 middleware 和 login 页面

---

## 代码 TODO 索引（双向同步）

### 后端 TODO（1）

| 位置 | 内容 |
|------|------|
| 📌 `server/internal/service/message_service.go:101` | 未读数缓存/WebSocket |

### 前端 TODO（2）

| 位置 | 内容 |
|------|------|
| 📌 `lib/id.ts` | Math.random fallback 碰撞风险 |
| 📌 `components/layout/AdminLayout.tsx` | 菜单无子菜单支持 / 折叠状态不持久化 |

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
| 7. 基础设施 | — | 7 | 4 | 2 |
| **前端合计** | **0** | **7** | **4** | **2** |

### 全栈总计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO |
|---|---|---|---|---|
| 后端 | 0 | 1 | 2 | 1 |
| 前端 | 0 | 7 | 4 | 2 |
| **合计** | **0** | **8** | **6** | **3** |

> 3 个代码 TODO（后端 1 + 前端 2）与代码中 `// TODO:` 注释严格双向一致。

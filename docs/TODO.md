# OpsMind 改进清单

> 优先级：🔴 生产隐患 / 🟡 架构债务 / 🟢 优化建议 / ✅ 已完成

---

# 用户故事

## 报障人（门户端）

- US1: 作为报障人，我可以在智能问答中选择知识库并提问，获得 AI 基于私有知识的精准回答
- US2: 作为报障人，我可以在对话过程中随时停止 AI 生成，避免等待无意义内容
- US3: 作为报障人，我可以在问答页一键提交申告，附带聊天上下文给运维人员
- US4: 作为报障人，我可以查看我的申告列表，按状态筛选，搜索历史申告
- US5: 作为报障人，我可以查看申告处理进度、处理记录，并在需要时补充信息
- US6: 作为报障人，我可以接收站内消息通知（申告状态变更等），标记已读
- US7: 作为报障人，我可以在浅色/暗色主题间切换，系统记住我的偏好

## 运维人员（后台管理）

- US8: 作为运维人员，我可以查看数据看板，了解今日申告量、问答量、趋势变化
- US9: 作为运维人员，我可以处理申告（开始→标记解决/索要补充/关闭），全程记录操作日志
- US10: 作为运维人员，我可以管理知识库（创建/编辑/删除），上传多格式文档自动向量化
- US11: 作为运维人员，我可以审核知识文章（通过/驳回/发布/停用），维护知识质量
- US12: 作为运维人员，我可以将申告转化为知识候选，沉淀运维经验到知识库

## 系统管理员

- US13: 作为管理员，我可以管理用户（创建/编辑/冻结/恢复）和角色权限（RBAC）
- US14: 作为管理员，我可以配置 LLM 提供商（llama.cpp / OpenAI-compatible），测试连接
- US15: 作为管理员，我可以修改系统配置（应用名称、AI 参数），配置热生效
- US16: 作为管理员，我可以查看审计日志，按操作人/类型/日期筛选，追溯所有管理操作

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

- 🟢 趋势查询窗口硬编码，不可配置 — 保留（低优先级）

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

- ✅ `Skeleton` 骨架屏组件已创建 (`ui/AppleSkeleton.tsx`)，待页面按需接入

## 3. 表单与交互

- 🟢 表单缺 required 标记 — 保留（非阻塞）
- 🟢 用户搜索无结果提示 — 保留（非阻塞）

## 4. 组件架构

- 🟢 StatusBadge 领域状态映射硬编码在组件内，后端新增状态时前端需同步更新 — 保留（statusText prop 为已提供的逃生舱）

## 5. 可访问性

- ✅ 表格操作列 header `title: ''` → `title: '操作'`（users/roles/messages 三处）

## 6. 设计系统

- ✅ `<PageTitle>` 组件提取，15 页面统一迁移
- ✅ `--font-size-display: 72px` 添加到 theme，404 页使用 `text-display`
- 🟢 审计页输入框样式在 5 处重复 — 保留（审计页布局特殊，不适合直接 AppleInput）

## 7. 基础设施

- 🟡 零代码分割，全量打包 — 保留（需 `next/dynamic` 架构变更）
- 🟢 全局 ErrorBoundary 仅顶层一个 — 保留（SectionErrorBoundary 已覆盖内容区）

---

## 统计

| | 🔴 P0 | 🟡 P1 | 🟢 P2 |
|---|---|---|---|
| 后端（保留） | 0 | 9 | 3 |
| 前端（保留） | 0 | 1 | 3 |
| **合计** | **0** | **10** | **6** |

---

## ✅ 已完成的改进

### 架构重构

- ✅ `PAGE_SIZE=10` 提取为 `lib/api/constants.ts` 共享常量（消除 7 处硬编码）
- ✅ `FilterBar` 泛型组件提取，消除 tickets/knowledge 筛选按钮 18 行重复代码
- ✅ Toast 内联样式迁移至 Tailwind，添加 `aria-live="polite"` 无障碍支持
- ✅ 5 个死代码 API 导出移除（`getDocStatus`/`retryDoc`/`getLLMConfigDetail`/`addTicketRecord`/`logout(api)`）
- ✅ `ErrorFallback` 去导出，仅内部使用；`truncate`/`formatDateOnly` 移除

### UI/UX 精细化

- ✅ 全站触控目标 44×44px：icon 14→16，p-2→p-3.5，stop w-11 h-11
- ✅ 按压反馈：PageBtn/PortalLayout/AdminLayout 添加 active:scale-95
- ✅ 图标一致性：CheckCircle2→CheckCircle，全站 14-18px 统一，空状态 32px
- ✅ 间距网格对齐：gap-1→gap-2，3px→4px，18px→w-5
- ✅ WCAG AA 对比度：text-muted-48(暗色)/color-error/badge-warning-text/badge-neutral-text 全线达标

### 可访问性

- ✅ 3 处 aria-label 补全（select ×2 + file input）
- ✅ heading 层级 h1→h3 改为 h1→h2 三处
- ✅ ChatMessage 低置信度告警改用 `--badge-warning-text`（对比度 5.2:1）

### 功能完善

- ✅ 趋势图自定义日期上限 30 天校验，日期标签横排无滚动
- ✅ 筛选按钮 icon-only→icon+文字，浅色模式非激活态 bg-canvas+border-hairline
- ✅ 全站按钮图标补全（admin 申告/文章操作按钮、修改密码 Key 图标）
- ✅ 统计卡片 hover 阴影、font-bold、padding 优化
- ✅ ChatMessage 气泡半径使用 radius token（rounded-tr/tl-sm）

### 设计系统完善（本轮 #5）

- ✅ `<PageTitle>` 组件提取，15 页面迁移（消除 18 处 `text-hero font-semibold` 重复）
- ✅ `--font-size-display: 72px` 添加到 theme tokens，404 页 `text-[72px]` → `text-display`
- ✅ 表格操作列 `title: ''` → `title: '操作'`（users/roles/messages 三处，屏幕阅读器友好）
- ✅ `Skeleton` 骨架屏组件创建 (`ui/AppleSkeleton.tsx`)，`animate-pulse` + 统一 token

# OpsMind 验收测试流程文档

> **版本：** v1.0 | **日期：** 2026-06-23 | **适用版本：** OpsMind MVP

---

## 目录

1. [测试环境准备](#1-测试环境准备)
2. [预置数据概览](#2-预置数据概览)
3. [测试账号](#3-测试账号)
4. [测试场景总览](#4-测试场景总览)
5. [场景 A：认证与授权](#5-场景-a认证与授权)
6. [场景 B：LLM 配置管理](#6-场景-bllm-配置管理)
7. [场景 C：知识库管理](#7-场景-c知识库管理)
8. [场景 D：知识文章 — 手动创建全生命周期](#8-场景-d知识文章--手动创建全生命周期)
9. [场景 E：知识文章 — 文档上传与处理](#9-场景-e知识文章--文档上传与处理)
10. [场景 F：智能问答（RAG 管道）](#10-场景-f智能问答rag-管道)
11. [场景 G：申告工单全流程](#11-场景-g申告工单全流程)
12. [场景 H：用户与角色管理](#12-场景-h用户与角色管理)
13. [场景 I：数据看板与审计日志](#13-场景-i数据看板与审计日志)
14. [测试数据清单（人工填入）](#14-测试数据清单人工填入)
15. [测试文件清单](#15-测试文件清单)
16. [验收通过标准](#16-验收通过标准)

---

## 1. 测试环境准备

### 1.1 启动服务

```bash
# 在项目根目录执行

# 方式一：仅启动必须服务（不含 llama.cpp，使用 OpenAI API 作为 LLM 后端）
docker compose up -d --build

# 方式二：含本地 llama.cpp（需先下载模型：make model-download）
docker compose --profile ai-local up -d --build
```

### 1.2 数据库初始化

```bash
# 步骤 1：执行 DDL 增强脚本（pgvector 扩展 + HNSW 索引 + 列注释）
make db-init

# 步骤 2：加载必要种子数据（角色 + 用户 + 菜单 + LLM 配置 + 系统配置）
make db-seed
```

> **注意：** `make db-seed` 加载的是 `seed_essential.sql`，仅包含静态必要数据。知识库、知识文章、申告工单、站内消息等动态数据需要在测试过程中通过 API/UI 人工创建。

### 1.3 服务验证

```bash
# 检查服务状态
docker compose ps

# 预期输出：4 个服务均为 healthy（或 running）
#   opsmind-server  → healthy
#   opsmind-web     → healthy
#   postgres        → healthy
#   minio           → healthy

# 验证后端 API 可达
curl http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123"}'
# 预期：返回 access_token 和用户信息
```

---

## 2. 预置数据概览

`seed_essential.sql` 执行后，数据库中预置以下静态数据：

### 2.1 角色（4 个）

| ID | 角色名 | 权限码 |
|----|--------|--------|
| 1 | 系统管理员 | `user:manage`, `ticket:read/write/manage`, `knowledge:read/write/create/manage/review`, `dashboard:read`, `audit:read`, `system:config` |
| 2 | 运维人员 | `ticket:read`, `ticket:write`, `knowledge:read`, `knowledge:write` |
| 3 | 知识库管理员 | `knowledge:read/write/create/manage/review` |
| 4 | 报障人 | 无后台权限 |

### 2.2 菜单（8 个）

| ID | 名称 | 路径 |
|----|------|------|
| 1 | 仪表盘 | `/admin/dashboard` |
| 2 | 申告管理 | `/admin/tickets` |
| 3 | 知识库 | `/admin/knowledge` |
| 4 | 用户管理 | `/admin/users` |
| 5 | 角色管理 | `/admin/roles` |
| 6 | 审计日志 | `/admin/audit-logs` |
| 7 | 模型配置 | `/admin/config/llm` |
| 8 | 系统配置 | `/admin/config/system` |

> 所有角色关联全部菜单（菜单可见性由前端结合权限码控制）。

### 2.3 模型配置（3 条）

| ID | 名称 | 类型 | LLM 模型 | Embedding 模型 | 默认 |
|----|------|------|----------|---------------|------|
| 1 | 本地 llama.cpp | llama.cpp | Qwen3-4B-Q4_K_M | Qwen3-Embedding-0.6B-Q8_0 | ✓ |
| 2 | OpenAI GPT-4o-mini | OpenAI-compatible | gpt-4o-mini | text-embedding-3-small | |
| 3 | 本地 llama.cpp (宿主机) | llama.cpp | Qwen3-4B-Q4_K_M | Qwen3-Embedding-0.6B-Q8_0 | |

### 2.4 系统配置（9 条）

| Key | Value |
|-----|-------|
| `app_name` | `OpsMind` |
| `ai.rag_enabled` | `true` |
| `ai.top_k` | `5` |
| `ai.threshold` | `0.6` |
| `ai.max_history_messages` | `10` |
| `ai.rag_query_rewrite` | `true` |
| `ai.rag_multi_route` | `true` |
| `ai.rag_hybrid` | `true` |
| `ai.rag_rerank` | `true` |

### 2.5 未预置的动态数据（需人工创建）

以下数据 **不包含在 `seed_essential.sql` 中**，但提供了可直接执行的 SQL INSERT 模板。执行 `make db-seed` 后，连接 PostgreSQL 按需执行对应的 INSERT 语句即可快速搭建测试数据：

```bash
# 进入 PostgreSQL 容器执行 SQL
docker compose exec -T postgres psql -U opsmind -d opsmind
```

#### 2.5.1 知识库（KnowledgeBase）— 对应场景 C

```sql
INSERT INTO knowledge_bases (id, name, description, rag_workspace_slug, embedding_model, vector_dimension, llm_config_id, created_by, created_at, updated_at) VALUES
(1, 'IT 运维知识库', 'IT 运维常见问题和解决方案，涵盖网络、VPN、打印机、邮箱等',   'it-ops',         'Qwen3-Embedding-0.6B-Q8_0', 1024, 1, 1, NOW(), NOW()),
(2, '信息安全规范库', '信息安全管理制度、合规要求、安全操作规范',                     'info-sec',       'Qwen3-Embedding-0.6B-Q8_0', 1024, 1, 1, NOW(), NOW());

SELECT setval('knowledge_bases_id_seq', (SELECT MAX(id) FROM knowledge_bases));
```

#### 2.5.2 知识文章 — 手动创建（KnowledgeArticle）— 对应场景 D

```sql
-- source_type: 1=手动输入; status: 1=草稿 2=待审核 3=已通过 4=已发布 0=已停用
INSERT INTO knowledge_articles (id, kb_id, title, content, category, tags, status, source_type, word_count, created_by, created_at, updated_at) VALUES
(1, 1,
 'VPN 连接超时怎么办？',
 '## 问题描述

VPN 客户端连接时长时间无响应，最终提示「连接超时」。

## 解决方案

1. 检查当前网络是否能正常访问外网（打开百度测试）
2. 确认 VPN 服务器地址为 vpn.company.com:443
3. 尝试切换 VPN 协议：设置 → 首选项 → 协议 → L2TP/IPSec
4. 清除 VPN 客户端缓存：关闭客户端 → 删除 %LocalAppData%\AnyConnect 目录 → 重新连接
5. 尝试备用服务器 vpn2.company.com

## 如仍无法解决

请提交申告并提供当前 IP 地址（打开 https://ip.company.com 查看），以便网络组排查。',
 '网络与VPN',
 '["VPN", "连接", "超时"]',
 4, 1, 240, 1, NOW(), NOW()),

(2, 1,
 'Outlook 邮箱无法收发邮件排查步骤',
 '## 问题现象

- Outlook 客户端显示「已断开」或「正在连接...」
- 发送邮件卡在发件箱
- 无法接收新邮件

## 排查步骤

### 第一步：检查网络连接
确认当前网络能正常访问外网。公司内网用户确认连接的是 Corp-WiFi 或有线网络。

### 第二步：检查 Outlook 状态
1. 查看 Outlook 右下角状态栏
2. 如显示「需要密码」，点击输入域账号密码

### 第三步：尝试网页版
访问 https://mail.company.com 用相同账号登录。
- 网页版正常 → 问题在客户端

### 第四步：重建 Outlook 配置文件
1. 关闭 Outlook
2. 控制面板 → 邮件 (Microsoft Outlook) → 显示配置文件
3. 删除当前配置文件 → 添加新配置文件
4. 按提示输入姓名、邮箱、密码

## 备注
邮箱服务器为 Exchange Online（Office 365），配置文件中服务器地址应为 outlook.office365.com。',
 '邮箱与办公',
 '["Outlook", "邮箱", "邮件", "Exchange"]',
 4, 1, 350, 1, NOW(), NOW()),

(3, 1,
 '打印机卡纸处理指南',
 '## 适用机型

HP LaserJet M404 / M507 系列黑白激光打印机。

## 卡纸常见位置及处理

### 1. 进纸盒卡纸（最常见）
- 拉出进纸盒
- 轻轻取出卡住的纸张（注意纸张方向，勿撕裂）
- 检查纸张是否受潮或褶皱，如有则更换新纸

### 2. 硒鼓区域卡纸
- 打开前盖
- 取出硒鼓
- 检查并取出卡纸

### 3. 出纸口卡纸
- 从出纸口轻轻拉出纸张

## 预防措施
- 使用 70-80g 标准 A4 复印纸
- 装纸前将纸张抖松，防止静电粘连
- 纸盒内纸张不超过 MAX 标记线

## 如问题持续
打印机面板显示错误代码时，记录代码后提交申告。常见代码：
- 13.XX.XX = 卡纸
- 11.XX = 纸盒问题
- 50.X = 定影器错误',
 '办公设备',
 '["打印机", "卡纸", "HP"]',
 1, 1, 280, 1, NOW(), NOW());

SELECT setval('knowledge_articles_id_seq', (SELECT MAX(id) FROM knowledge_articles));
```

> **注意：** 以上 SQL 直接插入已发布（status=4）的文章，跳过了草稿→审核→发布流程，适合快速验证 RAG 检索。若需测试完整状态机（场景 D 8.5-8.10），请通过 API 逐步骤操作，或先插入 status=1 的文章再走审核流程。

#### 2.5.3 申告工单（Ticket）— 对应场景 G

```sql
-- status: 1=待处理 2=处理中 3=需补充信息 4=已解决 5=已关闭
-- urgency: 1=一般 2=紧急 3=非常紧急
-- impact_scope: 0=未指定 1=个人 2=部门/团队 3=全员
INSERT INTO tickets (id, ticket_no, user_id, title, description, urgency, impact_scope, affected_systems, contact_phone, contact_email, status, supplement_count, source, created_at, updated_at) VALUES
(1, 'TK-20260624-000001', 5,
 '3 楼东侧打印机频繁卡纸',
 '今天上午开始，3 楼东侧公共打印机（HP LaserJet M404）频繁卡纸，已发生 5 次以上。每次需要手动取出卡纸，严重影响部门日常工作。尝试过重启打印机但问题依旧。',
 2, 2,
 '["打印机", "打印服务"]',
 '13800000005', 'zhaoyonghu@opsmind.local',
 4, 0, 1, NOW(), NOW()),

(2, 'TK-20260624-000002', 5,
 'VPN 连接每隔 10 分钟自动断开',
 '远程办公时 VPN 每隔 10-20 分钟就自动断开，需要手动重连，影响远程工作效率。已检查家庭网络正常，重启过电脑和路由器。',
 3, 1,
 '["VPN"]',
 '13800000005', 'zhaoyonghu@opsmind.local',
 5, 1, 1, NOW(), NOW());

SELECT setval('tickets_id_seq', (SELECT MAX(id) FROM tickets));
```

#### 2.5.4 申告处理记录（TicketRecord）— 对应场景 G

```sql
-- 申告 1 的处理记录：待处理 → 处理中 → 已解决
INSERT INTO ticket_records (ticket_id, operator_id, action, content, created_at) VALUES
(1, 2, 'start',   '已接单，正在排查 3 楼打印机。',                                   '2026-06-24 09:15:00'),
(1, 2, 'note',    '已到达 3 楼现场。检查发现硒鼓废粉盒已满，已更换。测试打印 10 张无卡纸。', '2026-06-24 10:00:00'),
(1, 2, 'resolve', '打印机关机更换废粉盒后恢复正常。已测试正常打印。',                       '2026-06-24 10:30:00'),

-- 申告 2 的处理记录：待处理 → 处理中 → 需补充信息 → 补充 → 处理中 → 已关闭
(2, 2, 'start',        '已接单。',                                                         '2026-06-24 09:20:00'),
(2, 2, 'request_info', '请提供：1) 宽带运营商和套餐带宽；2) 是否同时有 Teams 通话或大文件下载；3) VPN 客户端日志。', '2026-06-24 10:00:00'),
(2, 5, 'supplement',   '补充：1) 宽带为中国电信 300M；2) VPN 断开时没有 Teams 通话或下载；3) 日志已发送至 IT 邮箱。', '2026-06-24 11:00:00'),
(2, 2, 'close',        '用户超过 7 天未响应，且问题无法复现，关闭此申告。',                       '2026-06-24 18:00:00');
```

#### 2.5.5 问答会话（ChatSession）— 对应场景 F

```sql
-- kb_id 关联知识库 1，user_id 关联 reporter1
INSERT INTO chat_sessions (id, user_id, kb_id, question, answer, sources, confidence, feedback, duration_ms, created_at) VALUES
(1, 5, 1,
 '如何解决 VPN 连接超时的问题？',
 'VPN 连接超时的问题可以通过以下步骤排查：

1. 检查当前网络是否能正常访问外网
2. 确认 VPN 服务器地址为 vpn.company.com:443
3. 尝试切换 VPN 协议为 L2TP/IPSec
4. 清除 VPN 客户端缓存
5. 尝试备用服务器 vpn2.company.com

如仍无法解决，建议提交申告并提供当前 IP 地址。',
 '[{"doc_name":"VPN 连接超时怎么办？","chunk_content":"VPN 客户端连接时长时间无响应...","confidence":0.92}]',
 0.92, 1, 3200, NOW()),

(2, 5, 1,
 '如何配置思科路由器 BGP 协议？',
 '很抱歉，当前知识库暂未收录关于思科路由器 BGP 协议配置的详细内容。建议您通过申告系统提交具体需求，相关运维同事会尽快处理。',
 '[]',
 0.23, 0, 1500, NOW());

SELECT setval('chat_sessions_id_seq', (SELECT MAX(id) FROM chat_sessions));
```

#### 2.5.6 对话消息（ChatMessage）— 对应场景 F

```sql
-- 会话 1 的两轮对话（user → assistant → user → assistant）
INSERT INTO chat_messages (session_id, role, content, sources, confidence, pipeline_metrics, created_at) VALUES
-- 第一轮
(1, 'user',      '如何解决 VPN 连接超时的问题？',
 NULL, NULL, NULL,
 '2026-06-24 14:00:00'),
(1, 'assistant', 'VPN 连接超时的问题可以通过以下步骤排查...',
 '[{"doc_name":"VPN 连接超时怎么办？","confidence":0.92}]', 0.92,
 '{"query_rewrite":{"duration_ms":120},"vector_retrieve":{"duration_ms":220,"count":5},"bm25_retrieve":{"duration_ms":80,"count":3},"hybrid_fuse":{"duration_ms":15},"rerank":{"duration_ms":180},"llm_generate":{"duration_ms":2500,"tokens":128}}',
 '2026-06-24 14:00:03'),
-- 第二轮（多轮追问，指代消解）
(1, 'user',      '它的备用服务器地址是什么？',
 NULL, NULL, NULL,
 '2026-06-24 14:01:00'),
(1, 'assistant', 'VPN 的备用服务器地址是 vpn2.company.com。当主服务器 vpn.company.com:443 不可用时，可尝试连接备用服务器。',
 '[{"doc_name":"VPN 连接超时怎么办？","confidence":0.88}]', 0.88,
 '{"query_rewrite":{"rewritten":"VPN 备用服务器地址","duration_ms":110},"vector_retrieve":{"duration_ms":190,"count":5},"bm25_retrieve":{"duration_ms":75,"count":3},"hybrid_fuse":{"duration_ms":12},"rerank":{"duration_ms":165},"llm_generate":{"duration_ms":1800,"tokens":56}}',
 '2026-06-24 14:01:04');
```

#### 2.5.7 站内消息（Message）— 对应场景 G+I

```sql
-- 申告处理过程中的站内通知
INSERT INTO messages (user_id, title, content, type, related_type, related_id, is_read, created_at) VALUES
(5, '申告处理通知',
 '您的申告「3 楼东侧打印机频繁卡纸」(TK-20260624-000001) 已被 张运维 接单处理，当前状态：处理中。',
 'ticket_update', 'ticket', 1, false, '2026-06-24 09:15:00'),

(5, '申告已解决',
 '您的申告「3 楼东侧打印机频繁卡纸」(TK-20260624-000001) 已解决。处理结果：打印机关机更换废粉盒后恢复正常。如有疑问请回复本条消息。',
 'ticket_resolved', 'ticket', 1, false, '2026-06-24 10:30:00'),

(5, '申告需补充信息',
 '您的申告「VPN 连接每隔 10 分钟自动断开」(TK-20260624-000002) 需要补充信息。请登录门户查看详情。',
 'ticket_supplement', 'ticket', 2, false, '2026-06-24 10:00:00'),

(5, '申告已关闭',
 '您的申告「VPN 连接每隔 10 分钟自动断开」(TK-20260624-000002) 已关闭。如有疑问请重新提交申告。',
 'ticket_closed', 'ticket', 2, false, '2026-06-24 18:00:00');
```

#### 2.5.8 审计日志（AuditLog）— 操作时自动生成

审计日志由 Service 层在关键操作时自动写入，无需手动 INSERT。触发操作包括：

- 用户管理：创建/编辑/冻结/恢复用户
- 知识管理：创建/审核/发布/停用文章
- 申告管理：状态变更（start/resolve/close 等）
- 系统配置：修改系统配置项
- LLM 配置：创建/更新/删除 LLM 配置

若需验证审计日志列表（场景 I 13.2），先执行上述 2.5.1-2.5.7 的数据导入（触发 Service 层的审计写入），再查询 `audit_logs` 表即可看到自动生成的记录。

---

## 3. 测试账号

| 用户名 | 密码 | 角色 | 说明 |
|--------|------|------|------|
| `admin` | `Admin@123` | 系统管理员 | 全部权限，可访问所有后台功能 |
| `operator1` | `OpsMind@123` | 运维人员 | 处理申告，查看知识库 |
| `operator2` | `OpsMind@123` | 运维人员 | 同上（用于测试审核人≠创建人规则） |
| `knowledge` | `Knowledge@123` | 知识库管理员 | 维护知识库，审核文章 |
| `reporter1` | `Reporter@123` | 报障人 | 门户端：问答、申告提交、进度查询 |
| `reporter2` | `Reporter@123` | 报障人（首次登录） | 同上，`first_login=true`，需修改密码 |

---

## 4. 测试场景总览

```
场景 A：认证与授权           ← 所有场景的前置
场景 B：LLM 配置管理          ← 场景 C/D/E/F 的前置
场景 C：知识库管理            ← 场景 D/E 的前置
场景 D：知识文章手动创建      ← 场景 F 的前置（需已发布文章）
场景 E：文档上传与处理        ← 场景 F 的前置（需已上传处理完成的文档）
场景 F：智能问答（RAG 管道）  ← 核心场景
场景 G：申告工单全流程        ← 核心场景
场景 H：用户与角色管理        ← 独立场景
场景 I：数据看板与审计日志    ← 依赖前面场景产生的数据
```

> **建议按顺序测试。** 场景 A→B→C→D/E→F→G 有强依赖关系，场景 H/I 可独立或最后执行。

---

## 5. 场景 A：认证与授权

### 5.1 登录

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "Admin@123"
}
```

**验证点：**
- [ ] 返回 `code=0`，`access_token` 和 `refresh_token` 非空
- [ ] `user.real_name` = "系统管理员"
- [ ] `roles` 包含 "系统管理员"
- [ ] `permissions` 包含 `user:manage`、`ticket:manage`、`knowledge:manage` 等
- [ ] `menus` 返回 8 个菜单项

**异常测试：**

| 测试项 | 输入 | 预期 code | 预期 message |
|--------|------|-----------|-------------|
| 用户名不存在 | `{"username":"nobody","password":"xxx"}` | 10003 | 用户名或密码错误 |
| 密码错误 | `{"username":"admin","password":"wrong"}` | 10003 | 用户名或密码错误 |
| 账号已冻结 | `{"username":"frozen_user","password":"xxx"}` | 10002 | 账号已被冻结 |
| 缺少字段 | `{"username":"admin"}` | 10003 | 参数校验失败 |

### 5.2 Token 刷新

```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "<login 返回的 refresh_token>"
}
```

**验证点：**
- [ ] 返回新的 `access_token` 和 `refresh_token`
- [ ] 旧 `refresh_token` 失效（再次使用返回 10001）

### 5.3 修改密码

```http
POST /api/v1/auth/me/change-password
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "old_password": "Admin@123",
  "new_password": "OpsMind@2024"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 使用旧密码登录返回 10003
- [ ] 使用新密码登录成功

> **测试完成后请改回原密码 `Admin@123`，以免影响后续测试。**

**密码策略异常测试：**

| 测试项 | 新密码 | 预期 code |
|--------|--------|-----------|
| 过短 | `Ab1` | 10003 |
| 缺大写 | `admin@123` | 10003 |
| 缺小写 | `ADMIN@123` | 10003 |
| 缺数字 | `Admin@xxx` | 10003 |

### 5.4 登出

```http
POST /api/v1/auth/me/logout
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "refresh_token": "<当前 refresh_token>"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 登出后 refresh_token 被拉黑，刷新返回 10001

### 5.5 权限校验

使用 `reporter1` 账号登录后尝试访问后台接口：

```http
GET /api/v1/admin/users
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回 `code=10002`（无权限）

---

## 6. 场景 B：LLM 配置管理

> **前置条件：** 使用 `admin` 账号登录并获取 token。

### 6.1 查看配置列表

```http
GET /api/v1/admin/llm-configs
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 3 条预置配置
- [ ] ID=1 的配置 `is_default=true`，`provider_type=1`（llama.cpp）
- [ ] ID=2 的配置 `is_default=false`，`provider_type=2`（OpenAI）

### 6.2 查看配置详情

```http
GET /api/v1/admin/llm-configs/1
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] `api_key` 掩码显示（空字符串或 `****`）

### 6.3 测试连接

```http
POST /api/v1/admin/llm-configs/1/test
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 若 LLM 服务可用：`code=0`，`success=true`，含 `latency_ms` 和 `model`
- [ ] 若 LLM 服务不可用：`code=20001`，message 包含错误原因

### 6.4 创建新配置

```http
POST /api/v1/admin/llm-configs
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "测试配置-DeepSeek",
  "provider_type": 2,
  "base_url": "https://api.deepseek.com/v1",
  "api_key": "sk-test-key-12345",
  "llm_model": "deepseek-chat",
  "embedding_model": "text-embedding-3-small",
  "max_tokens": 8192,
  "vector_dimension": 1536,
  "is_default": false
}
```

**验证点：**
- [ ] 返回 `code=0`，含新配置 ID
- [ ] 列表中可见新配置（3 条）

### 6.5 设为默认 — 热替换

```http
PUT /api/v1/admin/llm-configs/3
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "测试配置-DeepSeek",
  "provider_type": 2,
  "base_url": "https://api.deepseek.com/v1",
  "api_key": "sk-test-key-12345",
  "llm_model": "deepseek-chat",
  "embedding_model": "text-embedding-3-small",
  "max_tokens": 8192,
  "vector_dimension": 1536,
  "is_default": true
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 重新查看列表，ID=3 的 `is_default=true`，ID=1 的 `is_default=false`

> **测试完成后将 ID=1 改回默认：** PUT `/api/v1/admin/llm-configs/1`，`is_default: true`

### 6.6 删除非默认配置

```http
DELETE /api/v1/admin/llm-configs/3
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 列表中恢复为 2 条

**异常测试：** 尝试删除默认配置（ID=1）
- [ ] 返回 `code=10003`，message 包含「不能删除默认配置」

### 6.7 更新配置

```http
PUT /api/v1/admin/llm-configs/2
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "OpenAI API（测试更新）",
  "provider_type": 2,
  "base_url": "https://api.openai.com/v1",
  "api_key": "",
  "llm_model": "gpt-4o-mini",
  "embedding_model": "text-embedding-3-small",
  "max_tokens": 16384,
  "vector_dimension": 1536,
  "is_default": false
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 详情中 `name` 已更新，`api_key` 保留原值（不传 `api_key` 不清空）

---

## 7. 场景 C：知识库管理

> **前置条件：** admin 管理员账号已登录。LLM 配置至少有 1 条默认配置。

### 7.1 创建知识库

使用 `admin` 账号创建 2 个知识库：

```http
POST /api/v1/admin/knowledge-bases
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "IT 运维知识库",
  "description": "IT 运维常见问题和解决方案，涵盖网络、VPN、打印机、邮箱等",
  "embedding_model": "Qwen3-Embedding-0.6B-Q8_0",
  "vector_dimension": 1024,
  "llm_config_id": 1
}
```

将返回的 ID 记为 **`kb_id_1`**（预期为 1）。

```http
POST /api/v1/admin/knowledge-bases
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "信息安全规范库",
  "description": "信息安全管理制度、合规要求、安全操作规范",
  "embedding_model": "Qwen3-Embedding-0.6B-Q8_0",
  "vector_dimension": 1024,
  "llm_config_id": 1
}
```

将返回的 ID 记为 **`kb_id_2`**（预期为 2）。

**验证点：**
- [ ] 两次创建均返回 `code=0`
- [ ] 返回的 `data.id` 递增

**异常测试：** 重复创建同名知识库
- [ ] 返回 `code=10005`（资源冲突）

### 7.2 查看知识库列表

```http
GET /api/v1/admin/knowledge-bases
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 2 条记录
- [ ] 包含 `name`、`description`、`embedding_model`、`vector_dimension`、`article_count`（当前为 0）

### 7.3 门户端知识库列表

使用 `reporter1` 账号：

```http
GET /api/v1/portal/knowledge-bases
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回知识库基本信息（仅 `id`、`name`、`description`）
- [ ] 不包含 `embedding_model`、`llm_config_id` 等管理字段

### 7.4 更新知识库

```http
PUT /api/v1/admin/knowledge-bases/1
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "IT 运维知识库（2024版）",
  "description": "更新后的描述信息"
}
```

**验证点：**
- [ ] 返回 `code=0`，名称和描述已更新

---

## 8. 场景 D：知识文章 — 手动创建全生命周期

> **前置条件：** 知识库 `kb_id_1`（IT 运维知识库）已创建。使用 `admin` 或 `knowledge` 账号。

### 8.1 创建文章（草稿）

使用 `admin` 账号在 `kb_id_1` 下创建 3 篇文章：

**文章 1：**

```http
POST /api/v1/admin/knowledge-bases/1/articles
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "title": "VPN 连接超时怎么办？",
  "content": "## 问题描述\n\nVPN 客户端连接时长时间无响应，最终提示「连接超时」。\n\n## 解决方案\n\n1. 检查当前网络是否能正常访问外网（打开百度测试）\n2. 确认 VPN 服务器地址为 vpn.company.com:443\n3. 尝试切换 VPN 协议：设置 → 首选项 → 协议 → L2TP/IPSec\n4. 清除 VPN 客户端缓存：关闭客户端 → 删除 %LocalAppData%\\AnyConnect 目录 → 重新连接\n5. 尝试备用服务器 vpn2.company.com\n\n## 如仍无法解决\n\n请提交申告并提供当前 IP 地址（打开 https://ip.company.com 查看），以便网络组排查。",
  "source_type": 1,
  "category": "网络与VPN",
  "tags": ["VPN", "连接", "超时"]
}
```

将返回 ID 记为 **`article_id_1`**。

**文章 2：**

```http
POST /api/v1/admin/knowledge-bases/1/articles
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "title": "Outlook 邮箱无法收发邮件排查步骤",
  "content": "## 问题现象\n\n- Outlook 客户端显示「已断开」或「正在连接...」\n- 发送邮件卡在发件箱\n- 无法接收新邮件\n\n## 排查步骤\n\n### 第一步：检查网络连接\n确认当前网络能正常访问外网。公司内网用户确认连接的是 Corp-WiFi 或有线网络。\n\n### 第二步：检查 Outlook 状态\n1. 查看 Outlook 右下角状态栏\n2. 如显示「需要密码」，点击输入域账号密码\n3. 如显示「正在连接...」，等待 1 分钟后仍无变化则进入下一步\n\n### 第三步：尝试网页版\n访问 https://mail.company.com 用相同账号登录。\n- 网页版正常 → 问题在客户端，继续第四步\n- 网页版也不正常 → 可能是账号或服务器问题，联系 IT 服务台\n\n### 第四步：重建 Outlook 配置文件\n1. 关闭 Outlook\n2. 控制面板 → 邮件 (Microsoft Outlook) → 显示配置文件\n3. 删除当前配置文件 → 添加新配置文件\n4. 按提示输入姓名、邮箱、密码\n5. 完成后设为「始终使用此配置文件」\n6. 重新打开 Outlook\n\n## 备注\n邮箱服务器为 Exchange Online（Office 365），配置文件中服务器地址应为 outlook.office365.com。",
  "source_type": 1,
  "category": "邮箱与办公",
  "tags": ["Outlook", "邮箱", "邮件", "Exchange"]
}
```

将返回 ID 记为 **`article_id_2`**。

**文章 3：**

```http
POST /api/v1/admin/knowledge-bases/1/articles
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "title": "打印机卡纸处理指南",
  "content": "## 适用机型\n\nHP LaserJet M404 / M507 系列黑白激光打印机。\n\n## 卡纸常见位置及处理\n\n### 1. 进纸盒卡纸（最常见）\n- 拉出进纸盒\n- 轻轻取出卡住的纸张（注意纸张方向，勿撕裂）\n- 检查纸张是否受潮或褶皱，如有则更换新纸\n- 重新装入纸张，调整纸宽导轨至合适位置\n\n### 2. 硒鼓区域卡纸\n- 打开前盖\n- 取出硒鼓\n- 检查并取出卡纸\n- 重新安装硒鼓，合上前盖\n\n### 3. 出纸口卡纸\n- 从出纸口轻轻拉出纸张\n- 如纸张已撕裂，打开后盖检查是否有纸屑残留\n\n## 预防措施\n- 使用 70-80g 标准 A4 复印纸\n- 装纸前将纸张抖松，防止静电粘连\n- 纸盒内纸张不超过 MAX 标记线\n- 避免使用已褶皱或受潮的纸张\n\n## 如问题持续\n打印机面板显示错误代码时，记录代码后提交申告。常见代码：\n- 13.XX.XX = 卡纸\n- 11.XX = 纸盒问题\n- 50.X = 定影器错误（需更换维护套件）",
  "source_type": 1,
  "category": "办公设备",
  "tags": ["打印机", "卡纸", "HP"]
}
```

将返回 ID 记为 **`article_id_3`**。

**验证点（每篇文章）：**
- [ ] 返回 `code=0`
- [ ] `status=1`（草稿）
- [ ] `word_count` 自动计算

### 8.2 查看文章列表

```http
GET /api/v1/admin/knowledge-bases/1/articles?status=1
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 3 篇文章，状态均为「草稿」
- [ ] 分页信息正确（`total=3`）

### 8.3 查看文章详情

```http
GET /api/v1/admin/articles/1
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回完整文章内容、标签、分类
- [ ] `chunks` 数组为空（未发布时无分块）
- [ ] `status_text` = "草稿"
- [ ] `source_type_text` = "手动输入"

### 8.4 编辑文章

```http
PUT /api/v1/admin/articles/1
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "title": "VPN 连接超时问题处理（修订版）",
  "content": "## 问题描述\n\nVPN 客户端连接时长时间无响应，最终提示「连接超时」。\n\n## 解决方案\n\n1. 检查当前网络是否能正常访问外网\n2. 确认 VPN 服务器地址为 vpn.company.com:443\n3. 尝试切换 VPN 协议为 L2TP/IPSec\n4. 清除 VPN 客户端缓存\n5. 尝试备用服务器 vpn2.company.com\n6. 如使用家庭网络，检查路由器 VPN Passthrough 设置\n\n## 如仍无法解决\n\n提交申告并提供当前 IP 地址，以便网络组排查。",
  "category": "网络与VPN",
  "tags": ["VPN", "连接", "超时"]
}
```

**验证点：**
- [ ] 仅编辑草稿状态文章成功

### 8.5 提交审核

对文章 1 (`article_id_1`) 提交审核：

```http
POST /api/v1/admin/articles/1/submit-review
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 文章状态变为「待审核(2)」

### 8.6 审核操作

使用 `knowledge` 账号（知识库管理员，不是文章 1 的创建者 admin）登录，审核文章 1：

```http
POST /api/v1/admin/articles/1/review
Authorization: Bearer <knowledge_token>
Content-Type: application/json

{
  "approved": true,
  "review_comment": "内容准确，步骤清晰，通过审核。"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 文章状态变为「已通过(3)」

**异常测试 — 审核人=创建人：** 用 `admin` 审核自己创建的文章 2
- [ ] 返回 `code=10003`（审核人不能是创建人）

用 `knowledge` 账号审核文章 2：

```http
POST /api/v1/admin/articles/2/review
Authorization: Bearer <knowledge_token>
Content-Type: application/json

{
  "approved": false,
  "review_comment": "邮箱服务器地址信息需要确认，目前公司正在迁移到 Exchange Online，请更新后重新提交。"
}
```

**验证点：**
- [ ] 文章 2 状态变为「已驳回(5)」
- [ ] `review_comment` 已记录

### 8.7 驳回后编辑重提

用 `admin` 编辑被驳回的文章 2：

```http
PUT /api/v1/admin/articles/2
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "title": "Outlook 邮箱无法收发邮件排查步骤（修订版）",
  "content": "...（更新后的内容）",
  "category": "邮箱与办公",
  "tags": ["Outlook", "邮箱"]
}
```

```http
POST /api/v1/admin/articles/2/submit-review
Authorization: Bearer <admin_token>
```

```http
POST /api/v1/admin/articles/2/review
Authorization: Bearer <knowledge_token>
Content-Type: application/json

{
  "approved": true,
  "review_comment": "已更新，通过。"
}
```

**验证点：**
- [ ] 文章 2 状态变为「已通过(3)」

### 8.8 发布文章

发布文章 1 和文章 2：

```http
POST /api/v1/admin/articles/1/publish
Authorization: Bearer <knowledge_token>
```

```http
POST /api/v1/admin/articles/2/publish
Authorization: Bearer <knowledge_token>
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 文章状态变为「已发布(4)」
- [ ] `chunk_count` > 0（已生成分块和向量）
- [ ] 文章详情中 `chunks` 数组非空
- [ ] `published_by` 记录了发布人

> **注意：** 发布操作会调用 Embedding API 生成向量并写入 pgvector。如果 Embedding 服务不可用，发布将失败（`code=20001`）。

### 8.9 停用与启用

```http
POST /api/v1/admin/articles/1/disable
Authorization: Bearer <knowledge_token>
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 文章状态变为「已停用(0)」
- [ ] 文章详情中 `chunks` 为空（向量已清除）

```http
POST /api/v1/admin/articles/1/enable
Authorization: Bearer <knowledge_token>
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 文章状态恢复为「已发布(4)」
- [ ] `chunks` 重新生成（向量已重建）

### 8.10 完整状态机校验

| 当前状态 | 操作 | 预期结果 |
|----------|------|----------|
| 草稿(1) | 提交审核 | ✓ 变为待审核(2) |
| 草稿(1) | 直接发布 | ✗ code=10003 |
| 待审核(2) | 通过 | ✓ 变为已通过(3) |
| 待审核(2) | 驳回 | ✓ 变为已驳回(5) |
| 待审核(2) | 发布 | ✗ code=10003 |
| 已通过(3) | 发布 | ✓ 变为已发布(4) |
| 已发布(4) | 停用 | ✓ 变为已停用(0) |
| 已停用(0) | 启用 | ✓ 变为已发布(4) |
| 已发布(4) | 提交审核 | ✗ code=10003 |

---

## 9. 场景 E：知识文章 — 文档上传与处理

> **前置条件：** 知识库 `kb_id_1` 已创建，Embedding 服务可用。使用 `admin` 或 `knowledge` 账号。
>
> **测试文件位置：** `test/files/` 目录，包含 md/txt/docx/pdf 四种格式。

### 9.1 上传 Markdown 文件

```bash
curl -X POST http://localhost:8080/api/v1/admin/knowledge-bases/1/documents/upload \
  -H "Authorization: Bearer <admin_token>" \
  -F "files=@test/files/md/网络故障排查指南.md"
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] `documents[0].article_id` 返回新文章 ID（记为 `doc_article_md`）
- [ ] `documents[0].file_name` = "网络故障排查指南.md"
- [ ] `documents[0].file_type` = "md"
- [ ] `documents[0].process_status` = "pending"

### 9.2 上传 TXT 文件

```bash
curl -X POST http://localhost:8080/api/v1/admin/knowledge-bases/1/documents/upload \
  -H "Authorization: Bearer <admin_token>" \
  -F "files=@test/files/txt/VPN使用常见问题.txt"
```

### 9.3 批量上传（MD + TXT + DOCX + PDF）

```bash
curl -X POST http://localhost:8080/api/v1/admin/knowledge-bases/2/documents/upload \
  -H "Authorization: Bearer <admin_token>" \
  -F "files=@test/files/md/新员工IT入职指南.md" \
  -F "files=@test/files/txt/会议室设备操作说明.txt" \
  -F "files=@test/files/docx/邮件系统使用规范.docx" \
  -F "files=@test/files/pdf/信息安全管理制度.pdf"
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] `documents` 数组包含 4 个元素，各有独立的 `article_id`
- [ ] 4 个文件上传到知识库 2

### 9.4 查询文档处理状态

取 9.1 上传的文档 ID（`doc_article_md`），轮询处理状态：

```http
GET /api/v1/admin/knowledge-bases/1/documents/{doc_article_md}/status
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 初始状态为 `pending`
- [ ] 随后状态按 `pending → parsing → chunking → embedding → indexing → completed` 流转
- [ ] 最终状态为 `completed`

> **预计处理时间：** 小文件（<50KB）约 5-30 秒，取决于 Embedding API 性能。

### 9.5 处理完成后验证

```http
GET /api/v1/admin/articles/{doc_article_md}
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] `process_status` = "completed"
- [ ] `source_type` = 2（文档上传）
- [ ] `content` 包含文档解析后的文本
- [ ] `word_count` > 0
- [ ] `file_type` = "md"
- [ ] `minio_path` 非空（文件已存储到 MinIO）

### 9.6 异常文件上传测试

| 测试项 | 操作 | 预期 code | 预期 message |
|--------|------|-----------|-------------|
| 不支持格式 | 上传 `.exe` 文件 | 10003 | 不支持的文件格式 |
| 文件过大 | 上传 >50MB 文件 | 10003 | 文件过大 |
| KB 不存在 | 上传到 KB 999 | 10004 | 知识库不存在 |

---

## 10. 场景 F：智能问答（RAG 管道）

> **前置条件：**
> - 知识库 `kb_id_1` 中至少已有 2 篇已发布文章（场景 D 的 article 1+2）和 1 份处理完成的文档（场景 E 的 md）
> - LLM 默认配置可用（场景 B 验证通过）
> - 使用 `reporter1` 或 `reporter2` 账号（门户端）

### 10.1 创建问答会话

```http
POST /api/v1/portal/chat-sessions
Authorization: Bearer <reporter1_token>
Content-Type: application/json

{
  "kb_id": 1,
  "title": "VPN 连接问题咨询"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] `data.session_id` 返回会话 ID（记为 `session_id`）
- [ ] `data.kb_id` = 1

### 10.2 发送消息（SSE 流式）

> **重要：** SSE 流式响应无法通过普通 curl 测试，推荐使用以下方式之一：
> - 前端 UI：http://localhost:3000/portal/chat
> - 浏览器 DevTools Network 面板观察 EventStream
> - 或使用 `curl -N` 观察原始 SSE 事件

**测试问题 1（应命中 VPN 知识）：**

```
如何解决 VPN 连接超时的问题？
```

**预期 SSE 事件序列：**

```
data: {"type":"step","id":"query_rewrite","label":"查询改写"}
data: {"type":"step","id":"multi_route","label":"多路检索"}
data: {"type":"step","id":"vector_retrieve","label":"向量检索"}
data: {"type":"step","id":"bm25_retrieve","label":"BM25检索"}
data: {"type":"step","id":"hybrid_fuse","label":"结果融合"}
data: {"type":"step","id":"rerank","label":"重排序"}
data: {"type":"step","id":"llm_generate","label":"LLM 生成"}
data: {"type":"token","content":"VPN 连接"}
data: {"type":"token","content":"超时..."}
...
data: {"type":"done","metadata":{...}}
```

**验证点：**
- [ ] 收到全部 7 个 `step` 事件（顺序如上）
- [ ] 收到多个 `token` 事件，内容逐步拼接成完整答案
- [ ] 收到 `done` 事件
- [ ] `done` 事件中 `metadata.confidence` 应 > 0.6（知识库中有匹配内容）
- [ ] `metadata.sources` 数组非空，至少包含 1 个相关知识来源
- [ ] `metadata.sources[].confidence` > 0
- [ ] `metadata.pipeline.steps` 包含各步骤耗时
- [ ] `metadata.can_submit_ticket` = false（高置信度）

**测试问题 2（多轮对话——利用会话历史）：**

```
它的备用服务器地址是什么？
```

**验证点：**
- [ ] 查询改写步骤将指代词「它」解析为「VPN」
- [ ] 答案引用知识库中关于 VPN 备用服务器（vpn2.company.com）的内容

**测试问题 3（低置信度——引导申告）：**

```
如何配置思科路由器 BGP 协议？
```

**验证点：**
- [ ] `metadata.confidence` 应 < 0.6（知识库中无此内容）
- [ ] `metadata.can_submit_ticket` = true
- [ ] 答案中包含兜底提示（建议提交申告）

### 10.3 查看会话列表

```http
GET /api/v1/portal/chat-sessions
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回已创建的会话列表

### 10.4 查看会话详情（含多轮对话历史）

```http
GET /api/v1/portal/chat-sessions/{session_id}
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] `messages` 包含多轮 Q&A 历史
- [ ] `messages[0].role` = "user"（第一轮用户问题）
- [ ] `messages[1].role` = "assistant"（第一轮 AI 回答）
- [ ] `messages[2].role` = "user"（第二轮用户追问）
- [ ] assistant 消息包含 `sources` 和 `confidence`
- [ ] `pipeline` 包含各步骤耗时

### 10.5 提交反馈

```http
POST /api/v1/portal/chat-sessions/{session_id}/feedback
Authorization: Bearer <reporter1_token>
Content-Type: application/json

{
  "feedback": 1
}
```

**验证点：**
- [ ] `feedback=1`（已解决）成功
- [ ] `feedback=2`（未解决）成功
- [ ] `feedback=0` 返回 10003（不允许）
- [ ] 再次提交反馈（重复操作）：后端限制反馈只能提交一次

### 10.6 删除会话

```http
DELETE /api/v1/portal/chat-sessions/{session_id}
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 再次查看返回 404

---

## 11. 场景 G：申告工单全流程

> **前置条件：** 至少有一个知识库、已发布文章、LLM 配置可用。
> 涉及账号：`reporter1`（报障人）、`operator1`（运维人员）、`operator2`（运维人员）、`admin`（管理员）。

### 11.1 门户端 — 创建申告

使用 `reporter1` 账号：

```http
POST /api/v1/portal/tickets
Authorization: Bearer <reporter1_token>
Content-Type: application/json

{
  "title": "3 楼东侧打印机频繁卡纸",
  "description": "今天上午开始，3 楼东侧公共打印机（HP LaserJet M404）频繁卡纸，已发生 5 次以上。每次需要手动取出卡纸，严重影响部门日常工作。尝试过重启打印机但问题依旧。",
  "urgency": 2,
  "impact_scope": 2,
  "affected_systems": ["打印机", "打印服务"],
  "contact_phone": "13800000005",
  "contact_email": "zhaoyonghu@opsmind.local"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 自动生成 `ticket_no`（格式：`TK-YYYYMMDD-XXXXXX`）

记下返回的 ticket ID，记为 **`ticket_id_1`**（或通过列表查询获取）。

创建第二份申告：

```http
POST /api/v1/portal/tickets
Authorization: Bearer <reporter1_token>
Content-Type: application/json

{
  "title": "VPN 连接每隔 10 分钟自动断开",
  "description": "远程办公时 VPN 每隔 10-20 分钟就自动断开，需要手动重连，影响远程工作效率。已检查家庭网络正常，重启过电脑和路由器。",
  "urgency": 3,
  "impact_scope": 1,
  "contact_phone": "13800000005",
  "contact_email": "zhaoyonghu@opsmind.local"
}
```

记为 **`ticket_id_2`**。

### 11.2 门户端 — 查询我的申告

```http
GET /api/v1/portal/tickets?page=1&page_size=10
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回 2 条申告
- [ ] 状态均为「待处理(1)」
- [ ] 只能看到自己的申告

### 11.3 门户端 — 申告详情

```http
GET /api/v1/portal/tickets/{ticket_id_1}
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回完整申告信息
- [ ] `records` 数组当前为空（暂无处理记录）
- [ ] `supplement_count` = 0

### 11.4 后台管理 — 查看全部申告

使用 `operator1` 账号：

```http
GET /api/v1/admin/tickets?status=1
Authorization: Bearer <operator1_token>
```

**验证点：**
- [ ] 返回 2 条待处理的申告
- [ ] 可看到所有用户的申告（非仅自己）

### 11.5 后台 — 开始处理申告 1

```http
PATCH /api/v1/admin/tickets/{ticket_id_1}/status
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "action": "start",
  "result": "已接单，正在排查 3 楼打印机。"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 申告状态变为「处理中(2)」
- [ ] 申告详情中 `records` 包含这条操作记录

### 11.6 后台 — 开始处理申告 2 + 索要补充信息

```http
PATCH /api/v1/admin/tickets/{ticket_id_2}/status
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "action": "start",
  "result": "已接单。"
}
```

```http
PATCH /api/v1/admin/tickets/{ticket_id_2}/status
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "action": "request_info",
  "result": "请提供以下信息：1) 家庭宽带运营商和套餐带宽；2) 连接 VPN 时是否同时有 Teams 通话或大文件下载；3) VPN 客户端日志文件。"
}
```

**验证点：**
- [ ] 状态变为「需补充信息(3)」
- [ ] `supplement_count` = 1
- [ ] `reporter1` 的站内消息收到补充信息通知

### 11.7 门户端 — 补充信息

使用 `reporter1` 账号：

```http
PATCH /api/v1/portal/tickets/{ticket_id_2}/supplement
Authorization: Bearer <reporter1_token>
Content-Type: application/json

{
  "content": "补充信息如下：1) 宽带为中国电信 300M；2) VPN 断开时没有 Teams 通话或下载；3) VPN 日志见附件（略）。"
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 状态恢复为「处理中(2)」
- [ ] `supplement_count` = 1

### 11.8 后台 — 添加处理记录

```http
POST /api/v1/admin/tickets/{ticket_id_1}/records
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "action": "note",
  "content": "已到达 3 楼现场。检查发现硒鼓废粉盒已满，已更换。测试打印 10 张无卡纸。建议下周定期维护。"
}
```

### 11.9 后台 — 解决申告 1

```http
PATCH /api/v1/admin/tickets/{ticket_id_1}/status
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "action": "resolve",
  "result": "打印机关机更换废粉盒后恢复正常。已测试正常打印。"
}
```

**验证点：**
- [ ] 状态变为「已解决(4)」
- [ ] `reporter1` 站内消息收到已解决通知

### 11.10 后台 — 关闭申告 2

```http
PATCH /api/v1/admin/tickets/{ticket_id_2}/status
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "action": "close",
  "result": "用户超过 7 天未响应，且问题无法复现，关闭此申告。"
}
```

**验证点：**
- [ ] 状态变为「已关闭(5)」

### 11.11 后台 — 从申告生成知识候选

基于已解决的申告 1 创建知识文章：

```http
POST /api/v1/admin/tickets/{ticket_id_1}/knowledge-candidate
Authorization: Bearer <operator1_token>
Content-Type: application/json

{
  "kb_id": 1
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 在知识库 1 的文章列表中出现一篇新文章
- [ ] 标题为「申告经验 - 3 楼东侧打印机频繁卡纸」（自动生成）
- [ ] 状态为「草稿(1)」

### 11.12 状态机校验

| 当前状态 | 操作 | 预期结果 |
|----------|------|----------|
| 待处理(1) | start | ✓ 变为处理中(2) |
| 待处理(1) | request_info | ✗ code=10003 |
| 处理中(2) | request_info | ✓ 变为需补充(3) |
| 需补充(3) | supplement（门户） | ✓ 变为处理中(2) |
| 处理中(2) | resolve | ✓ 变为已解决(4) |
| 处理中(2) | close | ✓ 变为已关闭(5) |
| 需补充(3) | close | ✓ 变为已关闭(5) |
| 已解决(4) | close | ✗ code=10003 |
| 已关闭(5) | 任何操作 | ✗ code=10003 |
| 同一申告 request_info 第 4 次 | — | ✗ code=10003（最多 3 次） |

---

## 12. 场景 H：用户与角色管理

> **前置条件：** 使用 `admin` 账号。

### 12.1 用户列表

```http
GET /api/v1/admin/users?page=1&page_size=10
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 6 个预置用户
- [ ] 含角色信息和状态

### 12.2 创建新用户

```http
POST /api/v1/admin/users
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "username": "testuser",
  "password": "Test@12345",
  "real_name": "测试用户",
  "phone": "13800001000",
  "email": "testuser@opsmind.local",
  "role_ids": [4]
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 新用户 `status=1`（正常）
- [ ] 新用户角色为「报障人」

**异常测试：**

| 测试项 | 输入 | 预期 code |
|--------|------|-----------|
| 重名 | 再次创建 `testuser` | 10005 |
| 弱密码 | `password: "123"` | 10003 |
| 缺角色 | 不传 `role_ids` | 10003 |

### 12.3 编辑用户

```http
PUT /api/v1/admin/users/{new_user_id}
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "username": "testuser",
  "real_name": "测试用户（修改）",
  "phone": "13800001001",
  "email": "testuser_new@opsmind.local",
  "role_ids": [4, 2]
}
```

**验证点：**
- [ ] 返回 `code=0`
- [ ] 用户信息已更新，角色变为「报障人 + 运维人员」

### 12.4 冻结用户

```http
POST /api/v1/admin/users/{new_user_id}/freeze
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 `code=0`，状态变为「冻结」
- [ ] 该用户无法登录（返回 10002）

### 12.5 恢复用户

```http
POST /api/v1/admin/users/{new_user_id}/unfreeze
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 `code=0`，状态恢复为「正常」
- [ ] 该用户可正常登录

**异常测试：** 冻结已冻结的用户 / 恢复已正常的用户
- [ ] 分别返回 `code=10006` / `code=10007`

### 12.6 角色列表

```http
GET /api/v1/admin/roles
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回 4 个预置角色，含权限列表

---

## 13. 场景 I：数据看板与审计日志

> **前置条件：** 前面的场景已产生足够的业务数据（问答、申告、知识操作）。

### 13.1 数据看板

```http
GET /api/v1/admin/dashboard
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回统计卡片数据（今日申告、待处理、处理中、已解决、今日问答、知识条目数等）
- [ ] 数值与实际操作产生的数据一致

### 13.2 审计日志

```http
GET /api/v1/admin/audit-logs?page=1&page_size=20
Authorization: Bearer <admin_token>
```

**验证点：**
- [ ] 返回操作日志列表
- [ ] 包含知识文章创建/发布、申告处理、用户管理等操作记录
- [ ] 每条记录包含操作人、操作类型、目标资源、时间

### 13.3 站内消息

使用 `reporter1` 账号：

```http
GET /api/v1/portal/messages
Authorization: Bearer <reporter1_token>
```

**验证点：**
- [ ] 返回申告处理过程中的站内通知（已解决通知等）
- [ ] `is_read` 字段正确

---

## 14. 测试数据清单（人工填入）

以下数据需要在测试过程中通过 API/UI 人工创建：

### 14.1 知识库

| 序号 | 名称 | 说明 | 创建者 |
|------|------|------|--------|
| 1 | IT 运维知识库 | 场景 C 创建，含 VPN、打印机、邮箱等 | admin |
| 2 | 信息安全规范库 | 场景 C 创建，含安全制度 | admin |

### 14.2 知识文章 — 手动创建

| 序号 | KB | 标题 | 状态路径 | 创建者 |
|------|-----|------|----------|--------|
| 1 | 1 | VPN 连接超时问题处理 | 草稿→待审核→通过→发布→停用→启用 | admin |
| 2 | 1 | Outlook 邮箱无法收发邮件排查步骤 | 草稿→待审核→驳回→编辑→重提→通过→发布 | admin |
| 3 | 1 | 打印机卡纸处理指南 | 草稿（保持，用于后续测试） | admin |

### 14.3 知识文章 — 文档上传

| 序号 | KB | 文件 | 格式 | 预期处理结果 |
|------|-----|------|------|-------------|
| 1 | 1 | 网络故障排查指南.md | MD | completed |
| 2 | 1 | VPN使用常见问题.txt | TXT | completed |
| 3 | 2 | 新员工IT入职指南.md | MD | completed |
| 4 | 2 | 会议室设备操作说明.txt | TXT | completed |
| 5 | 2 | 邮件系统使用规范.docx | DOCX | completed |
| 6 | 2 | 信息安全管理制度.pdf | PDF | completed |

### 14.4 问答会话

| 序号 | KB | 问题 | 预期置信度 |
|------|-----|------|-----------|
| 1 | 1 | 如何解决 VPN 连接超时的问题？ | > 0.6 |
| 2 | 1 | 它的备用服务器地址是什么？（多轮追问） | > 0.6 |
| 3 | 1 | 如何配置思科路由器 BGP 协议？ | < 0.6 |

### 14.5 申告工单

| 序号 | 提交者 | 标题 | 状态流转 |
|------|--------|------|----------|
| 1 | reporter1 | 3 楼东侧打印机频繁卡纸 | 待处理→处理中→已解决 |
| 2 | reporter1 | VPN 连接每隔 10 分钟自动断开 | 待处理→处理中→需补充→补充→处理中→已关闭 |

---

## 15. 测试文件清单

```
test/
├── README.md                              # 本文档
└── files/
    ├── generate_docs.py                   # DOCX/PDF 生成脚本
    ├── md/
    │   ├── 网络故障排查指南.md             # 约 4.2 KB，含路由追踪、DNS 排查等
    │   └── 新员工IT入职指南.md             # 约 4.4 KB，含 IT 设备申请、账号配置
    ├── txt/
    │   ├── VPN使用常见问题.txt             # 约 4.5 KB，含 8 个 FAQ
    │   └── 会议室设备操作说明.txt          # 约 4.4 KB，含 Poly Studio 操作
    ├── docx/
    │   ├── 邮件系统使用规范.docx           # 约 38 KB
    │   └── 信息安全管理制度.docx           # 约 38 KB
    └── pdf/
        ├── 邮件系统使用规范.pdf            # 约 192 KB
        └── 信息安全管理制度.pdf            # 约 196 KB
```

所有测试文件内容均为企业 IT 运维相关知识，确保 RAG 管道能正确分块、embedding 和检索。

---

## 16. 验收通过标准

### 16.1 认证与授权（场景 A）
- [ ] 6 个预置账号均可正常登录
- [ ] Token 刷新和登出逻辑正确
- [ ] RBAC 权限控制生效（报障人不能访问后台接口）
- [ ] 密码策略校验生效

### 16.2 模型配置（场景 B）
- [ ] 预置 2 条配置可见
- [ ] 连接测试返回正确结果
- [ ] 热替换即时生效（设为默认后无需重启）
- [ ] 不允许删除默认配置

### 16.3 知识库管理（场景 C）
- [ ] 创建/编辑/列表/详情功能正常
- [ ] 门户端列表不暴露管理字段

### 16.4 知识文章（场景 D+E）
- [ ] 文章审核状态机完整流转（草稿→待审核→通过/驳回→发布→停用/启用）
- [ ] 审核人≠创建人校验生效
- [ ] 驳回后编辑重提全链路通畅
- [ ] 发布自动生成向量（chunks 非空）
- [ ] 停用清除向量（chunks 为空）
- [ ] 启用重建向量
- [ ] 文档上传 4 种格式均成功
- [ ] 文档异步处理状态正确流转
- [ ] 处理完成后 content 为非空文本

### 16.5 智能问答（场景 F）
- [ ] SSE 流式输出 7 个步骤事件齐全
- [ ] 多轮对话指代消解生效
- [ ] 高置信度问题正确引用知识库
- [ ] 低置信度问题触发申告引导
- [ ] 反馈功能正常

### 16.6 申告工单（场景 G）
- [ ] 状态机完整流转（5 种状态，5 种操作）
- [ ] 补充信息上限（3 次）生效
- [ ] 已解决/已关闭状态不可再操作
- [ ] 站内消息通知正确发送
- [ ] 知识候选生成功能正常

### 16.7 用户与角色（场景 H）
- [ ] 用户 CRUD + 冻结/恢复全链路正常
- [ ] 角色与权限正确关联

### 16.8 数据看板与审计（场景 I）
- [ ] 看板统计数据与实际操作一致
- [ ] 审计日志完整记录关键操作

### 16.9 异常处理
- [ ] AI 服务不可用时返回 `code=20001`（非 crash）
- [ ] RAG 检索失败时返回 `code=20002`
- [ ] 非法状态转换返回 `code=10003`
- [ ] 未登录访问需认证接口返回 `code=10001`

---

> **测试完成后：** 建议执行 `docker compose down -v` 清理数据卷，然后重新 `make db-init && make db-seed` 恢复到初始状态，以便下次测试。

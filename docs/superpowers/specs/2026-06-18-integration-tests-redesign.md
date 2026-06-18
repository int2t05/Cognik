# 集成测试全面重写 — 设计文档

> 日期：2026-06-18 | 目标：根据 `docs/API/` 全部 API 文档，重写 `server/tests/integration/` 测试代码，全面覆盖所有端点。

## 1. 目标

- 按 API 文档逐端点覆盖：成功路径 + 参数校验 + 业务规则 + 权限 + 边界
- 添加 `seedReporter` 和 `seedOperator` 支持跨角色权限测试
- 文件组织与 API 文档一一对应，每文件职责单一

## 2. 文件结构（~15 个测试文件，~155 用例）

| 文件 | 对应文档 | 用例数 |
|------|---------|--------|
| `api_setup_test.go` | 基础设施（重写增强） | - |
| `api_auth_test.go` | `auth.md` — 4 端点 | ~14 |
| `api_user_test.go` | `users.md` — 6 端点 | ~14 |
| `api_role_test.go` | `roles.md` — 7 端点 | ~15 |
| `api_ticket_test.go` | `tickets.md` — 8 端点 | ~22 |
| `api_chat_test.go` | `chat.md` — 6 端点 | ~14 |
| `api_knowledge_test.go` | `knowledge.md` — 16 端点 | ~28 |
| `api_llm_config_test.go` | `llm-config.md` — 6 端点 | ~14 |
| `api_dashboard_test.go` | `dashboard.md` — 2 端点 | ~6 |
| `api_audit_test.go` | `audit-log.md` — 审计 | ~8 |
| `api_message_test.go` | `audit-log.md` — 消息 | ~8 |
| `api_config_test.go` | `audit-log.md` — 系统配置 | ~6 |
| `api_health_test.go` | `audit-log.md` — 健康检查 | ~3 |
| `api_permission_test.go` | 跨模块 RBAC 权限矩阵 | ~12 |
| `seed_test.go` | 保持不变 | 6 |

## 3. 基础设施增强 (`api_setup_test.go`)

### 3.1 新增 seed 函数

```go
// 三种用户类型覆盖全权限测试
ts.seedAdmin(t)      // 系统管理员 — 全部权限
ts.seedReporter(t)   // 报障人 — 仅门户端
ts.seedOperator(t)   // 运维人员 — ticket:read/write, knowledge:read/write

// 登录辅助
ts.loginAs(t, username, password) → token
```

### 3.2 新增断言辅助

```go
assertCode(t, resp, 0)           // code=0
assertErrorCode(t, resp, 10003)  // 特定错误码
assertStatusCode(t, resp, 400)   // HTTP 状态码
assertNotFound(t, resp)          // code=10004
assertForbidden(t, resp)         // code=10002
assertUnauthorized(t, resp)      // code=10001
```

### 3.3 测试命名规范

```
TestAPI_<Module>_<Scenario> — 严格遵循
例: TestAPI_Auth_LoginSuccess
    TestAPI_User_CreateDuplicate
    TestAPI_Ticket_CloseFromPending
```

## 4. 逐模块测试用例清单

### 4.1 Auth (`api_auth_test.go`) — ~14 用例

**POST /api/v1/auth/login**
- TestAPI_Auth_LoginSuccess — 正常登录返回 token
- TestAPI_Auth_LoginWrongPassword — 密码错误
- TestAPI_Auth_LoginUserNotFound — 用户不存在（与密码错误同 code）
- TestAPI_Auth_LoginMissingFields — 缺少 password
- TestAPI_Auth_LoginFrozenAccount — 冻结账号拒绝登录
- TestAPI_Auth_LoginFirstLoginFlag — 首次登录标志自动清除

**POST /api/v1/auth/refresh**
- TestAPI_Auth_RefreshSuccess — 正常刷新
- TestAPI_Auth_RefreshWithAccessToken — 用 access_token 刷新 → 10001
- TestAPI_Auth_RefreshWithRevokedToken — 登出后刷新 → 10001
- TestAPI_Auth_RefreshMissingToken — 缺失 refresh_token

**POST /api/v1/auth/me/change-password**
- TestAPI_Auth_ChangePasswordSuccess — 正常改密 + 新密码可登录
- TestAPI_Auth_ChangePasswordWrongOld — 旧密码错误
- TestAPI_Auth_ChangePasswordWeak — 弱密码拒绝
- TestAPI_Auth_ChangePasswordUnauthenticated — 无 token

**POST /api/v1/auth/me/logout**
- TestAPI_Auth_LogoutSuccess — 正常登出+refresh 失效
- TestAPI_Auth_LogoutMissingToken — 缺少 refresh_token

### 4.2 Users (`api_user_test.go`) — ~14 用例

- TestAPI_User_List — 分页列表 + keyword 搜索
- TestAPI_User_CreateSuccess — 创建含 role_ids
- TestAPI_User_CreateDuplicate — 用户名重复 → 10005
- TestAPI_User_CreateMissingFields — 缺少必填字段 → 400
- TestAPI_User_CreateWeakPassword — 弱密码 → 10003
- TestAPI_User_Detail — 详情含 roles
- TestAPI_User_UpdateSuccess — 更新信息+角色
- TestAPI_User_UpdateNotFound — 404
- TestAPI_User_FreezeSuccess — 冻结成功
- TestAPI_User_FreezeSelf — 不能冻结自己
- TestAPI_User_FreezeAlreadyFrozen — 重复冻结 → 10006
- TestAPI_User_UnfreezeSuccess — 恢复成功
- TestAPI_User_UnfreezeAlreadyActive — 重复恢复 → 10007
- TestAPI_User_FrozenCannotLogin — 冻结后登录拒绝

### 4.3 Roles & Menus (`api_role_test.go`) — ~15 用例

- TestAPI_Role_List — 分页列表 + keyword 搜索
- TestAPI_Role_CreateSuccess — 创建含权限列表
- TestAPI_Role_CreateMissingName — 缺少 name
- TestAPI_Role_CreateDuplicate — 名称重复 → 10005
- TestAPI_Role_Detail — 详情含 permissions
- TestAPI_Role_DetailInvalidID — 无效 ID → 10003
- TestAPI_Role_DetailNotFound — 角色不存在 → 10004
- TestAPI_Role_UpdateSuccess — 更新名称+权限全量替换
- TestAPI_Role_UpdateDuplicate — 名称冲突 → 10005
- TestAPI_Role_DeleteSuccess — 删除无用户角色
- TestAPI_Role_DeleteWithUsers — 有关联用户 → 10005
- TestAPI_Role_DeleteBuiltin — 删除系统管理员角色 → 10002
- TestAPI_Role_DeleteNotFound — 404
- TestAPI_Menu_List — 菜单列表非空
- TestAPI_RoleMenu_Update — 更新角色菜单关联

### 4.4 Tickets (`api_ticket_test.go`) — ~22 用例

**门户端**
- TestAPI_Ticket_PortalCreateFull — 创建含所有可选字段
- TestAPI_Ticket_PortalCreateWithChatContext — 含 chat_context
- TestAPI_Ticket_PortalCreateMissingTitle — 缺少 title
- TestAPI_Ticket_PortalList — 我的申告列表含分页
- TestAPI_Ticket_PortalDetail — 详情含 records
- TestAPI_Ticket_PortalSupplement — 需补充信息状态下补充
- TestAPI_Ticket_PortalSupplementNotPending — 非需补充状态拒绝
- TestAPI_Ticket_PortalSupplementEmptyContent — 空内容拒绝
- TestAPI_Ticket_PortalDetailNonOwner — 非本人申告 → 需验证

**后台管理**
- TestAPI_Ticket_AdminList — 全部列表 + status 筛选
- TestAPI_Ticket_AdminListUrgencyFilter — urgency 筛选
- TestAPI_Ticket_AdminDetail — 后台详情
- TestAPI_Ticket_AdminStart — start 操作 (1→2)
- TestAPI_Ticket_AdminRequestInfo — request_info (2→3)
- TestAPI_Ticket_AdminResolve — resolve (2→4)
- TestAPI_Ticket_AdminClose — close 操作 (1/2/3→5)
- TestAPI_Ticket_AdminCloseResolved — 已解决不能再关闭
- TestAPI_Ticket_AdminInvalidAction — 非法 action
- TestAPI_Ticket_AdminInvalidTransition — 非法状态转换
- TestAPI_Ticket_AdminAddRecord — 添加处理记录
- TestAPI_Ticket_AdminAddRecordInvalidAction — 非法 record action
- TestAPI_Ticket_KnowledgeCandidate — 生成知识候选

### 4.5 Chat (`api_chat_test.go`) — ~14 用例

- TestAPI_Chat_CreateSession — 创建会话
- TestAPI_Chat_CreateSessionNoKB — 缺少 kb_id
- TestAPI_Chat_CreateSessionKBNotFound — 知识库不存在
- TestAPI_Chat_ListSessions — 会话列表
- TestAPI_Chat_SessionDetail — 详情含 messages
- TestAPI_Chat_SessionDetailNotFound — 404
- TestAPI_Chat_DeleteSession — 删除自己会话
- TestAPI_Chat_DeleteSessionNonOwner — 非属主删除 → 10002
- TestAPI_Chat_StreamSSE — SSE 流式
- TestAPI_Chat_StreamInvalidSession — 无效 session ID
- TestAPI_Chat_StreamMissingQuestion — 缺少 question
- TestAPI_Chat_Feedback — 提交反馈 1/2
- TestAPI_Chat_FeedbackInvalid — 无效反馈值
- TestAPI_Chat_DeleteThenList — 删除后列表验证

### 4.6 Knowledge (`api_knowledge_test.go`) — ~28 用例

**知识库 CRUD (6 用例)**
- TestAPI_KB_Create — 创建知识库
- TestAPI_KB_CreateMissingName — 缺少 name
- TestAPI_KB_CreateDuplicate — 同名 → 10005
- TestAPI_KB_PortalList — 门户端列表（仅 id/name/description）
- TestAPI_KB_AdminList — 后台列表含 embedding 字段
- TestAPI_KB_Update — 更新名称描述
- TestAPI_KB_UpdateNotFound — 404
- TestAPI_KB_DeleteCascade — 删除含文章的知识库

**文章 CRUD (8 用例)**
- TestAPI_Article_Create — 创建文章（草稿状态）
- TestAPI_Article_CreateWithTags — 含分类/标签
- TestAPI_Article_CreateKBNotFound — KB 不存在
- TestAPI_Article_List — 列表含分页
- TestAPI_Article_ListWithFilters — status/source_type/process_status 筛选
- TestAPI_Article_Detail — 详情含 chunks
- TestAPI_Article_DetailNotFound — 404
- TestAPI_Article_Update — 草稿状态更新
- TestAPI_Article_UpdateNonEditable — 非草稿/驳回状态拒绝编辑

**审核流程 (6 用例)**
- TestAPI_Article_SubmitReview — 提交审核 (1→2)
- TestAPI_Article_SubmitReviewNotDraft — 非草稿提交拒绝
- TestAPI_Article_ReviewApprove — 审核通过 (2→3)
- TestAPI_Article_ReviewReject — 审核驳回 (2→5)
- TestAPI_Article_ReviewRejectNoComment — 驳回无 comment 拒绝
- TestAPI_Article_ReviewSelfReview — 审核自己文章拒绝

**发布与停用 (4 用例)**
- TestAPI_Article_Publish — 发布 (3→4)
- TestAPI_Article_PublishNotApproved — 非审核通过发布拒绝
- TestAPI_Article_Disable — 停用 (4→0)
- TestAPI_Article_Enable — 启用 (0→4)

**文档上传 (4 用例)**
- TestAPI_Document_Upload — 上传文档（multipart）→ 测试环境可能跳过
- TestAPI_Document_Status — 查询处理状态
- TestAPI_Document_Retry — 重试失败文档
- TestAPI_Document_RetryNotFailed — 非 failed 状态重试拒绝

### 4.7 LLM Config (`api_llm_config_test.go`) — ~14 用例

- TestAPI_LLMConfig_Create — 创建配置含 api_key
- TestAPI_LLMConfig_CreateMissingFields — 缺少必填
- TestAPI_LLMConfig_CreateDuplicate — 名称重复 → 10005
- TestAPI_LLMConfig_List — 列表含 api_key 脱敏
- TestAPI_LLMConfig_Detail — 详情
- TestAPI_LLMConfig_DetailNotFound — 404
- TestAPI_LLMConfig_Update — 更新配置
- TestAPI_LLMConfig_UpdateIsDefault — 新默认自动切换旧默认
- TestAPI_LLMConfig_UpdateNonExistent — 404
- TestAPI_LLMConfig_Delete — 非默认配置删除成功
- TestAPI_LLMConfig_DeleteDefault — 默认配置拒绝删除
- TestAPI_LLMConfig_DeleteNotFound — 404
- TestAPI_LLMConfig_TestConnection — 测试连接 → 可能成功或 20001
- TestAPI_LLMConfig_KeyMasked — 验证 api_key 在列表和详情中脱敏

### 4.8 Dashboard (`api_dashboard_test.go`) — ~6 用例

- TestAPI_Dashboard_Stats — 统计数据含所有字段
- TestAPI_Dashboard_StatsAfterCreate — 创建申告后计数更新
- TestAPI_Dashboard_Trends — 趋势数据
- TestAPI_Dashboard_TrendsDateRange — 日期范围校验
- TestAPI_Dashboard_TrendsMissingParams — 缺少参数
- TestAPI_Dashboard_TrendsEndBeforeStart — 结束日期早于开始 → 10003

### 4.9 Audit (`api_audit_test.go`) — ~8 用例

- TestAPI_Audit_List — 分页列表
- TestAPI_Audit_ListWithActionFilter — action 精确筛选
- TestAPI_Audit_ListWithTargetType — target_type 筛选
- TestAPI_Audit_ListWithOperator — operator_id 筛选
- TestAPI_Audit_ListWithDateRange — date_from/date_to 筛选
- TestAPI_Audit_LogCreatedOnAction — 操作产生审计日志
- TestAPI_Audit_ListInvalidPage — 无效分页参数

### 4.10 Messages (`api_message_test.go`) — ~8 用例

- TestAPI_Message_List — 消息列表
- TestAPI_Message_ListWithType — 按类型筛选
- TestAPI_Message_ListWithReadStatus — 按已读/未读筛选
- TestAPI_Message_MarkRead — 标记已读+返回未读数
- TestAPI_Message_MarkReadNonExistent — 标记不存在消息
- TestAPI_Message_MarkReadOtherUser — 无法标记他人消息
- TestAPI_Message_UnreadCount — 未读计数
- TestAPI_Message_ListWrongUser — 只能看自己的消息

### 4.11 Config (`api_config_test.go`) — ~6 用例

- TestAPI_Config_Get — 获取配置
- TestAPI_Config_GetNotFound — 不存在的 key → 404
- TestAPI_Config_Update — 更新配置
- TestAPI_Config_UpdateReadBack — 更新后读回验证
- TestAPI_Config_UpdateEmptyKey — 空 key 拒绝

### 4.12 Health (`api_health_test.go`) — ~3 用例

- TestAPI_Health_Liveness — 存活探针
- TestAPI_Health_Readiness — 就绪探针

### 4.13 Permissions (`api_permission_test.go`) — ~12 用例

用 reporter/operator token 验证 RBAC 拒绝：

- TestAPI_Perm_Reporter_AdminUsers — 报障人访问 /admin/users → 10002
- TestAPI_Perm_Reporter_AdminRoles — 报障人访问 /admin/roles → 10002
- TestAPI_Perm_Reporter_AdminTickets — 报障人访问 /admin/tickets → 10002
- TestAPI_Perm_Reporter_AdminKB — 报障人访问 /admin/knowledge-bases → 10002
- TestAPI_Perm_Reporter_AdminLLM — 报障人访问 /admin/llm-configs → 10002
- TestAPI_Perm_Reporter_AdminDashboard — 报障人访问 /admin/dashboard → 10002
- TestAPI_Perm_Reporter_AdminAudit — 报障人访问 /admin/audit-logs → 10002
- TestAPI_Perm_Reporter_AdminConfig — 报障人访问 /admin/configs → 10002
- TestAPI_Perm_Operator_TicketRead — 运维人员可读申告
- TestAPI_Perm_Operator_UserManage — 运维人员无 user:manage
- TestAPI_Perm_Operator_SystemConfig — 运维人员无 system:config
- TestAPI_Perm_NoAuth — 无 Token 访问受保护路由 → 10001

## 5. 实施策略

1. **先重写 `api_setup_test.go`**：添加 Reporter/Operator seed，增强辅助函数
2. **按依赖顺序重写**：Auth → Users → Roles → Tickets → Chat → Knowledge → LLM → Dashboard → Audit → Messages → Config → Health → Permissions
3. **每写完一个文件立即运行验证**
4. **最后运行全量测试确认无回归**

## 6. 运行命令

```bash
# 单个文件
go test -tags=integration ./tests/integration/ -v -run "TestAPI_Auth"

# 全部集成测试（串行避免数据库冲突）
go test -tags=integration ./tests/integration/ -v -p 1

# 快速运行（跳过需要外部服务的测试）
go test -tags=integration ./tests/integration/ -v -p 1 -short
```

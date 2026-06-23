-- OpsMind 完整演示数据集
--
-- 包含：角色、用户、菜单、LLM 配置（本地 llama.cpp）、知识库、知识文章、
--       申告工单、处理记录、站内消息。
-- 适用于功能演示和完整开发环境。
-- 可重复执行：先 DELETE 再 INSERT。
--
-- 手动加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/seed_demo.sql

BEGIN;

-- 清理已有数据（按外键依赖逆序）
DELETE FROM messages;
DELETE FROM audit_logs;
DELETE FROM chat_messages;
DELETE FROM chat_sessions;
DELETE FROM ticket_records;
DELETE FROM tickets;
DELETE FROM knowledge_chunks;
DELETE FROM knowledge_articles;
DELETE FROM knowledge_bases;
DELETE FROM llm_configs;
DELETE FROM role_menus;
DELETE FROM user_roles;
DELETE FROM menus;
DELETE FROM users;
DELETE FROM roles;
DELETE FROM system_configs;

-- =============================================================================
-- 角色与权限
-- =============================================================================

INSERT INTO roles (id, name, description, permissions, created_at, updated_at) VALUES
(1, '系统管理员', '系统全局管理', '["user:manage","ticket:read","ticket:write","ticket:manage","knowledge:read","knowledge:write","knowledge:create","knowledge:manage","knowledge:review","dashboard:read","audit:read","system:config"]', NOW(), NOW()),
(2, '运维人员',     '处理申告和回访', '["ticket:read","ticket:write","knowledge:read","knowledge:write"]', NOW(), NOW()),
(3, '知识库管理员', '维护和审核知识', '["knowledge:read","knowledge:write","knowledge:create","knowledge:manage","knowledge:review"]', NOW(), NOW()),
(4, '报障人',       '门户端用户',     '[]', NOW(), NOW());

SELECT setval('roles_id_seq', (SELECT MAX(id) FROM roles));

-- =============================================================================
-- 菜单
-- =============================================================================

INSERT INTO menus (id, name, path, icon, parent_id, sort_order, type) VALUES
(1, '仪表盘',     '/admin/dashboard',     'dashboard',  0, 1, 'menu'),
(2, '申告管理',   '/admin/tickets',       'ticket',     0, 2, 'menu'),
(3, '知识库',     '/admin/knowledge',     'book',       0, 3, 'menu'),
(4, '用户管理',   '/admin/users',         'user',       0, 4, 'menu'),
(5, '角色管理',   '/admin/roles',         'shield',     0, 5, 'menu'),
(6, '审计日志',   '/admin/audit-logs',    'file-text',  0, 6, 'menu'),
(7, '模型配置',   '/admin/model-config',  'cpu',        0, 7, 'menu'),
(8, 'LLM 配置',   '/admin/llm-config',    'cpu',        0, 8, 'menu'),
(9, '系统配置',   '/admin/system-config', 'settings',   0, 9, 'menu');

SELECT setval('menus_id_seq', (SELECT MAX(id) FROM menus));

-- 角色-菜单关联（所有角色拥有全部菜单）
INSERT INTO role_menus (role_id, menu_id)
SELECT r.id, m.id FROM roles r, menus m;

-- =============================================================================
-- 用户
-- =============================================================================

INSERT INTO users (id, username, password_hash, real_name, phone, email, status, first_login, created_at, updated_at) VALUES
(1, 'admin',     '$2a$10$G5FBz7I3ne4Avj7j.kyhz.uo9TCY7/OADw3RLL/15AKl97kl7AS2.', '系统管理员', '13800000001', 'admin@opsmind.local',      1, true,  NOW(), NOW()),
(2, 'operator1', '$2a$10$BuBFnBkWINTypuEztzlYi.AazINGfwz9HQuzcV/yXsZAgw5B5OW.C', '张运维',     '13800000002', 'zhangyunwei@opsmind.local', 1, true,  NOW(), NOW()),
(3, 'operator2', '$2a$10$BuBFnBkWINTypuEztzlYi.AazINGfwz9HQuzcV/yXsZAgw5B5OW.C', '李运维',     '13800000003', 'liyunwei@opsmind.local',    1, true,  NOW(), NOW()),
(4, 'knowledge', '$2a$10$IUGaQylkRdufn3de7SlpkOZZNR6nCYzA.AWkKuU/amj3FWky3C6xm', '王知识',     '13800000004', 'wangzhishi@opsmind.local',  1, true,  NOW(), NOW()),
(5, 'reporter1', '$2a$10$/qkn/UAKYhUmRtmefmfG1uy2UJLVMizGozRvicRJNbJzv3yiWUKby', '赵用户',     '13800000005', 'zhaoyonghu@opsmind.local',  1, true,  NOW(), NOW()),
(6, 'reporter2', '$2a$10$/qkn/UAKYhUmRtmefmfG1uy2UJLVMizGozRvicRJNbJzv3yiWUKby', '钱用户',     '13800000006', 'qianyonghu@opsmind.local',  1, false, NOW(), NOW());

SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));

-- 用户-角色关联
INSERT INTO user_roles (user_id, role_id) VALUES
(1, 1), (2, 2), (3, 2), (4, 3), (5, 4), (6, 4);

-- =============================================================================
-- 系统配置
-- =============================================================================

INSERT INTO system_configs (key, value, description, updated_by, updated_at) VALUES
('app_name', '"OpsMind"', '应用名称，显示在页面标题和系统通知中', 1, NOW());

-- =============================================================================
-- LLM 配置
-- =============================================================================

INSERT INTO llm_configs (id, name, provider_type, base_url, embedding_base_url, api_key, llm_model, embedding_model, system_prompt, max_tokens, vector_dimension, is_default, created_at, updated_at) VALUES
(1, '本地 llama.cpp',     1, 'http://llama-cpp:8080/v1',  'http://llama-cpp-emb:8080/v1',  '',                        'Qwen3-4B-Q4_K_M',             'Qwen3-Embedding-0.6B-Q8_0',                   NULL, 8192,  1024, true,  NOW(), NOW()),
(2, 'OpenAI GPT-4o-mini', 2, 'https://api.openai.com/v1', '',                              'sk-your-openai-api-key',  'gpt-4o-mini',           'text-embedding-3-small',   NULL, 16384, 1536, false, NOW(), NOW());

SELECT setval('llm_configs_id_seq', (SELECT MAX(id) FROM llm_configs));

-- =============================================================================
-- 知识库
-- =============================================================================

INSERT INTO knowledge_bases (id, name, description, rag_workspace_slug, llm_config_id, embedding_model, vector_dimension, created_by, created_at, updated_at) VALUES
(1, 'IT 运维 FAQ', '常见的 IT 运维问题和解决方案', 'opsmind-it-ops', 1, 'Qwen3-Embedding-0.6B-Q8_0', 1024, 1, NOW(), NOW());

SELECT setval('knowledge_bases_id_seq', (SELECT MAX(id) FROM knowledge_bases));

-- =============================================================================
-- 知识文章
-- =============================================================================

INSERT INTO knowledge_articles (id, kb_id, title, content, source_type, category, tags, status, word_count, chunk_count, created_by, created_at, updated_at) VALUES
(1, 1, '如何重置 VPN 密码？',
 '请登录 VPN 自助服务平台 https://vpn.company.com，点击「忘记密码」按提示操作。如无法自助重置，请联系 IT 服务台（分机 8888）。',
 1, '网络与VPN', '["VPN","密码","自助"]', 4, 68, 2, 1, NOW(), NOW()),
(2, 1, '电脑无法连接公司 WiFi 怎么办？',
 '请按以下步骤排查：1. 确认 WiFi 开关已打开；2. 忘记该网络后重新连接；3. 重启电脑；4. 如仍无法连接，请提交申告并提供工位信息。',
 1, '网络与WiFi', '["WiFi","连接","网络"]', 4, 78, 2, 1, NOW(), NOW()),
(3, 1, 'Outlook 邮箱无法收发邮件？',
 '请检查：1. 网络连接是否正常；2. Outlook 客户端是否显示「已连接」；3. 尝试网页版邮箱 https://mail.company.com；4. 如网页版正常但客户端异常，请重新配置邮箱账户。',
 1, '邮箱与办公', '["Outlook","邮箱","邮件"]', 4, 95, 2, 1, NOW(), NOW()),
(4, 1, '打印机显示脱机如何处理？',
 '请依次尝试：1. 检查打印机电源和网线；2. 在电脑设备和打印机中右键打印机→查看打印内容→取消所有文档→取消脱机使用打印机；3. 重启打印机。',
 1, '办公设备', '["打印机","脱机","办公"]', 2, 73, 0, 4, NOW(), NOW()),
(5, 1, '新员工入职 IT 设备申请流程？',
 '新员工入职需提前 3 个工作日在 OA 系统提交 IT 设备申请单。标配：ThinkPad T14 + 24寸显示器 + 键鼠套装。',
 1, '入职与账号', '["入职","设备","新员工"]', 1, 56, 0, 3, NOW(), NOW());

SELECT setval('knowledge_articles_id_seq', (SELECT MAX(id) FROM knowledge_articles));

-- =============================================================================
-- 知识切片（不含向量，由 embedding 服务生成）
-- =============================================================================

INSERT INTO knowledge_chunks (article_id, kb_id, content, chunk_index, embedding_model, vector_dimension, created_at) VALUES
(1, 1, '如何重置 VPN 密码？请登录 VPN 自助服务平台。', 0, 'Qwen3-Embedding-0.6B-Q8_0', 1024, NOW()),
(1, 1, '如无法自助重置，请联系 IT 服务台（分机 8888）。', 1, 'Qwen3-Embedding-0.6B-Q8_0', 1024, NOW()),
(2, 1, '电脑无法连接公司 WiFi 怎么办？请按以下步骤排查。', 0, 'Qwen3-Embedding-0.6B-Q8_0', 1024, NOW()),
(2, 1, '确认 WiFi 开关已打开，忘记该网络后重新连接，重启电脑。', 1, 'Qwen3-Embedding-0.6B-Q8_0', 1024, NOW()),
(3, 1, 'Outlook 邮箱无法收发邮件？请检查网络连接和客户端状态。', 0, 'Qwen3-Embedding-0.6B-Q8_0', 1024, NOW());

-- =============================================================================
-- 申告工单
-- =============================================================================

INSERT INTO tickets (id, ticket_no, user_id, title, description, urgency, impact_scope, contact_phone, contact_email, status, supplement_count, source, created_at, updated_at) VALUES
(1, 'TK-DEMO-0001', 5, '3 楼打印机故障',
 '3 楼东侧公共打印机（型号 HP LaserJet M404）频繁卡纸，今天已发生 5 次，影响部门日常工作。',
 2, 2, '13800000005', 'zhaoyonghu@opsmind.local', 1, 0, 1, NOW() - INTERVAL '6 days', NOW()),
(2, 'TK-DEMO-0002', 5, 'VPN 连接频繁断开',
 '远程办公时 VPN 每隔 10-20 分钟自动断开，需重新连接。已尝试重启路由器和电脑，问题依旧。',
 3, 1, '13800000005', NULL, 2, 0, 1, NOW() - INTERVAL '5 days', NOW()),
(3, 'TK-DEMO-0003', 6, '新笔记本无法安装开发工具',
 '申请的新 ThinkPad T14 到手后发现无法安装 Visual Studio 2022，安装程序报错缺少 .NET Framework 4.8。',
 1, 1, '13800000006', 'qianyonghu@opsmind.local', 3, 1, 1, NOW() - INTERVAL '4 days', NOW()),
(4, 'TK-DEMO-0004', 5, '邮箱签名无法修改',
 'Outlook 中无法修改个人邮箱签名，点击保存后无反应。',
 1, 1, '13800000005', NULL, 4, 0, 1, NOW() - INTERVAL '2 days', NOW());

SELECT setval('tickets_id_seq', (SELECT MAX(id) FROM tickets));

-- 申告处理记录
INSERT INTO ticket_records (ticket_id, operator_id, action, content, created_at) VALUES
(2, 2, 'start',        '已接单，正在排查 VPN 服务器日志。',                                             NOW() - INTERVAL '5 days 1 hour'),
(3, 2, 'start',        '已接单。',                                                                      NOW() - INTERVAL '4 days 1 hour'),
(3, 2, 'request_info', '请提供操作系统版本和已安装的 .NET Framework 版本信息。',                         NOW() - INTERVAL '4 days'),
(4, 2, 'start',        '已接单，排查中。',                                                              NOW() - INTERVAL '2 days 1 hour'),
(4, 2, 'resolve',      '问题原因为 Outlook 客户端插件冲突，已禁用冲突插件，签名功能恢复正常。',          NOW() - INTERVAL '2 days');

-- =============================================================================
-- 站内消息
-- =============================================================================

INSERT INTO messages (id, user_id, type, related_type, related_id, title, content, is_read, created_at) VALUES
(1, 5, 'ticket_status',     'ticket', 2, '申告处理中',
 '您的申告「VPN 连接频繁断开」已被运维人员接单处理。', false, NOW() - INTERVAL '5 days 1 hour'),
(2, 6, 'ticket_supplement', 'ticket', 3, '请补充申告信息',
 '运维人员需要您补充以下信息：操作系统版本和已安装的 .NET Framework 版本。', false, NOW() - INTERVAL '4 days'),
(3, 5, 'ticket_resolved',   'ticket', 4, '申告已解决',
 '您的申告「邮箱签名无法修改」已解决。如有问题请反馈。', true, NOW() - INTERVAL '2 days');

SELECT setval('messages_id_seq', (SELECT MAX(id) FROM messages));

COMMIT;

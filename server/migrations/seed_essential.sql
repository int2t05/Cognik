-- OpsMind 必要种子数据集（角色 + 用户 + 菜单 + LLM 配置 + 系统配置）
--
-- 仅加载系统运行所需的最少静态数据。动态数据（知识库、知识文章、
-- 申告工单、处理记录、站内消息）在运行时通过 API/UI 人工创建。
-- 可重复执行：先 DELETE 再 INSERT。
--
-- 手动加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/seed_essential.sql

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
(7, '模型配置',   '/admin/config/llm',    'cpu',        0, 7, 'menu'),
(8, '系统配置',   '/admin/config/system', 'settings',   0, 8, 'menu');

SELECT setval('menus_id_seq', (SELECT MAX(id) FROM menus));

-- 角色-菜单关联（所有角色拥有全部菜单）
INSERT INTO role_menus (role_id, menu_id)
SELECT r.id, m.id FROM roles r, menus m;

-- =============================================================================
-- 用户（密码 bcrypt cost=10）
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
('app_name',                '"OpsMind"', '应用名称，显示在页面标题和系统通知中', 1, NOW()),
('ai.rag_enabled',          'true',      '全局 RAG 检索开关',                  1, NOW()),
('ai.top_k',                '5',         'RAG 默认检索 Top K',                 1, NOW()),
('ai.threshold',            '0.6',       'AI 置信度阈值',                      1, NOW()),
('ai.max_history_messages', '10',        '多轮对话历史消息数上限',               1, NOW()),
('ai.rag_query_rewrite',    'true',      'RAG 查询改写开关',                    1, NOW()),
('ai.rag_multi_route',      'true',      'RAG 多路检索开关',                    1, NOW()),
('ai.rag_hybrid',           'true',      'RAG BM25 混合检索开关',               1, NOW()),
('ai.rag_rerank',           'true',      'RAG 重排序开关',                      1, NOW());

-- =============================================================================
-- LLM 配置
-- =============================================================================

INSERT INTO llm_configs (id, name, provider_type, base_url, embedding_base_url, api_key, llm_model, embedding_model, system_prompt, max_tokens, vector_dimension, is_default, created_at, updated_at) VALUES
(1, '本地 llama.cpp',     1, 'http://llama-cpp:8081/v1',  'http://llama-cpp-emb:8082/v1', '',             'Qwen3-4B-Q4_K_M',            'Qwen3-Embedding-0.6B-Q8_0', NULL, 8192,  1024, true,  NOW(), NOW()),
(2, 'OpenAI GPT-4o-mini', 2, 'https://api.openai.com/v1', '',                              'sk-your-openai-api-key',  'gpt-4o-mini',           'text-embedding-3-small',      NULL, 16384, 1536, false, NOW(), NOW());

SELECT setval('llm_configs_id_seq', (SELECT MAX(id) FROM llm_configs));

COMMIT;

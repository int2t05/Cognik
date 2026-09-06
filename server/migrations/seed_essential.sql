-- Cognik 必要种子数据集（角色 + 用户 + 菜单 + LLM 配置 + 系统配置）
--
-- 仅加载系统运行所需的最少静态数据。动态数据（知识库、知识文章、
-- 工单工单、处理记录、站内消息）在运行时通过 API/UI 人工创建。
-- 可重复执行：先 DELETE 再 INSERT。
--
-- 手动加载方式：
--   docker compose exec -T postgres psql -U cognik -d cognik < server/migrations/seed_essential.sql

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
DELETE FROM role_menus;
DELETE FROM user_roles;
DELETE FROM menus;
DELETE FROM users;
DELETE FROM roles;

-- =============================================================================
-- 角色与权限
-- =============================================================================

INSERT INTO roles (id, name, description, permissions, created_at, updated_at) VALUES
(1, '系统管理员', '系统全局管理', '["user:manage","ticket:read","ticket:write","ticket:manage","knowledge:read","knowledge:write","knowledge:create","knowledge:manage","knowledge:review","dashboard:read","audit:read","system:config"]', NOW(), NOW()),
(2, '团队成员',     '处理工单和回访', '["ticket:read","ticket:write","knowledge:read","knowledge:write"]', NOW(), NOW()),
(3, '知识库管理员', '维护和审核知识', '["knowledge:read","knowledge:write","knowledge:create","knowledge:manage","knowledge:review"]', NOW(), NOW()),
(4, '普通成员',       '门户端用户',     '[]', NOW(), NOW());

SELECT setval('roles_id_seq', (SELECT MAX(id) FROM roles));

-- =============================================================================
-- 菜单
-- =============================================================================

INSERT INTO menus (id, name, path, icon, parent_id, sort_order, type) VALUES
(1, '仪表盘',     '/admin/dashboard',     'dashboard',  0, 1, 'menu'),
(2, '工单管理',   '/admin/tickets',       'ticket',     0, 2, 'menu'),
(3, '知识库',     '/admin/knowledge',     'book',       0, 3, 'menu'),
(4, '用户管理',   '/admin/users',         'user',       0, 4, 'menu'),
(5, '角色管理',   '/admin/roles',         'shield',     0, 5, 'menu'),
(6, '审计日志',   '/admin/audit-logs',    'file-text',  0, 6, 'menu'),
(7, '系统配置',   '/admin/config/system', 'settings',   0, 7, 'menu');

SELECT setval('menus_id_seq', (SELECT MAX(id) FROM menus));

-- 角色-菜单关联（所有角色拥有全部菜单）
INSERT INTO role_menus (role_id, menu_id)
SELECT r.id, m.id FROM roles r, menus m;

-- =============================================================================
-- 用户（密码 bcrypt cost=10）
-- =============================================================================

INSERT INTO users (id, username, password_hash, real_name, phone, email, status, first_login, created_at, updated_at) VALUES
(1, 'admin',     '$2a$10$G5FBz7I3ne4Avj7j.kyhz.uo9TCY7/OADw3RLL/15AKl97kl7AS2.', '系统管理员', '13800000001', 'admin@cognik.local',      1, true,  NOW(), NOW()),
(2, 'operator1', '$2a$10$BuBFnBkWINTypuEztzlYi.AazINGfwz9HQuzcV/yXsZAgw5B5OW.C', '张工',     '13800000002', 'zhangyunwei@cognik.local', 1, true,  NOW(), NOW()),
(3, 'operator2', '$2a$10$BuBFnBkWINTypuEztzlYi.AazINGfwz9HQuzcV/yXsZAgw5B5OW.C', '李工',     '13800000003', 'liyunwei@cognik.local',    1, true,  NOW(), NOW()),
(4, 'knowledge', '$2a$10$IUGaQylkRdufn3de7SlpkOZZNR6nCYzA.AWkKuU/amj3FWky3C6xm', '王知识',     '13800000004', 'wangzhishi@cognik.local',  1, true,  NOW(), NOW()),
(5, 'reporter1', '$2a$10$/qkn/UAKYhUmRtmefmfG1uy2UJLVMizGozRvicRJNbJzv3yiWUKby', '赵用户',     '13800000005', 'zhaoyonghu@cognik.local',  1, true,  NOW(), NOW()),
(6, 'reporter2', '$2a$10$/qkn/UAKYhUmRtmefmfG1uy2UJLVMizGozRvicRJNbJzv3yiWUKby', '钱用户',     '13800000006', 'qianyonghu@cognik.local',  1, false, NOW(), NOW());

SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));

-- 用户-角色关联
INSERT INTO user_roles (user_id, role_id) VALUES
(1, 1), (2, 2), (3, 2), (4, 3), (5, 4), (6, 4);

-- =============================================================================
-- 配置
-- =============================================================================
-- LLM/Embedding/RAG/Search/Upload 配置均从 .env 读取,不入 DB。

COMMIT;

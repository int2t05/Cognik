-- OpsMind 数据库初始化脚本（DDL + 演示数据）
--
-- 此脚本与 GORM AutoMigrate 协同工作：
--   - 基础表结构由 GORM AutoMigrate 在服务启动时自动创建
--   - 本脚本处理 GORM 无法覆盖的部分（pgvector 扩展、HNSW 索引、列注释、
--     旧字段迁移），并加载演示数据
--
-- 加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/001_init.sql
-- 或：
--   make seed
--
-- ⚠️ 演示数据部分会先清除已有数据再插入，仅用于开发/演示环境。

-- =============================================================================
-- DDL 迁移（幂等：所有语句使用 IF NOT EXISTS / DROP IF EXISTS）
-- =============================================================================

-- pgvector 扩展（GORM AutoMigrate 也会创建，此语句幂等）
CREATE EXTENSION IF NOT EXISTS vector;

-- knowledge_bases：移除旧字段，添加 LLM 配置关联（升级旧 schema 时使用）
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS rag_workspace_slug;
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS rag_workspace_slug varchar(128);
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS llm_config_id bigint NOT NULL DEFAULT 0;

-- knowledge_articles：升级旧 schema → 统一文章模型
ALTER TABLE knowledge_articles DROP COLUMN IF EXISTS question;
ALTER TABLE knowledge_articles DROP COLUMN IF EXISTS rag_document_location;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'knowledge_articles' AND column_name = 'answer'
    ) THEN
        ALTER TABLE knowledge_articles RENAME COLUMN answer TO content;
    END IF;
END $$;

ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS title         varchar(255) NOT NULL DEFAULT '';
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS source_type  smallint NOT NULL DEFAULT 1;
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS word_count   bigint NOT NULL DEFAULT 0;
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS chunk_count  bigint NOT NULL DEFAULT 0;
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS file_type    varchar(16);
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS minio_path   varchar(512);
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS process_status varchar(16) NOT NULL DEFAULT 'pending';
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS process_error text;

UPDATE knowledge_articles SET title = LEFT(COALESCE(content, '未命名文章'), 50) WHERE title = '';

COMMENT ON COLUMN knowledge_articles.source_type IS '来源类型：1=手动输入, 2=文档上传';
COMMENT ON COLUMN knowledge_articles.process_status IS '文档处理状态：pending/parsing/chunking/embedding/completed/failed';

-- knowledge_chunks：pgvector 向量存储
ALTER TABLE knowledge_chunks DROP COLUMN IF EXISTS sync_status;
ALTER TABLE knowledge_chunks DROP COLUMN IF EXISTS sync_error;
ALTER TABLE knowledge_chunks DROP COLUMN IF EXISTS synced_at;

ALTER TABLE knowledge_chunks ADD COLUMN IF NOT EXISTS kb_id        bigint NOT NULL DEFAULT 0;
ALTER TABLE knowledge_chunks ADD COLUMN IF NOT EXISTS chunk_index  bigint NOT NULL DEFAULT 0;

-- embedding 列：halfvec(1024) — 由 VectorStore 适配器通过 SQL 管理，不走 GORM
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'knowledge_chunks' AND column_name = 'embedding'
    ) THEN
        ALTER TABLE knowledge_chunks ADD COLUMN embedding halfvec(1024);
    END IF;
END $$;

COMMENT ON COLUMN knowledge_chunks.embedding IS 'halfvec 半精度向量（1024 维），pgvector 余弦相似度检索';

-- llm_configs：LLM/Embedding 提供商配置
-- 注意：GORM AutoMigrate 使用 bigint 而非 integer，本 CREATE 仅在表不存在时生效
CREATE TABLE IF NOT EXISTS llm_configs (
    id                bigint PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name              varchar(128) NOT NULL,
    provider_type     smallint NOT NULL DEFAULT 1,
    base_url          varchar(512) NOT NULL,
    embedding_base_url varchar(512),
    api_key           varchar(512),
    llm_model         varchar(128) NOT NULL,
    embedding_model   varchar(128) NOT NULL,
    system_prompt     text,
    max_tokens        bigint NOT NULL DEFAULT 8192,
    vector_dimension  bigint NOT NULL DEFAULT 1024,
    is_default        boolean NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE llm_configs IS 'LLM/Embedding 提供商配置';
COMMENT ON COLUMN llm_configs.provider_type IS '1=llama.cpp, 2=OpenAI-compatible';
COMMENT ON COLUMN llm_configs.api_key IS 'AES-256 加密存储';
COMMENT ON COLUMN llm_configs.vector_dimension IS 'bge-m3=1024, text-embedding-3-small=1536';
COMMENT ON COLUMN llm_configs.is_default IS '默认配置，最多一条为 true';

CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_default ON llm_configs (is_default) WHERE is_default = true;

-- 索引（HNSW 每次重建以保证参数一致）
DROP INDEX IF EXISTS idx_chunks_embedding;
CREATE INDEX idx_chunks_embedding ON knowledge_chunks
    USING hnsw (embedding halfvec_cosine_ops)
    WITH (m = 16, ef_construction = 200);

CREATE INDEX IF NOT EXISTS idx_chunks_kb_id            ON knowledge_chunks (kb_id);
CREATE INDEX IF NOT EXISTS idx_chunks_article_id       ON knowledge_chunks (article_id);
CREATE INDEX IF NOT EXISTS idx_articles_status         ON knowledge_articles (status);
CREATE INDEX IF NOT EXISTS idx_articles_process_status  ON knowledge_articles (process_status);

-- chat_messages：RAG 管道耗时（JSONB）
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS pipeline_metrics jsonb;
COMMENT ON COLUMN chat_messages.pipeline_metrics IS 'RAG 管道各步骤耗时（ms）';

-- =============================================================================
-- 演示数据
-- =============================================================================

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

-- 角色
INSERT INTO roles (id, name, description, permissions, created_at, updated_at) VALUES
(1, '系统管理员', '系统全局管理', '["user:manage","ticket:read","ticket:write","ticket:manage","knowledge:read","knowledge:write","knowledge:create","knowledge:manage","knowledge:review","dashboard:read","audit:read","system:config"]', NOW(), NOW()),
(2, '运维人员',     '处理申告和回访', '["ticket:read","ticket:write","knowledge:read","knowledge:write"]', NOW(), NOW()),
(3, '知识库管理员', '维护和审核知识', '["knowledge:read","knowledge:write","knowledge:create","knowledge:manage","knowledge:review"]', NOW(), NOW()),
(4, '报障人',       '门户端用户',     '[]', NOW(), NOW());

SELECT setval('roles_id_seq', (SELECT MAX(id) FROM roles));

-- 菜单
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

-- 用户（密码 bcrypt cost=10）
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

-- 系统配置
INSERT INTO system_configs (key, value, description, updated_by, updated_at) VALUES
('app_name', '"OpsMind"', '应用名称，显示在页面标题和系统通知中', 1, NOW());

-- LLM 配置
INSERT INTO llm_configs (id, name, provider_type, base_url, api_key, llm_model, embedding_model, system_prompt, max_tokens, vector_dimension, is_default, created_at, updated_at) VALUES
(1, '本地 llama.cpp',     1, 'http://llama-cpp:8080/v1',  '',                        'qwen3-4b',             'bge-m3',                   NULL, 8192,  1024, true,  NOW(), NOW()),
(2, 'OpenAI GPT-4o-mini', 2, 'https://api.openai.com/v1', 'sk-your-openai-api-key',  'gpt-4o-mini',           'text-embedding-3-small',   NULL, 16384, 1536, false, NOW(), NOW());

SELECT setval('llm_configs_id_seq', (SELECT MAX(id) FROM llm_configs));

-- 知识库
INSERT INTO knowledge_bases (id, name, description, rag_workspace_slug, llm_config_id, embedding_model, vector_dimension, created_by, created_at, updated_at) VALUES
(1, 'IT 运维 FAQ', '常见的 IT 运维问题和解决方案', 'opsmind-it-ops', 1, 'bge-m3', 1024, 1, NOW(), NOW());

SELECT setval('knowledge_bases_id_seq', (SELECT MAX(id) FROM knowledge_bases));

-- 知识文章
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

-- 知识切片
INSERT INTO knowledge_chunks (article_id, kb_id, content, chunk_index, embedding_model, vector_dimension, created_at) VALUES
(1, 1, '如何重置 VPN 密码？请登录 VPN 自助服务平台。', 0, 'bge-m3', 1024, NOW()),
(1, 1, '如无法自助重置，请联系 IT 服务台（分机 8888）。', 1, 'bge-m3', 1024, NOW()),
(2, 1, '电脑无法连接公司 WiFi 怎么办？请按以下步骤排查。', 0, 'bge-m3', 1024, NOW()),
(2, 1, '确认 WiFi 开关已打开，忘记该网络后重新连接，重启电脑。', 1, 'bge-m3', 1024, NOW()),
(3, 1, 'Outlook 邮箱无法收发邮件？请检查网络连接和客户端状态。', 0, 'bge-m3', 1024, NOW());

-- 申告工单（使用相对日期，24h-6d，均在 7 天自动关闭窗口内）
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

-- 申告处理记录（日期与对应工单对齐）
INSERT INTO ticket_records (ticket_id, operator_id, action, content, created_at) VALUES
(2, 2, 'start',        '已接单，正在排查 VPN 服务器日志。',                                             NOW() - INTERVAL '5 days 1 hour'),
(3, 2, 'start',        '已接单。',                                                                      NOW() - INTERVAL '4 days 1 hour'),
(3, 2, 'request_info', '请提供操作系统版本和已安装的 .NET Framework 版本信息。',                         NOW() - INTERVAL '4 days'),
(4, 2, 'start',        '已接单，排查中。',                                                              NOW() - INTERVAL '2 days 1 hour'),
(4, 2, 'resolve',      '问题原因为 Outlook 客户端插件冲突，已禁用冲突插件，签名功能恢复正常。',          NOW() - INTERVAL '2 days');

-- 站内消息（日期与对应工单对齐）
INSERT INTO messages (id, user_id, type, related_type, related_id, title, content, is_read, created_at) VALUES
(1, 5, 'ticket_status',     'ticket', 2, '申告处理中',
 '您的申告「VPN 连接频繁断开」已被运维人员接单处理。', false, NOW() - INTERVAL '5 days 1 hour'),
(2, 6, 'ticket_supplement', 'ticket', 3, '请补充申告信息',
 '运维人员需要您补充以下信息：操作系统版本和已安装的 .NET Framework 版本。', false, NOW() - INTERVAL '4 days'),
(3, 5, 'ticket_resolved',   'ticket', 4, '申告已解决',
 '您的申告「邮箱签名无法修改」已解决。如有问题请反馈。', true, NOW() - INTERVAL '2 days');

SELECT setval('messages_id_seq', (SELECT MAX(id) FROM messages));

COMMIT;

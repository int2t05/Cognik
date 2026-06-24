-- OpsMind 数据库清空脚本
--
-- 删除所有表中的数据，保留表结构（DDL）。用于开发/测试环境重置。
--
-- 与 seed_essential.sql 互为逆操作：
--   1. 清空数据：psql -U opsmind -d opsmind < server/migrations/clear_all.sql
--   2. 重新播种：psql -U opsmind -d opsmind < server/migrations/seed_essential.sql
--
-- 手动加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/clear_all.sql

BEGIN;

-- 按外键依赖逆序删除，避免外键约束冲突
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

-- 重置所有自增序列（使下次 INSERT 从 1 开始）
-- 动态收集并重置：遍历所有序列，统一重置到 1
DO $$
DECLARE
    seq_name text;
BEGIN
    FOR seq_name IN
        SELECT c.relname
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relkind = 'S'
          AND n.nspname = 'public'
          AND c.relname NOT LIKE '%_pkey%'
        ORDER BY c.relname
    LOOP
        EXECUTE format('ALTER SEQUENCE %I RESTART WITH 1', seq_name);
    END LOOP;
END $$;

COMMIT;

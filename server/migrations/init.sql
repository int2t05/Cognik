-- OpsMind 数据库 DDL 增强脚本
--
-- 此脚本与 GORM AutoMigrate 协同工作：
--   - 基础表结构由 GORM AutoMigrate 在服务启动时自动创建
--   - 本脚本处理 GORM 无法覆盖的部分（pgvector 扩展、HNSW 索引、列注释、旧字段迁移）
--
-- 所有语句幂等（IF NOT EXISTS / DROP IF EXISTS），可重复执行。
--
-- 手动加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/init.sql

-- =============================================================================
-- DDL 迁移
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
ALTER TABLE knowledge_chunks ADD COLUMN IF NOT EXISTS chunk_hash   varchar(64);

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
-- 注意：GORM AutoMigrate 使用 bigint，本 CREATE 仅在表不存在时生效
CREATE TABLE IF NOT EXISTS llm_configs (
    id                 bigint PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name               varchar(128) NOT NULL,
    llm_base_url       varchar(512) DEFAULT '',
    llm_api_key        varchar(512),
    embedding_base_url varchar(512),
    embedding_api_key  varchar(512),
    llm_model          varchar(128) NOT NULL,
    embedding_model    varchar(128) NOT NULL,
    system_prompt      text,
    max_tokens         bigint NOT NULL DEFAULT 8192,
    vector_dimension   bigint NOT NULL DEFAULT 1024,
    is_default         boolean NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE llm_configs IS 'LLM/Embedding 提供商配置';
COMMENT ON COLUMN llm_configs.llm_api_key IS 'AES-256 加密存储';
COMMENT ON COLUMN llm_configs.vector_dimension IS 'Qwen3-Embedding=1024, text-embedding-3-small=1536';
COMMENT ON COLUMN llm_configs.is_default IS '默认配置，最多一条为 true';

CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_default ON llm_configs (is_default) WHERE is_default = true;

-- HNSW 向量索引（每次重建以保证参数一致）
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

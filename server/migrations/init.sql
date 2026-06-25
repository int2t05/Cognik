-- OpsMind 数据库 DDL 增强脚本
--
-- GORM AutoMigrate 负责基础表结构，本脚本处理 GORM 无法覆盖的部分：
--   - pgvector 扩展
--   - halfvec 向量列
--   - HNSW 索引
--   - 列注释
--
-- 加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/init.sql

-- =============================================================================
-- pgvector 扩展
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS vector;

-- =============================================================================
-- knowledge_chunks：halfvec 向量列（GORM 不支持此类型）
-- =============================================================================

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

-- =============================================================================
-- HNSW 向量索引
-- =============================================================================

DROP INDEX IF EXISTS idx_chunks_embedding;
CREATE INDEX idx_chunks_embedding ON knowledge_chunks
    USING hnsw (embedding halfvec_cosine_ops)
    WITH (m = 16, ef_construction = 200);

-- =============================================================================
-- chat_messages：JSONB 列 + 注释（GORM 创建列但不设注释）
-- =============================================================================

COMMENT ON COLUMN chat_messages.pipeline_metrics IS 'RAG 管道各步骤耗时（ms）';
COMMENT ON COLUMN chat_messages.confidence_raw IS '原始综合置信度分数 [0,1]，用于分位数统计';

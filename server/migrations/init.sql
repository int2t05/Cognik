-- OpsMind 数据库 DDL 增强脚本
--
-- GORM AutoMigrate 已覆盖表结构 + halfvec 列 + HNSW 索引。
-- 本脚本仅补充 GORM 无法处理的列注释。
--
-- 加载方式：
--   docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/init.sql

-- =============================================================================
-- pgvector 扩展（幂等，AutoMigrate 也执行，此处显式声明）
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS vector;

-- =============================================================================
-- 列注释（GORM 不支持 COMMENT ON COLUMN）
-- =============================================================================

COMMENT ON COLUMN knowledge_chunks.embedding IS 'halfvec 半精度向量（固定 1024 维），pgvector 余弦相似度检索';
COMMENT ON COLUMN chat_messages.pipeline_metrics IS 'RAG 管道各步骤耗时（ms）';
COMMENT ON COLUMN chat_messages.confidence_raw IS '原始综合置信度分数 [0,1]，用于分位数统计';

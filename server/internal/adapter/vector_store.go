// Package adapter 提供外部服务的适配层。
//
// vector_store.go 定义 VectorStore 接口和 pgvector 实现。
// 使用 pgvector halfvec 类型 + HNSW 索引存储和检索向量。
//
// 设计决策（ADR-V2-003）：
// 接口明确暴露 pgvector 概念（halfvec、cosine），不追求数据库无关抽象。
// pgvector 的 halfvec/HNSW/<=> 算子是不可替代的核心能力，
// 过度抽象反而限制性能优化空间。
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"gorm.io/gorm"
)

// =============================================================================
// 接口定义
// =============================================================================

// VectorStore 定义 pgvector 向量存储接口。
type VectorStore interface {
	// BatchInsert 批量写入分块向量（使用 halfvec 半精度）。
	BatchInsert(ctx context.Context, chunks []VectorChunk) error

	// CosineSearch 余弦相似度检索（使用 pgvector <=> 算子）。
	CosineSearch(ctx context.Context, kbID int64, embedding []float32, topK int) ([]SearchResult, error)

	// DeleteByArticle 删除指定文章的所有向量分块。
	DeleteByArticle(ctx context.Context, articleID int64) error

	// DeleteByKB 删除指定知识库的所有向量分块。
	DeleteByKB(ctx context.Context, kbID int64) error

	// CountByKB 统计知识库的分块总数。
	CountByKB(ctx context.Context, kbID int64) (int64, error)

	// ReplaceVectors 原子替换文章向量：事务内写入新向量后删除旧向量。
	ReplaceVectors(ctx context.Context, articleID int64, chunks []VectorChunk) error

	// GetChunksByArticle 获取指定文章的所有分块内容（不含向量）。
	GetChunksByArticle(ctx context.Context, articleID int64) ([]ChunkContent, error)

	// GetChunkSnapshots 获取指定文章的所有分块快照（含 chunk_hash 和 embedding 文本表示）。
	//
	// 为什么需要 embedding 文本表示：增量发布时需要复用旧 chunk 的 embedding，
	// halfvec 列通过 ::text 强制转换为 pgvector 数组字面量字符串（如 [0.1,0.2,...]），
	// 可被 float32ToPgVector 的逆向解析还原为 []float32。
	GetChunkSnapshots(ctx context.Context, articleID int64) ([]ChunkSnapshot, error)
}

// =============================================================================
// 类型定义
// =============================================================================

// VectorChunk 待写入的向量分块。
type VectorChunk struct {
	ArticleID       int64     `json:"article_id"`
	KBID            int64     `json:"kb_id"`
	Content         string    `json:"content"`
	ChunkIndex      int       `json:"chunk_index"`
	Embedding       []float32 `json:"embedding"`
	EmbeddingModel  string    `json:"embedding_model"`
	VectorDimension int       `json:"vector_dimension"`
	ChunkHash       string    `json:"chunk_hash"` // SHA256 增量比对
}

// SearchResult 向量检索结果。
type SearchResult struct {
	ChunkID    int64   `json:"chunk_id"`
	ArticleID  int64   `json:"article_id"`
	Content    string  `json:"content"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
}

// ChunkContent 分块内容（不含向量，用于重索引）。
type ChunkContent struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	ChunkIndex int    `json:"chunk_index"`
}

// ChunkSnapshot 分块快照（含 chunk_hash 和 embedding 文本表示，用于增量发布比对）。
//
// EmbeddingText 是 pgvector halfvec 列的 ::text 形式（如 [0.123,0.456,...]），
// 由 parsePgVectorText 还原为 []float32。
type ChunkSnapshot struct {
	ChunkHash     string `json:"chunk_hash"`
	EmbeddingText string `json:"embedding_text"`
}

// =============================================================================
// pgvector 实现
// =============================================================================

// PgvectorStore 实现 VectorStore，使用 pgvector 扩展。
//
// 复用 GORM 的 *sql.DB 连接池，避免创建独立连接池造成双池浪费。
type PgvectorStore struct {
	db *sql.DB
}

// NewPgvectorStore 创建 PgvectorStore 实例，复用 GORM DB 连接池。
func NewPgvectorStore(gormDB *gorm.DB) (*PgvectorStore, error) {
	db, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 GORM 底层 sql.DB 失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pgvector Ping 失败: %w", err)
	}
	return &PgvectorStore{db: db}, nil
}

// Close 关闭与 GORM 共享的底层连接（由 GORM 管理生命周期，此方法为 no-op）。
func (s *PgvectorStore) Close() error { return nil }

// =============================================================================
// BatchInsert — 批量写入向量
// =============================================================================

// BatchInsert 批量写入分块向量。
//
// SQL: INSERT INTO knowledge_chunks (article_id, kb_id, content, chunk_index,
//
//	embedding, embedding_model, vector_dimension, created_at)
//	VALUES ($1, $2, $3, $4, $5::halfvec, $6, $7, NOW())
func (s *PgvectorStore) BatchInsert(ctx context.Context, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// 校验所有 chunk 的 embedding 维度一致，维度不匹配时在应用层提前报错，
	// 避免到 pgvector 层才失败（错误信息不友好且难以定位是哪个 chunk 的问题）。
	var expectedDim int
	for i, c := range chunks {
		if len(c.Embedding) == 0 {
			continue
		}
		if expectedDim == 0 {
			expectedDim = len(c.Embedding)
		} else if len(c.Embedding) != expectedDim {
			return fmt.Errorf("chunk %d embedding 维度不一致: 预期 %d, 实际 %d (article_id=%d, chunk_index=%d)",
				i, expectedDim, len(c.Embedding), c.ArticleID, c.ChunkIndex)
		}
		// VectorDimension 字段应与实际 embedding 长度一致
		if c.VectorDimension != 0 && c.VectorDimension != len(c.Embedding) {
			return fmt.Errorf("chunk %d VectorDimension 与实际 embedding 长度不匹配: VectorDimension=%d, len(embedding)=%d (article_id=%d)",
				i, c.VectorDimension, len(c.Embedding), c.ArticleID)
		}
	}

	// 构建批量 INSERT（含 chunk_hash 用于增量比对）
	query := `INSERT INTO knowledge_chunks
		(article_id, kb_id, content, chunk_index, embedding, embedding_model, vector_dimension, chunk_hash, created_at)
		VALUES `

	var placeholders []string
	var args []interface{}
	for i, c := range chunks {
		base := i * 8
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d::halfvec, $%d, $%d, $%d, NOW())",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8))
		args = append(args, c.ArticleID, c.KBID, c.Content, c.ChunkIndex,
			float32ToPgVector(c.Embedding), c.EmbeddingModel, c.VectorDimension, c.ChunkHash)
	}

	query += strings.Join(placeholders, ", ")
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("批量写入向量失败 (count=%d): %w", len(chunks), err)
	}
	return nil
}

// =============================================================================
// CosineSearch — 余弦相似度检索
// =============================================================================

// CosineSearch 使用 cosine 距离算子 (<=>) 检索最相似的 topK 个向量。
//
// SQL: SELECT id, article_id, content, chunk_index,
//
//	1 - (embedding <=> $1::halfvec) AS score
//	FROM knowledge_chunks
//	WHERE kb_id = $2
//	ORDER BY embedding <=> $1::halfvec
//	LIMIT $3
//
// <=> 算子返回余弦距离（越小越相似），1 - distance 转换为相似度分数。
func (s *PgvectorStore) CosineSearch(ctx context.Context, kbID int64, embedding []float32, topK int) ([]SearchResult, error) {
	// 空 embedding 防护：空向量检索无意义，提前报错避免数据库报错
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding 为空，无法执行向量检索 (kb_id=%d)", kbID)
	}
	// topK 范围钳位：过小无意义，过大会拖慢检索且超出业务需求
	if topK <= 0 {
		topK = 10
	} else if topK > 100 {
		topK = 100
	}

	query := `SELECT id, article_id, content, chunk_index,
		1 - (embedding <=> $1::halfvec) AS score
		FROM knowledge_chunks
		WHERE kb_id = $2
		ORDER BY embedding <=> $1::halfvec
		LIMIT $3`

	rows, err := s.db.QueryContext(ctx, query, float32ToPgVector(embedding), kbID, topK)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败 (kb_id=%d): %w", kbID, err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.ArticleID, &r.Content, &r.ChunkIndex, &r.Score); err != nil {
			return nil, fmt.Errorf("扫描检索结果失败: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// =============================================================================
// Delete / Count / Get
// =============================================================================

// DeleteByArticle 按文章 ID 删除所有向量分块。
func (s *PgvectorStore) DeleteByArticle(ctx context.Context, articleID int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM knowledge_chunks WHERE article_id = $1", articleID)
	if err != nil {
		return fmt.Errorf("删除文章向量失败 (article_id=%d): %w", articleID, err)
	}
	return nil
}

// DeleteByKB 按知识库 ID 删除所有向量分块。
func (s *PgvectorStore) DeleteByKB(ctx context.Context, kbID int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM knowledge_chunks WHERE kb_id = $1", kbID)
	if err != nil {
		return fmt.Errorf("删除知识库向量失败 (kb_id=%d): %w", kbID, err)
	}
	return nil
}

// ReplaceVectors 原子替换文章向量：事务内先删旧向量，再写新向量。
// 先删后写：删除成功但写入失败时事务回滚，旧向量完整恢复。
func (s *PgvectorStore) ReplaceVectors(ctx context.Context, articleID int64, chunks []VectorChunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ReplaceVectors 开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM knowledge_chunks WHERE article_id = $1", articleID); err != nil {
		return fmt.Errorf("ReplaceVectors 删除旧向量失败: %w", err)
	}

	query := "INSERT INTO knowledge_chunks (article_id, kb_id, content, chunk_index, embedding, embedding_model, vector_dimension, chunk_hash, created_at) VALUES "
	var placeholders []string
	var args []interface{}
	for i, ch := range chunks {
		base := i * 8
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d::halfvec, $%d, $%d, $%d, NOW())",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8))
		args = append(args, ch.ArticleID, ch.KBID, ch.Content, ch.ChunkIndex,
			float32ToPgVector(ch.Embedding), ch.EmbeddingModel, ch.VectorDimension, ch.ChunkHash)
	}
	query += strings.Join(placeholders, ", ")
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("ReplaceVectors 写入新向量失败 (count=%d): %w", len(chunks), err)
	}

	return tx.Commit()
}

// CountByKB 统计知识库的分块总数。
func (s *PgvectorStore) CountByKB(ctx context.Context, kbID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM knowledge_chunks WHERE kb_id = $1", kbID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计分块数失败 (kb_id=%d): %w", kbID, err)
	}
	return count, nil
}

// GetChunksByArticle 获取指定文章的所有分块内容（不含向量，用于重索引）。
func (s *PgvectorStore) GetChunksByArticle(ctx context.Context, articleID int64) ([]ChunkContent, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, content, chunk_index FROM knowledge_chunks WHERE article_id = $1 ORDER BY chunk_index",
		articleID)
	if err != nil {
		return nil, fmt.Errorf("查询分块失败 (article_id=%d): %w", articleID, err)
	}
	defer rows.Close()

	var chunks []ChunkContent
	for rows.Next() {
		var c ChunkContent
		if err := rows.Scan(&c.ID, &c.Content, &c.ChunkIndex); err != nil {
			return nil, fmt.Errorf("扫描分块失败: %w", err)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// GetChunkSnapshots 获取指定文章的所有分块快照（含 chunk_hash 和 embedding 文本表示）。
//
// embedding 列通过 ::text 强制转换为 pgvector 数组字面量字符串，
// 由 parsePgVectorText 还原为 []float32 供增量发布复用。
func (s *PgvectorStore) GetChunkSnapshots(ctx context.Context, articleID int64) ([]ChunkSnapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT chunk_hash, embedding::text FROM knowledge_chunks WHERE article_id = $1 ORDER BY chunk_index",
		articleID)
	if err != nil {
		return nil, fmt.Errorf("查询分块快照失败 (article_id=%d): %w", articleID, err)
	}
	defer rows.Close()

	var snapshots []ChunkSnapshot
	for rows.Next() {
		var ss ChunkSnapshot
		if err := rows.Scan(&ss.ChunkHash, &ss.EmbeddingText); err != nil {
			return nil, fmt.Errorf("扫描分块快照失败: %w", err)
		}
		snapshots = append(snapshots, ss)
	}
	return snapshots, rows.Err()
}

// =============================================================================
// 辅助函数
// =============================================================================

// ParsePgVectorText 将 pgvector ::text 输出（如 "[0.123,0.456,...]"）还原为 []float32。
//
// 为什么需要此函数：增量发布时需要复用旧 chunk 的 embedding，
// halfvec 列通过 ::text 输出为 pgvector 数组字面量字符串，
// 解析后可直接用于 VectorChunk.Embedding。
func ParsePgVectorText(text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if len(text) < 2 || text[0] != '[' || text[len(text)-1] != ']' {
		return nil, fmt.Errorf("非法的 pgvector text 格式: %s", text)
	}
	inner := text[1 : len(text)-1]
	if inner == "" {
		return []float32{}, nil
	}
	parts := strings.Split(inner, ",")
	result := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var f float64
		if _, err := fmt.Sscanf(p, "%f", &f); err != nil {
			return nil, fmt.Errorf("解析 pgvector 数值失败: %q: %w", p, err)
		}
		result = append(result, float32(f))
	}
	return result, nil
}

// float32ToPgVector 将 []float32 转换为 pgvector 兼容的数组字面量字符串。
//
// pgvector 接受 ARRAY[...] 或 [val1,val2,...] 格式。
// 使用 [val1,val2,...] 格式配合 ::halfvec 显式类型转换。
//
// 对 NaN 和 ±Inf 使用 0.0 替代——pgvector 不接受非有限浮点数，
// 而 NaN/Inf 在 normalized embedding 中不应出现（出现意味着上游 bug），
// 0.0 替代是最小伤害的降级策略（不影响向量维度），同时记录 Warn 便于排查上游问题。
func float32ToPgVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	// halfvec(FP16) 精度约 3.3 位十进制有效数字，%.8f 已充分保留原始值
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		// 使用 math.IsNaN 和 math.IsInf 精确检测非有限浮点数
		// pgvector 不接受 NaN/Inf，0.0 替代是最小伤害的降级策略
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			f = 0.0
			slog.Warn("向量含 NaN/Inf，已替换为 0.0", "index", i)
		}
		b.WriteString(fmt.Sprintf("%.8f", f))
	}
	b.WriteByte(']')
	return b.String()
}

// Package adapter 提供外部服务的适配层。
package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
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

	// ReplaceVectors 原子替换文章向量（事务内先删旧后写新）。
	ReplaceVectors(ctx context.Context, articleID int64, chunks []VectorChunk) error

	// GetChunksByArticle 获取指定文章的所有分块内容（不含向量）。
	GetChunksByArticle(ctx context.Context, articleID int64) ([]ChunkContent, error)

	// GetChunkSnapshots 获取分块快照（含 chunk_hash 和 embedding::text），用于增量发布复用旧 embedding。
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

// ChunkSnapshot 分块快照（含 chunk_hash 和 embedding::text），用于增量发布比对。
// EmbeddingText 由 ParsePgVectorText 还原为 []float32。
type ChunkSnapshot struct {
	ChunkHash     string `json:"chunk_hash"`
	EmbeddingText string `json:"embedding_text"`
}

// =============================================================================
// pgvector 实现
// =============================================================================

// PgvectorStore 实现 VectorStore，复用 GORM 连接池。
type PgvectorStore struct {
	db         *sql.DB
	maxRetries int
	efSearch   int // HNSW 查询时 ef_search（>0 时在只读事务内 SET LOCAL，连接池安全）
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
	return &PgvectorStore{db: db, maxRetries: defaultMaxRetries}, nil
}

// SetEfSearch 设置 HNSW 查询时 ef_search（ef_search 必须 ≥ LIMIT；越大召回越高、延迟越大）。
// 仅查询时生效，不重建索引。0 表示用 pgvector 默认值 40。
func (s *PgvectorStore) SetEfSearch(n int) { s.efSearch = n }

// Close 关闭与 GORM 共享的底层连接（由 GORM 管理生命周期，此方法为 no-op）。
func (s *PgvectorStore) Close() error { return nil }

// =============================================================================
// BatchInsert — 批量写入向量
// =============================================================================

// BatchInsert 批量写入分块向量。
func (s *PgvectorStore) BatchInsert(ctx context.Context, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// 维度校验在应用层做，避免到 pgvector 才失败、难以定位
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
		if c.VectorDimension != 0 && c.VectorDimension != len(c.Embedding) {
			return fmt.Errorf("chunk %d VectorDimension 与实际 embedding 长度不匹配: VectorDimension=%d, len(embedding)=%d (article_id=%d)",
				i, c.VectorDimension, len(c.Embedding), c.ArticleID)
		}
	}

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

// CosineSearch 使用 <=> 算子检索 topK 相似向量；1 - 距离 = 相似度分数。
func (s *PgvectorStore) CosineSearch(ctx context.Context, kbID int64, embedding []float32, topK int) ([]SearchResult, error) {
	return s.CosineSearchWithFilter(ctx, kbID, embedding, topK, nil)
}

// CosineSearchWithFilter 带 metadata 过滤的向量检索。
// tags 为空时不加过滤；非空时用 JSONB ?| 操作符匹配任一标签。
func (s *PgvectorStore) CosineSearchWithFilter(ctx context.Context, kbID int64, embedding []float32, topK int, tags []string) ([]SearchResult, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding 为空，无法执行向量检索 (kb_id=%d)", kbID)
	}
	if topK <= 0 {
		topK = 10
	} else if topK > 100 {
		topK = 100
	}

	query := `SELECT kc.id, kc.article_id, kc.content, kc.chunk_index,
		1 - (kc.embedding <=> $1::halfvec) AS score
		FROM knowledge_chunks kc
		JOIN knowledge_articles ka ON ka.id = kc.article_id
		WHERE kc.kb_id = $2 AND ka.status = 4`
	args := []interface{}{float32ToPgVector(embedding), kbID}

	if len(tags) > 0 {
		tagsJSON, _ := json.Marshal(tags)
		query += ` AND ka.tags ?| $3::jsonb`
		args = append(args, string(tagsJSON))
	}

	query += ` ORDER BY kc.embedding <=> $1::halfvec LIMIT ` + fmt.Sprintf("%d", topK)

	// efSearch>0 时在只读事务内 SET LOCAL，作用域限单连接（连接池安全）。
	// SET LOCAL 不接受参数占位符，用 %d（int 不可能注入）。
	var (
		results []SearchResult
		err     error
	)
	if s.efSearch > 0 {
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, fmt.Errorf("开启 ef_search 事务失败 (kb_id=%d): %w", kbID, txErr)
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL hnsw.ef_search = %d", s.efSearch)); err != nil {
			return nil, fmt.Errorf("设置 ef_search 失败: %w", err)
		}
		rows, qerr := tx.QueryContext(ctx, query, args...)
		results, err = scanSearchRows(rows, kbID)
		if err == nil {
			err = qerr
		}
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("提交 ef_search 事务失败 (kb_id=%d): %w", kbID, err)
		}
	} else {
		rows, qerr := s.db.QueryContext(ctx, query, args...)
		results, err = scanSearchRows(rows, kbID)
		if err == nil {
			err = qerr
		}
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// scanSearchRows 扫描向量检索结果；调用方负责 QueryContext 错误（rows 为 nil 时返回 nil）。
func scanSearchRows(rows *sql.Rows, kbID int64) ([]SearchResult, error) {
	if rows == nil {
		return nil, nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.ArticleID, &r.Content, &r.ChunkIndex, &r.Score); err != nil {
			return nil, fmt.Errorf("扫描检索结果失败 (kb_id=%d): %w", kbID, err)
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

// ReplaceVectors 原子替换文章向量（事务内先删旧后写新，回滚时旧向量完整恢复）。
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

// GetChunkSnapshots 获取分块快照（含 chunk_hash 和 embedding::text），用于增量发布复用。
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

// ParsePgVectorText 将 pgvector ::text 输出还原为 []float32，用于增量发布复用旧 embedding。
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

// float32ToPgVector 将 []float32 转为 pgvector 数组字面量（配合 ::halfvec）。
// NaN/Inf 用 0.0 替代（pgvector 不接受非有限值），记 Warn 以排查上游。
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
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			f = 0.0
			slog.Warn("向量含 NaN/Inf，已替换为 0.0", "index", i)
		}
		b.WriteString(fmt.Sprintf("%.8f", f))
	}
	b.WriteByte(']')
	return b.String()
}

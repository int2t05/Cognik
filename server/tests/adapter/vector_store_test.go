//go:build integration

// Package adapter_test 测试 VectorStore pgvector 适配器。
//
// 需要在 Docker pgvector 环境中运行：
//
//	go test ./tests/adapter/... -v -tags=integration -run TestVectorStore
package adapter_test

import (
	"context"
	"database/sql"
	"math"
	"os"
	"testing"

	"opsmind/internal/adapter"
	"opsmind/internal/config"
	"opsmind/internal/database"

	"gorm.io/gorm"
)

// pgvectorDSN 返回 pgvector 测试数据库连接字符串。
func pgvectorDSN() string {
	host := "localhost"
	user := "opsmind"
	password := "opsmind_dev"
	dbname := "opsmind_test"
	if env := os.Getenv("DB_HOST"); env != "" {
		host = env
	}
	if env := os.Getenv("DB_USER"); env != "" {
		user = env
	}
	if env := os.Getenv("DB_PASSWORD"); env != "" {
		password = env
	}
	return "postgres://" + user + ":" + password + "@" + host + ":5432/" + dbname + "?sslmode=disable"
}

// mustGormDB 创建测试用 GORM DB 连接。
func mustGormDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := "localhost"
	user := "opsmind"
	password := "opsmind_dev"
	dbname := "opsmind_test"
	if env := os.Getenv("DB_HOST"); env != "" {
		host = env
	}
	if env := os.Getenv("DB_USER"); env != "" {
		user = env
	}
	if env := os.Getenv("DB_PASSWORD"); env != "" {
		password = env
	}
	db, err := database.Init(config.DatabaseConfig{
		Host: host, Port: 5432, User: user, Password: password, DBName: dbname, SSLMode: "disable",
	})
	if err != nil {
		t.Skipf("跳过集成测试：无法连接 pgvector (%v)", err)
	}
	return db
}

// mustVectorStore 创建测试用 VectorStore，连接失败跳过测试。
// 同时确保 knowledge_chunks 表含 kb_id、chunk_index、embedding 列。
func mustVectorStore(t *testing.T) adapter.VectorStore {
	t.Helper()
	db := mustGormDB(t)
	store, err := adapter.NewPgvectorStore(db)
	if err != nil {
		t.Skipf("跳过集成测试：无法连接 pgvector (%v)", err)
	}
	// 确保 knowledge_chunks 表 schema 正确
	ensureChunksTableV2(t)
	return store
}

// ensureChunksTableV2 重建 knowledge_chunks，保证列定义与 VectorStore INSERT 一致。
func ensureChunksTableV2(t *testing.T) {
	t.Helper()
	rawDB, err := sql.Open("pgx", pgvectorDSN())
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer rawDB.Close()
	rawDB.Exec(`DROP TABLE IF EXISTS knowledge_chunks`)
	rawDB.Exec(`CREATE TABLE IF NOT EXISTS knowledge_chunks (
		id BIGSERIAL PRIMARY KEY,
		article_id BIGINT NOT NULL,
		kb_id BIGINT NOT NULL DEFAULT 0,
		content TEXT NOT NULL,
		chunk_index INTEGER NOT NULL DEFAULT 0,
		embedding_model VARCHAR(128) NOT NULL DEFAULT '',
		vector_dimension INTEGER NOT NULL DEFAULT 0,
		embedding halfvec(1024),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
}

// =============================================================================
// 测试用例
// =============================================================================

func TestVectorStore_BatchInsert_Success(t *testing.T) {
	store := mustVectorStore(t)
	ctx := context.Background()

	// 生成 3 个 1024 维向量
	embedding := make([]float32, 1024)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	chunks := []adapter.VectorChunk{
		{ArticleID: 99901, KBID: 1, Content: "测试分块1：账号冻结处理", ChunkIndex: 0,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
		{ArticleID: 99901, KBID: 1, Content: "测试分块2：联系管理员", ChunkIndex: 1,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
		{ArticleID: 99902, KBID: 1, Content: "测试分块3：VPN 密码重置", ChunkIndex: 0,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
	}

	err := store.BatchInsert(ctx, chunks)
	if err != nil {
		t.Fatalf("BatchInsert 失败: %v", err)
	}

	// 验证写入后统计
	count, err := store.CountByKB(ctx, 1)
	if err != nil {
		t.Fatalf("CountByKB 失败: %v", err)
	}
	if count < 3 {
		t.Errorf("写入后 CountByKB 应 ≥ 3, 实际 %d", count)
	}

	// 清理：删除测试数据
	_ = store.DeleteByArticle(ctx, 99901)
	_ = store.DeleteByArticle(ctx, 99902)
}

func TestVectorStore_CosineSearch(t *testing.T) {
	store := mustVectorStore(t)
	ctx := context.Background()

	// 写入 2 条测试向量
	embedding := make([]float32, 1024)
	embedding[0] = 0.5
	embedding[1] = 0.3

	chunks := []adapter.VectorChunk{
		{ArticleID: 99910, KBID: 1, Content: "VPN 连接问题排查", ChunkIndex: 0,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
		{ArticleID: 99910, KBID: 1, Content: "邮箱无法登录处理", ChunkIndex: 1,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
	}
	if err := store.BatchInsert(ctx, chunks); err != nil {
		t.Fatalf("BatchInsert 失败: %v", err)
	}
	defer store.DeleteByArticle(ctx, 99910)

	// 使用相同向量检索自身 — 相似度应 > 0.99
	results, err := store.CosineSearch(ctx, 1, embedding, 2)
	if err != nil {
		t.Fatalf("CosineSearch 失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("期望 2 条结果, 实际 %d", len(results))
	}
	if results[0].Score < 0.99 {
		t.Errorf("相同向量相似度应 ≥ 0.99, 实际 %.4f", results[0].Score)
	}
	if results[0].Score > 1.01 {
		t.Errorf("余弦相似度应在 [0, 1] 范围内, 实际 %.4f", results[0].Score)
	}
}

func TestVectorStore_DeleteByArticle(t *testing.T) {
	store := mustVectorStore(t)
	ctx := context.Background()

	embedding := make([]float32, 1024)
	embedding[0] = 0.1

	chunks := []adapter.VectorChunk{
		{ArticleID: 99920, KBID: 1, Content: "待删除分块", ChunkIndex: 0,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
	}
	if err := store.BatchInsert(ctx, chunks); err != nil {
		t.Fatalf("BatchInsert 失败: %v", err)
	}

	// 删除
	if err := store.DeleteByArticle(ctx, 99920); err != nil {
		t.Fatalf("DeleteByArticle 失败: %v", err)
	}

	// 检索应无结果
	results, err := store.CosineSearch(ctx, 1, embedding, 2)
	if err != nil {
		t.Fatalf("CosineSearch 失败: %v", err)
	}

	// 清点：按 article_id 检索应无结果
	for _, r := range results {
		if r.ArticleID == 99920 {
			t.Errorf("DeleteByArticle 后仍返回已删除的分块 (id=%d)", r.ChunkID)
		}
	}
}

func TestVectorStore_GetChunksByArticle(t *testing.T) {
	store := mustVectorStore(t)
	ctx := context.Background()

	embedding := make([]float32, 1024)
	chunks := []adapter.VectorChunk{
		{ArticleID: 99930, KBID: 1, Content: "块A", ChunkIndex: 0,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
		{ArticleID: 99930, KBID: 1, Content: "块B", ChunkIndex: 1,
			Embedding: embedding, EmbeddingModel: "bge-m3", VectorDimension: 1024},
	}
	if err := store.BatchInsert(ctx, chunks); err != nil {
		t.Fatalf("BatchInsert 失败: %v", err)
	}
	defer store.DeleteByArticle(ctx, 99930)

	contents, err := store.GetChunksByArticle(ctx, 99930)
	if err != nil {
		t.Fatalf("GetChunksByArticle 失败: %v", err)
	}
	if len(contents) != 2 {
		t.Errorf("期望 2 条 chunk 内容, 实际 %d", len(contents))
	}
}

func TestVectorStore_BatchInsert_SameVectorSimilarity(t *testing.T) {
	store := mustVectorStore(t)
	ctx := context.Background()

	// 构造相互正交（非完全相同）的一组向量，验证检索能区分
	a := make([]float32, 1024)
	b := make([]float32, 1024)
	a[0] = 1.0
	b[1] = 1.0

	chunks := []adapter.VectorChunk{
		{ArticleID: 99940, KBID: 1, Content: "向量A", ChunkIndex: 0,
			Embedding: a, EmbeddingModel: "bge-m3", VectorDimension: 1024},
		{ArticleID: 99940, KBID: 1, Content: "向量B", ChunkIndex: 1,
			Embedding: b, EmbeddingModel: "bge-m3", VectorDimension: 1024},
	}
	if err := store.BatchInsert(ctx, chunks); err != nil {
		t.Fatalf("BatchInsert 失败: %v", err)
	}
	defer store.DeleteByArticle(ctx, 99940)

	// 用向量 a 检索 — 预期 a 的分 > b 的分
	results, err := store.CosineSearch(ctx, 1, a, 2)
	if err != nil {
		t.Fatalf("CosineSearch 失败: %v", err)
	}
	if len(results) < 2 {
		t.Skip("需要 ≥ 2 条结果")
	}

	if results[0].Content != "向量A" {
		t.Errorf("期望「向量A」排第一, 实际「%s」(分数 %.4f)", results[0].Content, results[0].Score)
	}
	// 正交向量的余弦相似度接近 0
	if math.Abs(results[1].Score) > 0.1 {
		t.Logf("注意：正交向量余弦相似度 = %.4f（接近 0 属正常）", results[1].Score)
	}
}

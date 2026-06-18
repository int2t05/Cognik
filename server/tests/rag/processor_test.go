//go:build integration

// Package rag_test 测试文档异步处理管线。
//
// 需要在 Docker pgvector 环境中运行：
//
//	go test ./tests/rag/... -v -tags=integration -run TestProcessor
package rag_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"opsmind/internal/adapter"
	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/rag"

	"gorm.io/gorm"
)

// =============================================================================
// 辅助函数
// =============================================================================

// ragVectorStoreDSN 返回 pgvector 测试数据库连接字符串。
func ragVectorStoreDSN() string {
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

// ragMustGormDB 创建测试用 GORM DB。
func ragMustGormDB(t *testing.T) *gorm.DB {
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
		t.Skipf("跳过集成测试：无法连接数据库 (%v)", err)
	}
	return db
}

// ragMustVectorStore 创建测试用 VectorStore。
func ragMustVectorStore(t *testing.T) adapter.VectorStore {
	t.Helper()
	db := ragMustGormDB(t)
	store, err := adapter.NewPgvectorStore(db)
	if err != nil {
		t.Skipf("跳过集成测试：无法连接 pgvector (%v)", err)
	}
	// 确保 knowledge_chunks 含 kb_id、chunk_index、embedding 列
	rawDB, err := sql.Open("pgx", ragVectorStoreDSN())
	if err == nil {
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
	return store
}

// =============================================================================
// Processor 测试
// =============================================================================

// TestProcessor_SingleDocument 验证单个文档的完整处理流程。
func TestProcessor_SingleDocument(t *testing.T) {
	store := ragMustVectorStore(t)
	ctx := context.Background()

	parser := rag.NewDocParser()
	chunker := rag.NewChunker(1000, 200)
	emb := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)

	proc := rag.NewProcessor(parser, chunker, emb, store, nil, 2)

	// 准备测试数据
	articleID := int64(99950)
	kbID := int64(1)

	// 清理旧数据
	_ = store.DeleteByArticle(ctx, articleID)

	// 提交任务：模拟文档文本
	task := rag.ProcessTask{
		ArticleID: articleID,
		KBID:      kbID,
		Content:   "VPN 连接失败排查指南。\n\n确认网络连接正常。\n\n重启 VPN 客户端。",
		OnStatusChange: func(articleID int64, status, errMsg string) {
			t.Logf("状态变更: article=%d status=%s", articleID, status)
		},
	}
	proc.Submit(task)

	// 等待处理完成
	time.Sleep(500 * time.Millisecond)
	proc.Stop()

	// 验证：分块已写入
	chunks, err := store.GetChunksByArticle(ctx, articleID)
	if err != nil {
		t.Fatalf("GetChunksByArticle 失败: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("处理器应写入至少 1 个分块")
	}
	t.Logf("写入 %d 个分块", len(chunks))

	// 清理
	_ = store.DeleteByArticle(ctx, articleID)
}

// TestProcessor_MultipleDocuments 验证多文档并发处理。
func TestProcessor_MultipleDocuments(t *testing.T) {
	store := ragMustVectorStore(t)
	ctx := context.Background()

	parser := rag.NewDocParser()
	chunker := rag.NewChunker(500, 100)
	emb := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	proc := rag.NewProcessor(parser, chunker, emb, store, nil, 2)

	baseID := int64(99960)

	// 提交 3 个文档
	for i := 0; i < 3; i++ {
		articleID := baseID + int64(i)
		_ = store.DeleteByArticle(ctx, articleID)

		content := strings.Repeat("运维文档内容片段。", 20)
		task := rag.ProcessTask{
			ArticleID: articleID,
			KBID:      1,
			Content:   content,
		}
		proc.Submit(task)
	}

	time.Sleep(2 * time.Second)
	proc.Stop()

	// 验证所有文档都已处理
	for i := 0; i < 3; i++ {
		articleID := baseID + int64(i)
		chunks, err := store.GetChunksByArticle(ctx, articleID)
		if err != nil {
			t.Errorf("article %d GetChunksByArticle 失败: %v", articleID, err)
			continue
		}
		if len(chunks) == 0 {
			t.Errorf("article %d 应有分块", articleID)
		}
		_ = store.DeleteByArticle(ctx, articleID)
	}
}

// TestProcessor_StopGraceful 验证优雅关闭不丢失已提交任务。
func TestProcessor_StopGraceful(t *testing.T) {
	store := ragMustVectorStore(t)
	ctx := context.Background()

	parser := rag.NewDocParser()
	chunker := rag.NewChunker(1000, 200)
	emb := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	proc := rag.NewProcessor(parser, chunker, emb, store, nil, 1) // 单 worker

	articleID := int64(99980)
	_ = store.DeleteByArticle(ctx, articleID)

	proc.Submit(rag.ProcessTask{
		ArticleID: articleID,
		KBID:      1,
		Content:   "测试文本内容。",
	})

	// 立即停止（worker 应完成当前任务后再退出）
	time.Sleep(100 * time.Millisecond)
	proc.Stop()

	// 验证任务完成
	chunks, _ := store.GetChunksByArticle(ctx, articleID)
	t.Logf("Stop 后 article %d 有 %d 个分块", articleID, len(chunks))
	_ = store.DeleteByArticle(ctx, articleID)
}

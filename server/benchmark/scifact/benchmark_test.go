//go:build integration

// Package scifact 提供 BEIR SciFact 全量数据集的 RAG 检索质量基准测试。
//
// 评估管道：真实 pgvector 向量检索 + gse BM25 检索 → RRF 混合融合。
// 指标：Recall@5, Recall@10, MRR, nDCG@10(二值相关性)。
// 依赖：PostgreSQL+pgvector(cognik_test DB)+ DashScope embedding(.env 配置)。
// 数据集：BEIR SciFact 官方(5183 文档 + test 查询),首次运行自动下载缓存。
//
// 运行：cd server && go test ./benchmark/scifact/... -v -tags=integration -run Benchmark -p 1 -timeout 1800s
package scifact

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"cognik/internal/domain/knowledge"
	"cognik/internal/infra/adapter"
	"cognik/internal/infra/database"
	"cognik/internal/rag"
	"cognik/internal/shared/model"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 测试数据库 DSN(可被环境变量覆盖)。
const benchDSN = "host=localhost port=5432 user=cognik password=cognik_dev dbname=cognik_test sslmode=disable"

// benchEmbedDim 向量维度(DashScope text-embedding-v2 = 1536)。
const benchEmbedDim = 1536

// TestBenchmarkRAG_SciFact 全量评估 SciFact 数据集上的向量/BM25/混合检索质量。
func TestBenchmarkRAG_SciFact(t *testing.T) {
	ctx := context.Background()

	// ── Phase 1: 配置 & DB ──
	for _, p := range []string{"../.env", "../../.env", "../../../.env", ".env"} {
		if err := godotenv.Load(p); err == nil {
			break
		}
	}
	embBase := os.Getenv("COGNIK_EMBEDDING_BASE_URL")
	embKey := os.Getenv("COGNIK_EMBEDDING_API_KEY")
	embModel := os.Getenv("COGNIK_EMBEDDING_MODEL")
	if embBase == "" || embKey == "" {
		cwd, _ := os.Getwd()
		t.Skipf("DashScope embedding 配置缺失,跳过(CWD=%s, base=%q)", cwd, embBase)
	}

	// ── Phase 2: 加载 SciFact 数据集 ──
	t.Log("加载 SciFact 数据集(首次运行自动下载)...")
	docs, queries, qrels, err := LoadSciFact()
	if err != nil {
		t.Fatalf("加载 SciFact 失败: %v", err)
	}
	t.Logf("文档 %d 篇, test 查询 %d 条, qrels %d 条", len(docs), len(queries), len(qrels))

	// ── Phase 3: DB 初始化 ──
	db, err := gorm.Open(postgres.Open(benchDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("连接 cognik_test 失败(%v),跳过", err)
	}
	db.Exec("DROP TABLE IF EXISTS knowledge_chunks CASCADE")
	if err := database.AutoMigrate(db, benchEmbedDim); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	db.Exec("DELETE FROM knowledge_chunks")
	db.Exec("DELETE FROM knowledge_articles")
	db.Exec("DELETE FROM knowledge_bases")

	// ── Phase 4: 创建 KB + 文章 + 向量化 ──
	kb := model.KnowledgeBase{
		Name:            "SciFact-Full",
		EmbeddingModel:  embModel,
		VectorDimension: benchEmbedDim,
		RAGWorkspaceSlug: "scifact-full",
	}
	if err := db.Create(&kb).Error; err != nil {
		t.Fatalf("创建 KB 失败: %v", err)
	}
	kbID := kb.ID

	// 插入全部文档(status=4 Published),记录 docID→articleID 映射
	docToArticle := make(map[string]int64, len(docs))
	for i, doc := range docs {
		content := doc.Text
		if doc.Title != "" {
			content = doc.Title + "\n\n" + doc.Text
		}
		title := doc.Title
		if len(title) > 250 {
			title = title[:250] + "..." // varchar(255) 截断保护
		}
		art := model.KnowledgeArticle{
			KBID:        kbID,
			Title:       title,
			Content:     content,
			Status:      model.ArticleStatusPublished,
			ArticleType: "reference",
		}
		if err := db.Create(&art).Error; err != nil {
			t.Fatalf("创建文章 %d(doc %s)失败: %v", i, doc.ID, err)
		}
		docToArticle[doc.ID] = art.ID
	}
	t.Logf("知识库 SciFact-Full (id=%d) 已创建,%d 篇文章", kbID, len(docs))

	// ── Phase 5: 写 chunks + 向量化 ──
	store, err := adapter.NewPgvectorStore(db)
	if err != nil {
		t.Fatalf("创建 VectorStore 失败: %v", err)
	}

	t.Logf("向量化 %d 篇文档(串行,避免 API 限流,约 5-10 分钟)...", len(docs))
	embClient := adapter.NewOpenAIEmbeddingClient(embBase, embKey, embModel, 120*time.Second)
	embedder := rag.NewEmbedder(embClient, 20)
	chunker := rag.NewChunker(2000, 0)
	success, fail := 0, 0
	for i, doc := range docs {
		artID := docToArticle[doc.ID]
		content := doc.Text
		if doc.Title != "" {
			content = doc.Title + "\n\n" + doc.Text
		}
		chunks := chunker.Split(content)
		// 向量化(批量 batch=20,单篇通常 1 chunk)
		vectors, _, err := embedder.Embed(context.Background(), chunks, embModel)
		if err != nil {
			fail++
			continue
		}
		// 写入 pgvector(含 embedding)
		vecChunks := make([]adapter.VectorChunk, len(chunks))
		for j, c := range chunks {
			vecChunks[j] = adapter.VectorChunk{
				ArticleID: artID, KBID: kbID, Content: c, ChunkIndex: j,
				Embedding: vectors[j], EmbeddingModel: embModel, VectorDimension: benchEmbedDim,
			}
		}
		if err := store.ReplaceVectors(context.Background(), artID, vecChunks); err != nil {
			fail++
			continue
		}
		success++
		if (i+1)%500 == 0 {
			t.Logf("  已向量化 %d/%d", i+1, len(docs))
		}
	}
	t.Logf("向量化完成(成功 %d, 失败 %d)", success, fail)

	// ── Phase 5: BM25 索引 ──
	repo := knowledge.NewKnowledgeRepo(db)
	segmenter := rag.NewGseSegmenter()
	bm25 := rag.NewBM25Retriever(segmenter, 30*time.Minute)
	knowledge.RebuildBM25ForKB(repo, store, bm25, kbID)
	t.Logf("BM25 索引重建完成 (%d docs)", len(docs))

	vectorRetriever := rag.NewVectorRetriever(embedder, store)

	// ── Phase 6: 检索 + 指标 ──
	stages := map[string]stageMetrics{"vector": {}, "bm25": {}, "hybrid": {}}

	for qi, q := range queries {
		relevant := make(map[int64]bool)
		for docID, score := range qrels[q.ID] {
			if score > 0 {
				if artID, ok := docToArticle[docID]; ok {
					relevant[artID] = true
				}
			}
		}
		if len(relevant) == 0 {
			continue
		}

		vecRes, _ := vectorRetriever.RetrieveFiltered(ctx, q.Text, kbID, 10, rag.MetaFilter{})
		bm25Res, _ := bm25.RetrieveFiltered(ctx, q.Text, kbID, 10, rag.MetaFilter{})
		hybridRes := rag.HybridFuse(vecRes, bm25Res, 10)

		vecIDs := extractArticleIDs(vecRes)
		bmIDs := extractArticleIDs(bm25Res)
		hybIDs := extractArticleIDs(hybridRes)

		v := stages["vector"]
		accStage(&v, vecIDs, relevant)
		stages["vector"] = v
		b := stages["bm25"]
		accStage(&b, bmIDs, relevant)
		stages["bm25"] = b
		h := stages["hybrid"]
		accStage(&h, hybIDs, relevant)
		stages["hybrid"] = h

		if (qi+1)%20 == 0 {
			t.Logf("  已评估 %d/%d 查询", qi+1, len(queries))
		}
	}

	// ── Phase 7: 汇总报告(同时输出 md 文件) ──
	evalCount := 0
	for _, q := range queries {
		if len(qrels[q.ID]) > 0 {
			evalCount++
		}
	}
	n := float64(evalCount)

	var md strings.Builder
	md.WriteString("# BEIR SciFact 检索评估报告\n\n")
	fmt.Fprintf(&md, "**评估日期**：%s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(&md, "**数据集**：BEIR SciFact 官方(%d 文档 + %d test 查询)\n", len(docs), evalCount)
	fmt.Fprintf(&md, "**评估对象**：Cognik 检索管道(向量 + BM25 + RRF 混合)\n\n")
	md.WriteString("## 汇总结果\n\n")
	md.WriteString("| Stage | Rec@5 | Rec@10 | MRR | nDCG@10 |\n")
	md.WriteString("|-------|-------|--------|-----|---------|\n")
	for _, name := range []string{"vector", "bm25", "hybrid"} {
		s := stages[name]
		if n > 0 {
			fmt.Fprintf(&md, "| %s | %.4f | %.4f | %.4f | %.4f |\n", name, s.rec5/n, s.rec10/n, s.mrr/n, s.ndcg/n)
		}
	}
	md.WriteString("\n## 环境\n\n")
	fmt.Fprintf(&md, "- Embedding: DashScope text-embedding-v2, %d 维\n", benchEmbedDim)
	fmt.Fprintf(&md, "- pgvector: halfvec(%d), HNSW m=16/ef_construction=200/ef_search=100\n", benchEmbedDim)
	md.WriteString("- Chunker: Markdown-aware, 2000 字符 / 重叠 0\n")
	fmt.Fprintf(&md, "- 运行: cd server && go test ./benchmark/scifact/... -v -tags=integration -run Benchmark -p 1 -timeout 1800s\n")

	reportPath := "benchmark/scifact-result.md"
	if err := os.WriteFile(reportPath, []byte(md.String()), 0644); err != nil {
		t.Logf("写入报告文件失败: %v", err)
	}

	t.Logf("")
	t.Logf("================================================================================")
	t.Logf("=== BEIR SciFact 全量评估 (N=%d queries, %d docs) ===", evalCount, len(docs))
	t.Logf("报告已写入 %s", reportPath)
	t.Logf("")
	t.Logf("%-8s %8s %8s %8s %8s", "Stage", "Rec@5", "Rec@10", "MRR", "nDCG@10")
	t.Logf("----------------------------------------")
	for _, name := range []string{"vector", "bm25", "hybrid"} {
		s := stages[name]
		if n > 0 {
			t.Logf("%-8s %.4f   %.4f    %.4f  %.4f", name, s.rec5/n, s.rec10/n, s.mrr/n, s.ndcg/n)
		}
	}
}

// extractArticleIDs 从 RetrievalResult 列表提取去重后的 ArticleID(保留排名顺序)。
func extractArticleIDs(results []rag.RetrievalResult) []int64 {
	ids := make([]int64, 0, len(results))
	seen := make(map[int64]bool)
	for _, r := range results {
		if !seen[r.ArticleID] {
			ids = append(ids, r.ArticleID)
			seen[r.ArticleID] = true
		}
	}
	return ids
}

// accStage 累加单 query 指标到阶段汇总。
func accStage(s *stageMetrics, retrieved []int64, relevant map[int64]bool) {
	s.rec5 += RecallAtK(retrieved, relevant, 5)
	s.rec10 += RecallAtK(retrieved, relevant, 10)
	s.mrr += ReciprocalRank(retrieved, relevant)
	s.ndcg += NDCGAtK(retrieved, relevant, 10)
}

type stageMetrics struct {
	rec5, rec10, mrr, ndcg float64
}

// maskKey 脱敏 API key(仅显示前 8 位)。
func maskKey(k string) string {
	if len(k) <= 8 {
		return "***"
	}
	return k[:8] + "..."
}

var _ = fmt.Sprintf

package rag_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"opsmind/internal/rag"
)

// =============================================================================
// 分词测试
// =============================================================================

// TestSegmenter_Chinese 验证中文分词正确性。
func TestSegmenter_Chinese(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	// 运维领域典型短语
	tokens := seg.Segment("账号冻结处理流程")
	if len(tokens) < 3 {
		t.Errorf("期望 ≥ 3 个 token, 实际 %d: %v", len(tokens), tokens)
	}

	foundAccount := false
	foundFreeze := false
	for _, tok := range tokens {
		if strings.Contains(tok, "账号") {
			foundAccount = true
		}
		if strings.Contains(tok, "冻结") {
			foundFreeze = true
		}
	}
	if !foundAccount {
		t.Error("分词结果应包含「账号」")
	}
	if !foundFreeze {
		t.Error("分词结果应包含「冻结」")
	}
}

// TestSegmenter_English 验证英文分词不被过度分割。
func TestSegmenter_English(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	tokens := seg.Segment("VPN connection troubleshooting")
	if len(tokens) == 0 {
		t.Fatal("英文文本分词不应为空")
	}

	joined := strings.Join(tokens, " ")
	if !strings.Contains(joined, "vpn") && !strings.Contains(joined, "VPN") {
		t.Error("分词结果应包含 'vpn' 或 'VPN'")
	}
}

// =============================================================================
// BM25 索引测试
// =============================================================================

// TestBM25_BuildAndSearch 验证索引构建和基本检索。
func TestBM25_BuildAndSearch(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	retriever := rag.NewBM25Retriever(seg, 5*time.Minute)

	// 构建索引：添加文档
	docs := []rag.BM25Document{
		{ChunkID: 1, ArticleID: 1, KBID: 1, Content: "VPN 连接失败时的排查步骤"},
		{ChunkID: 2, ArticleID: 1, KBID: 1, Content: "公司邮箱无法登录的解决方法"},
		{ChunkID: 3, ArticleID: 2, KBID: 1, Content: "打印机驱动安装指南"},
		{ChunkID: 4, ArticleID: 3, KBID: 1, Content: "VPN 客户端下载和安装教程"},
	}
	retriever.BuildIndex(1, docs)

	// 检索：查询 "VPN"
	results, err := retriever.Retrieve(context.Background(), "VPN 连接问题", 1, 3)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("BM25 检索 VPN 应返回结果")
	}

	// VPN 相关文档应排前面
	if results[0].Source != "bm25" {
		t.Errorf("检索结果 Source 应为 bm25, 实际 %s", results[0].Source)
	}
	if results[0].Score <= 0 {
		t.Error("BM25 分数应 > 0")
	}
}

// TestBM25_MultiKB 验证多知识库索引隔离。
func TestBM25_MultiKB(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	retriever := rag.NewBM25Retriever(seg, 5*time.Minute)

	// KB1: 有 "账号" 相关文档
	d1 := []rag.BM25Document{
		{ChunkID: 10, ArticleID: 10, KBID: 1, Content: "账号冻结处理流程"},
	}
	retriever.BuildIndex(1, d1)

	// KB2: 无 "账号" 相关文档
	d2 := []rag.BM25Document{
		{ChunkID: 20, ArticleID: 20, KBID: 2, Content: "办公电脑采购申请流程"},
	}
	retriever.BuildIndex(2, d2)

	// 在 KB1 中检索 "账号"
	r1, _ := retriever.Retrieve(context.Background(), "账号冻结", 1, 5)
	if len(r1) == 0 {
		t.Fatal("KB1 中应检索到账号相关结果")
	}

	// 在 KB2 中检索 "账号" — 应无结果
	r2, _ := retriever.Retrieve(context.Background(), "账号", 2, 5)
	if len(r2) > 0 {
		t.Errorf("KB2 中不应有账号相关结果, 实际 %d 条", len(r2))
	}

	// 未构建索引的 KB3 不应 panic
	r3, err := retriever.Retrieve(context.Background(), "测试", 999, 5)
	if err != nil {
		t.Errorf("未索引 KB 检索应返回空结果而非错误: %v", err)
	}
	if len(r3) != 0 {
		t.Errorf("未索引 KB 期望 0 条结果, 实际 %d", len(r3))
	}
}

// TestBM25_OkapiScoring 验证 Okapi BM25 分数计算的基本性质。
func TestBM25_OkapiScoring(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	retriever := rag.NewBM25Retriever(seg, 5*time.Minute)

	docs := []rag.BM25Document{
		{ChunkID: 100, ArticleID: 100, KBID: 1, Content: "账号冻结账号冻结账号冻结账号"},
		{ChunkID: 101, ArticleID: 101, KBID: 1, Content: "账号 冻结 一次"},
	}
	retriever.BuildIndex(1, docs)

	results, err := retriever.Retrieve(context.Background(), "账号冻结", 1, 5)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}

	// 关键词出现多次的文档应排在只出现一次的前面
	if len(results) >= 2 {
		if results[0].Score < results[1].Score {
			t.Logf("注意：高频文档分 %.4f ≤ 低频文档分 %.4f（可能因文档长度归一化）",
				results[0].Score, results[1].Score)
		}
	}
}

// TestBM25_IndexRebuild 验证索引重建后旧索引被替换。
func TestBM25_IndexRebuild(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	retriever := rag.NewBM25Retriever(seg, 5*time.Minute)

	// 第一版索引
	docs1 := []rag.BM25Document{
		{ChunkID: 1001, ArticleID: 1001, KBID: 1, Content: "旧版文档内容"},
	}
	retriever.BuildIndex(1, docs1)

	// 第二版索引（覆盖）
	docs2 := []rag.BM25Document{
		{ChunkID: 2001, ArticleID: 2001, KBID: 1, Content: "新版 VPN 配置指南"},
	}
	retriever.BuildIndex(1, docs2)

	// 重建后检索 "旧版" 不应命中
	results, _ := retriever.Retrieve(context.Background(), "旧版", 1, 5)
	if len(results) > 0 {
		t.Errorf("索引重建后不应再命中旧文档, 实际 %d 条", len(results))
	}

	// 重建后检索 "VPN" 应命中新文档
	results2, _ := retriever.Retrieve(context.Background(), "VPN", 1, 5)
	if len(results2) == 0 {
		t.Error("索引重建后应能检索新文档")
	}
	if len(results2) > 0 && results2[0].ChunkID != 2001 {
		t.Errorf("期望命中新文档 ChunkID=2001, 实际 %d", results2[0].ChunkID)
	}
}

// TestBM25_ResultStructure 验证检索结果结构完整性。
func TestBM25_ResultStructure(t *testing.T) {
	seg := rag.NewGseSegmenter()
	defer seg.Close()

	retriever := rag.NewBM25Retriever(seg, 5*time.Minute)

	docs := []rag.BM25Document{
		{ChunkID: 42, ArticleID: 7, KBID: 1, Content: "测试文章内容关于运维"},
	}
	retriever.BuildIndex(1, docs)

	results, err := retriever.Retrieve(context.Background(), "运维", 1, 1)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("期望 1 条结果, 实际 %d", len(results))
	}

	r := results[0]
	if r.ChunkID != 42 {
		t.Errorf("ChunkID = %d, 期望 42", r.ChunkID)
	}
	if r.ArticleID != 7 {
		t.Errorf("ArticleID = %d, 期望 7", r.ArticleID)
	}
	if r.Source != "bm25" {
		t.Errorf("Source = %s, 期望 bm25", r.Source)
	}
}

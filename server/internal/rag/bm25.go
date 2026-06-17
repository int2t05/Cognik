// Package rag 实现自建 RAG 检索引擎。
//
// bm25.go 实现 BM25 倒排索引 + 中文分词检索。
//
// 架构设计：
//   - Segmenter 接口：抽象分词器，方便替换不同分词库
//   - BM25Index：单知识库的倒排索引 + Okapi BM25 评分
//   - BM25Retriever：管理多知识库索引的生命周期（懒加载 + TTL）
//
// 为什么使用 BM25 而非 TF-IDF：
// BM25 的文档长度归一化（参数 b）比 TF-IDF 更健壮，
// 长文档不会因为包含更多词条而获得不公平的高分。
// Okapi 变体是学术界和工业界公认的稀疏检索基线。
//
// 为什么懒加载 + TTL（ADR-V2-004）：
// BM25 索引构建需要遍历知识库全量分块并分词，
// 对于大知识库可能耗时数秒。懒加载（首次检索时构建）
// 避免启动时全量加载，TTL 保证索引定期刷新。
package rag

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-ego/gse"
)

// =============================================================================
// Segmenter 接口 + gse 实现
// =============================================================================

// Segmenter 定义分词器接口。
type Segmenter interface {
	// Segment 对文本分词，返回 token 列表。
	Segment(text string) []string
	// Close 释放分词器资源。
	Close()
}

// GseSegmenter 使用 gse 库的中文分词器。
//
// 为什么选择 gse：
// gse 是纯 Go 实现，无 CGO 依赖，交叉编译友好。
// 支持中文、英文、日文等多语言分词，
// 内置词典覆盖常见中文词汇，MVP 阶段无需额外训练。
//
// 线程安全：Segment 方法通过 mu 保护 gse 内部状态（HMM 标记器在 Cut 时修改内部结构）。
type GseSegmenter struct {
	seg gse.Segmenter
	mu  sync.Mutex
}

// NewGseSegmenter 创建 gse 分词器实例。
//
// 加载内置中文词典，首次调用约需 100-200ms 加载词典文件。
func NewGseSegmenter() *GseSegmenter {
	s := &GseSegmenter{}
	if err := s.seg.LoadDict(); err != nil {
		slog.Warn("gse 词典加载失败，分词将回退到字符级切分，BM25 检索质量下降", "error", err)
	}
	return s
}

// Segment 对文本分词（线程安全）。
func (s *GseSegmenter) Segment(text string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seg.Cut(text, true) // true = 启用 HMM 分词
}

// Close 释放分词器资源（gse 无显式释放需求）。
func (s *GseSegmenter) Close() {}

// =============================================================================
// BM25 常量
// =============================================================================

const (
	// bm25K1 BM25 词频饱和参数。
	// k1=1.5 是 Okapi 论文推荐值，平衡了词频的饱和速度和区分度。
	bm25K1 = 1.5

	// bm25B BM25 文档长度归一化参数。
	// b=0.75 是 Okapi 论文推荐值，给中等长度文档更多权重。
	bm25B = 0.75

	// bm25DefaultTopK topK <= 0 时的默认返回数。
	bm25DefaultTopK = 10

	// bm25LargeDocCount 文档超量阈值（超过时打 warn）。
	bm25LargeDocCount = 100_000
)

// =============================================================================
// BM25Document — 索引文档
// =============================================================================

// BM25Document 待索引的文档（知识分块）。
type BM25Document struct {
	ChunkID    int64  `json:"chunk_id"`
	ArticleID  int64  `json:"article_id"`
	KBID       int64  `json:"kb_id"`
	Content    string `json:"content"`
	ChunkIndex int    `json:"chunk_index"`
}

// =============================================================================
// BM25Index — 倒排索引 + 打分
// =============================================================================

// bm25Posting 倒排索引的单个命中记录。
type bm25Posting struct {
	ChunkID  int64 // 分块 ID
	TermFreq int   // 该词在该文档中的出现次数
}

// BM25Index 单知识库的倒排索引。
//
// 为什么用 map 而非 B-Tree：
// MVP 阶段知识库规模有限（< 10万篇），
// Go 原生 map 的 O(1) 查询性能足够。
//
// 超过 10 万篇时 recordLargeIndex 会打 warn 日志，
// 后续可考虑分片索引或 Roaring Bitmap 压缩 posting list。
type BM25Index struct {
	// 倒排索引：token → posting list
	inverted map[string][]bm25Posting
	// 文档长度：ChunkID → rune count
	// TODO(rag/bm25): rune 计数对中英文长度拉伸不均（中文 1-2 字符=1 词，英文 5-6 字符=1 词），
	// 应改用 tokenizer 输出的词数作为文档长度，BM25 的 b 参数才能正确归一化。
	docLens map[int64]int
	// 文档元数据：ChunkID → {ArticleID, ChunkIndex, Content}
	docMeta map[int64]BM25Document
	// 平均文档长度
	avgdl float64
	// 文档总数
	docCount int
}

// newBM25Index 创建空的 BM25 索引。
func newBM25Index() *BM25Index {
	return &BM25Index{
		inverted: make(map[string][]bm25Posting),
		docLens:  make(map[int64]int),
		docMeta:  make(map[int64]BM25Document),
	}
}

// =============================================================================
// BM25Retriever — 多知识库管理
// =============================================================================

// BM25Retriever 实现 Retriever 接口，管理多知识库的 BM25 索引。
//
// 每个知识库有独立的索引，通过懒加载按需构建。
// 索引在 TTL 后自动失效，下次检索时重建。
type BM25Retriever struct {
	segmenter Segmenter
	ttl       time.Duration

	mu       sync.RWMutex
	indexes  map[int64]*bm25Entry // kbID → 索引条目
	building map[int64]bool       // kbID → 是否正在构建中（避免并发重复构建）
}

// bm25Entry 单个知识库的 BM25 索引及其元数据。
type bm25Entry struct {
	index     *BM25Index
	documents []BM25Document // 保存原始文档用于重建
	builtAt   time.Time
}

// NewBM25Retriever 创建 BM25Retriever 实例。
//
// ttl 为索引自动过期时间。设 0 禁用自动过期，索引永久有效。
func NewBM25Retriever(seg Segmenter, ttl time.Duration) *BM25Retriever {
	return &BM25Retriever{
		segmenter: seg,
		ttl:       ttl,
		indexes:   make(map[int64]*bm25Entry),
		building:  make(map[int64]bool),
	}
}

// BuildIndex 为知识库构建（或重建）BM25 索引。
//
// 构建期间该 kbID 的并发 BuildIndex 调用会被跳过。
// 调用方（如 Processor）应在自己的 goroutine 中调用此方法避免阻塞。
func (r *BM25Retriever) BuildIndex(kbID int64, docs []BM25Document) {
	r.mu.Lock()
	if r.building[kbID] {
		r.mu.Unlock()
		return
	}
	r.building[kbID] = true
	r.mu.Unlock()

	// 保障 building 标志位一定被清除，防止 panic/OOM 导致永久阻塞
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("BM25 索引构建 panic，已清除 building 标志位", "kb_id", kbID, "panic", rec)
		}
		r.mu.Lock()
		delete(r.building, kbID)
		r.mu.Unlock()
	}()

	idx := r.buildIndex(docs)

	r.mu.Lock()
	r.indexes[kbID] = &bm25Entry{
		index:     idx,
		documents: docs,
		builtAt:   time.Now(),
	}
	r.mu.Unlock()

	slog.Info("BM25 索引构建完成", "kb_id", kbID, "docs", idx.docCount)
}

// buildIndex 构建 BM25 倒排索引的内部实现。
func (r *BM25Retriever) buildIndex(docs []BM25Document) *BM25Index {
	idx := newBM25Index()
	if len(docs) == 0 {
		return idx
	}

	idx.docCount = len(docs)
	var totalLen int

	for _, doc := range docs {
		idx.docMeta[doc.ChunkID] = doc
		tokens := r.segmenter.Segment(doc.Content)
		docLen := utf8.RuneCountInString(doc.Content)
		idx.docLens[doc.ChunkID] = docLen
		totalLen += docLen

		// 统计词频，按 ChunkID 分组
		tf := make(map[string]int)
		for _, tok := range tokens {
			if isValidToken(tok) {
				tf[tok]++
			}
		}

		// 写入倒排索引
		for tok, freq := range tf {
			idx.inverted[tok] = append(idx.inverted[tok], bm25Posting{
				ChunkID:  doc.ChunkID,
				TermFreq: freq,
			})
		}
	}

	if idx.docCount > 0 {
		idx.avgdl = float64(totalLen) / float64(idx.docCount)
	}

	recordLargeIndex(idx)
	return idx
}

// Retrieve 执行 BM25 检索，返回 topK 个结果。
//
// 如果知识库索引不存在或已过期，返回空结果（不报错）。
func (r *BM25Retriever) Retrieve(ctx context.Context, query string, kbID int64, topK int) ([]RetrievalResult, error) {
	if topK <= 0 {
		topK = bm25DefaultTopK
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	entry, exists := r.indexes[kbID]
	r.mu.RUnlock()

	if !exists || entry == nil || entry.index.docCount == 0 {
		return nil, nil
	}

	// TTL 检查：过期时自动重建索引
	if r.ttl > 0 && time.Since(entry.builtAt) > r.ttl {
		entry = r.tryRefreshIndex(kbID)
		if entry == nil || entry.index.docCount == 0 {
			return nil, nil
		}
	}

	// 对查询分词并过滤低质量 token
	queryTokens := r.segmenter.Segment(query)
	filtered := make([]string, 0, len(queryTokens))
	for _, tok := range queryTokens {
		if isValidToken(tok) {
			filtered = append(filtered, tok)
		}
	}

	// 计算每个文档的 BM25 分数
	scores := r.scoreQuery(entry.index, filtered)

	// 按分数降序排列
	type scoredDoc struct {
		chunkID int64
		score   float64
	}
	var sorted []scoredDoc
	for chunkID, score := range scores {
		if score > 0 {
			sorted = append(sorted, scoredDoc{chunkID: chunkID, score: score})
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	// 截取 topK
	if len(sorted) > topK {
		sorted = sorted[:topK]
	}

	// 转换为 RetrievalResult
	results := make([]RetrievalResult, 0, len(sorted))
	for _, s := range sorted {
		meta := entry.index.docMeta[s.chunkID]
		results = append(results, RetrievalResult{
			ChunkID:    s.chunkID,
			ArticleID:  meta.ArticleID,
			Content:    meta.Content,
			Score:      s.score,
			Source:     "bm25",
			ChunkIndex: meta.ChunkIndex,
		})
	}
	return results, nil
}

// tryRefreshIndex 在写锁保护下重建过期的 BM25 索引。
func (r *BM25Retriever) tryRefreshIndex(kbID int64) *bm25Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, exists := r.indexes[kbID]
	if !exists || e == nil || time.Since(e.builtAt) <= r.ttl {
		return e
	}

	if len(e.documents) > 0 && !r.building[kbID] {
		r.building[kbID] = true
		idx := r.buildIndex(e.documents)
		r.indexes[kbID] = &bm25Entry{
			index:     idx,
			documents: e.documents,
			builtAt:   time.Now(),
		}
		r.building[kbID] = false
	}
	return r.indexes[kbID]
}

// scoreQuery 计算查询与所有文档的 BM25 分数。
func (r *BM25Retriever) scoreQuery(idx *BM25Index, queryTokens []string) map[int64]float64 {
	scores := make(map[int64]float64)
	N := float64(idx.docCount)

	// 对每个查询 token 累加分数
	seenTokens := make(map[string]bool)
	for _, token := range queryTokens {
		if seenTokens[token] {
			continue
		}
		seenTokens[token] = true

		postings, exists := idx.inverted[token]
		if !exists {
			continue
		}

		// IDF = ln((N - n + 0.5) / (n + 0.5) + 1)
		n := float64(len(postings))
		idf := math.Log((N-n+0.5)/(n+0.5) + 1.0)

		for _, p := range postings {
			docLen := float64(idx.docLens[p.ChunkID])

			// 防止 avgdl=0 导致除零
			avgdl := idx.avgdl
			if avgdl == 0 {
				avgdl = 1
			}

			// tf_norm = f * (k1 + 1) / (f + k1 * (1 - b + b * dl/avgdl))
			f := float64(p.TermFreq)
			normFactor := bm25K1 * (1 - bm25B + bm25B*docLen/avgdl)
			tfNorm := (f * (bm25K1 + 1)) / (f + normFactor)

			scores[p.ChunkID] += idf * tfNorm
		}
	}
	return scores
}

// =============================================================================
// token 过滤
// =============================================================================

// isValidToken 判断 token 是否有效。
//
// 过滤规则：空串、纯空白、纯标点符号、单字节 token（如英文单个字母）。
// 中文单字保留（如"药"、"税"在特定语境中可能有关键检索价值）。
func isValidToken(tok string) bool {
	if tok == "" {
		return false
	}
	// 单字节 token 通常无检索价值（英文单字母）
	if len(tok) == 1 && tok[0] < 128 {
		return false
	}
	// 纯空白
	if strings.TrimSpace(tok) == "" {
		return false
	}
	// 纯标点/符号
	allPunct := true
	for _, r := range tok {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) && !unicode.IsSpace(r) {
			allPunct = false
			break
		}
	}
	return !allPunct
}

// recordLargeIndex 文档超量时记录 warn 日志。
func recordLargeIndex(idx *BM25Index) {
	if idx.docCount > bm25LargeDocCount {
		slog.Warn("BM25 索引文档数超阈值，内存压力升高",
			"doc_count", idx.docCount,
			"threshold", bm25LargeDocCount,
		)
	}
}

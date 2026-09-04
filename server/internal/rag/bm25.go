// Package rag 实现自建 RAG 检索引擎。
//
// bm25.go 实现 BM25 倒排索引 + 中文分词检索，含单知识库索引懒加载与 TTL 刷新。
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

// GseSegmenter 使用 gse 库的中文分词器，纯 Go 无 CGO 依赖。
// Segment 线程安全（HMM 标记器在 Cut 时修改内部状态，需 mu 保护）。
type GseSegmenter struct {
	seg        gse.Segmenter
	mu         sync.Mutex
	dictLoaded bool // 词典是否加载成功，未加载时回退到字符级切分
}

// NewGseSegmenter 创建 gse 分词器实例，首次加载词典约 100-200ms。
func NewGseSegmenter() *GseSegmenter {
	s := &GseSegmenter{}
	if err := s.seg.LoadDict(); err != nil {
		slog.Warn("gse 词典加载失败，分词将回退到字符级切分，BM25 检索质量下降", "error", err)
	} else {
		s.dictLoaded = true
	}
	return s
}

// Segment 对文本分词（线程安全）。词典加载失败时回退字符级切分，确保 BM25 仍可返回结果。
func (s *GseSegmenter) Segment(text string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dictLoaded {
		return s.seg.Cut(text, true) // true = 启用 HMM 分词
	}
	// 回退：字符级切分（isValidToken 会过滤掉无效单字符）
	tokens := make([]string, 0)
	for _, r := range text {
		tokens = append(tokens, string(r))
	}
	return tokens
}

// Close 释放分词器资源（gse 无显式释放需求）。
func (s *GseSegmenter) Close() {}

// Loaded 返回词典是否加载成功，供调用方判断检索质量。
func (s *GseSegmenter) Loaded() bool {
	return s.dictLoaded
}

// =============================================================================
// BM25 常量
// =============================================================================

const (
	// bm25K1 BM25 词频饱和参数，Okapi 推荐值 1.5。
	bm25K1 = 1.5

	// bm25B BM25 文档长度归一化参数，Okapi 推荐值 0.75。
	bm25B = 0.75

	// bm25TagBoost 标签词频倍增系数，标签匹配权重放大（标签词加入 tf 但不计入 docLen）。
	bm25TagBoost = 3

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
	ChunkID    int64    `json:"chunk_id"`
	ArticleID  int64    `json:"article_id"`
	KBID       int64    `json:"kb_id"`
	Content    string   `json:"content"`
	ChunkIndex int      `json:"chunk_index"`
	Tags       []string `json:"tags"`  // 文章标签，作为 BM25 关键词补充索引
	Title      string   `json:"title"` // 文章标题，enriched text 重复两次增加权重
	Source     string   `json:"source"` // 来源类型
}

// =============================================================================
// BM25Index — 倒排索引 + 打分
// =============================================================================

// bm25Posting 倒排索引的单个命中记录。
type bm25Posting struct {
	ChunkID  int64 // 分块 ID
	TermFreq int   // 该词在该文档中的出现次数
}

// BM25Index 单知识库的倒排索引。超 10 万篇时 recordLargeIndex 打 warn。
type BM25Index struct {
	// 倒排索引：token → posting list
	inverted map[string][]bm25Posting
	// 文档长度：用 token 数而非 rune 计数，避免中英文长度拉伸导致 b 参数归一化失效
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

// BM25Retriever 实现 Retriever 接口，管理多知识库 BM25 索引的懒加载与 TTL 刷新。
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

// NewBM25Retriever 创建 BM25Retriever 实例。ttl 为索引过期时间，0 禁用自动过期。
func NewBM25Retriever(seg Segmenter, ttl time.Duration) *BM25Retriever {
	return &BM25Retriever{
		segmenter: seg,
		ttl:       ttl,
		indexes:   make(map[int64]*bm25Entry),
		building:  make(map[int64]bool),
	}
}

// BuildIndex 为知识库构建（或重建）BM25 索引。构建期间该 kbID 的并发调用被跳过。
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

		// BM25 Enriched Texts：title×2 + tags + content 构造富文本索引（参考 Open WebUI）
		// title 重复两次增加 BM25 词频权重，提升标题关键词召回率
		enriched := doc.Content
		if doc.Title != "" {
			enriched = doc.Title + " " + doc.Title + " " + enriched
		}
		tokens := r.segmenter.Segment(enriched)

		// 统计词频 + 有效词数（token 数作文档长度，见 docLens 注释）
		tf := make(map[string]int)
		docLen := 0
		for _, tok := range tokens {
			if isValidToken(tok) {
				tf[tok]++
				docLen++
			}
		}

		// 标签关键词补充：标签词加入 tf 但不计入 docLen，乘以 bm25TagBoost 放大权重
		for _, tag := range doc.Tags {
			for _, tok := range r.segmenter.Segment(tag) {
				if isValidToken(tok) {
					tf[tok] += bm25TagBoost
				}
			}
		}

		idx.docLens[doc.ChunkID] = docLen
		totalLen += docLen

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

// Retrieve 执行 BM25 检索，返回 topK 个结果。索引不存在或已过期时返回空结果（不报错）。
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

	scores := r.scoreQuery(entry.index, filtered)

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

	if len(sorted) > topK {
		sorted = sorted[:topK]
	}

	results := make([]RetrievalResult, 0, len(sorted))
	for _, s := range sorted {
		meta := entry.index.docMeta[s.chunkID]
		results = append(results, RetrievalResult{
			ChunkID:        s.chunkID,
			ArticleID:      meta.ArticleID,
			Content:        meta.Content,
			Score:          s.score,
			RawCosineScore: 0, // BM25 路径无向量余弦
			Source:         "bm25",
			ChunkIndex:     meta.ChunkIndex,
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
		func() {
			defer func() {
				r.building[kbID] = false
				if rec := recover(); rec != nil {
					slog.Error("BM25 tryRefreshIndex panic 已恢复", "kb_id", kbID, "panic", rec)
				}
			}()
			idx := r.buildIndex(e.documents)
			r.indexes[kbID] = &bm25Entry{
				index:     idx,
				documents: e.documents,
				builtAt:   time.Now(),
			}
		}()
	}
	return r.indexes[kbID]
}

// scoreQuery 计算查询与所有文档的 BM25 分数。
func (r *BM25Retriever) scoreQuery(idx *BM25Index, queryTokens []string) map[int64]float64 {
	scores := make(map[int64]float64)
	N := float64(idx.docCount)

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

// isValidToken 判断 token 是否有效，过滤空串、纯空白、纯标点、单字节 token，保留中文单字。
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

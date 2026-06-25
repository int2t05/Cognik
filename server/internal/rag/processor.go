// Package rag 实现自建 RAG 检索引擎。
//
// processor.go 实现文档异步处理管线。
//
// 为什么使用 goroutine pool 而非直接同步处理：
// 文档处理（解析→分块→embedding→pgvector 写入）可能耗时数十秒甚至数分钟，
// 同步处理会阻塞 HTTP 请求。goroutine pool 模式将耗时任务异步化，
// 客户端通过 process_status 轮询进度。
//
// 优雅关闭设计：
// Stop() 通过 stopped 标志位 + close(taskCh) 双重防护，
// 两次 Stop 调用安全（幂等），Submit 在 Stop 后返回错误。
package rag

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"opsmind/internal/adapter"
)

// defaultTaskTimeout 单个任务最大处理时长（5 分钟），与 embedding HTTP 超时一致。
const defaultTaskTimeout = 5 * time.Minute

// =============================================================================
// ProcessTask — 处理任务
// =============================================================================

// ProcessTask 单个文档处理任务。
//
// 支持两种来源：
//   - MinIO 路径：设置 Bucket/Key/FileType，worker 自动下载并解析
//   - 纯文本：设置 Content，worker 直接分块（手动创建的文章）
//
// EmbeddingModel 为空时使用全局默认模型（回退行为）。
type ProcessTask struct {
	ArticleID      int64                                          `json:"article_id"`
	KBID           int64                                          `json:"kb_id"`
	Content        string                                         `json:"content"`
	Bucket         string                                         `json:"bucket"`
	Key            string                                         `json:"key"`
	FileType       string                                         `json:"file_type"`
	EmbeddingModel string                                         `json:"embedding_model"` // KB 绑定模型，空则回退全局默认
	OnStatusChange func(articleID int64, status, errMsg string) `json:"-"`
	OnMetrics      func(articleID int64, wordCount, chunkCount int) `json:"-"`
}

// =============================================================================
// Processor — 异步文档处理器
// =============================================================================

// Processor 管理文档处理的 goroutine pool。
type Processor struct {
	parser   DocumentParser
	chunker  TextChunker
	embedder TextEmbedder
	store    adapter.VectorStore
	storage  adapter.StorageClient

	taskCh   chan ProcessTask
	poolSize int
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc

	stopped  atomic.Bool   // Stop 幂等防护
	closeOnce sync.Once    // taskCh 安全关闭
}

// NewProcessor 创建文档处理器实例。
//
// storage 可以为 nil（MinIO 不可用时自动降级到 Content 模式）。
// poolSize 为 worker goroutine 数量，建议 2-4。
func NewProcessor(parser DocumentParser, chunker TextChunker, embedder TextEmbedder, store adapter.VectorStore, storage adapter.StorageClient, poolSize int) *Processor {
	if poolSize <= 0 {
		poolSize = 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Processor{
		parser:   parser,
		chunker:  chunker,
		embedder: embedder,
		store:    store,
		storage:  storage,
		taskCh:   make(chan ProcessTask, 100),
		poolSize: poolSize,
		ctx:      ctx,
		cancel:   cancel,
	}

	for i := 0; i < poolSize; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	return p
}

// Submit 提交处理任务（非阻塞）。Stop 后或队列满时返回错误。
func (p *Processor) Submit(task ProcessTask) (err error) {
	notifyFailed := func(msg string) {
		if task.OnStatusChange != nil {
			task.OnStatusChange(task.ArticleID, "failed", msg)
		}
	}
	if p.stopped.Load() {
		notifyFailed("处理器已关闭")
		return fmt.Errorf("处理器已关闭")
	}
	defer func() {
		if r := recover(); r != nil {
			notifyFailed("处理器已关闭")
			err = fmt.Errorf("处理器已关闭")
		}
	}()
	select {
	case p.taskCh <- task:
		return nil
	default:
		notifyFailed("处理队列已满")
		return fmt.Errorf("处理队列已满")
	}
}

// Stop 优雅关闭处理器（幂等，可重复调用）。
func (p *Processor) Stop() {
	if !p.stopped.CompareAndSwap(false, true) {
		return // 已停止，幂等返回
	}
	p.cancel()
	p.closeOnce.Do(func() { close(p.taskCh) })
	p.wg.Wait()
}

// worker 处理任务循环。
//
// 内置 panic recovery，崩溃后自动恢复继续处理后续任务。
// 每个任务派生带独立超时的子 context。
func (p *Processor) worker(id int) {
	defer p.wg.Done()

	for task := range p.taskCh {
		p.processWithRecovery(id, task)
	}
}

// processWithRecovery 带 panic recovery 的任务处理包装。
func (p *Processor) processWithRecovery(workerID int, task ProcessTask) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("文档处理 worker panic，已恢复",
				"worker_id", workerID,
				"article_id", task.ArticleID,
				"panic", r,
			)
			p.updateStatus(task, "failed", fmt.Sprintf("内部错误：%v", r))
		}
	}()

	// 派生带超时的子 context，防止单个任务卡住永久占用 worker
	ctx, cancel := context.WithTimeout(p.ctx, defaultTaskTimeout)
	defer cancel()

	p.processTask(ctx, task)
}

// chunkWithHash 分块文本及其 SHA256 哈希。
type chunkWithHash struct {
	text string
	hash string
}

// resolveContent 获取任务正文——MinIO 路径下载解析，或直接用纯文本。
func (p *Processor) resolveContent(ctx context.Context, task ProcessTask) (string, error) {
	if task.Bucket == "" || task.Key == "" {
		return task.Content, nil
	}
	if p.storage == nil {
		return "", fmt.Errorf("StorageClient 未初始化")
	}
	reader, err := p.storage.Download(ctx, task.Bucket, task.Key)
	if err != nil {
		return "", fmt.Errorf("从 MinIO 下载文件失败: %w", err)
	}
	defer reader.Close()

	fileType := task.FileType
	if fileType == "" {
		if idx := strings.LastIndex(task.Key, "."); idx >= 0 {
			fileType = task.Key[idx+1:]
		}
	}
	return p.parser.Parse(reader, fileType)
}

// loadOldEmbeddings 查询旧分块 snapshot，构建 hash→向量映射以复用未变 chunk。
func (p *Processor) loadOldEmbeddings(ctx context.Context, articleID int64) map[string][]float32 {
	old := map[string][]float32{}
	snapshots, err := p.store.GetChunkSnapshots(ctx, articleID)
	if err != nil {
		slog.Warn("查询旧分块快照失败，回退到全量 embedding", "article_id", articleID, "error", err)
		return old
	}
	for _, ss := range snapshots {
		if ss.ChunkHash == "" || ss.EmbeddingText == "" {
			continue
		}
		emb, err := adapter.ParsePgVectorText(ss.EmbeddingText)
		if err != nil {
			slog.Warn("解析旧 embedding 失败，跳过该分块", "article_id", articleID, "chunk_hash", ss.ChunkHash, "error", err)
			continue
		}
		old[ss.ChunkHash] = emb
	}
	return old
}

// computeHashes 为分块计算跨文章唯一的 SHA256 哈希。
func computeHashes(articleID int64, chunks []string) []chunkWithHash {
	result := make([]chunkWithHash, len(chunks))
	for i, c := range chunks {
		result[i] = chunkWithHash{text: c, hash: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s", articleID, c))))}
	}
	return result
}

// processTask 处理单个文档的完整流程：获取正文 → 分块 → 增量 embedding → 写入 pgvector。
func (p *Processor) processTask(ctx context.Context, task ProcessTask) {
	articleID := task.ArticleID

	// 1. 获取正文
	p.updateStatus(task, "parsing", "")
	content, err := p.resolveContent(ctx, task)
	if err != nil {
		p.updateStatus(task, "failed", err.Error())
		return
	}
	if strings.TrimSpace(content) == "" {
		p.updateStatus(task, "failed", "文档内容为空")
		return
	}

	// 2. 分块
	if ctx.Err() != nil {
		p.updateStatus(task, "failed", "任务已取消: "+ctx.Err().Error())
		return
	}
	p.updateStatus(task, "chunking", "")
	chunks := p.chunker.Split(content)
	if len(chunks) == 0 {
		p.updateStatus(task, "failed", "分块结果为空")
		return
	}
	if task.OnMetrics != nil {
		task.OnMetrics(articleID, len([]rune(content)), len(chunks))
	}

	// 3. 增量比对：计算 hash → 分离需 embedding 的变更分块
	oldEmbeddings := p.loadOldEmbeddings(ctx, articleID)
	allChunks := computeHashes(articleID, chunks)

	var changedIndices []int
	var changedTexts []string
	for i, ch := range allChunks {
		if _, ok := oldEmbeddings[ch.hash]; ok {
			continue
		}
		changedIndices = append(changedIndices, i)
		changedTexts = append(changedTexts, ch.text)
	}

	// 4. Embedding（仅变更分块）
	var newVectors [][]float32
	if len(changedTexts) > 0 {
		if ctx.Err() != nil {
			p.updateStatus(task, "failed", "任务已取消: "+ctx.Err().Error())
			return
		}
		p.updateStatus(task, "embedding", "")
		newVectors, _, err = p.embedder.Embed(ctx, changedTexts, task.EmbeddingModel)
		if err != nil {
			p.updateStatus(task, "failed", fmt.Sprintf("embedding 失败: %v", err))
			return
		}
		if len(newVectors) != len(changedTexts) {
			p.updateStatus(task, "failed", fmt.Sprintf("embedding 数量不匹配: 期望 %d, 实际 %d", len(changedTexts), len(newVectors)))
			return
		}
		slog.Debug("增量 embedding", "article_id", articleID, "total", len(chunks), "changed", len(changedTexts), "reused", len(chunks)-len(changedTexts))
	} else {
		slog.Debug("全部 chunk 未变更，跳过 embedding", "article_id", articleID, "total", len(chunks))
	}

	changedVecMap := make(map[int][]float32, len(changedIndices))
	for j, idx := range changedIndices {
		changedVecMap[idx] = newVectors[j]
	}

	// 5. 写入 pgvector
	p.updateStatus(task, "indexing", "")
	vc := make([]adapter.VectorChunk, len(chunks))
	for i, ch := range allChunks {
		var emb []float32
		if v, ok := changedVecMap[i]; ok {
			emb = v
		} else if v, ok := oldEmbeddings[ch.hash]; ok {
			emb = v
		} else {
			p.updateStatus(task, "failed", fmt.Sprintf("chunk %d (%s) 无可用 embedding", i, ch.hash[:16]))
			return
		}
		vc[i] = adapter.VectorChunk{
			ArticleID: articleID, KBID: task.KBID, Content: ch.text, ChunkIndex: i,
			Embedding: emb, EmbeddingModel: task.EmbeddingModel,
			VectorDimension: len(emb), ChunkHash: ch.hash,
		}
	}
	if err := p.store.ReplaceVectors(ctx, articleID, vc); err != nil {
		p.updateStatus(task, "failed", fmt.Sprintf("写入向量失败: %v", err))
		return
	}
	p.updateStatus(task, "completed", "")
}

// updateStatus 更新处理状态（通过回调）。
func (p *Processor) updateStatus(task ProcessTask, status, errMsg string) {
	if task.OnStatusChange != nil {
		task.OnStatusChange(task.ArticleID, status, errMsg)
	}
}

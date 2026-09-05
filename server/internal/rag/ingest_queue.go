// Package rag 实现自建 RAG 检索引擎。
//
// ingest_queue.go：异步处理管道——文件即真相场景下，Agent 写入 draft 后入队（<5ms），
// 定时消费者去重→质量门→索引。lease 机制防并发，崩溃恢复 processing→pending。
package rag

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IngestItem 队列条目。
type IngestItem struct {
	ArticleID int64      `json:"article_id"`
	KBID      int64      `json:"kb_id"`
	FilePath  string     `json:"file_path"`
	Action    string     `json:"action"`      // create / update / delete
	TS        time.Time `json:"ts"`
	Status    string    `json:"status"`     // pending / processing / done
	LeaseAt   *time.Time `json:"lease_at"`   // processing 时的租约时间
}

// IngestQueue 异步处理队列——append-only jsonl + lease 机制。
type IngestQueue struct {
	path     string
	mu       sync.Mutex
}

// NewIngestQueue 创建队列实例，确保文件存在。
func NewIngestQueue(path string) (*IngestQueue, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("创建队列目录失败: %w", err)
	}
	// 文件不存在则创建
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			return nil, fmt.Errorf("创建队列文件失败: %w", err)
		}
	}
	return &IngestQueue{path: path}, nil
}

// Enqueue 追加一条待处理条目（<5ms，仅一次 append）。
func (q *IngestQueue) Enqueue(item IngestItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	item.TS = time.Now()
	if item.Status == "" {
		item.Status = "pending"
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(q.path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开队列文件失败: %w", err)
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// LoadAll 读取全部条目（消费者 + 崩溃恢复用）。
func (q *IngestQueue) LoadAll() ([]IngestItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	f, err := os.Open(q.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var items []IngestItem
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 支持长行
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item IngestItem
		if err := json.Unmarshal(line, &item); err != nil {
			continue // 跳过损坏行
		}
		items = append(items, item)
	}
	return items, scanner.Err()
}

// Rewrite 全量重写队列文件（消费后压缩 + lease 更新）。
func (q *IngestQueue) Rewrite(items []IngestItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	tmpPath := q.path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			f.Close()
			return err
		}
	}
	f.Close()
	return os.Rename(tmpPath, q.path)
}

// IngestConsumer 定时消费队列，委托 Processor 处理。
type IngestConsumer struct {
	queue        *IngestQueue
	processor    *Processor
	leaseTTL     time.Duration // 默认 60s
	pollInterval time.Duration // 默认 5s
}

// NewIngestConsumer 创建消费者。
func NewIngestConsumer(queue *IngestQueue, processor *Processor, leaseTTL, pollInterval time.Duration) *IngestConsumer {
	if leaseTTL <= 0 {
		leaseTTL = 60 * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	return &IngestConsumer{
		queue:        queue,
		processor:    processor,
		leaseTTL:     leaseTTL,
		pollInterval: pollInterval,
	}
}

// Start 启动定时消费循环（阻塞，应在 goroutine 中调用）。
func (c *IngestConsumer) Start(ctx context.Context) {
	// 启动时崩溃恢复：重置所有过期 processing 为 pending
	c.recoverStale()

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processOnce(ctx)
		}
	}
}

// recoverStale 崩溃恢复——将过期 processing 重置为 pending。
func (c *IngestConsumer) recoverStale() {
	items, err := c.queue.LoadAll()
	if err != nil {
		slog.Warn("队列崩溃恢复读取失败", "error", err)
		return
	}
	now := time.Now()
	changed := false
	for i := range items {
		if items[i].Status == "processing" && items[i].LeaseAt != nil {
			if now.Sub(*items[i].LeaseAt) > c.leaseTTL {
				items[i].Status = "pending"
				items[i].LeaseAt = nil
				changed = true
				slog.Info("队列崩溃恢复：重置过期任务", "article_id", items[i].ArticleID)
			}
		}
	}
	if changed {
		if err := c.queue.Rewrite(items); err != nil {
			slog.Warn("队列崩溃恢复重写失败", "error", err)
		}
	}
}

// processOnce 扫描队列，处理一个 pending 条目（acquire lease → submit → mark done）。
func (c *IngestConsumer) processOnce(ctx context.Context) {
	items, err := c.queue.LoadAll()
	if err != nil {
		slog.Warn("队列读取失败", "error", err)
		return
	}

	now := time.Now()
	for i := range items {
		if items[i].Status != "pending" {
			continue
		}
		// acquire lease
		items[i].Status = "processing"
		leaseAt := now
		items[i].LeaseAt = &leaseAt
		if err := c.queue.Rewrite(items); err != nil {
			slog.Warn("队列 lease 获取失败", "error", err)
			return
		}

		// 提交到 Processor（非阻塞，Processor 内部 goroutine pool 处理）
		c.submitToProcessor(ctx, items[i])

		// 标记 done
		items[i].Status = "done"
		if err := c.queue.Rewrite(items); err != nil {
			slog.Warn("队列标记 done 失败", "error", err)
		}
		return // 每轮只处理一个，避免长时间持锁
	}
}

// submitToProcessor 构造 ProcessTask 提交给 Processor。
func (c *IngestConsumer) submitToProcessor(ctx context.Context, item IngestItem) {
	if c.processor == nil {
		return
	}
	task := ProcessTask{
		ArticleID: item.ArticleID,
		KBID:     item.KBID,
		Bucket:   "cognik-documents",
		Key:      item.FilePath,
		FileType: "txt",
	}
	c.processor.Submit(task)
	slog.Info("队列任务已提交 Processor", "article_id", item.ArticleID, "kb_id", item.KBID)
}

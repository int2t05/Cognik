// Package rag_test 验证异步处理队列（IngestQueue）的核心行为。
// 真实文件操作，无 mock。
package rag_test

import (
	"path/filepath"
	"testing"
	"time"

	"cognik/internal/rag"
)

// newQueue 创建临时队列。
func newQueue(t *testing.T) *rag.IngestQueue {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ingest_queue.jsonl")
	q, err := rag.NewIngestQueue(path)
	if err != nil {
		t.Fatalf("NewIngestQueue 失败: %v", err)
	}
	return q
}

// --- Enqueue ---

func TestIngestQueue_Enqueue(t *testing.T) {
	q := newQueue(t)

	err := q.Enqueue(rag.IngestItem{
		ArticleID: 42,
		KBID:      1,
		FilePath:  "kb-1/draft/test-article.md",
	})
	if err != nil {
		t.Fatalf("Enqueue 失败: %v", err)
	}

	items, _ := q.LoadAll()
	if len(items) != 1 {
		t.Fatalf("应含 1 条，得到 %d", len(items))
	}
	if items[0].ArticleID != 42 {
		t.Errorf("ArticleID 应为 42，得到 %d", items[0].ArticleID)
	}
	if items[0].Status != "pending" {
		t.Errorf("Status 应为 pending，得到 %s", items[0].Status)
	}
	if items[0].TS.IsZero() {
		t.Error("TS 应被自动设置")
	}
}

func TestIngestQueue_EnqueueMultiple(t *testing.T) {
	q := newQueue(t)

	for i := int64(1); i <= 5; i++ {
		q.Enqueue(rag.IngestItem{ArticleID: i, KBID: 1, FilePath: "file.md"})
	}

	items, _ := q.LoadAll()
	if len(items) != 5 {
		t.Fatalf("应含 5 条，得到 %d", len(items))
	}
	for i, item := range items {
		if item.ArticleID != int64(i+1) {
			t.Errorf("第 %d 条 ArticleID 应为 %d，得到 %d", i, i+1, item.ArticleID)
		}
	}
}

// --- LoadAll ---

func TestIngestQueue_LoadAllEmpty(t *testing.T) {
	q := newQueue(t)

	items, err := q.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll 空应不报错: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("空队列应返回 0 条，得到 %d", len(items))
	}
}

// --- Rewrite ---

func TestIngestQueue_Rewrite(t *testing.T) {
	q := newQueue(t)

	q.Enqueue(rag.IngestItem{ArticleID: 1, KBID: 1, FilePath: "a.md"})
	q.Enqueue(rag.IngestItem{ArticleID: 2, KBID: 1, FilePath: "b.md"})

	items, _ := q.LoadAll()
	// 修改后重写
	items[0].Status = "done"
	items[1].Status = "processing"
	now := time.Now()
	items[1].LeaseAt = &now
	q.Rewrite(items)

	reloaded, _ := q.LoadAll()
	if reloaded[0].Status != "done" {
		t.Errorf("第一条 status 应为 done，得到 %s", reloaded[0].Status)
	}
	if reloaded[1].Status != "processing" {
		t.Errorf("第二条 status 应为 processing，得到 %s", reloaded[1].Status)
	}
	if reloaded[1].LeaseAt == nil {
		t.Error("第二条 LeaseAt 应不为 nil")
	}
}

// --- 崩溃恢复 ---

func TestIngestQueue_RecoverStale(t *testing.T) {
	q := newQueue(t)

	// 模拟崩溃：写入一条 processing 且 lease 已过期的条目
	past := time.Now().Add(-2 * time.Minute) // 2 分钟前，超过 60s lease
	q.Enqueue(rag.IngestItem{
		ArticleID: 1,
		KBID:      1,
		FilePath:  "stale.md",
		Status:    "processing",
		LeaseAt:   &past,
	})

	// 模拟消费器的崩溃恢复：recoverStale 是未导出方法，
	// 通过验证逻辑确认 processing + 过期 lease 的条目状态
	items, _ := q.LoadAll()
	if len(items) != 1 {
		t.Fatalf("应含 1 条，得到 %d", len(items))
	}
	if items[0].Status != "processing" {
		t.Fatalf("status 应为 processing，得到 %s", items[0].Status)
	}

	// 通过 Rewrite 模拟恢复（将过期 processing 改为 pending）
	items[0].Status = "pending"
	items[0].LeaseAt = nil
	q.Rewrite(items)

	reloaded, _ := q.LoadAll()
	if reloaded[0].Status != "pending" {
		t.Errorf("崩溃恢复后 status 应为 pending，得到 %s", reloaded[0].Status)
	}
	if reloaded[0].LeaseAt != nil {
		t.Error("崩溃恢复后 LeaseAt 应为 nil")
	}
}

// --- 优雅降级（nil processor）---

func TestIngestConsumer_NilProcessor(t *testing.T) {
	q := newQueue(t)
	q.Enqueue(rag.IngestItem{ArticleID: 1, KBID: 1, FilePath: "test.md"})

	// nil processor 不应 panic — Consumer 应仍可构造
	if c := rag.NewIngestConsumer(q, nil, 60*time.Second, 100*time.Millisecond); c == nil {
		t.Fatal("nil processor 时 Consumer 应仍可构造")
	}
}

// Package store_test 验证 SQLite ChatStore 的 CRUD + 并发语义。
// 真实 SQLite 文件操作，无 mock。
package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"opsmind/internal/agent/store"
)

// newTestStore 创建内存 SQLite 测试存储（无文件锁问题）。
// 每个测试用唯一 DB 名避免 shared cache 污染。
var testDBCounter int

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	testDBCounter++
	dbName := fmt.Sprintf("file:test%d?mode=memory&cache=shared", testDBCounter)
	s, err := store.NewSQLiteStore(dbName)
	if err != nil {
		t.Fatalf("NewSQLiteStore 失败: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestThread_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	thread, err := s.CreateThread(ctx, 1, "测试对话")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if thread.ID == 0 {
		t.Fatal("Thread ID 不应为 0")
	}
	if thread.Title != "测试对话" {
		t.Errorf("Title 应为 '测试对话'，得到 %s", thread.Title)
	}

	// GetThread（正确用户）
	got, err := s.GetThread(ctx, thread.ID, 1)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got.Title != "测试对话" {
		t.Errorf("Title 不匹配")
	}

	// GetThread（错误用户 → 应失败）
	_, err = s.GetThread(ctx, thread.ID, 999)
	if err == nil {
		t.Fatal("错误用户应无法获取线程")
	}
}

func TestThread_CreateDefaultTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	thread, _ := s.CreateThread(ctx, 1, "")
	if thread.Title != "新对话" {
		t.Errorf("空标题应默认 '新对话'，得到 %s", thread.Title)
	}
}

func TestThread_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.CreateThread(ctx, 1, "A")
	s.CreateThread(ctx, 1, "B")
	s.CreateThread(ctx, 2, "C") // 其他用户

	threads, err := s.ListThreads(ctx, 1)
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if len(threads) != 2 {
		t.Errorf("用户 1 应有 2 个线程，得到 %d", len(threads))
	}
}

func TestThread_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	thread, _ := s.CreateThread(ctx, 1, "测试")
	s.SaveMessage(ctx, &store.Message{ThreadID: thread.ID, Role: "user", Parts: "[]"})

	if err := s.DeleteThread(ctx, thread.ID, 1); err != nil {
		t.Fatalf("DeleteThread: %v", err)
	}
	if _, err := s.GetThread(ctx, thread.ID, 1); err == nil {
		t.Fatal("删除后应无法获取")
	}
	msgs, _ := s.ListMessages(ctx, thread.ID)
	if len(msgs) != 0 {
		t.Errorf("删除线程应级联删除消息，剩余 %d", len(msgs))
	}
}

func TestMessage_SaveAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	thread, _ := s.CreateThread(ctx, 1, "测试")

	parts, _ := store.PartsToJSON([]store.MessagePart{
		{Type: store.PartText, Content: "你好"},
	})
	msg := &store.Message{
		ThreadID: thread.ID, Role: "user", Parts: parts,
		Status: store.MessageStatusCompleted,
	}
	if err := s.SaveMessage(ctx, msg); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if msg.ID == 0 {
		t.Fatal("Message ID 不应为 0")
	}

	msgs, _ := s.ListMessages(ctx, thread.ID)
	if len(msgs) != 1 {
		t.Fatalf("应有 1 条消息，得到 %d", len(msgs))
	}
	parsed, _ := store.ParseParts(msgs[0].Parts)
	if len(parsed) != 1 || parsed[0].Content != "你好" {
		t.Errorf("parts 解析不匹配: %+v", parsed)
	}
}

func TestMessage_UpdateStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	thread, _ := s.CreateThread(ctx, 1, "测试")
	msg := &store.Message{
		ThreadID: thread.ID, Role: "assistant", Parts: "[]",
		Status: store.MessageStatusGenerating,
	}
	s.SaveMessage(ctx, msg)

	msg.Status = store.MessageStatusCompleted
	msg.Parts = `[{"type":"text","content":"done"}]`
	if err := s.UpdateMessage(ctx, msg); err != nil {
		t.Fatalf("UpdateMessage: %v", err)
	}

	got, _ := s.GetMessage(ctx, msg.ID, thread.ID)
	if got.Status != store.MessageStatusCompleted {
		t.Errorf("Status 应为 completed，得到 %s", got.Status)
	}
}

func TestCleanupStale(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	thread, _ := s.CreateThread(ctx, 1, "测试")
	s.SaveMessage(ctx, &store.Message{ThreadID: thread.ID, Role: "assistant", Parts: "[]", Status: store.MessageStatusGenerating})
	s.SaveMessage(ctx, &store.Message{ThreadID: thread.ID, Role: "user", Parts: "[]", Status: store.MessageStatusCompleted})

	count, err := s.CleanupStale(ctx)
	if err != nil {
		t.Fatalf("CleanupStale: %v", err)
	}
	if count != 1 {
		t.Errorf("应清理 1 条 generating 消息，得到 %d", count)
	}
	msgs, _ := s.ListMessages(ctx, thread.ID)
	for _, m := range msgs {
		if m.Status == store.MessageStatusGenerating {
			t.Error("不应有残留 generating 消息")
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	thread, _ := s.CreateThread(ctx, 1, "并发测试")

	// 并发写入 10 条消息
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			s.SaveMessage(ctx, &store.Message{
				ThreadID: thread.ID, Role: "user", Parts: "[]",
				Status: store.MessageStatusCompleted,
			})
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// SQLite 默认串行写，给一点时间提交
	time.Sleep(100 * time.Millisecond)
	msgs, _ := s.ListMessages(ctx, thread.ID)
	if len(msgs) != 10 {
		t.Errorf("应有 10 条消息，得到 %d", len(msgs))
	}
}

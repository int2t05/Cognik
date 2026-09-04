// Package agent_test 验证 Agent 记忆存储（FileMemoryStore）的核心行为。
// 真实文件操作，无 mock。
package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cognos/internal/agent/tools"
)

// newMemoryStore 创建临时存储目录的 FileMemoryStore。
func newMemoryStore(t *testing.T) (*tools.FileMemoryStore, string) {
	t.Helper()
	dir := t.TempDir()
	return tools.NewFileMemoryStore(dir, 200), dir
}

// --- Remember ---

func TestMemoryStore_RememberGlobal(t *testing.T) {
	store, dir := newMemoryStore(t)
	ctx := context.Background()

	key, err := store.Remember(ctx, "PG 集群 3 节点流复制，主节点 192.168.1.10", "global", "pg-topology", 8, "")
	if err != nil {
		t.Fatalf("Remember 失败: %v", err)
	}
	if key != "pg-topology" {
		t.Errorf("key 应为 pg-topology，得到 %s", key)
	}

	// 验证文件已写入
	data, err := os.ReadFile(filepath.Join(dir, "memory/global/pg-topology.md"))
	if err != nil {
		t.Fatalf("记忆文件应存在: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: pg-topology") {
		t.Errorf("frontmatter 应含 name，得到:\n%s", content)
	}
	if !strings.Contains(content, "status: active") {
		t.Errorf("frontmatter 应含 status: active，得到:\n%s", content)
	}
	if !strings.Contains(content, "PG 集群 3 节点流复制") {
		t.Errorf("正文应含记忆文本，得到:\n%s", content)
	}

	// 验证 MEMORY.md 索引已更新
	idx, _ := os.ReadFile(filepath.Join(dir, "memory/global/MEMORY.md"))
	if !strings.Contains(string(idx), "pg-topology") {
		t.Errorf("MEMORY.md 应含 pg-topology 索引，得到:\n%s", string(idx))
	}
}

func TestMemoryStore_RememberSession(t *testing.T) {
	store, dir := newMemoryStore(t)
	ctx := context.Background()

	_, err := store.Remember(ctx, "排查 PG CPU 高，确认是 vacuum 未跑", "session", "pg-cpu-diag", 5, "sess-123")
	if err != nil {
		t.Fatalf("Remember session 失败: %v", err)
	}

	// 验证文件写入到 sessions/{id}/ 目录
	path := filepath.Join(dir, "memory/sessions/sess-123/pg-cpu-diag.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("会话记忆文件应存在: %s", path)
	}

	// 验证会话 MEMORY.md
	idx, _ := os.ReadFile(filepath.Join(dir, "memory/sessions/sess-123/MEMORY.md"))
	if !strings.Contains(string(idx), "pg-cpu-diag") {
		t.Errorf("会话 MEMORY.md 应含索引，得到:\n%s", string(idx))
	}
}

// --- Recall ---

func TestMemoryStore_RecallExactMatch(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "PostgreSQL vacuum 需定期检查，上次 CPU 高因 vacuum 未跑", "global", "vacuum-pattern", 7, "")
	store.Remember(ctx, "Redis OOM 排障：检查 maxmemory 配置", "global", "redis-oom", 6, "")

	results, err := store.Recall(ctx, "vacuum", "global", 5, "")
	if err != nil {
		t.Fatalf("Recall 失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("应检索到 vacuum 相关记忆")
	}
	if results[0].Score < 1.0 {
		t.Errorf("精确匹配 score 应为 1.0，得到 %.2f", results[0].Score)
	}
	if !strings.Contains(results[0].Content, "vacuum") {
		t.Errorf("检索结果应含 vacuum，得到: %s", results[0].Content)
	}
}

func TestMemoryStore_RecallNoMatch(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "PostgreSQL 排障", "global", "pg", 5, "")

	results, err := store.Recall(ctx, "kubernetes", "global", 5, "")
	if err != nil {
		t.Fatalf("Recall 失败: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("无匹配应返回空，得到 %d 条", len(results))
	}
}

func TestMemoryStore_RecallSkipsDisabled(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "vacuum 定期检查", "global", "vacuum", 5, "")
	store.Forget(ctx, "global", "vacuum", "")

	results, err := store.Recall(ctx, "vacuum", "global", 5, "")
	if err != nil {
		t.Fatalf("Recall 失败: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("已失效记忆不应被检索到，得到 %d 条", len(results))
	}
}

func TestMemoryStore_RecallDescriptionMatch(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "192.168.1.10 是主节点，流复制 3 节点", "global", "pg-topology", 8, "")

	// query 匹配 description（truncate 后的前 60 字符）而非正文
	results, err := store.Recall(ctx, "192.168", "global", 5, "")
	if err != nil {
		t.Fatalf("Recall 失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("应通过 description 匹配到记忆")
	}
}

// --- Forget ---

func TestMemoryStore_ForgetMarksDisabled(t *testing.T) {
	store, dir := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "待遗忘的记忆", "global", "to-forget", 3, "")
	err := store.Forget(ctx, "global", "to-forget", "")
	if err != nil {
		t.Fatalf("Forget 失败: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory/global/to-forget.md"))
	if !strings.Contains(string(data), "status: disabled") {
		t.Errorf("记忆应标记为 disabled，得到:\n%s", string(data))
	}
}

func TestMemoryStore_ForgetIdempotent(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "记忆", "global", "key", 5, "")
	store.Forget(ctx, "global", "key", "")

	// 二次 forget 不应报错
	err := store.Forget(ctx, "global", "key", "")
	if err != nil {
		t.Errorf("二次 forget 应幂等不报错，得到: %v", err)
	}
}

// --- Update ---

func TestMemoryStore_UpdateOverwrites(t *testing.T) {
	store, dir := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "旧内容", "global", "update-test", 5, "")
	err := store.Update(ctx, "global", "update-test", "新内容：已更新", "")
	if err != nil {
		t.Fatalf("Update 失败: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory/global/update-test.md"))
	content := string(data)
	if strings.Contains(content, "旧内容") {
		t.Errorf("旧内容应被覆盖，得到:\n%s", content)
	}
	if !strings.Contains(content, "新内容：已更新") {
		t.Errorf("应含新内容，得到:\n%s", content)
	}
}

func TestMemoryStore_UpdateNonExistentFails(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	err := store.Update(ctx, "global", "nonexistent", "text", "")
	if err == nil {
		t.Fatal("更新不存在的记忆应报错")
	}
}

// --- List ---

func TestMemoryStore_ListGlobal(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "记忆 A", "global", "key-a", 5, "")
	store.Remember(ctx, "记忆 B", "global", "key-b", 5, "")

	items, err := store.List(ctx, "global", "")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("应列出 2 条记忆，得到 %d", len(items))
	}
	// 按 key 排序
	if items[0].Key != "key-a" {
		t.Errorf("首条应为 key-a，得到 %s", items[0].Key)
	}
}

func TestMemoryStore_ListEmptyScope(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	items, err := store.List(ctx, "global", "")
	if err != nil {
		t.Fatalf("List 空应不报错: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("空 scope 应返回 0 条，得到 %d", len(items))
	}
}

// --- ListSessionEntries ---

func TestMemoryStore_ListSessionEntries(t *testing.T) {
	store, _ := newMemoryStore(t)
	ctx := context.Background()

	store.Remember(ctx, "诊断过程", "session", "diag-1", 5, "sess-42")
	store.Remember(ctx, "检查了 vacuum 状态", "session", "vacuum-check", 5, "sess-42")

	entries, err := store.ListSessionEntries(ctx, "sess-42")
	if err != nil {
		t.Fatalf("ListSessionEntries 失败: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("应返回 2 条会话记忆，得到 %d", len(entries))
	}
	// 验证含正文（不仅仅是 key/description）
	hasContent := false
	for _, e := range entries {
		if strings.Contains(e.Content, "vacuum") {
			hasContent = true
		}
	}
	if !hasContent {
		t.Error("ListSessionEntries 应返回正文内容，而非仅标题")
	}
}

// --- MEMORY.md 索引上限 ---

func TestMemoryStore_MemoryIndexMaxLines(t *testing.T) {
	dir := t.TempDir()
	store := tools.NewFileMemoryStore(dir, 5) // 极小上限
	ctx := context.Background()

	// 写入超出上限的条目
	for i := 0; i < 10; i++ {
		store.Remember(ctx, "记忆内容", "global", "key-"+string(rune('a'+i)), 1, "")
	}

	idx, _ := os.ReadFile(filepath.Join(dir, "memory/global/MEMORY.md"))
	lines := strings.Split(string(idx), "\n")
	// MEMORY.md 应被截断（header + 空行 + entries ≤ maxLines）
	if len(lines) > 7 {
		t.Errorf("MEMORY.md 应截断到 %d 行以内，得到 %d 行", 7, len(lines))
	}
}

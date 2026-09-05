// Package tools 提供 Agent 内置工具集。
//
// memory_file_store.go：MemoryStore 文件式实现——记忆以 md 文件存储，MEMORY.md 作索引。
// scope=session → memory/sessions/{id}/，scope=global → memory/global/。
// 参考 Claude Code ~/.claude/memories/：可检查、可编辑、无数据库。
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cognik/internal/agent"
)

// FileMemoryStore 记忆文件式存储。
type FileMemoryStore struct {
	storageRoot string // 存储根目录（如 "storage/"）
	maxLines    int    // MEMORY.md 最大行数
}

// NewFileMemoryStore 创建文件式记忆存储。
func NewFileMemoryStore(storageRoot string, maxLines int) *FileMemoryStore {
	if maxLines <= 0 {
		maxLines = 200
	}
	return &FileMemoryStore{storageRoot: storageRoot, maxLines: maxLines}
}

// memoryDir 返回某 scope 的目录（session 按 sessionID 隔离，global 全局共享）。
func (s *FileMemoryStore) memoryDir(scope, sessionID string) string {
	if scope == "session" {
		if sessionID == "" {
			sessionID = "default"
		}
		return filepath.Join(s.storageRoot, "sessions", sessionID)
	}
	return filepath.Join(s.storageRoot, "global")
}

// Remember 写入记忆到 md 文件 + 更新 MEMORY.md 索引。
func (s *FileMemoryStore) Remember(ctx context.Context, text, scope, key string, importance int, sessionID string) (string, error) {
	dir := s.memoryDir(scope, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成 frontmatter（name/description/created/modified/importance）
	now := time.Now().Format(time.RFC3339)
	description := truncateString(text, 60)
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\ncreated: %s\nmodified: %s\nimportance: %d\nstatus: active\n---\n\n%s\n",
		key, description, now, now, importance, text)

	filePath := filepath.Join(dir, key+".md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	// 更新 MEMORY.md 索引
	if err := s.updateMemoryIndex(dir, key, description); err != nil {
		return "", fmt.Errorf("更新索引失败: %w", err)
	}
	return key, nil
}

// Recall 子串匹配检索记忆（规模小，纯文本匹配即可）。
func (s *FileMemoryStore) Recall(ctx context.Context, query, scope string, limit int, sessionID string) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	dir := s.memoryDir(scope, sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []MemoryEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "MEMORY.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		fm, body := parseFrontmatter(content)
		if fm["status"] == "disabled" {
			continue // 跳过已失效
		}
		// 子串匹配（query 或 body 命中）
		score := 0.0
		bodyLower := strings.ToLower(body)
		if strings.Contains(bodyLower, queryLower) {
			score = 1.0
		} else if strings.Contains(strings.ToLower(fm["description"]), queryLower) {
			score = 0.5
		}
		if score > 0 {
			results = append(results, MemoryEntry{
				Content:  body,
				Score:    score,
				Source:   filepath.Join(dir, e.Name()),
				Metadata: fmToAny(fm),
			})
		}
	}

	// 按 score 降序，截断 limit
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Forget 标记记忆失效（frontmatter status: disabled，非物理删除）。
func (s *FileMemoryStore) Forget(ctx context.Context, scope, key string, sessionID string) error {
	dir := s.memoryDir(scope, sessionID)
	filePath := filepath.Join(dir, key+".md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取记忆失败: %w", err)
	}
	content := string(data)
	// 替换 status: active → status: disabled（若无 status 行则追加）
	if strings.Contains(content, "status: active") {
		content = strings.Replace(content, "status: active", "status: disabled", 1)
	} else if strings.Contains(content, "status: disabled") {
		return nil // 已失效
	} else {
		content = strings.Replace(content, "---\n", "---\nstatus: disabled\n", 1)
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}

// Update 同 key 覆盖写（保留 frontmatter，更新 modified + 正文）。
func (s *FileMemoryStore) Update(ctx context.Context, scope, key, text string, sessionID string) error {
	dir := s.memoryDir(scope, sessionID)
	filePath := filepath.Join(dir, key+".md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("记忆不存在: %w", err)
	}
	content := string(data)
	fm, _ := parseFrontmatter(content)
	now := time.Now().Format(time.RFC3339)
	fm["modified"] = now
	fm["description"] = truncateString(text, 60) // 更新描述
	// 重建 frontmatter + 新正文
	newContent := renderFrontmatter(fm) + "\n" + text + "\n"
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	// 更新索引 description
	description := truncateString(text, 60)
	return s.updateMemoryIndex(dir, key, description)
}

// List 列出某 scope 所有记忆条目（读 MEMORY.md）。
func (s *FileMemoryStore) List(ctx context.Context, scope string, sessionID string) ([]MemoryListItem, error) {
	dir := s.memoryDir(scope, sessionID)
	indexPath := filepath.Join(dir, "MEMORY.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []MemoryListItem
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- [") {
			continue
		}
		// 格式：- [key](key.md) — description
		key, desc := parseIndexLine(line)
		if key != "" {
			items = append(items, MemoryListItem{Key: key, Description: desc})
		}
	}
	return items, nil
}

// ListSessionEntries 列出会话记忆条目（含正文，供 SessionExtractor 提取用）。
// 实现 agent.SessionMemoryStore 接口。
func (s *FileMemoryStore) ListSessionEntries(ctx context.Context, sessionID string) ([]agent.SessionMemoryEntry, error) {
	dir := s.memoryDir("session", sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []agent.SessionMemoryEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "MEMORY.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		_, body := parseFrontmatter(string(data))
		key := strings.TrimSuffix(e.Name(), ".md")
		result = append(result, agent.SessionMemoryEntry{Key: key, Content: body})
	}
	return result, nil
}

// updateMemoryIndex 更新 MEMORY.md 索引（追加或更新一行）。
func (s *FileMemoryStore) updateMemoryIndex(dir, key, description string) error {
	indexPath := filepath.Join(dir, "MEMORY.md")
	data, _ := os.ReadFile(indexPath)
	lines := strings.Split(string(data), "\n")

	header := "# Memory Index"
	entryLine := fmt.Sprintf("- [%s](%s.md) — %s", key, key, description)

	// 检查是否已存在该 key
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, fmt.Sprintf("- [%s]", key)) {
			lines[i] = entryLine
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, entryLine)
	}

	// 重建：header + 空行 + entries（去空行，按 key 排序）
	var entries []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [") {
			entries = append(entries, line)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		ki := strings.TrimPrefix(strings.Split(entries[i], "]")[0], "- [")
		kj := strings.TrimPrefix(strings.Split(entries[j], "]")[0], "- [")
		return ki < kj
	})

	// 超出 maxLines 合并最旧条目（暂简化：截断）
	if len(entries) > s.maxLines-2 {
		entries = entries[:s.maxLines-2]
	}

	content := header + "\n\n" + strings.Join(entries, "\n") + "\n"
	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, indexPath)
}

// parseFrontmatter 解析 --- 分隔的 frontmatter，返回 map + 正文。
func parseFrontmatter(content string) (map[string]string, string) {
	fm := make(map[string]string)
	if !strings.HasPrefix(content, "---\n") {
		return fm, content
	}
	parts := strings.SplitN(content[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return fm, content
	}
	for _, line := range strings.Split(parts[0], "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			fm[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
		}
	}
	return fm, strings.TrimSpace(parts[1])
}

// renderFrontmatter 渲染 map 为 frontmatter 字符串。
func renderFrontmatter(fm map[string]string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	for k, v := range fm {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	sb.WriteString("---")
	return sb.String()
}

// parseIndexLine 解析 MEMORY.md 索引行：- [key](key.md) — description。
func parseIndexLine(line string) (key, desc string) {
	// - [key](key.md) — description
	if !strings.HasPrefix(line, "- [") {
		return "", ""
	}
	rest := line[3:]
	end := strings.Index(rest, "]")
	if end < 0 {
		return "", ""
	}
	key = rest[:end]
	if idx := strings.Index(line, "— "); idx >= 0 {
		desc = strings.TrimSpace(line[idx+len("— "):])
	}
	return key, desc
}

// truncateString 截断字符串到 maxLen 字符（rune 计数）。
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// fmToAny 将 map[string]string 转为 map[string]any。
func fmToAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Package tools 提供 Agent 内置工具集。
//
// memory.go：记忆工具（SyncTool）。action 参数区分 5 操作：
// remember/recall/forget/update/list。scope 区分 session（会话级）/global（跨会话）。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cognik/internal/agent"

	"cognik/internal/agent/llm"
)

// MemoryEntry 记忆检索结果。
type MemoryEntry struct {
	Content  string         `json:"content"`  // 记忆正文
	Score    float64        `json:"score"`    // 相关度
	Source   string         `json:"source"`   // 文件路径
	Metadata map[string]any `json:"metadata"` // frontmatter 元数据
}

// MemoryListItem 记忆列表条目（不返回全文）。
type MemoryListItem struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// MemoryStore 记忆存储抽象（session/global 文件式 + BM25）。
type MemoryStore interface {
	// Remember 写入记忆到 session 或 global。
	Remember(ctx context.Context, text, scope, key string, importance int, sessionID string) (string, error)
	// Recall 检索记忆（BM25 / 子串匹配）。
	Recall(ctx context.Context, query, scope string, limit int, sessionID string) ([]MemoryEntry, error)
	// Forget 标记失效（frontmatter status: disabled）。
	Forget(ctx context.Context, scope, key string, sessionID string) error
	// Update 更新已有记忆（同 key 覆盖）。
	Update(ctx context.Context, scope, key, text string, sessionID string) error
	// List 列出某 scope 所有记忆条目（读 MEMORY.md）。
	List(ctx context.Context, scope string, sessionID string) ([]MemoryListItem, error)
}

// MemoryTool 记忆工具（实现 agent.SyncTool）。
type MemoryTool struct {
	store MemoryStore
}

// NewMemoryTool 创建记忆工具。
func NewMemoryTool(store MemoryStore) *MemoryTool {
	return &MemoryTool{store: store}
}

// Info 返回工具元信息。
func (t *MemoryTool) Info() *llm.ToolInfo {
	return &llm.ToolInfo{
		Name: "memory",
		Desc: `Agent memory operations (remember/recall/forget/update/list) with session/global scope.
- recall: check memory BEFORE searching the KB — short-circuits redundant retrieval.
- remember: save only long-term valuable content (patterns, decisions, references), not ephemeral chatter.
- Do NOT use for: data already in the KB, or one-off conversation details.`,
		ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
			"action":     {Type: llm.String, Desc: "remember/recall/forget/update/list", Required: true},
			"scope":      {Type: llm.String, Desc: "session / global", Required: true},
			"text":       {Type: llm.String, Desc: "Memory text (action=remember/update)"},
			"query":      {Type: llm.String, Desc: "Search query (action=recall)"},
			"key":        {Type: llm.String, Desc: "Memory key (action=forget/update)"},
			"importance": {Type: llm.Integer, Desc: "1-10 (action=remember)"},
			"limit":      {Type: llm.Integer, Desc: "Max results (default 5, action=recall)"},
		}),
	}
}

// memoryParams memory 工具参数。
type memoryParams struct {
	Action     string `json:"action"`
	Scope      string `json:"scope"`
	Text       string `json:"text,omitempty"`
	Query      string `json:"query,omitempty"`
	Key        string `json:"key,omitempty"`
	Importance int    `json:"importance,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// Call 按 action 路由到 MemoryStore 方法。
func (t *MemoryTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var p memoryParams
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if p.Scope != "session" && p.Scope != "global" {
		return "", fmt.Errorf("scope must be session or global")
	}

	switch p.Action {
	case "remember":
		return t.doRemember(ctx, p)
	case "recall":
		return t.doRecall(ctx, p)
	case "forget":
		return t.doForget(ctx, p)
	case "update":
		return t.doUpdate(ctx, p)
	case "list":
		return t.doList(ctx, p)
	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *MemoryTool) doRemember(ctx context.Context, p memoryParams) (string, error) {
	if strings.TrimSpace(p.Text) == "" {
		return "", fmt.Errorf("text is required for action=remember")
	}
	if p.Key == "" {
		return "", fmt.Errorf("key is required for action=remember")
	}
	key, err := t.store.Remember(ctx, p.Text, p.Scope, p.Key, p.Importance, agent.SessionIDFromCtx(ctx))
	if err != nil {
		return "", fmt.Errorf("写入记忆失败: %w", err)
	}
	return fmt.Sprintf("记忆已写入（scope=%s, key=%s）", p.Scope, key), nil
}

func (t *MemoryTool) doRecall(ctx context.Context, p memoryParams) (string, error) {
	query := strings.TrimSpace(p.Query)
	if query == "" {
		return "无相关记忆", nil
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 5
	}
	entries, err := t.store.Recall(ctx, p.Query, p.Scope, limit, agent.SessionIDFromCtx(ctx))
	if err != nil {
		return "", fmt.Errorf("检索记忆失败: %w", err)
	}
	if len(entries) == 0 {
		return "无相关记忆", nil
	}
	var sb strings.Builder
	for i, e := range entries {
		fmt.Fprintf(&sb, "[%d] score=%.3f %s\n    %s\n", i+1, e.Score, e.Source, e.Content)
	}
	return strings.TrimSpace(sb.String()), nil
}

func (t *MemoryTool) doForget(ctx context.Context, p memoryParams) (string, error) {
	if p.Key == "" {
		return "", fmt.Errorf("key is required for action=forget")
	}
	if err := t.store.Forget(ctx, p.Scope, p.Key, agent.SessionIDFromCtx(ctx)); err != nil {
		return "", fmt.Errorf("标记失效失败: %w", err)
	}
	return fmt.Sprintf("记忆已标记失效（scope=%s, key=%s）", p.Scope, p.Key), nil
}

func (t *MemoryTool) doUpdate(ctx context.Context, p memoryParams) (string, error) {
	if p.Key == "" {
		return "", fmt.Errorf("key is required for action=update")
	}
	if strings.TrimSpace(p.Text) == "" {
		return "", fmt.Errorf("text is required for action=update")
	}
	if err := t.store.Update(ctx, p.Scope, p.Key, p.Text, agent.SessionIDFromCtx(ctx)); err != nil {
		return "", fmt.Errorf("更新记忆失败: %w", err)
	}
	return fmt.Sprintf("记忆已更新（scope=%s, key=%s）", p.Scope, p.Key), nil
}

func (t *MemoryTool) doList(ctx context.Context, p memoryParams) (string, error) {
	items, err := t.store.List(ctx, p.Scope, agent.SessionIDFromCtx(ctx))
	if err != nil {
		return "", fmt.Errorf("列出记忆失败: %w", err)
	}
	if len(items) == 0 {
		return "无记忆条目", nil
	}
	var sb strings.Builder
	for i, item := range items {
		fmt.Fprintf(&sb, "[%d] %s — %s\n", i+1, item.Key, item.Description)
	}
	return strings.TrimSpace(sb.String()), nil
}

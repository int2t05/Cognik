// Package agent 提供自建 ReAct 循环与工具接口。
//
// session_lifecycle.go：会话生命周期管理——会话结束提取流。
// 扫描 sessions/{id}/ 的记忆条目，LLM 提取有长期价值的内容，写入 global/ + 更新 MEMORY.md。
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// SessionExtractor 会话结束提取器——从会话记忆提取长期价值内容到全局记忆。
type SessionExtractor struct {
	memoryStore SessionMemoryStore // 记忆存储接口（MemoryStore 的子集）
	summarize   SummarizeFunc      // LLM 摘要函数
}

// SessionMemoryStore 记忆存储接口（提取器所需的子集，避免循环依赖）。
type SessionMemoryStore interface {
	ListSessionEntries(ctx context.Context, sessionID string) ([]SessionMemoryEntry, error)
	Remember(ctx context.Context, text, scope, key string, importance int, sessionID string) (string, error)
}

// SessionMemoryEntry 会话记忆条目。
type SessionMemoryEntry struct {
	Key     string
	Content string
}

// NewSessionExtractor 创建会话提取器。
func NewSessionExtractor(store SessionMemoryStore, summarize SummarizeFunc) *SessionExtractor {
	return &SessionExtractor{
		memoryStore: store,
		summarize:   summarize,
	}
}

// Extract 会话结束提取——扫描会话记忆，LLM 提取有价值内容，写入全局记忆。
// 失败不阻塞会话删除（best-effort）。
func (e *SessionExtractor) Extract(ctx context.Context, threadID int64) error {
	if e == nil || e.memoryStore == nil {
		return nil
	}
	sessionID := fmt.Sprintf("%d", threadID)

	// 1. 扫描会话记忆条目
	entries, err := e.memoryStore.ListSessionEntries(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("扫描会话记忆失败: %w", err)
	}
	if len(entries) == 0 {
		return nil // 无会话记忆，跳过
	}

	// 2. 构造摘要请求消息
	var sb strings.Builder
	sb.WriteString("以下是一次会话中记录的记忆条目。提取有长期价值的内容（关键决策、问题解法、经验总结），")
	sb.WriteString("忽略仅与当前会话相关的临时信息。每条提取的记忆用一行描述。\n\n")
	for _, e := range entries {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", e.Key, e.Content)
	}

	// 3. LLM 提取
	msgs := []*schema.Message{
		schema.SystemMessage(sb.String()),
		schema.UserMessage("提取有长期价值的记忆，每条一行。"),
	}
	if e.summarize == nil {
		return nil
	}
	summary, err := e.summarize(ctx, msgs)
	if err != nil {
		return fmt.Errorf("LLM 提取失败: %w", err)
	}
	if strings.TrimSpace(summary) == "" {
		return nil
	}

	// 4. 写入全局记忆（每条提取结果作为一个 global 记忆条目）
	key := fmt.Sprintf("session-%d-extraction", threadID)
	_, err = e.memoryStore.Remember(ctx, summary, "global", key, 5, "")
	return err
}

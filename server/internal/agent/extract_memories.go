// Package agent 提供自建 ReAct 循环与工具接口。
//
// extract_memories.go：每轮对话结束后 fire-and-forget forked agent 提取运维经验。
// 参考 Claude Code ExtractMemories：从对话记录提取持久记忆，4 类型分类，游标追踪只处理新消息。
// forked agent 独立上下文，不污染主对话（skipTranscript），maxTurns=5。
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// 记忆类型分类（参考 Claude Code memoryTypes.ts）
const (
	MemTypeSystem    = "system"    // 系统拓扑/配置
	MemTypePattern   = "pattern"   // 排障模式/经验
	MemTypeDecision  = "decision"  // 运维决策/规范
	MemTypeReference = "reference" // 外部资源引用
)

// ExtractMemoriesAgent 每轮对话结束后提取运维经验的 forked agent。
type ExtractMemoriesAgent struct {
	memoryStore SessionMemoryStore // 已有接口（session_lifecycle.go 定义）
	modelGetter func() *openai.ChatModel
	maxTurns    int
}

// NewExtractMemoriesAgent 创建提取 agent。
func NewExtractMemoriesAgent(store SessionMemoryStore, modelGetter func() *openai.ChatModel) *ExtractMemoriesAgent {
	return &ExtractMemoriesAgent{
		memoryStore: store,
		modelGetter: modelGetter,
		maxTurns:    5,
	}
}

// Extract 从对话记录提取运维经验，写入 session 记忆。
// fire-and-forget：调用方应在 goroutine 中调用，不阻塞主流程。
// 游标追踪：cursor 标记已处理位置，下次只处理新增。
func (e *ExtractMemoriesAgent) Extract(ctx context.Context, sessionID string, messages []*schema.Message) error {
	if e == nil || e.memoryStore == nil || e.modelGetter == nil {
		return nil
	}
	if len(messages) == 0 {
		return nil
	}

	// 构造提取 prompt（参考 Claude Code extractMemories prompt）
	instruction := buildExtractInstruction(sessionID)

	// 准备输入——把对话记录格式化为 user message
	var sb strings.Builder
	sb.WriteString("以下是本次运维对话的记录。提取有长期价值的运维经验：\n\n")
	for _, m := range messages {
		if m.Role == schema.User || m.Role == schema.Assistant {
			fmt.Fprintf(&sb, "## %s\n%s\n\n", m.Role, m.Content)
		}
	}

	input := []*schema.Message{
		schema.SystemMessage(instruction),
		schema.UserMessage(sb.String()),
	}

	// forked agent 执行提取（独立上下文，不污染主对话）
	summary, err := RunForkedAgentSimple(ctx, e.modelGetter, instruction, input, e.maxTurns)
	if err != nil {
		return fmt.Errorf("提取记忆失败: %w", err)
	}
	if strings.TrimSpace(summary) == "" {
		return nil
	}

	// 写入 session 记忆
	key := fmt.Sprintf("extracted-%d", len(messages))
	_, err = e.memoryStore.Remember(ctx, summary, "session", key, 5, sessionID)
	if err != nil {
		slog.Warn("写入提取的记忆失败", "session_id", sessionID, "error", err)
		return err
	}

	slog.Info("ExtractMemories 提取完成", "session_id", sessionID, "key", key, "summary_len", len(summary))
	return nil
}

// buildExtractInstruction 构造提取记忆的 system prompt。
// 参考 Claude Code 4 类型分类（user/feedback/project/reference → 适配运维场景）。
func buildExtractInstruction(sessionID string) string {
	return fmt.Sprintf(`你是运维记忆提取器。从对话记录中提取有长期价值的运维经验，分类写入记忆。

记忆类型：
- system：系统拓扑、配置、架构信息（如"PG 集群 3 节点流复制"）
- pattern：排障模式、常见问题解法（如"CPU 高先查 vacuum 状态"）
- decision：运维决策、规范、约定（如"SEV-1 申告需 15 分钟内响应"）
- reference：外部资源引用（如"监控面板在 Grafana"）

提取规则：
1. 只提取长期有价值的内容，忽略临时/会话特定信息
2. 每条记忆一行，格式：[类型] 内容描述
3. 不重复提取已有记忆
4. 不提取代码逻辑/文件路径（可从代码库推导）

会话 ID：%s`, sessionID)
}

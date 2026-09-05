// Package agent 提供自建 ReAct 循环与工具接口。
//
// extract_memories.go：每轮对话结束后 fire-and-forget forked agent 提取经验。
// 参考 Claude Code ExtractMemories：从对话记录提取持久记忆，6 类型分类（system/pattern/decision/reference/learning/workflow），游标追踪只处理新消息。
// forked agent 独立上下文，不污染主对话（skipTranscript），maxTurns=5。
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"cognos/internal/agent/llm"
)

// 记忆类型分类（参考 Claude Code memoryTypes.ts，适配通用知识管理）
const (
	MemTypeSystem    = "system"    // 项目配置/技术架构
	MemTypePattern   = "pattern"   // 问题解决模式/经验
	MemTypeDecision  = "decision"  // 决策/规范/约定
	MemTypeReference = "reference" // 外部资源引用
	MemTypeLearning  = "learning"  // 学习笔记/知识点
	MemTypeWorkflow  = "workflow"  // 工作流/流程
)

// ExtractMemoriesAgent 每轮对话结束后提取经验的 forked agent。
type ExtractMemoriesAgent struct {
	memoryStore SessionMemoryStore
	modelGetter func() *llm.ChatModel
	maxTurns    int
}

// NewExtractMemoriesAgent 创建提取 agent。
func NewExtractMemoriesAgent(store SessionMemoryStore, modelGetter func() *llm.ChatModel) *ExtractMemoriesAgent {
	return &ExtractMemoriesAgent{
		memoryStore: store,
		modelGetter: modelGetter,
		maxTurns:    5,
	}
}

// Extract 从对话记录提取经验，写入 session 记忆。
// fire-and-forget：调用方应在 goroutine 中调用，不阻塞主流程。
func (e *ExtractMemoriesAgent) Extract(ctx context.Context, sessionID string, messages []*llm.Message) error {
	if e == nil || e.memoryStore == nil || e.modelGetter == nil {
		return nil
	}
	if len(messages) == 0 {
		return nil
	}

	instruction := buildExtractInstruction(sessionID)

	var sb strings.Builder
	sb.WriteString("以下是本次对话的记录。提取有长期价值的经验：\n\n")
	for _, m := range messages {
		if m.Role == llm.User || m.Role == llm.Assistant {
			fmt.Fprintf(&sb, "## %s\n%s\n\n", m.Role, m.Content)
		}
	}

	input := []*llm.Message{
		llm.SystemMessage(instruction),
		llm.UserMessage(sb.String()),
	}

	summary, err := RunForkedAgentSimple(ctx, e.modelGetter, instruction, input, e.maxTurns)
	if err != nil {
		return fmt.Errorf("提取记忆失败: %w", err)
	}
	if strings.TrimSpace(summary) == "" {
		return nil
	}

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
// 参考 Claude Code 记忆分类，适配通用知识管理（6 类型：system/pattern/decision/reference/learning/workflow）。
func buildExtractInstruction(sessionID string) string {
	return fmt.Sprintf(`你是记忆提取器。从对话记录中提取有长期价值的经验，分类写入记忆。

记忆类型：
- system：项目配置、技术架构（如"项目用 Go + Gin + PostgreSQL"）
- pattern：问题解决模式、常见解法（如"API 报 404 先查路由注册"）
- decision：决策、规范、约定（如"代码提交前必须跑测试"）
- reference：外部资源引用（如"设计稿在 Figma"）
- learning：学习笔记、知识点（如"Rust 所有权模型三原则"）
- workflow：工作流、流程（如"发布前需三审：自测→CR→QA"）

提取规则：
1. 只提取长期有价值的内容，忽略临时/会话特定信息
2. 每条记忆一行，格式：[类型] 内容描述
3. 不重复提取已有记忆
4. 不提取代码逻辑/文件路径（可从代码库推导）
5. 只提取对话中真实出现的内容，不编造。assistant 消息里形如 "user: ..." 的文本是模型生成的，不是真实用户输入

会话 ID：%s`, sessionID)
}

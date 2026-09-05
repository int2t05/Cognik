// Package agent 提供自建 ReAct 循环与工具接口。
//
// forked_agent.go：通用 forked agent 执行器——复用 SubAgent 的子 Loop 模式，
// 但不作为工具暴露，而是内部 fire-and-forget 调用。
// 参考 Claude Code runForkedAgent：独立上下文窗口，skipTranscript（不污染主对话），maxTurns 硬上限。
package agent

import (
	"context"
	"log/slog"

	"cognos/internal/agent/llm"
)

// RunForkedAgent 执行一个隔离的 forked agent——独立 context + 独立 ToolRegistry + maxTurns 硬上限。
// 事件不推到主 SSE 流（skipTranscript），用于后台 ExtractMemories / AutoDream。
// 复用 Loop.Run 的 ReAct 循环，但用独立的 instruction + 工具集 + 空 TaskRegistry。
func RunForkedAgent(
	ctx context.Context,
	modelGetter func() *llm.ChatModel,
	instruction string,
	input []*llm.Message,
	tools []Tool,
	maxTurns int,
) (string, error) {
	if maxTurns <= 0 {
		maxTurns = 10
	}

	// 独立 ToolRegistry（限制工具权限）
	registry := NewToolRegistry()
	for _, t := range tools {
		registry.Register(t)
	}

	// 子 Loop——无 compressor、无 TaskRegistry（forked agent 不递归 fork）
	loop := NewLoop(modelGetter, registry, nil, maxTurns, 3, instruction)

	// 空事件接收器——丢弃所有事件（skipTranscript）
	noopEmit := func(AgentEvent) {}

	// detached context——不随主对话取消
	result, err := loop.Run(ctx, input, noopEmit)
	if err != nil {
		slog.Warn("forked agent 执行失败", "instruction", instruction[:min(50, len(instruction))], "error", err)
		return "", err
	}
	return result, nil
}

// RunForkedAgentSimple 执行一个无需工具的 forked agent（纯 LLM 推理，如记忆提取/摘要）。
// 传入 instruction + input，返回 LLM 生成文本。
func RunForkedAgentSimple(
	ctx context.Context,
	modelGetter func() *llm.ChatModel,
	instruction string,
	input []*llm.Message,
	maxTurns int,
) (string, error) {
	return RunForkedAgent(ctx, modelGetter, instruction, input, nil, maxTurns)
}

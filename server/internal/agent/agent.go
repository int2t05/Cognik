// Package agent 提供 Agent Loop 基座。
// agent.go：ReactAgent + SubAgent 构造。
//
// 主 Agent 使用 react.NewAgent（ToolCallingModel + 9 工具 + StreamToolCallChecker）。
// SubAgent 通过 adk.NewAgentTool 包装为工具，父 Agent 可委托任务给子 Agent。
// 对标 Claude Code 的 Agent 工具（Explore/Plan/general-purpose）。
package agent

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	agenttools "opsmind/internal/agent/tools"
)

const defaultInstruction = `You are OpsMind, an AI operations assistant.
You can use tools to help answer questions. Use tools when needed to
gather information or perform actions. Be concise and helpful.
When a task is complex, delegate to a sub-agent using the task tool.`

const defaultMaxStep = 20

// AgentFactory 构造 Agent（每请求一个实例）。
type AgentFactory struct {
	modelFactory *ChatModelFactory
	toolFactory  *agenttools.ToolFactory
	instruction  string
	maxStep      int
}

// NewAgentFactory 创建工厂。
func NewAgentFactory(modelFactory *ChatModelFactory, toolFactory *agenttools.ToolFactory) *AgentFactory {
	return &AgentFactory{
		modelFactory: modelFactory,
		toolFactory:  toolFactory,
		instruction:  defaultInstruction,
		maxStep:      defaultMaxStep,
	}
}

// SetInstruction 设置系统提示词。
func (f *AgentFactory) SetInstruction(instruction string) {
	if instruction != "" {
		f.instruction = instruction
	}
}

// NewAgent 构造主 Agent（含子 Agent 工具）。
func (f *AgentFactory) NewAgent(ctx context.Context) (*react.Agent, error) {
	chatModel := f.modelFactory.GetModel()
	if chatModel == nil {
		return nil, errors.New("ChatModel 未初始化")
	}

	tools := f.toolFactory.BuildTools()

	// 注册子 Agent 为工具
	subAgentTools := f.buildSubAgentTools(ctx)
	tools = append(tools, subAgentTools...)

	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: tools,
		},
		MaxStep:               f.maxStep,
		StreamToolCallChecker: streamToolCallChecker,
	})
}

// buildSubAgentTools 构造子 Agent 并包装为工具。
// 两个内置子 Agent：
//   - research：只读探查（read_file/glob/grep/list_dir），对标 Claude Code Explore
//   - coder：读写操作（bash/async_bash/edit_file/write_file/mkdir），对标 Claude Code general-purpose
func (f *AgentFactory) buildSubAgentTools(ctx context.Context) []tool.BaseTool {
	chatModel := f.modelFactory.GetModel()
	if chatModel == nil {
		return nil
	}

	// research 子 Agent（只读探查）
	researchAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "research",
		Description: "A research assistant that can read files, search content, and list directories. Use for read-only exploration tasks.",
		Instruction: "You are a research assistant. Read files, search content, list directories. Report findings concisely.",
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: f.toolFactory.BuildReadOnlyTools(),
			},
		},
	})
	if err != nil {
		return nil
	}

	// coder 子 Agent（读写操作）
	coderAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "coder",
		Description: "A coding assistant that can execute commands, edit files, and create directories. Use for write operations and code execution.",
		Instruction: "You are a coding assistant. Execute commands, edit files, create directories. Be careful with destructive operations.",
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: f.toolFactory.BuildTools(),
			},
		},
	})
	if err != nil {
		return nil
	}

	return []tool.BaseTool{
		adk.NewAgentTool(ctx, researchAgent),
		adk.NewAgentTool(ctx, coderAgent),
	}
}

// streamToolCallChecker 处理 llama.cpp 非标准流式 tool call 格式。
func streamToolCallChecker(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if len(msg.ToolCalls) > 0 {
			return true, nil
		}
		if len(msg.Content) == 0 {
			continue
		}
		return false, nil
	}
}

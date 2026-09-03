// Package agent 提供 Agent Loop 基座。
// agent.go：ReactAgent 构造 + 工具注册 + StreamToolCallChecker（llama.cpp 兼容）。
//
// 使用 react.NewAgent 配置 ToolCallingModel 字段。每请求构造一个 Agent 实例（不复用）。
// MaxStep 20 = ~9 次工具调用。StreamToolCallChecker 处理 llama.cpp 非标准流式 tool call。
package agent

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	agenttools "opsmind/internal/agent/tools"
)

// defaultInstruction 默认系统提示词（LLMConfig.SystemPrompt 为空时使用）。
const defaultInstruction = `You are OpsMind, an AI operations assistant.
You can use tools to help answer questions. Use tools when needed to
gather information or perform actions. Be concise and helpful.`

// defaultMaxStep 默认最大步数（每轮 ChatModel+ToolsNode=2 step，20 ≈ 9 次工具调用）。
const defaultMaxStep = 20

// AgentFactory 构造 ReactAgent（每请求一个实例）。
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

// SetInstruction 设置系统提示词（从 LLMConfig.SystemPrompt 注入）。
func (f *AgentFactory) SetInstruction(instruction string) {
	if instruction != "" {
		f.instruction = instruction
	}
}

// NewAgent 构造一个 ReactAgent。每请求调用（实例不复用）。
func (f *AgentFactory) NewAgent(ctx context.Context) (*react.Agent, error) {
	chatModel := f.modelFactory.GetModel()
	if chatModel == nil {
		return nil, errors.New("ChatModel 未初始化")
	}

	tools := f.toolFactory.BuildTools()

	// 注入系统提示词（Instruction）到输入消息前
	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel, // ChatModel 作为工具调用模型
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: tools,
		},
		MaxStep:             f.maxStep,
		StreamToolCallChecker: streamToolCallChecker, // llama.cpp 流式 tool call 检测
	})
}

// streamToolCallChecker 处理 llama.cpp 非标准流式 tool call 格式。
// llama.cpp 可能先输出文本再输出 tool call，或发前导空 chunk。
// 此 checker 读完流判断是否含 ToolCalls（参考 Eino FAQ 自定义 checker 模式）。
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
			continue // 跳过前导空 chunk
		}
		return false, nil // 首个非空 content 且无 tool call = 不是工具调用
	}
}

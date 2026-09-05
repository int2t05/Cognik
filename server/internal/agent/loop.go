// agent/loop.go：自建 ReAct 循环。
//
// 真异步恢复（fire-and-forget + completion-notification）：
// 循环：LLM 流式生成 → 检测 tool_calls → 派发（Sync 阻塞 / Async 立即返回 Task）
// → 无 tool_calls 但有 pending → waitForAny 等完成 → 注入 user notification 恢复 → 循环。
//
// 异步契约：AsyncTool.Dispatch 立即返回 Task，立即产出 tool_result("task xxx 已派发")
// 满足 Anthropic API（每 tool_use 一个 result）；完成通知是新 user 消息（非第二个 tool_result）。
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	"cognik/internal/agent/llm"
)

const (
	defaultMaxStep                = 20
	defaultMaxConsecutiveDispatch = 3
)

// Loop 自建 ReAct 循环。
type Loop struct {
	modelGetter            func() *llm.ChatModel // 动态获取 ChatModel（热切换安全，不持快照）
	registry               *ToolRegistry
	taskRegistry           *TaskRegistry
	maxStep                int
	maxConsecutiveDispatch int
	instruction            string             // 系统提示词（Run 时 prepend 为 SystemMessage）
	toolInfos              []*llm.ToolInfo // 从 registry 提取，绑给 LLM
	compressor             *Compressor        // 上下文压缩器（nil 时不压缩）
}

// LoopOption Loop 函数选项。
type LoopOption func(*Loop)

// WithCompressor 注入上下文压缩器。
func WithCompressor(c *Compressor) LoopOption {
	return func(l *Loop) { l.compressor = c }
}

// NewLoop 创建循环。
// modelGetter 动态获取 ChatModel（热切换时返回新实例）；maxStep<=0 用默认值。
// instruction 为系统提示词，Run 时 prepend 为首条 SystemMessage。
func NewLoop(modelGetter func() *llm.ChatModel, registry *ToolRegistry, taskRegistry *TaskRegistry, maxStep, maxConsecutiveDispatch int, instruction string, opts ...LoopOption) *Loop {
	if maxStep <= 0 {
		maxStep = defaultMaxStep
	}
	if maxConsecutiveDispatch <= 0 {
		maxConsecutiveDispatch = defaultMaxConsecutiveDispatch
	}
	// 从 registry 提取所有工具的 ToolInfo，供 LLM 决策调用。
	toolInfos := make([]*llm.ToolInfo, 0, len(registry.tools))
	for _, t := range registry.tools {
		if info := t.Info(); info != nil {
			toolInfos = append(toolInfos, info)
		}
	}
	l := &Loop{
		modelGetter:            modelGetter,
		registry:               registry,
		taskRegistry:           taskRegistry,
		maxStep:                maxStep,
		maxConsecutiveDispatch: maxConsecutiveDispatch,
		instruction:            instruction,
		toolInfos:              toolInfos,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Run 执行 ReAct 循环，emit 推流式事件，返回最终回答。
// ctx 应为 detached（带超时）— 客户端断开不停止生成。
func (l *Loop) Run(ctx context.Context, messages []*llm.Message, emit EventSink) (string, error) {
	// 系统提示词 prepend 为 SystemMessage（首条非 system 时注入）。
	if l.instruction != "" && (len(messages) == 0 || messages[0].Role != llm.System) {
		messages = append([]*llm.Message{llm.SystemMessage(l.instruction)}, messages...)
	}
	var pending []*Task
	consecutiveDispatches := 0
	// 所有退出路径 cancel 未完成后台任务（无 goroutine 泄漏）。
	defer func() {
		for _, t := range pending {
			t.Cancel()
		}
	}()

	for step := 0; step < l.maxStep; step++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// 上下文压缩：每步 LLM 调用前对消息历史执行六级压缩（Tool Result Budget → Snip → Microcompact → HeadAndTail → 去重 → Autocompact）。
		if l.compressor != nil {
			messages = l.compressor.Compress(ctx, messages)
		}

		msg, err := l.drainModelStream(ctx, messages, emit)
		if err != nil {
			return "", err
		}
		messages = append(messages, msg)

		// 无 tool_calls → 准备结束；有 pending 则等完成通知注入后继续。
		if len(msg.ToolCalls) == 0 {
			if len(pending) == 0 {
				return msg.Content, nil
			}
			// LLM 想停但后台在跑 → 等下一个完成，注入 user notification，继续循环。
			completed, wErr := waitForAny(ctx, pending)
			if wErr != nil {
				return "", wErr
			}
			notification := fmt.Sprintf(
				"<task-notification>\n<task-id>%s</task-id>\n<tool-use-id>%s</tool-use-id>\n<status>%s</status>\n<result>%s</result>\n</task-notification>\n注意：结果已展示给用户，不要复述子代理的输出，直接基于结果继续回答用户问题。",
				completed.ID, completed.ToolCallID, completed.Status, completed.Result)
			// user 消息（非 tool_result）— 满足 API 契约。
			messages = append(messages, llm.UserMessage(notification))
			// 完成事件带 TaskID，前端归入 dispatch_subagent 卡片（不混入主 Agent 文本）。
			emit(AgentEvent{Type: EventToolCall, ID: completed.ID, Label: "task_completion", TaskID: completed.ID})
			emit(AgentEvent{Type: EventToolResult, ID: completed.ID, Content: completed.Result, TaskID: completed.ID})
			pending = removeTask(pending, completed)
			consecutiveDispatches = 0
			continue
		}

		// 派发本轮所有 tool_calls，立即产出 tool_result 满足 API 契约（每 tool_use 恰好一个 result）。
		for _, tc := range msg.ToolCalls {
			emit(AgentEvent{Type: EventToolCall, ID: tc.ID, Label: tc.Function.Name, Content: tc.Function.Arguments})
			tool := l.registry.Get(tc.Function.Name)
			if tool == nil {
				// LLM 幻觉调用了不存在的工具 → 返回错误 result，让 LLM 自我纠正。
				errMsg := fmt.Sprintf("工具 '%s' 不存在", tc.Function.Name)
				messages = append(messages, llm.ToolMessage(errMsg, tc.ID))
				emit(AgentEvent{Type: EventToolResult, ID: tc.ID, Content: errMsg})
				continue
			}
			switch t := tool.(type) {
			case SyncTool:
				result, callErr := t.Call(ctx, tc.Function.Arguments, emit)
				if callErr != nil {
					result = "错误: " + callErr.Error()
				}
				messages = append(messages, llm.ToolMessage(result, tc.ID))
				emit(AgentEvent{Type: EventToolResult, ID: tc.ID, Content: result})
			case AsyncTool:
				taskCtx, cancel := context.WithCancel(ctx) // 子 ctx，cancel 链到父
				task, dErr := t.Dispatch(taskCtx, tc.Function.Arguments, emit)
				if dErr != nil {
					ack := "派发失败: " + dErr.Error()
					messages = append(messages, llm.ToolMessage(ack, tc.ID))
					emit(AgentEvent{Type: EventToolResult, ID: tc.ID, Content: ack})
					cancel() // 派发失败，立即清理子 ctx
					continue
				}
				task.ToolCallID = tc.ID // 映射回 tool_use_id（notification 用）
				task.cancel = cancel
				if l.taskRegistry != nil {
					l.taskRegistry.Register(task)
				}
				ack := fmt.Sprintf("task %s 已派发，运行中", task.ID)
				messages = append(messages, llm.ToolMessage(ack, tc.ID))
				// ack tool_result 带 TaskID，service 据此把后续子 Agent 事件归入此卡片（不立即 done）。
				emit(AgentEvent{Type: EventToolResult, ID: tc.ID, Content: ack, TaskID: task.ID})
				pending = append(pending, task)
				consecutiveDispatches++
			}
		}

		// 守卫：连续派发不等待，防 LLM 无限 dispatch 烧步数。
		if consecutiveDispatches > l.maxConsecutiveDispatch {
			messages = append(messages, llm.UserMessage(
				fmt.Sprintf("你已有 %d 个后台任务在运行。请等待它们完成后再派发新任务。", len(pending))))
			consecutiveDispatches = 0
		}
		// 循环回到 LLM：可继续调同步工具或派发更多子 Agent。
	}
	return "", errors.New("超过最大步数")
}

// drainModelStream 排空 LLM 流式输出：emit reasoning/token 事件，返回聚合完整 message。
// 用 llm.ConcatMessages 聚合 tool_calls 增量（按 Index 拼 Arguments，处理流式 delta）。
func (l *Loop) drainModelStream(ctx context.Context, messages []*llm.Message, emit EventSink) (*llm.Message, error) {
	m := l.modelGetter()
	if m == nil {
		return nil, errors.New("ChatModel 未初始化")
	}
	// 工具描述透明转成 OpenAI tools 字段，直接传入 Stream——无 WithTools 黑盒。
	var tools []llm.OpenAITool
	if len(l.toolInfos) > 0 {
		var err error
		tools, err = llm.ToOpenAITools(l.toolInfos)
		if err != nil {
			return nil, fmt.Errorf("转换工具失败: %w", err)
		}
	}
	reader, err := m.Stream(ctx, messages, tools)
	if err != nil {
		return nil, fmt.Errorf("LLM 流式调用失败: %w", err)
	}
	return drainStream(ctx, reader, emit)
}

// drainStream 排空 StreamReader，emit 事件，ConcatMessages 聚合完整 message。
func drainStream(ctx context.Context, reader *llm.StreamReader[*llm.Message], emit EventSink) (*llm.Message, error) {
	defer reader.Close()
	var chunks []*llm.Message
	for {
		if err := ctx.Err(); err != nil {
			break
		}
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			emit(AgentEvent{Type: EventError, Error: err.Error()})
			break
		}
		if msg.ReasoningContent != "" {
			emit(AgentEvent{Type: EventReasoning, Content: msg.ReasoningContent})
		}
		if msg.Content != "" {
			emit(AgentEvent{Type: EventToken, Content: msg.Content})
		}
		chunks = append(chunks, msg)
	}
	if len(chunks) == 0 {
		return &llm.Message{Role: llm.Assistant}, nil
	}
	merged, err := llm.ConcatMessages(chunks)
	if err != nil {
		// 聚合失败降级：用最后一个 chunk（至少不丢失 tool_calls）。
		return chunks[len(chunks)-1], nil
	}
	return merged, nil
}

// waitForAny 阻塞到 pending 中任意 task 完成或 ctx 取消。
// 用 reflect.Select 多路复用所有 task.done channel，无额外 goroutine，无泄漏。
func waitForAny(ctx context.Context, tasks []*Task) (*Task, error) {
	cases := make([]reflect.SelectCase, len(tasks)+1)
	for i, t := range tasks {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.done)}
	}
	cases[len(tasks)] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}

	chosen, _, _ := reflect.Select(cases)
	if chosen == len(tasks) {
		return nil, ctx.Err()
	}
	return tasks[chosen], nil
}

// removeTask 从切片移除指定 task（返回新切片，保持顺序）。
func removeTask(tasks []*Task, target *Task) []*Task {
	out := make([]*Task, 0, len(tasks)-1)
	for _, t := range tasks {
		if t != target {
			out = append(out, t)
		}
	}
	return out
}

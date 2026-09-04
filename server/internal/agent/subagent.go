// agent/subagent.go：SubAgent 系统 + dispatch_subagent 工具（AsyncTool）。
//
// 主 Agent 通过 dispatch_subagent 委托任务给内置 SubAgent：
//   - 子 Agent fresh context（SystemMessage(instruction) + UserMessage(task)，不继承父对话）
//   - 子 Loop 后台跑，事件经 emit 透传到父 SSE（tool_call/reasoning/token 实时可见）
//   - 完成时 task.markDone，父 Loop 的 waitForAny 捕获后注入 user notification 恢复
//   - 可递归：maxDepth 约束嵌套深度
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// SubAgent 内置子 Agent 定义。
type SubAgent struct {
	Name        string   // 唯一标识（dispatch_subagent 的 agent 参数值）
	Description string   // LLM 决策依据（dispatch_subagent 工具 description 列出）
	Instruction string   // 系统提示词（prepend 为 SystemMessage）
	Tools       []string // 工具名子集（registry.Subset 取）
	MaxStep     int      // 子 Loop 最大步数（低于父 Loop，约束执行时长）
}

// DispatchSubagentTool AsyncTool：主 Agent 委托任务给内置 SubAgent。
type DispatchSubagentTool struct {
	agents   map[string]*SubAgent
	model    *ChatModelFactory // 动态获取 ChatModel（热切换安全）
	registry *ToolRegistry    // 父 registry，用于 Subset 取子 Agent 工具
	maxDepth int              // 递归深度上限（默认 3）
}

// NewDispatchSubagentTool 创建 dispatch_subagent 工具。
func NewDispatchSubagentTool(agents map[string]*SubAgent, model *ChatModelFactory, registry *ToolRegistry, maxDepth int) *DispatchSubagentTool {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return &DispatchSubagentTool{agents: agents, model: model, registry: registry, maxDepth: maxDepth}
}

// Info 返回工具元信息。description 列出可选 agent，供 LLM 决策。
func (d *DispatchSubagentTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "dispatch_subagent",
		Desc: d.buildDescription(),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"agent": {
				Type:     schema.String,
				Desc:     fmt.Sprintf("子 Agent 名称（可选: %s）", d.agentNames()),
				Required: true,
			},
			"task": {
				Type:     schema.String,
				Desc:     "任务描述（子 Agent 要做什么）",
				Required: true,
			},
		}),
	}
}

// Dispatch 立即返回 Task，后台 goroutine 跑子 Loop。
func (d *DispatchSubagentTool) Dispatch(ctx context.Context, argsJSON string, emit EventSink) (*Task, error) {
	var params struct {
		Agent string `json:"agent"`
		Task  string `json:"task"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if params.Task == "" {
		return nil, fmt.Errorf("task is required")
	}

	sa, ok := d.agents[params.Agent]
	if !ok {
		return nil, fmt.Errorf("子 Agent '%s' 不存在（可选: %s）", params.Agent, d.agentNames())
	}

	taskCtx, cancel := context.WithCancel(ctx)
	task := newTask(taskKindSubagent, cancel)

	go func() {
		defer cancel() // 清理 taskCtx（幂等，与 task.Cancel 安全共存）
		// 子 Agent fresh context：SystemMessage(instruction) + UserMessage(task)，不继承父对话。
		messages := []*schema.Message{
			schema.SystemMessage(sa.Instruction),
			schema.UserMessage(params.Task),
		}
		// 子 Loop 用独立 registry（父 registry 的工具子集 + 递归 dispatch_subagent）。
		subRegistry := NewToolRegistry()
		for _, t := range d.registry.Subset(sa.Tools) {
			subRegistry.Register(t)
		}
		// 递归：若 maxDepth>0，子 Agent 也可派发更深子 Agent（深度递减约束）。
		if d.maxDepth > 0 {
			subRegistry.Register(&DispatchSubagentTool{
				agents: d.agents, model: d.model, registry: d.registry, maxDepth: d.maxDepth - 1,
			})
		}
		// 子 Agent 事件打 TaskID 标记——service/reducer 据此归入 dispatch_subagent 卡片，不混入主 Agent 文本。
		scopedEmit := func(evt AgentEvent) {
			evt.TaskID = task.ID
			emit(evt)
		}
		innerLoop := NewLoop(d.model.GetModel, subRegistry, nil, sa.MaxStep, defaultMaxConsecutiveDispatch, "")
		result, err := innerLoop.Run(taskCtx, messages, scopedEmit)
		task.markDone(result, err)
	}()

	return task, nil
}

// buildDescription 构造工具描述（含可用 agent 列表）。
func (d *DispatchSubagentTool) buildDescription() string {
	return fmt.Sprintf("Dispatch a subagent to work on a task in the background. Returns immediately; the result arrives as a task-notification when the subagent completes. Available agents: %s.", d.agentList())
}

// agentNames 返回逗号分隔的 agent 名。
func (d *DispatchSubagentTool) agentNames() string {
	names := make([]string, 0, len(d.agents))
	for n := range d.agents {
		names = append(names, n)
	}
	return fmt.Sprintf("%v", names)
}

// agentList 返回 agent 名+描述列表（供 description）。
func (d *DispatchSubagentTool) agentList() string {
	parts := make([]string, 0, len(d.agents))
	for name, sa := range d.agents {
		parts = append(parts, fmt.Sprintf("%s (%s)", name, sa.Description))
	}
	return strings.Join(parts, ", ")
}

// 内置 SubAgent 定义。

// ResearchSubAgent 只读探查子 Agent。
var ResearchSubAgent = &SubAgent{
	Name:        "research",
	Description: "A research assistant that can read files, search content, and list directories. Use for read-only exploration tasks.",
	Instruction: "You are a research assistant. Read files, search content, list directories. Report findings concisely.",
	Tools:      []string{"read_file", "glob", "grep", "list_dir"},
	MaxStep:     15,
}

// CoderSubAgent 读写操作子 Agent。
var CoderSubAgent = &SubAgent{
	Name:        "coder",
	Description: "A coding assistant that can execute commands, edit files, and create directories. Use for write operations and code execution.",
	Instruction: "You are a coding assistant. Execute commands, edit files, create directories. Be careful with destructive operations.",
	Tools:       []string{"bash", "read_file", "write_file", "edit_file", "mkdir", "glob", "grep", "list_dir"},
	MaxStep:     15,
}

// DeepResearchSubAgent 深度网络调研子 Agent（提示词驱动）。
var DeepResearchSubAgent = &SubAgent{
	Name:        "deep_research",
	Description: "A deep research assistant that searches the web, fetches pages, and generates knowledge base articles. Use for deep research tasks requiring external information.",
	Instruction: deepResearchInstruction,
	Tools:       []string{"web_search", "web_fetch", "kb"},
	MaxStep:     15,
}

// deepResearchInstruction 深度研究八条原则。
const deepResearchInstruction = `You are a deep research assistant for IT operations. Search the web, fetch pages, and produce structured Markdown articles for the knowledge base. Follow these principles:

1. 结论先行 — 文章先给答案（TL;DR），不是调研过程。读者读完第一段就知道核心结论，方法/过程在后。
2. 搜索不信片段 — 搜索摘要不是证据。web_search 找到线索后，必须 web_fetch 抓取源页面确认内容，不靠 snippet 下结论。
3. 对抗性验证 — 负面断言（"不存在/未发布/不支持"）必须尝试证否。搜不到 ≠ 不存在；无法证实或证伪的标记 UNVERIFIED。
4. 源优先级 — 官方文档 > GitHub repo > 技术博客 > SEO 文章。不靠搜索排名排序结果；优先权威源，丢弃 SEO listicle 和重复内容。
5. 引用在断言处 — 每个关键论断行内标注来源 [1]，frontmatter sources 维护编号→URL 映射。不是末尾 URL 堆砌。
6. 区分事实与推断 — 明确区分"源说了什么"（事实）和"因此我认为"（推断）。推断标记为推断，不确定的标记 UNVERIFIED。
7. 主线贯穿 — 一句研究主线贯穿全文，每个章节服务主线。不服务主线的发现砍掉，不因为"调研了就写"。
8. 避坑清单 — 文章末尾列出负面发现（"X 有 Y 限制"/"Z 方案不适用于 W 场景"），每条带证据或证否痕迹。不只写正面推荐。

产出格式：kb(action=create) 写入知识库时，正文用行内引用 [1][2]，frontmatter sources 含 url+title+accessed。文章默认 Draft 状态，人工审核后 Published 进 RAG。`

// Package llm 自建 LLM 传输层——OpenAI 兼容消息/工具类型 + net/http 客户端。
// 工具描述注入与系统提示词组装完全透明可控。
//
// types.go：消息/工具/参数 schema 类型 + 构造器 + JSON Schema 生成。
package llm

import (
	"encoding/json"
	"sort"
	"strings"
)

// RoleType 消息角色。
type RoleType string

const (
	System    RoleType = "system"
	User      RoleType = "user"
	Assistant RoleType = "assistant"
	Tool      RoleType = "tool"
)

// DataType 参数类型（JSON Schema type 值）。
type DataType string

const (
	Object  DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

// FunctionCall 函数调用载荷。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// ToolCall LLM 发起的工具调用。
// Index 用于流式合并——同一 assistant 消息内多个 tool_call 按 Index 累积 Arguments。
type ToolCall struct {
	Index    *int         `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type"` // 默认 "function"
	Function FunctionCall `json:"function"`
}

// Message 对话消息——OpenAI Chat Completion 兼容。
type Message struct {
	Role             RoleType   `json:"role"`
	Content          string     `json:"content"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"` // 仅 Tool 角色
	ToolName         string     `json:"tool_name,omitempty"`    // 仅 Tool 角色
	ReasoningContent string     `json:"reasoning_content,omitempty"`
}

// SystemMessage 系统提示词消息。
func SystemMessage(content string) *Message {
	return &Message{Role: System, Content: content}
}

// UserMessage 用户消息。
func UserMessage(content string) *Message {
	return &Message{Role: User, Content: content}
}

// AssistantMessage 助手消息（可带 tool_calls）。
func AssistantMessage(content string, toolCalls []ToolCall) *Message {
	return &Message{Role: Assistant, Content: content, ToolCalls: toolCalls}
}

// ToolMessage 工具结果消息（tool_call_id 关联回 tool_use）。
func ToolMessage(content, toolCallID string) *Message {
	return &Message{Role: Tool, Content: content, ToolCallID: toolCallID}
}

// ParameterInfo 工具参数描述。
type ParameterInfo struct {
	Type      DataType                 // 参数类型
	ElemInfo  *ParameterInfo           // 数组元素类型（Type=Array 时）
	SubParams map[string]*ParameterInfo // 对象子参数（Type=Object 时）
	Desc      string                   // 描述
	Enum      []string                 // 枚举值（Type=String 时）
	Required  bool                     // 是否必填
}

// ParamsOneOf 工具参数 schema——params 表示法（map[string]*ParameterInfo）。
// Cognik 工具均用此表示，不使用外部 JSONSchema 包。
type ParamsOneOf struct {
	params map[string]*ParameterInfo
}

// NewParamsOneOfByParams 用参数 map 构造 ParamsOneOf。
func NewParamsOneOfByParams(params map[string]*ParameterInfo) *ParamsOneOf {
	return &ParamsOneOf{params: params}
}

// ToJSONSchema 转为 OpenAI function parameters 的 JSON Schema（map[string]any）。
// 输出形如 {"type":"object","properties":{...},"required":[...]}。
func (p *ParamsOneOf) ToJSONSchema() (map[string]any, error) {
	if p == nil || len(p.params) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}, nil
	}
	properties := make(map[string]any, len(p.params))
	required := make([]string, 0, len(p.params))
	for name, pi := range p.params {
		properties[name] = paramInfoToSchema(pi)
		if pi != nil && pi.Required {
			required = append(required, name)
		}
	}
	sort.Strings(required) // 稳定输出，便于测试与缓存
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema, nil
}

// paramInfoToSchema 单个 ParameterInfo → JSON Schema 片段。
func paramInfoToSchema(pi *ParameterInfo) map[string]any {
	if pi == nil {
		return map[string]any{"type": String}
	}
	schema := map[string]any{"type": pi.Type}
	if pi.Desc != "" {
		schema["description"] = pi.Desc
	}
	if len(pi.Enum) > 0 {
		schema["enum"] = pi.Enum
	}
	if pi.Type == Array && pi.ElemInfo != nil {
		schema["items"] = paramInfoToSchema(pi.ElemInfo)
	}
	if pi.Type == Object && len(pi.SubParams) > 0 {
		subs := make(map[string]any, len(pi.SubParams))
		subReq := make([]string, 0)
		for n, sp := range pi.SubParams {
			subs[n] = paramInfoToSchema(sp)
			if sp != nil && sp.Required {
				subReq = append(subReq, n)
			}
		}
		sort.Strings(subReq)
		schema["properties"] = subs
		if len(subReq) > 0 {
			schema["required"] = subReq
		}
	}
	return schema
}

// ToolInfo 工具元信息（name/desc/params）。
// 嵌入 *ParamsOneOf 携带参数 schema，供 ToOpenAITools 序列化。
type ToolInfo struct {
	Name string
	Desc string
	*ParamsOneOf
}

// OpenAITool OpenAI function-calling tool 定义（序列化用）。
type OpenAITool struct {
	Type     string             `json:"type"` // "function"
	Function OpenAIFunctionDef  `json:"function"`
}

// OpenAIFunctionDef OpenAI function 定义。
type OpenAIFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Params      map[string]any `json:"parameters,omitempty"`
}

// ToOpenAITools 把 ToolInfo 列表转为 OpenAI tools 数组。
// 工具描述 Desc 原样传入 Description——透明可控，无黑盒转换。
func ToOpenAITools(toolInfos []*ToolInfo) ([]OpenAITool, error) {
	tools := make([]OpenAITool, 0, len(toolInfos))
	for _, ti := range toolInfos {
		if ti == nil {
			continue
		}
		params, err := ti.ToJSONSchema()
		if err != nil {
			return nil, err
		}
		tools = append(tools, OpenAITool{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        ti.Name,
				Description: ti.Desc,
				Params:      params,
			},
		})
	}
	return tools, nil
}

// MarshalJSON ToolInfo 序列化（兼容 JSON 持久化，如 session 存储）。
func (t *ToolInfo) MarshalJSON() ([]byte, error) {
	type alias struct {
		Name   string                   `json:"name"`
		Desc   string                   `json:"desc"`
		Params map[string]*ParameterInfo `json:"params,omitempty"`
	}
	a := alias{Name: t.Name, Desc: t.Desc}
	if t.ParamsOneOf != nil {
		a.Params = t.ParamsOneOf.params
	}
	return json.Marshal(a)
}

// ConcatMessages 合并流式 chunks 为一条完整消息。
// content/reasoning 按序拼接；tool_calls 按 Index 累积 Arguments（首个 chunk 给 id+name，
// 后续 chunk 给 arguments 片段）。
func ConcatMessages(msgs []*Message) (*Message, error) {
	if len(msgs) == 0 {
		return &Message{Role: Assistant}, nil
	}
	var content, reasoning strings.Builder
	tcs := map[int]*ToolCall{} // Index → 累积中的 tool_call
	tcsOrder := []int{}        // 按 Index 出现顺序
	role := Assistant
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.Role != "" {
			role = msg.Role
		}
		content.WriteString(msg.Content)
		reasoning.WriteString(msg.ReasoningContent)
		for _, tc := range msg.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			acc, ok := tcs[idx]
			if !ok {
				acc = &ToolCall{Index: &idx, ID: tc.ID, Type: tc.Type}
				if acc.Type == "" {
					acc.Type = "function"
				}
				tcs[idx] = acc
				tcsOrder = append(tcsOrder, idx)
			}
			if tc.ID != "" {
				acc.ID = tc.ID
			}
			if tc.Function.Name != "" {
				acc.Function.Name = tc.Function.Name
			}
			acc.Function.Arguments += tc.Function.Arguments // 累积 JSON 片段
		}
	}
	merged := &Message{Role: role, Content: content.String(), ReasoningContent: reasoning.String()}
	for _, idx := range tcsOrder {
		merged.ToolCalls = append(merged.ToolCalls, *tcs[idx])
	}
	return merged, nil
}

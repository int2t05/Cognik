// Package llm_test 验证自建 LLM 传输层：SSE 流式解析 + tool_calls delta 聚合 + 工具描述转换。
// 无 mock——用真实格式的 SSE 数据（模拟 OpenAI 流），验证增量拼装正确。
package llm_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"cognik/internal/agent/llm"
)

// 仿 OpenAI Chat Completion 流式响应——content 流 + reasoning + tool_calls 跨多块拼 Arguments。
const sseStream = `data: {"choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}

data: {"choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"choices":[{"index":0,"delta":{"content":" world"}}]}

data: {"choices":[{"index":0,"delta":{"reasoning_content":"thinking"}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"kb","arguments":"{\"action\":"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"search\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`

// TestSSEStreamConcat 联合验证 SSE 解析（delta 输出）+ ConcatMessages（聚合）。
// reader 输出 delta，ConcatMessages 按 Index 累积 Arguments，最终得到完整消息。
func TestSSEStreamConcat(t *testing.T) {
	body := io.NopCloser(strings.NewReader(sseStream))
	sr := llm.NewSSEStreamReader(context.Background(), body)
	defer sr.Close()

	var chunks []*llm.Message
	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv err: %v", err)
		}
		chunks = append(chunks, msg)
	}

	merged, err := llm.ConcatMessages(chunks)
	if err != nil {
		t.Fatalf("ConcatMessages err: %v", err)
	}
	if merged.Content != "Hello world" {
		t.Errorf("content = %q, want %q", merged.Content, "Hello world")
	}
	if merged.ReasoningContent != "thinking" {
		t.Errorf("reasoning = %q, want %q", merged.ReasoningContent, "thinking")
	}
	if len(merged.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(merged.ToolCalls))
	}
	tc := merged.ToolCalls[0]
	if tc.ID != "call_1" || tc.Function.Name != "kb" {
		t.Errorf("tool_call id/name = %s/%s, want call_1/kb", tc.ID, tc.Function.Name)
	}
	if want := `{"action":"search"}`; tc.Function.Arguments != want {
		t.Errorf("arguments = %q, want %q", tc.Function.Arguments, want)
	}
}

// TestToOpenAIToolsPreservesDesc 验证工具描述原样进入 tools 字段（透明注入）。
func TestToOpenAIToolsPreservesDesc(t *testing.T) {
	infos := []*llm.ToolInfo{
		{
			Name: "kb",
			Desc: "Knowledge base operations. action: search/get/list.",
			ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
				"action": {Type: llm.String, Desc: "search/get/list", Required: true},
				"kb_id":  {Type: llm.Integer, Desc: "Target KB ID", Required: true},
			}),
		},
	}
	tools, err := llm.ToOpenAITools(infos)
	if err != nil {
		t.Fatalf("ToOpenAITools err: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(tools))
	}
	if tools[0].Function.Name != "kb" {
		t.Errorf("name = %s, want kb", tools[0].Function.Name)
	}
	if tools[0].Function.Description != infos[0].Desc {
		t.Errorf("desc 未原样保留: got %q", tools[0].Function.Description)
	}
	params := tools[0].Function.Params
	if params["type"] != "object" {
		t.Errorf("params type = %v, want object", params["type"])
	}
	req, _ := params["required"].([]string)
	if len(req) != 2 {
		t.Errorf("required len = %d, want 2", len(req))
	}
	props, _ := params["properties"].(map[string]any)
	if _, ok := props["action"]; !ok {
		t.Error("properties 缺 action")
	}
}

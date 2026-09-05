// agent/llm/client.go：OpenAI 兼容 ChatModel——net/http 直连 /v1/chat/completions。
// Generate（非流式）/Stream（SSE 流式）；工具描述通过 ToOpenAITools 透明转成 tools 字段传入请求；
// 流式 tool_calls delta 按 Index 累积 Arguments。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatModelConfig ChatModel 配置（OpenAI 兼容端点）。
type ChatModelConfig struct {
	APIKey  string // API Key（llama.cpp 可空）
	Model   string // 模型名
	BaseURL string // 如 http://localhost:8081/v1（不含 /chat/completions）
	Timeout time.Duration
}

// ChatModel OpenAI 兼容 LLM 客户端。
type ChatModel struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string // 不含尾部 /
}

// NewChatModel 构造客户端。BaseURL 去尾部 /，Timeout 默认 5min。
func NewChatModel(cfg ChatModelConfig) *ChatModel {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	return &ChatModel{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		baseURL:    base,
	}
}

// requestChatCompletion 底层 HTTP 调用——stream 决定是否 SSE。
// 返回 response body（stream=true 调用方负责关闭）或非流式完整 JSON。
func (m *ChatModel) requestChatCompletion(ctx context.Context, messages []*Message, tools []OpenAITool, stream bool) (*http.Response, error) {
	reqBody := m.buildRequestBody(messages, tools, stream)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM 请求失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM 返回 %d: %s", resp.StatusCode, string(raw))
	}
	return resp, nil
}

// requestBody OpenAI Chat Completion 请求体（序列化用）。
type requestBody struct {
	Model    string        `json:"model"`
	Messages []reqMessage  `json:"messages"`
	Tools    []OpenAITool   `json:"tools,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

// reqMessage 请求侧消息（按角色序列化 content/tool_calls/tool_call_id）。
type reqMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []reqTool  `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// reqTool 请求侧 tool_call（省略 Index）。
type reqTool struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function FunctionCall   `json:"function"`
}

// buildRequestBody 组装请求体。
func (m *ChatModel) buildRequestBody(messages []*Message, tools []OpenAITool, stream bool) requestBody {
	msgs := make([]reqMessage, 0, len(messages))
	for _, msg := range messages {
		rm := reqMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			typ := tc.Type
			if typ == "" {
				typ = "function"
			}
			rm.ToolCalls = append(rm.ToolCalls, reqTool{ID: tc.ID, Type: typ, Function: tc.Function})
		}
		msgs = append(msgs, rm)
	}
	return requestBody{Model: m.model, Messages: msgs, Tools: tools, Stream: stream}
}

// Generate 非流式生成。供记忆提取/压缩器/crag 等 forked agent 调用。
func (m *ChatModel) Generate(ctx context.Context, messages []*Message) (*Message, error) {
	resp, err := m.requestChatCompletion(ctx, messages, nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	var cr completionResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w (body: %s)", err, truncate(raw, 500))
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回无 choices (body: %s)", truncate(raw, 500))
	}
	ch := cr.Choices[0]
	msg := &Message{
		Role:             Assistant,
		Content:          ch.Message.Content,
		ToolCalls:        fromRespToolCalls(ch.Message.ToolCalls),
		ReasoningContent: ch.Message.ReasoningContent,
	}
	return msg, nil
}

// Stream 流式生成。返回 StreamReader，调用方 Recv 逐 chunk 读取。
// tools 直接传入请求的 tools 字段，无需单独注册步骤。
func (m *ChatModel) Stream(ctx context.Context, messages []*Message, tools []OpenAITool) (*StreamReader[*Message], error) {
	resp, err := m.requestChatCompletion(ctx, messages, tools, true)
	if err != nil {
		return nil, err
	}
	return NewSSEStreamReader(ctx, resp.Body), nil
}

// SSE 流式响应类型（OpenAI Chat Completion stream）。
type completionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Message      struct {
			Role             string         `json:"role"`
			Content          string         `json:"content"`
			ToolCalls        []respToolCall  `json:"tool_calls"`
			ReasoningContent string         `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// streamChunk 流式 delta（单个 SSE data 块）。
type streamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role             string         `json:"role"`
			Content          string         `json:"content"`
			ToolCalls        []respToolCall  `json:"tool_calls"`
			ReasoningContent string         `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// respToolCall OpenAI 响应侧 tool_call（Index 用于流式合并）。
type respToolCall struct {
	Index    *int        `json:"index,omitempty"`
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function FunctionCall `json:"function"`
}

// fromRespToolCalls 响应 tool_call → 自建 ToolCall（非流式，直接转）。
func fromRespToolCalls(rtcs []respToolCall) []ToolCall {
	out := make([]ToolCall, 0, len(rtcs))
	for _, rtc := range rtcs {
		typ := rtc.Type
		if typ == "" {
			typ = "function"
		}
		out = append(out, ToolCall{Index: rtc.Index, ID: rtc.ID, Type: typ, Function: rtc.Function})
	}
	return out
}

// truncate 截断字节切片用于错误信息。
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}

// StreamReader 泛型流式读取器——channel 实现，提供 Recv/Close 接口。
// Cognos loop.go drainStream 用 Recv() (T, error) + Close()，EOF 检测用 io.EOF。
type StreamReader[T any] struct {
	ch     chan T
	closed chan struct{}
	once   bool
}

// Recv 读取下一个元素。无数据时阻塞；流结束返回 io.EOF。
func (sr *StreamReader[T]) Recv() (T, error) {
	v, ok := <-sr.ch
	if !ok {
		var zero T
		return zero, io.EOF
	}
	return v, nil
}

// Close 关闭读取器。幂等。
func (sr *StreamReader[T]) Close() {
	if sr.once {
		return
	}
	sr.once = true
	close(sr.closed)
}

// NewSSEStreamReader 启动 goroutine 解析 SSE 流，推 Message chunks 到 channel。
// 每个 chunk 产出一条 delta *Message（content/reasoning 片段 + tool_call delta），
// 调用方用 ConcatMessages 聚合为完整消息。
func NewSSEStreamReader(ctx context.Context, body io.ReadCloser) *StreamReader[*Message] {
	sr := &StreamReader[*Message]{
		ch:     make(chan *Message, 32),
		closed: make(chan struct{}),
	}
	go func() {
		defer body.Close()
		defer close(sr.ch)
		scanner := bufio.NewScanner(body)
		// 单行可能较长（大 tool_call arguments），放宽缓冲。
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			select {
			case <-sr.closed:
				return
			case <-ctx.Done():
				return
			default:
			}
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // 跳过无法解析的块（如注释/心跳）
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			ch := chunk.Choices[0]
			msg := &Message{Role: Assistant}
			if ch.Delta.Role != "" {
				msg.Role = RoleType(ch.Delta.Role)
			}
			if ch.Delta.Content != "" {
				msg.Content = ch.Delta.Content
			}
			if ch.Delta.ReasoningContent != "" {
				msg.ReasoningContent = ch.Delta.ReasoningContent
			}
			// tool_calls 输出原始 delta（含 index/id/name/arguments 片段）；
			// 跨 chunk 的 Arguments 累积由 ConcatMessages 按 Index 完成。
			for _, rtc := range ch.Delta.ToolCalls {
				idx := 0
				if rtc.Index != nil {
					idx = *rtc.Index
				}
				typ := rtc.Type
				if typ == "" {
					typ = "function"
				}
				tc := ToolCall{Index: &idx, ID: rtc.ID, Type: typ, Function: rtc.Function}
				msg.ToolCalls = append(msg.ToolCalls, tc)
			}
			select {
			case sr.ch <- msg:
			case <-sr.closed:
				return
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			sr.ch <- &Message{} // 错误以空消息标记（调用方可在 Err 后处理）
		}
	}()
	return sr
}

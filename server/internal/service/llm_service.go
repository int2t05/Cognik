// Package service 实现 RAG 管道 + LLM 调用的统一编排。
//
// LLMService 封装检索→prompt→LLM 流式/同步调用，供 ChatService 统一调度。
// 单独拆分的原因是 ChatService 关注会话生命周期，LLMService 关注 RAG+LLM 编排，两者职责不同。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"opsmind/internal/adapter"
	"opsmind/internal/dto/response"
	"opsmind/internal/rag"
)

// =============================================================================
// 消费者接口
// =============================================================================

// ragPipeline 定义 LLMService 所需的 RAG 管道接口。
// 与 ChatService 的 chatPipeline 等价——各自定义自己需要的子集。
type ragPipeline interface {
	Execute(ctx context.Context, query string, kbID int64, opts rag.RAGOptions, onStep rag.StepCallback) (*rag.RAGResult, error)
}

// =============================================================================
// 流式事件类型
// =============================================================================

// StreamEvent 流式响应中的单个事件，JSON 标签对应 SSE 线格式。
type StreamEvent struct {
	Type     string          `json:"type"`               // "step" | "token" | "done" | "error"
	Content  string          `json:"content,omitempty"`  // token 文本（type=token）
	ID       string          `json:"id,omitempty"`        // 步骤标识（type=step）
	Label    string          `json:"label,omitempty"`     // 步骤显示名（type=step）
	Error    string          `json:"error,omitempty"`     // 错误信息（type=error）
	Metadata *StreamDoneMeta `json:"metadata,omitempty"`  // 完成元数据（type=done）
}

// StreamDoneMeta done 事件携带的会话元数据。
// 对应前端 ChatSessionResponse 接口字段。
type StreamDoneMeta struct {
	SessionID       int64                 `json:"session_id"`
	Question        string                `json:"question"`
	Answer          string                `json:"answer"`
	Sources         []response.SourceItem `json:"sources"`
	Confidence      float64               `json:"confidence"`
	CanSubmitTicket bool                  `json:"can_submit_ticket"`
	DurationMS      int                   `json:"duration_ms"`
	Feedback        int16                 `json:"feedback"`
	CreatedAt       string                `json:"created_at"`
	Pipeline        *ChatPipelineMeta     `json:"pipeline,omitempty"`
}

// =============================================================================
// 管道元数据类型
// =============================================================================

// ChatPipelineMeta 管道执行元数据。
type ChatPipelineMeta struct {
	Steps           []ChatPipelineStep `json:"steps"`
	TotalDurationMS int                `json:"total_duration_ms"`
}

// ChatPipelineStep 管道单步骤耗时。
type ChatPipelineStep struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DurationMS int    `json:"duration_ms"`
}

// =============================================================================
// LLMService
// =============================================================================

// LLMService 封装 RAG + LLM 调用编排。StreamChat 用于 SSE 流式，SyncChat 用于非流式。
type LLMService struct {
	llmClient          adapter.LLMClient
	configMgr          *LLMConfigManager
	defaultModel       string
	pipeline           ragPipeline
	maxHistoryMessages int // 多轮对话历史消息数上限（滑动窗口，默认 10）
}

// NewLLMService 创建 LLMService 实例。
// maxHistoryMessages 控制注入 prompt 的历史消息数上限（0=不限制，默认 10）。
func NewLLMService(llmClient adapter.LLMClient, configMgr *LLMConfigManager, defaultModel string, pipeline ragPipeline, maxHistoryMessages int) *LLMService {
	if maxHistoryMessages <= 0 {
		maxHistoryMessages = 10 // 默认最近 10 条消息（约 5 轮 Q&A）
	}
	return &LLMService{
		llmClient:          llmClient,
		configMgr:          configMgr,
		defaultModel:       defaultModel,
		pipeline:           pipeline,
		maxHistoryMessages: maxHistoryMessages,
	}
}

// SetLLMClient 替换 LLM 客户端（默认配置变更时由回调调用）。
func (s *LLMService) SetLLMClient(client adapter.LLMClient) {
	s.llmClient = client
}

// =============================================================================
// SyncChat — 非流式问答
// =============================================================================

// SyncChatResult 非流式问答的返回结果。
type SyncChatResult struct {
	Answer     string
	Sources    []response.SourceItem
	Confidence float64
	Pipeline   *ChatPipelineMeta
}

// SyncChat 执行 RAG 检索 + LLM 同步生成。
// history 为多轮对话历史，在 RAG 上下文前注入。
func (s *LLMService) SyncChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage) (*SyncChatResult, error) {
	start := time.Now()

	// Step 1: RAG 管道检索
	chunks, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, nil)
	if err != nil {
		return nil, err
	}

	// Step 2: 无检索结果 → 兜底答案
	if len(chunks) == 0 {
		return &SyncChatResult{
			Answer:     "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理。",
			Confidence: 0,
			Pipeline:   pipeMeta,
		}, nil
	}

	// Step 3: LLM 同步生成（仅当 llmClient 可用）
	var answer string
	if s.llmClient != nil {
		messages := s.buildMessages(chunks, question, history)
		model, maxTokens := s.getModelConfig()
		llmResp, llmErr := s.llmClient.ChatCompletion(ctx, adapter.ChatRequest{
			Messages:    messages,
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		})
		if llmErr != nil {
			answer = "AI 服务不可用，请稍后重试或提交申告。"
		} else {
			answer = llmResp.Content
		}
	} else {
		// 无 LLM：返回检索内容摘要
		var sb strings.Builder
		for i, c := range chunks {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c.Content))
		}
		answer = "以下是与您问题相关的知识条目：\n\n" + sb.String()
	}

	// 合并管道耗时与 LLM 生成耗时
	if pipeMeta != nil {
		pipeMeta.Steps = append(pipeMeta.Steps, ChatPipelineStep{
			ID:         "llm_generate",
			Label:      "LLM 生成",
			DurationMS: int(time.Since(start).Milliseconds()) - pipeMeta.TotalDurationMS,
		})
		pipeMeta.TotalDurationMS = int(time.Since(start).Milliseconds())
	}

	return &SyncChatResult{
		Answer:     answer,
		Sources:    extractSources(chunks),
		Confidence: maxConfidence(chunks),
		Pipeline:   pipeMeta,
	}, nil
}

// =============================================================================
// StreamChat — 流式问答
// =============================================================================

// StreamChat 执行 RAG 检索 + LLM 流式生成，返回事件通道供 SSE 代理。
// history 为多轮对话历史，在 RAG 上下文前注入。
func (s *LLMService) StreamChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		start := time.Now()

		// Step 1: RAG 管道检索（实时发送 step 事件到前端）
		onStep := func(evt rag.StepEvent) {
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: evt.ID, Label: evt.Label})
		}
		chunks, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, onStep)
		if err != nil {
			eventCh <- StreamEvent{Type: "error", Error: err.Error()}
			return
		}

		// Step 2: 无检索结果 → 直接发送 done
		if len(chunks) == 0 {
			select {
			case eventCh <- StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
				Answer:          "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理。",
				Confidence:      0,
				CanSubmitTicket: true,
				DurationMS:      int(time.Since(start).Milliseconds()),
			}}:
			case <-ctx.Done():
			}
			return
		}

		// Step 3: LLM 流式生成（仅当 llmClient 可用）
		if s.llmClient == nil {
			// 无 LLM：模拟流式输出检索摘要
			var sb strings.Builder
			for i, c := range chunks {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c.Content))
			}
			answer := "以下是与您问题相关的知识条目：\n\n" + sb.String()
			s.sendSimulated(ctx, eventCh, answer, extractSources(chunks), maxConfidence(chunks), int(time.Since(start).Milliseconds()))
			return
		}

		// 发送 LLM 生成步骤事件
		sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: "llm_generate", Label: "LLM 生成"})

		messages := s.buildMessages(chunks, question, history)
		model, maxTokens := s.getModelConfig()
		tokenCh, llmErr := s.llmClient.ChatCompletionStream(ctx, adapter.ChatRequest{
			Messages:    messages,
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		})
		if llmErr != nil {
			eventCh <- StreamEvent{Type: "error", Error: "LLM 流式调用失败: " + llmErr.Error()}
			return
		}

		// 逐 token 输出 + 缓冲完整答案
		var answerBuf strings.Builder
	streamLoop:
		for chunk := range tokenCh {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if chunk.Error != nil {
				eventCh <- StreamEvent{Type: "error", Error: "LLM 生成中断: " + chunk.Error.Error()}
				return
			}
			if chunk.Content != "" {
				answerBuf.WriteString(chunk.Content)
				if ok := sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: chunk.Content}); !ok {
					return
				}
			}
			if chunk.FinishReason != "" {
				break streamLoop
			}
		}

		// 合并管道耗时
		fullAnswer := answerBuf.String()
		sources := extractSources(chunks)
		confidence := maxConfidence(chunks)
		durationMS := int(time.Since(start).Milliseconds())
		if pipeMeta != nil {
			pipeMeta.TotalDurationMS = durationMS
		}

		sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
			Answer:          fullAnswer,
			Sources:         sources,
			Confidence:      confidence,
			CanSubmitTicket: confidence < 0.6,
			DurationMS:      durationMS,
			Pipeline:        pipeMeta,
		}})
	}()

	return eventCh, nil
}

// =============================================================================
// 内部方法
// =============================================================================

// executeRAG 执行 RAG 管道检索，返回 chunk 列表和管道指标。
//
// 第二个返回值 pipelineMeta 可能为 nil（pipeline 不可用时）。
func (s *LLMService) executeRAG(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, onStep rag.StepCallback) ([]rag.RetrievalResult, *ChatPipelineMeta, error) {
	if s.pipeline == nil {
		return nil, nil, nil
	}

	var steps []ChatPipelineStep
	start := time.Now()

	result, err := s.pipeline.Execute(ctx, question, kbID, opts, onStep)
	if err != nil {
		return nil, nil, fmt.Errorf("知识检索失败: %w", err)
	}

	if result != nil {
		// 转换 StepMetric → ChatPipelineStep
		for _, m := range result.Metrics.Steps {
			steps = append(steps, ChatPipelineStep{
				ID:         m.StepID,
				Label:      m.Label,
				DurationMS: int(m.DurationMS),
			})
		}
		return result.Chunks, &ChatPipelineMeta{
			Steps:           steps,
			TotalDurationMS: int(time.Since(start).Milliseconds()),
		}, nil
	}

	return nil, nil, nil
}

// buildMessages 将 RAG chunk 和历史对话构建为 LLM 请求消息。
// history 按滑动窗口截断（maxHistoryMessages 控制），避免长对话超出上下文窗口。
// 系统提示词优先使用 LLM 配置中的 SystemPrompt，为空时回退到默认提示词。
func (s *LLMService) buildMessages(chunks []rag.RetrievalResult, question string, history []adapter.ChatMessage) []adapter.ChatMessage {
	systemPrompt := "你是一个运维知识助手。根据以下知识库内容回答用户问题。如果知识库中没有相关信息，请如实说明。"
	if s.configMgr != nil {
		if cfg := s.configMgr.GetConfig(); cfg != nil && cfg.SystemPrompt != "" {
			systemPrompt = cfg.SystemPrompt
		}
	}
	var ctxBuilder strings.Builder
	for i, chunk := range chunks {
		ctxBuilder.WriteString(fmt.Sprintf("【参考资料 %d】%s\n", i+1, chunk.Content))
	}

	msgs := []adapter.ChatMessage{
		{Role: "system", Content: systemPrompt},
	}

	// 滑动窗口截断历史消息：只保留最近 N 条
	if s.maxHistoryMessages > 0 && len(history) > s.maxHistoryMessages {
		history = history[len(history)-s.maxHistoryMessages:]
	}
	for _, h := range history {
		msgs = append(msgs, h)
	}

	msgs = append(msgs, adapter.ChatMessage{
		Role: "user", Content: fmt.Sprintf("知识库内容：\n%s\n\n用户问题：%s", ctxBuilder.String(), question),
	})

	return msgs
}

// getModelConfig 从 LLMConfigManager 读取当前模型和 maxTokens。
//
// 优先级：DB 热配置 > config.yaml 默认值。configMgr 为 nil 或 DB 无配置时回退到 defaultModel。
// defaultModel 来自 config.yaml OPSMIND_LLM_MODEL，运行时不依赖硬编码字符串。
func (s *LLMService) getModelConfig() (model string, maxTokens int) {
	model = s.defaultModel
	maxTokens = 2048
	if s.configMgr != nil {
		if cfg := s.configMgr.GetConfig(); cfg != nil {
			if cfg.LLMModel != "" {
				model = cfg.LLMModel
			}
			if cfg.MaxTokens > 0 {
				maxTokens = cfg.MaxTokens
			}
		}
	}
	return
}

// sendSimulated 无 LLM 时的模拟流式输出（检索内容摘要）。
func (s *LLMService) sendSimulated(ctx context.Context, eventCh chan<- StreamEvent, answer string, sources []response.SourceItem, confidence float64, durationMS int) {
	// 发送 LLM 生成步骤事件
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: "llm_generate", Label: "LLM 生成"})

	runes := []rune(answer)
	chunkSize := 5
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		if ok := sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: string(runes[i:end])}); !ok {
			return
		}
	}
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
		Answer:          answer,
		Sources:         sources,
		Confidence:      confidence,
		CanSubmitTicket: confidence < 0.6,
		DurationMS:      durationMS,
	}})
}

// =============================================================================
// 公共辅助函数
// =============================================================================

// sendOrCancel 向 channel 发送事件，同时监听 ctx 取消。
// 返回 false 表示 ctx 已取消，调用方应停止后续发送。
func sendOrCancel(ctx context.Context, ch chan<- StreamEvent, evt StreamEvent) bool {
	select {
	case ch <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

// extractSources 从检索结果中提取前端展示用的来源列表。
func extractSources(chunks []rag.RetrievalResult) []response.SourceItem {
	sources := make([]response.SourceItem, 0, len(chunks))
	for _, c := range chunks {
		sources = append(sources, response.SourceItem{
			DocName:      fmt.Sprintf("chunk_%d", c.ChunkID),
			ChunkContent: c.Content,
			Confidence:   c.Score,
		})
	}
	return sources
}

// maxConfidence 取检索结果中的最高相关度分数，钳位到 [0,1]。
//
// 为什么需要钳位：BM25 原始分数无理论上界，RRF 融合后分数在 ~[0,1] 范围。
// 多路检索场景下 RRF 已归一化处理；单路向量检索的 cosine 分数本就 ≤1。
// 钳位确保不同检索路径的 confidence 在统一量纲下比较。
func maxConfidence(chunks []rag.RetrievalResult) float64 {
	var max float64
	for _, c := range chunks {
		if c.Score > max {
			max = c.Score
		}
	}
	if max > 1 {
		max = 1
	}
	return max
}

// Package service 实现 RAG 管道 + LLM 调用的统一编排。
//
// LLMService 封装检索→prompt→LLM 流式/同步调用，供 ChatService 统一调度。
// 单独拆分的原因是 ChatService 关注会话生命周期，LLMService 关注 RAG+LLM 编排，两者职责不同。
package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
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
	Type     string             `json:"type"`               // "step" | "chunks" | "token" | "done" | "error"
	Seq      int                `json:"seq"`                // 生成内单调递增序号，用于断点续传去重
	Content  string             `json:"content,omitempty"`  // token 文本（type=token）或 reasoning 内容
	ID       string             `json:"id,omitempty"`        // 步骤标识（type=step）
	Label    string             `json:"label,omitempty"`     // 步骤显示名（type=step）
	Error    string             `json:"error,omitempty"`     // 错误信息（type=error）
	Chunks   []rag.ChunkDisplay `json:"chunks,omitempty"`   // 检索 chunk 展示分（type=chunks）
	Metadata *StreamDoneMeta    `json:"metadata,omitempty"`  // 完成元数据（type=done）
}

// StreamDoneMeta done 事件携带的会话元数据。
// 对应前端 ChatSessionResponse 接口字段。
type StreamDoneMeta struct {
	SessionID         int64                 `json:"session_id"`
	Question          string                `json:"question"`
	Answer            string                `json:"answer"`
	Sources           []response.SourceItem `json:"sources"`
	Confidence        float64               `json:"confidence"`
	ConfidenceRaw     float64               `json:"confidence_raw"`
	ConfidenceLevel   string                `json:"confidence_level"`
	CanSubmitTicket   bool                  `json:"can_submit_ticket"`
	DurationMS        int                   `json:"duration_ms"`
	Feedback          int16                 `json:"feedback"`
	CreatedAt         string                `json:"created_at"`
	Pipeline           *ChatPipelineMeta     `json:"pipeline,omitempty"`
	UserMessageID      int64                 `json:"user_message_id,omitempty"`
	AssistantMessageID int64                 `json:"assistant_message_id,omitempty"`
}

// =============================================================================
// 管道元数据类型
// =============================================================================

// ChatPipelineMeta 管道执行元数据。
type ChatPipelineMeta struct {
	Steps           []ChatPipelineStep `json:"steps"`
	TotalDurationMS int                `json:"total_duration_ms"`
}

// ChatPipelineStep 管道单步骤耗时与状态。
type ChatPipelineStep struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DurationMS int    `json:"duration_ms"`
	Success    bool   `json:"success"`
}

// =============================================================================
// LLMService
// =============================================================================

// LLMService 封装 RAG + LLM 调用编排。StreamChat 用于 SSE 流式，SyncChat 用于非流式。
type LLMService struct {
	mu                 sync.Mutex
	llmClient          adapter.LLMClient
	configMgr          *LLMConfigManager
	defaultModel       string
	pipeline           ragPipeline
	embedder           *rag.Embedder // 用于 S_qa 答案向量化
	maxHistoryMessages int           // 多轮对话历史消息数上限（滑动窗口，默认 10）
	configWarnOnce     sync.Once     // 缺少 DB 配置时仅 Warn 一次
}

// NewLLMService 创建 LLMService 实例。
// maxHistoryMessages 控制注入 prompt 的历史消息数上限（0=不限制，默认 10）。
func NewLLMService(llmClient adapter.LLMClient, configMgr *LLMConfigManager, defaultModel string, pipeline ragPipeline, embedder *rag.Embedder, maxHistoryMessages int) *LLMService {
	if maxHistoryMessages <= 0 {
		maxHistoryMessages = 10 // 默认最近 10 条消息（约 5 轮 Q&A）
	}
	return &LLMService{
		llmClient:          llmClient,
		configMgr:          configMgr,
		defaultModel:       defaultModel,
		pipeline:           pipeline,
		embedder:           embedder,
		maxHistoryMessages: maxHistoryMessages,
	}
}

// SetLLMClient 替换 LLM 客户端（默认配置变更时由回调调用）。
func (s *LLMService) SetLLMClient(client adapter.LLMClient) {
	s.mu.Lock()
	s.llmClient = client
	s.mu.Unlock()
}

// getLLMClient 线程安全地获取当前 LLM 客户端。
func (s *LLMService) getLLMClient() adapter.LLMClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.llmClient
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
	ragResult, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, nil)
	if err != nil {
		return nil, err
	}

	chunks := ragResult.Chunks

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
	if client := s.getLLMClient(); client != nil {
		messages := s.buildMessages(chunks, question, history)
		model, maxTokens := s.getModelConfig()
		llmResp, llmErr := client.ChatCompletion(ctx, adapter.ChatRequest{
			Messages:    messages,
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		})
		if llmErr != nil {
			answer = "当前 AI 服务暂不可用，请提交申告由人工处理"
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

	confRaw, _ := s.computeConfidence(chunks, ragResult.QuestionEmbedding, answer)

	return &SyncChatResult{
		Answer:     answer,
		Sources:    extractSources(chunks),
		Confidence: confRaw,
		Pipeline:   pipeMeta,
	}, nil
}

// =============================================================================
// StreamChat — 流式问答
// =============================================================================

// 降级常量：RAG 不可用/无结果时的提示前缀和 LLM 不可用时的固定回复。
const (
	ragDisabledNotice  = "\n\n> ⚠️ 当前已关闭知识库检索，以下回答由 AI 直接生成，可能不够准确。\n\n"
	ragNoResultNotice  = "\n\n> ⚠️ 当前暂未找到足够匹配的知识，以下回答由 AI 直接生成，仅供参考。\n\n"
	llmUnavailableText = "抱歉，当前 AI 服务暂不可用。建议您：\n1. 稍后重试\n2. 提交运维申告由人工处理\n3. 联系运维团队获取帮助"
)

// StreamChat 执行 RAG 检索 + LLM 流式生成，返回事件通道供 SSE 代理。
//
// 降级策略（三级）：
//  1. RAG 可用 → 正常检索+生成
//  2. RAG 禁用/无结果 → 发送提示 notice token → 直接 LLM 生成（无知识上下文）
//  3. LLM 不可用 → 返回固定降级语句
//
// history 为多轮对话历史，在 RAG 上下文前注入。
func (s *LLMService) StreamChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage, enableThinking bool) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		start := time.Now()

		// Step 1: RAG 管道检索（实时发送 step 事件到前端）
		onStep := func(evt rag.StepEvent) {
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: evt.ID, Label: evt.Label})
		}
		ragResult, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, onStep)
		if err != nil {
			// RAG 管道失败 → 降级尝试无知识库 LLM
			slog.Warn("RAG 检索失败，降级为纯 LLM 模式", "error", err)
			ragResult = &rag.RAGResult{} // 确保走 LLM-only 分支
		}
			chunks := ragResult.Chunks

		// 判断是否需要发送降级提示
			ragDisabled := opts.DisableRetrieval
			ragEmpty := len(chunks) == 0
			if ragDisabled || ragEmpty {
				var notice string
				if ragDisabled {
					notice = ragDisabledNotice
				} else {
					notice = ragNoResultNotice
				}
			s.sendNoticeToken(ctx, eventCh, notice)
				sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
					Answer:           "当前未找到足够匹配的知识，无法生成回答。",
					Confidence:        0,
					ConfidenceRaw:     0,
					ConfidenceLevel:   "low",
					CanSubmitTicket:   true,
					DurationMS:        int(time.Since(start).Milliseconds()),
				}})
				return
			}

			// 发送 chunks SSE 事件（检索完成后、LLM 生成前）
			if len(ragResult.ChunkDisplays) > 0 {
				sendOrCancel(ctx, eventCh, StreamEvent{Type: "chunks", Chunks: ragResult.ChunkDisplays})
			}

			// Step 2: LLM 流式生成
		// Step 2: LLM 流式生成
		if s.getLLMClient() == nil {
			s.sendFallback(ctx, eventCh, start)
			return
		}

		sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: "llm_generate", Label: "LLM 生成"})

		messages := s.buildMessages(chunks, question, history)
		model, maxTokens := s.getModelConfig()
		tokenCh, llmErr := s.getLLMClient().ChatCompletionStream(ctx, adapter.ChatRequest{
			Messages:       messages,
			Model:          model,
			MaxTokens:      maxTokens,
			Temperature:    0.3,
			EnableThinking: enableThinking,
		})
		if llmErr != nil {
			// LLM 不可用 → 降级固定回复
			slog.Error("LLM 流式调用失败，降级固定回复", "error", llmErr)
			s.sendFallback(ctx, eventCh, start)
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
				if answerBuf.Len() > 0 {
					partialAnswer := answerBuf.String()
					confRaw, confLevel := s.computeConfidence(chunks, ragResult.QuestionEmbedding, partialAnswer)
					sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
						Answer:          answerBuf.String(),
						Sources:         extractSources(chunks),
						Confidence:       confRaw,
						ConfidenceRaw:    confRaw,
						ConfidenceLevel:  confLevel,
						CanSubmitTicket:  confLevel != "high",
						DurationMS:      int(time.Since(start).Milliseconds()),
					}})
				} else {
					s.sendFallback(ctx, eventCh, start)
				}
				return
			}
			if chunk.Reasoning != "" {
				// 思考内容透传到前端（保持 SSE 连接活跃，前端显示"思考中..."）
				sendOrCancel(ctx, eventCh, StreamEvent{Type: "reasoning", Content: chunk.Reasoning})
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

		fullAnswer := answerBuf.String()
		// 空回答降级为固定回复
		if strings.TrimSpace(fullAnswer) == "" {
			s.sendFallback(ctx, eventCh, start)
			return
		}

		sources := extractSources(chunks)
		confRaw, confLevel := s.computeConfidence(chunks, ragResult.QuestionEmbedding, fullAnswer)
		durationMS := int(time.Since(start).Milliseconds())
		if pipeMeta != nil {
			pipeMeta.TotalDurationMS = durationMS
		}

		sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
			Answer:          fullAnswer,
			Sources:         sources,
			Confidence:       confRaw,
			ConfidenceRaw:    confRaw,
			ConfidenceLevel:  confLevel,
			CanSubmitTicket:  confLevel != "high",
			DurationMS:      durationMS,
			Pipeline:        pipeMeta,
		}})
	}()

	return eventCh, nil
}

// sendNoticeToken 发送降级提示 notice 作为 token 事件（前端显示为灰色引文）。
func (s *LLMService) sendNoticeToken(ctx context.Context, eventCh chan<- StreamEvent, notice string) {
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: notice})
}

// sendFallback 发送 LLM 不可用时的固定降级回复。
func (s *LLMService) sendFallback(ctx context.Context, eventCh chan<- StreamEvent, start time.Time) {
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: llmUnavailableText})
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
		Answer:          llmUnavailableText,
		Confidence:      0,
		ConfidenceRaw:    0,
		ConfidenceLevel:  "low",
		CanSubmitTicket: true,
		DurationMS:      int(time.Since(start).Milliseconds()),
	}})
}

// =============================================================================
// 内部方法
// =============================================================================

// executeRAG 执行 RAG 管道检索，返回完整 RAGResult 和管道指标。
//
// 第二个返回值 pipelineMeta 可能为 nil（pipeline 不可用时）。
func (s *LLMService) executeRAG(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, onStep rag.StepCallback) (*rag.RAGResult, *ChatPipelineMeta, error) {
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
				Success:    m.Success,
			})
		}
		return result, &ChatPipelineMeta{
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
	systemPrompt := "你是企业运维知识助手。严格仅根据下方「知识库内容」回答，禁止使用外部知识。\n\n规则：\n1. 每条事实后必须标注来源编号，如 [1]、[2]。无编号的回答视为无效\n2. 知识库有答案 → 原文复述，不编造细节\n3. 知识库无相关信息 → 只回复「当前知识库未收录此问题，建议提交申告由运维人员处理」\n4. 回答简洁，列表/步骤优先，不闲聊"
	if s.configMgr != nil {
		if cfg := s.configMgr.GetConfig(); cfg != nil && cfg.SystemPrompt != "" {
			systemPrompt = cfg.SystemPrompt
		}
	}
	var ctxBuilder strings.Builder
	for i, chunk := range chunks {
		ctxBuilder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, chunk.Content))
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
// 优先级：DB 热配置 > config.yaml 默认值。configMgr 为 nil 或 DB 无默认配置时回退到 defaultModel。
// 缺少 DB 配置时仅 Warn 一次（sync.Once），避免每条消息重复刷屏。
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
			return
		}
	}
	s.configWarnOnce.Do(func() {
		slog.Info("LLM 配置使用 config.yaml 默认值（DB 中未设置默认 LLM 配置）", "model", model)
	})
	return
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

// extractSources 用综合置信度生成前端展示用的来源列表。
//
// 每个 source 的 Confidence 取自 chunk.ConfRaw（综合置信度 [0,1]），
// 而非原始余弦相似度，确保前端展示与后端评分逻辑一致。
// Sources 以 JSONB 落库到 chat_messages.sources，前端刷新后读到持久化的一致分数。
func extractSources(chunks []rag.RetrievalResult) []response.SourceItem {
	sources := make([]response.SourceItem, len(chunks))
	for i, c := range chunks {
		sources[i] = response.SourceItem{
			DocName:      fmt.Sprintf("来源 %d", i+1),
			ChunkContent: c.Content,
			Confidence:   c.ConfRaw,
		}
	}
	return sources
}

// =============================================================================
// 置信度计算
// =============================================================================

// computeConfidence 计算原始综合分和置信度等级。
//
// 公式：Conf_raw = α × S_retrieval + (1-α) × S_qa
// α 根据答案长度动态调整（短答案 S_qa 噪声大，降低权重）。
// 空答案强制 Conf_raw=0, level="low"。
func (s *LLMService) computeConfidence(chunks []rag.RetrievalResult, questionEmb []float32, answer string) (float64, string) {
	if strings.TrimSpace(answer) == "" {
		return 0, "low"
	}

	sRetrieval := computeSRetrieval(chunks)
	sQA := 0.0
	if len(questionEmb) > 0 && s.embedder != nil {
		sQA = computeSQA(s.embedder, questionEmb, answer)
	}

	alpha := answerLenAlpha(len([]rune(answer)))
	confRaw := alpha*sRetrieval + (1-alpha)*sQA

	// 精度钳位
	if confRaw < 0 {
		confRaw = 0
	}
	if confRaw > 1 {
		confRaw = 1
	}

	// 阈值默认值：上线后通过分位数计算校准
	const defaultLowT, defaultHighT = 0.40, 0.70
	level := "low"
	if confRaw >= defaultHighT {
		level = "high"
	} else if confRaw >= defaultLowT {
		level = "medium"
	}

	return confRaw, level
}

// computeSRetrieval 计算检索聚合分（综合置信度 ConfRaw 的排名加权平均）。
//
// Pipeline 已通过 computeConfidenceScores 计算了每个 chunk 的综合置信度 ConfRaw，
// 这里仅做排名加权的聚合——排名靠前的 chunk 对整体置信度贡献更大。
//
// S_retrieval = Σ(w_i × ConfRaw_i) / Σ(w_i)
// w_i = 1/(rank_i + 1)，rank 从 0 开始。
//
// 为什么用 ConfRaw 而非 RawCosineScore：
// ConfRaw 已综合了 S_qa、BM25、Rerank 三层信号，是 chunk 级的最终置信度。
// 用 ConfRaw 聚合得到的是管道全貌的检索质量评估。
func computeSRetrieval(chunks []rag.RetrievalResult) float64 {
	if len(chunks) == 0 {
		return 0
	}

	var sumWeighted, sumWeights float64
	for i, c := range chunks {
		w := 1.0 / float64(i+1) // rank 从 0 开始，首位权重最高
		score := c.ConfRaw
		if score == 0 {
			// ConfRaw 为 0 时回退到 RawCosineScore（兜底：computeConfidenceScores
			// 仅在有 BM25/Rerank 信号时才覆盖 ConfRaw）
			score = c.RawCosineScore
		}
		sumWeighted += w * score
		sumWeights += w
	}
	if sumWeights == 0 {
		return 0
	}
	return sumWeighted / sumWeights
}

// computeSQA 计算问答匹配分：question 与 answer embedding 的余弦相似度。
//
// 对 answer 调用一次 embedding，与已缓存的 question embedding 比较。
// embedding 失败时降级返回 0（不阻塞主流程）。
func computeSQA(embedder *rag.Embedder, questionEmb []float32, answer string) float64 {
	vecs, _, err := embedder.Embed(context.Background(), []string{answer}, "")
	if err != nil || len(vecs) == 0 {
		slog.Warn("S_qa embedding 失败，降级为 0", "error", err)
		return 0
	}
	return cosineSimilarity(questionEmb, vecs[0])
}

// cosineSimilarity 计算两个 float32 向量的余弦相似度。
func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// answerLenAlpha 根据答案长度调整 α 权重。
//
// 短答案 embedding 噪声大，降低 S_qa 权重：
//
//	≥20 字符 → α=0.7（正常）
//	5~19 字符 → α=0.85（降权）
//	<5 字符 → α=1.0（S_qa 不计入）
func answerLenAlpha(length int) float64 {
	switch {
	case length >= 20:
		return 0.7
	case length >= 5:
		return 0.85
	default:
		return 1.0
	}
}


// Package rag 实现自建 RAG 检索引擎。
//
// crag.go：CRAG（Corrective RAG）充分性评估——检索后评估结果质量，决定是否补充搜索。
// 参考 Yan et al. 2024 (arxiv 2401.15884)：轻量 evaluator → strong/ambiguous/weak 三态。
// strong 直通生成（零额外成本）；weak 触发 Agent web_search fallback（经 verdict 文本信号）。
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cognik/internal/agent/llm"
)

// VerdictLevel CRAG 评估三态。
type VerdictLevel string

const (
	VerdictStrong     VerdictLevel = "strong"     // 检索充分，直接生成
	VerdictAmbiguous  VerdictLevel = "ambiguous"  // 边界，可选 LLM 二次判定
	VerdictWeak       VerdictLevel = "weak"       // 检索不足，建议 web_search 补充
)

// Verdict CRAG 评估结果。
type Verdict struct {
	Level      VerdictLevel // 三态
	Sufficient bool         // strong=true，其余 false
	Confidence float64      // top-1 ConfRaw
	Reason     string       // 人类可读理由
}

// SufficiencyEvaluator 检索充分性评估器接口。
// 实现：ThresholdEvaluator（纯函数，零成本）+ 可选 LLM 评估器（仅 Ambiguous 带）。
type SufficiencyEvaluator interface {
	// Evaluate 按检索结果 + query 产出 Verdict。query 供 LLM 评估器使用，阈值评估器忽略。
	Evaluate(ctx context.Context, query string, results []RetrievalResult) (Verdict, error)
}

// ThresholdEvaluator 阈值评估器——纯函数，基于 top-1 ConfRaw 与 low/high 阈值比较。
// 复用 ai.confidence_threshold_low/high（默认 0.40/0.70）。无 LLM 调用，强路径零成本。
type ThresholdEvaluator struct {
	Low  float64 // 下界：< Low → weak
	High float64 // 上界：>= High → strong
}

// NewThresholdEvaluator 创建阈值评估器。
func NewThresholdEvaluator(low, high float64) *ThresholdEvaluator {
	if low <= 0 {
		low = 0.40
	}
	if high <= 0 {
		high = 0.70
	}
	if high <= low {
		high = low + 0.10
	}
	return &ThresholdEvaluator{Low: low, High: high}
}

// Evaluate 按 top-1 ConfRaw 产出 Verdict。
// 空结果 → weak；ConfRaw>=High → strong；>=Low → ambiguous；否则 weak。
func (e *ThresholdEvaluator) Evaluate(_ context.Context, _ string, results []RetrievalResult) (Verdict, error) {
	if len(results) == 0 {
		return Verdict{Level: VerdictWeak, Sufficient: false, Confidence: 0, Reason: "检索结果为空"}, nil
	}
	topConf := results[0].ConfRaw
	if results[0].ConfRaw == 0 {
		// 兜底：ConfRaw 未计算时用 Score
		topConf = results[0].Score
	}
	switch {
	case topConf >= e.High:
		return Verdict{Level: VerdictStrong, Sufficient: true, Confidence: topConf,
			Reason: fmt.Sprintf("置信度 %.2f >= 阈值 %.2f，检索充分", topConf, e.High)}, nil
	case topConf >= e.Low:
		return Verdict{Level: VerdictAmbiguous, Sufficient: false, Confidence: topConf,
			Reason: fmt.Sprintf("置信度 %.2f 在边界 [%.2f, %.2f)，建议核实", topConf, e.Low, e.High)}, nil
	default:
		return Verdict{Level: VerdictWeak, Sufficient: false, Confidence: topConf,
			Reason: fmt.Sprintf("置信度 %.2f < 阈值 %.2f，检索不足", topConf, e.Low)}, nil
	}
}

// ChainEvaluator 链式评估器——阈值先判；仅 Ambiguous 且 LLM 评估器非 nil 时调 LLM。
// Strong/Weak 走阈值（强路径零成本）；LLM 失败降级阈值（永不阻塞）。
type ChainEvaluator struct {
	threshold *ThresholdEvaluator
	llm       SufficiencyEvaluator // 可选；nil 时退化为纯阈值
}

// NewChainEvaluator 创建链式评估器。
func NewChainEvaluator(threshold *ThresholdEvaluator, llm SufficiencyEvaluator) *ChainEvaluator {
	return &ChainEvaluator{threshold: threshold, llm: llm}
}

// Evaluate 阈值先判；Ambiguous 带 LLM 二次判定（失败降级阈值）。
func (c *ChainEvaluator) Evaluate(ctx context.Context, query string, results []RetrievalResult) (Verdict, error) {
	v, err := c.threshold.Evaluate(ctx, query, results)
	if err != nil || v.Level != VerdictAmbiguous || c.llm == nil {
		return v, err
	}
	// Ambiguous 且 LLM 评估器就绪 → 二次判定（失败降级阈值，不阻塞）
	llmV, lerr := c.llm.Evaluate(ctx, query, results)
	if lerr != nil {
		return v, nil // 降级阈值结果
	}
	return llmV, nil
}

// LLMCRAGEvaluator 基于 LLM 的 CRAG 评估器（结构化输出 strong/ambiguous/weak + reason）。
// 廉价 LLM 调用；失败降级阈值（由 ChainEvaluator 捕获，永不阻塞检索返回）。
// 仅对 Ambiguous 带触发（ChainEvaluator 控制），强路径零成本。
type LLMCRAGEvaluator struct {
	modelGetter func() *llm.ChatModel
}

// NewLLMCRAGEvaluator 创建 LLM CRAG 评估器。
func NewLLMCRAGEvaluator(modelGetter func() *llm.ChatModel) *LLMCRAGEvaluator {
	return &LLMCRAGEvaluator{modelGetter: modelGetter}
}

// cragLLMResponse LLM 结构化输出契约。
type cragLLMResponse struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

// Evaluate 用廉价 LLM 判定检索充分性（取 top-3 chunk + score 作输入）。
func (e *LLMCRAGEvaluator) Evaluate(ctx context.Context, query string, results []RetrievalResult) (Verdict, error) {
	if e == nil || e.modelGetter == nil {
		return Verdict{Level: VerdictAmbiguous, Reason: "LLM 评估器未就绪"}, nil
	}
	m := e.modelGetter()
	if m == nil {
		return Verdict{Level: VerdictAmbiguous, Reason: "ChatModel 未初始化"}, nil
	}

	// 取 top-3 chunk（避免 prompt 过长 + 成本）
	topN := 3
	if len(results) < topN {
		topN = len(results)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("查询: %s\n\n检索结果（按相关性降序）:", query))
	for i := 0; i < topN; i++ {
		r := results[i]
		excerpt := r.Content
		if len([]rune(excerpt)) > 200 {
			excerpt = string([]rune(excerpt)[:200]) + "..."
		}
		sb.WriteString(fmt.Sprintf("\n[%d] score=%.3f\n%s", i+1, r.ConfRaw, excerpt))
	}

	prompt := fmt.Sprintf(`你是检索质量评估器。判断以下检索结果对查询的充分性。
仅用 JSON 回答，格式 {"level": "strong|ambiguous|weak", "reason": "一句话理由"}。
- strong: 结果直接覆盖查询意图，可据此回答
- ambiguous: 部分相关，可能需要补充
- weak: 结果与查询无关或明显不足

%s`, sb.String())

	resp, err := m.Generate(ctx, []*llm.Message{
		llm.SystemMessage(prompt),
		llm.UserMessage("评估检索充分性。"),
	})
	if err != nil {
		return Verdict{}, fmt.Errorf("CRAG LLM 评估失败: %w", err)
	}

	// 解析 JSON（容错：LLM 可能包裹 ```json）
	level, reason := parseCRAGJSON(resp.Content)
	conf := 0.0
	if len(results) > 0 {
		conf = results[0].ConfRaw
	}
	return Verdict{
		Level:      VerdictLevel(level),
		Sufficient: level == "strong",
		Confidence: conf,
		Reason:     reason,
	}, nil
}

// parseCRAGJSON 容错解析 LLM 输出的 JSON（剥离 ```json 围栏）。
func parseCRAGJSON(s string) (level, reason string) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	var r cragLLMResponse
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return "ambiguous", "LLM 输出解析失败，默认 ambiguous"
	}
	if r.Level != "strong" && r.Level != "ambiguous" && r.Level != "weak" {
		r.Level = "ambiguous"
	}
	if r.Reason == "" {
		r.Reason = "LLM 未给出理由"
	}
	return r.Level, r.Reason
}

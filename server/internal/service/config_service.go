// Package service 实现系统配置管理业务逻辑。
//
// ConfigService 提供系统配置的获取和更新功能。
// 支持白名单内的配置键读写，拒绝未知 key。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// configKeyMeta 定义配置键的元信息：期望类型和用途说明。
type configKeyMeta struct {
	ValueType   string // "string" | "number" | "bool"
	Description string // 配置项说明，写入 system_configs.description
}

// validConfigKeys 配置键白名单。
//
// 为什么用白名单而非自由 key-value：
// 自由 key-value 允许调用方写入任意键名，拼写错误导致静默创建无用配置项，
// 且前端无法区分「配置不存在」和「配置类型不符」。
var validConfigKeys = map[string]configKeyMeta{
	"app_name":                {ValueType: "string", Description: "应用名称，显示在页面标题和系统通知中"},
	"ai.rag_enabled":          {ValueType: "bool", Description: "全局 RAG 检索开关（关闭后为纯 LLM 对话模式）"},
	"ai.top_k":                {ValueType: "number", Description: "RAG 默认检索 Top K"},
	"ai.confidence_threshold_low":   {ValueType: "number", Description: "低置信阈值——Conf_raw 低于此值为低置信"},
	"ai.confidence_threshold_high":  {ValueType: "number", Description: "高置信阈值——Conf_raw 达到此值为高置信"},
	"ai.max_history_messages": {ValueType: "number", Description: "多轮对话历史消息数上限"},
	"ai.rag_query_rewrite":    {ValueType: "bool", Description: "RAG 查询改写开关"},
	"ai.rag_multi_route":      {ValueType: "bool", Description: "RAG 多路检索开关"},
	"ai.rag_hybrid":           {ValueType: "bool", Description: "RAG BM25 混合检索开关"},
	"ai.rag_rerank":           {ValueType: "bool", Description: "RAG 重排序开关"},
	"ai.enable_thinking":      {ValueType: "bool", Description: "流式回答启用思考模式（推理链提升质量但延迟 5-10x）"},
}

// ConfigService 系统配置管理服务。
type ConfigService struct {
	repo      *repository.ConfigRepo
	auditRepo *repository.AuditRepo
	chatRepo  confidenceScoreQuerier
}

// confidenceScoreQuerier 分位数计算所需的置信度分数查询接口。
type confidenceScoreQuerier interface {
	QueryRawScores(ctx context.Context, days int) ([]float64, error)
}

// SetChatRepo 注入 chat 仓库依赖（避免构造循环依赖）。
func (s *ConfigService) SetChatRepo(r confidenceScoreQuerier) {
	s.chatRepo = r
}

// NewConfigService 创建 ConfigService 实例。
func NewConfigService(repo *repository.ConfigRepo, auditRepo *repository.AuditRepo) *ConfigService {
	return &ConfigService{repo: repo, auditRepo: auditRepo}
}

// GetInt 读取整数配置，不存在或类型不匹配返回 (0, false)。
func (s *ConfigService) GetInt(ctx context.Context, key string) (int, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// GetFloat 读取浮点配置，不存在或类型不匹配返回 (0, false)。
func (s *ConfigService) GetFloat(ctx context.Context, key string) (float64, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return 0, false
	}
	if n, ok := v.(float64); ok {
		return n, true
	}
	return 0, false
}

// GetBool 读取布尔配置，不存在或类型不匹配返回 (false, false)。
func (s *ConfigService) GetBool(ctx context.Context, key string) (bool, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return false, false
	}
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

// GetConfig 获取指定 key 的配置值。
func (s *ConfigService) GetConfig(ctx context.Context, key string) (interface{}, error) {
	if _, ok := validConfigKeys[key]; !ok {
		return nil, AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}

	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 有效 key 但尚未初始化：返回 null 而非报错
		}
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(cfg.Value, &value); err != nil {
		return nil, fmt.Errorf("解析配置值失败: %w", err)
	}

	return value, nil
}

// UpdateConfig 更新或创建系统配置。
//
// value 会被序列化为 JSONB 存储，nil 被拒绝。
// 仅允许白名单内的 key 写入，同时写入白名单中对应的 description。
func (s *ConfigService) UpdateConfig(ctx context.Context, key string, value interface{}, updatedBy int64) error {
	meta, ok := validConfigKeys[key]
	if !ok {
		return AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}
	if value == nil {
		return AppError{Code: errcode.ErrParam, Message: "配置值不能为 nil"}
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置值失败: %w", err)
	}

	if err := s.repo.Upsert(ctx, key, meta.Description, datatypes.JSON(jsonBytes), updatedBy); err != nil {
		return err
	}
	s.auditRepo.Create(ctx, &model.AuditLog{
		OperatorID: updatedBy, Action: "config.update",
		TargetType: "config", TargetID: 0,
		Detail: datatypes.JSON(jsonBytes),
	})
	return nil
}

// ComputeThresholdsResult 分位数计算结果。
type ComputeThresholdsResult struct {
	P30         float64 `json:"p30"`
	P70         float64 `json:"p70"`
	SampleCount int     `json:"sample_count"`
	DateFrom    string  `json:"date_from"`
	DateTo      string  `json:"date_to"`
	Warning     string  `json:"warning,omitempty"`
}

// ComputeThresholds 从近 N 天 chat_messages 的 confidence_raw 中计算 P30/P70 分位数。
func (s *ConfigService) ComputeThresholds(ctx context.Context, days int) (*ComputeThresholdsResult, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	if s.chatRepo == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}

	scores, err := s.chatRepo.QueryRawScores(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("查询原始置信度分数失败: %w", err)
	}

	n := len(scores)
	if n == 0 {
		return &ComputeThresholdsResult{
			P30: 0.40, P70: 0.70, SampleCount: 0, Warning: "无可用数据，返回默认值",
		}, nil
	}

	p30 := percentile(scores, 0.30)
	p70 := percentile(scores, 0.70)

	if p30 < 0.10 {
		p30 = 0.10
	}
	if p70 > 0.95 {
		p70 = 0.95
	}
	if p70-p30 < 0.10 {
		p70 = p30 + 0.10
		if p70 > 0.95 {
			p70 = 0.95
		}
	}

	var warning string
	if n < 50 {
		warning = fmt.Sprintf("样本数量不足（%d < 50），建议积累更多数据后重新计算", n)
	}

	return &ComputeThresholdsResult{
		P30:         round2(p30),
		P70:         round2(p70),
		SampleCount: n,
		Warning:     warning,
	}, nil
}

// percentile 线性插值法计算第 p 百分位数（scores 须已升序）。
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := p * float64(n-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	return sorted[lo]*(1-(idx-float64(lo))) + sorted[hi]*(idx-float64(lo))
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

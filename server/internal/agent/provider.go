// Package agent 提供 Agent Loop 基座。
// provider.go：ChatModel 工厂 + atomic.Value 热切换。
//
// 复用 LLMConfigManager（atomic.Value 零锁读模式）喂 ChatModel；
// OnChange 回调触发 ChatModel 重建（与现有 LLM/Embedding 热切换共用同一回调）。

package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"cognik/internal/agent/llm"
	llmconfig "cognik/internal/domain/chat/llm_config"
	"cognik/internal/shared/model"
)

// ChatModelFactory 构造并热切换 ChatModel。
// 复用 LLMConfigManager.atomic.Value 零锁读模式（llm_config/service.go:40）。
type ChatModelFactory struct {
	current   atomic.Value // *llm.ChatModel
	configMgr *llmconfig.LLMConfigManager
}

// NewChatModelFactory 创建工厂。需后续调用 BuildInitial 构造初始 ChatModel。
func NewChatModelFactory(configMgr *llmconfig.LLMConfigManager) *ChatModelFactory {
	return &ChatModelFactory{configMgr: configMgr}
}

// GetModel 零锁返回当前 ChatModel；未初始化返回 nil。
func (f *ChatModelFactory) GetModel() *llm.ChatModel {
	v := f.current.Load()
	if v == nil {
		return nil
	}
	return v.(*llm.ChatModel)
}

// BuildInitial 启动时从 LLMConfigManager 构造初始 ChatModel。
func (f *ChatModelFactory) BuildInitial(ctx context.Context) error {
	cfg := f.configMgr.GetConfig()
	if cfg == nil {
		return fmt.Errorf("LLM 配置未初始化，无法构建 Agent ChatModel")
	}
	return f.rebuild(ctx, cfg)
}

// rebuild 用自建 llm.ChatModel 构造，BaseURL 指向 llama.cpp（或 OpenAI 兼容端点），原子切换。
func (f *ChatModelFactory) rebuild(ctx context.Context, cfg *model.LlmConfig) error {
	m := llm.NewChatModel(llm.ChatModelConfig{
		APIKey:  cfg.LLMAPIKey, // AfterFind 已解密
		Model:   cfg.LLMModel,
		BaseURL: cfg.LLMBaseURL, // http://localhost:8081/v1
	})
	f.current.Store(m)
	slog.Info("Agent ChatModel 已构建/重建", "model", cfg.LLMModel, "base_url", cfg.LLMBaseURL)
	return nil
}

// OnConfigChange 重建 ChatModel（由 LLMConfigManager.OnChange 回调触发）。
func (f *ChatModelFactory) OnConfigChange() {
	cfg := f.configMgr.GetConfig()
	if cfg == nil {
		return
	}
	if err := f.rebuild(context.Background(), cfg); err != nil {
		slog.Error("Agent ChatModel 热重建失败", "error", err)
	}
}

// Package agent 提供 Agent Loop 基座。
// provider.go：ChatModel 工厂——从 .env 构造,atomic.Value 零锁读。
package agent

import (
	"log/slog"
	"sync/atomic"

	"cognik/internal/agent/llm"
)

// ChatModelFactory 构造 ChatModel。.env 是唯一配置源,无 DB 依赖。
type ChatModelFactory struct {
	current atomic.Value // *llm.ChatModel
}

// NewChatModelFactory 创建工厂。需后续调用 BuildFromEnv 构造初始 ChatModel。
func NewChatModelFactory() *ChatModelFactory {
	return &ChatModelFactory{}
}

// GetModel 零锁返回当前 ChatModel；未初始化返回 nil。
func (f *ChatModelFactory) GetModel() *llm.ChatModel {
	v := f.current.Load()
	if v == nil {
		return nil
	}
	return v.(*llm.ChatModel)
}

// BuildFromEnv 从 .env 构造 ChatModel(env 是唯一配置源,不入库)。
func (f *ChatModelFactory) BuildFromEnv(baseURL, apiKey, model string) {
	m := llm.NewChatModel(llm.ChatModelConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
	})
	f.current.Store(m)
	slog.Info("Agent ChatModel 从 .env 构建", "model", model, "base_url", baseURL)
}

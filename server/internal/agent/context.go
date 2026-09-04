// Package agent 提供自建 ReAct 循环与工具接口。
//
// context.go：Agent 上下文 key 定义——会话 ID 等通过 ctx 在调用方 → Loop → 工具间传播。
// MemoryTool 通过 ctx 读取 sessionID，写入 sessions/{id}/，SessionExtractor 读同目录，端到端打通。
package agent

import "context"

// ctxKey context key 类型（避免与标准库或其他包的 string key 冲突）。
type ctxKey string

const (
	// CtxKeySessionID 会话 ID（调用方注入，memory 工具读取用于 session 记忆隔离）。
	CtxKeySessionID ctxKey = "session_id"
)

// SessionIDFromCtx 从 ctx 提取会话 ID，无则返回空串。
func SessionIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeySessionID).(string); ok {
		return v
	}
	return ""
}

// WithSessionID 返回注入了 sessionID 的 ctx。
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, CtxKeySessionID, sessionID)
}

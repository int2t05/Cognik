// Package pkg_test 测试公共工具包的导出 API。
//
// 本文件测试错误码常量定义。
package pkg_test

import (
	"testing"

	"opsmind/pkg/errcode"
)

// TestErrCodeValues 测试错误码值是否符合分段约定
func TestErrCodeValues(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"成功", errcode.Success, 0},
		{"未登录或令牌过期", errcode.ErrAuth, 10001},
		{"无权限", errcode.ErrForbidden, 10002},
		{"参数校验失败", errcode.ErrParam, 10003},
		{"资源不存在", errcode.ErrNotFound, 10004},
		{"资源冲突", errcode.ErrConflict, 10005},
		{"AI服务不可用", errcode.ErrAIUnavailable, 20001},
		{"RAG服务不可用", errcode.ErrRAGUnavailable, 20002},
		{"存储服务不可用", errcode.ErrStorageUnavailable, 20003},
		{"未知错误", errcode.ErrUnknown, 99999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("%s: 期望 %d，实际 %d", tt.name, tt.expected, tt.code)
			}
		})
	}
}

// TestErrCodeMessages 测试错误码关联的默认消息
func TestErrCodeMessages(t *testing.T) {
	tests := []struct {
		code    int
		message string
	}{
		{errcode.Success, "success"},
		{errcode.ErrAuth, "未登录或令牌过期"},
		{errcode.ErrForbidden, "无权限"},
		{errcode.ErrParam, "参数校验失败"},
		{errcode.ErrNotFound, "资源不存在"},
		{errcode.ErrConflict, "资源冲突"},
		{errcode.ErrAIUnavailable, "AI 服务不可用"},
		{errcode.ErrRAGUnavailable, "RAG 服务不可用"},
		{errcode.ErrStorageUnavailable, "存储服务不可用"},
		{errcode.ErrUnknown, "未知错误"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			msg := errcode.GetMessage(tt.code)
			if msg != tt.message {
				t.Errorf("错误码 %d: 期望消息 %q，实际 %q", tt.code, tt.message, msg)
			}
		})
	}
}

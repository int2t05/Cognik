//go:build integration

// setup_test.go 提供 system 测试包的共享变量与环境工具。
package system_test

import (
	"context"
	"os"
)

// bgCtx 测试用根上下文，供包内所有测试复用。
var bgCtx = context.Background()

// getEnv 读取环境变量，缺省时返回默认值，适配不同开发环境。
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

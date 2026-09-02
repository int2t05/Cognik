//go:build integration

// setup_test.go 提供 knowledge 测试包的共享变量。
package knowledge_test

import "context"

// bgCtx 测试用根上下文，供包内所有测试复用。
var bgCtx = context.Background()

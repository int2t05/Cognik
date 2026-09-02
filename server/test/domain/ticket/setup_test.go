//go:build integration

// setup_test.go 提供 ticket 测试包的共享变量。
package ticket_test

import "context"

// bgCtx 测试用根上下文，供包内所有测试复用。
var bgCtx = context.Background()

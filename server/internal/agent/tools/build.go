// Package tools 提供 Agent 内置工具集。
//
// 工具实现 agent.SyncTool 接口（自建，不依赖 Eino InvokableTool/StreamableTool）。
// build.go：扁平装配函数，main.go 调 Build(deps) 得 []agent.Tool，逐个 registry.Register。
package tools

import (
	"time"

	"opsmind/internal/agent"
	"opsmind/internal/infra/adapter"
)

// Deps 工具装配依赖（构造时注入，构造后不可变）。
type Deps struct {
	WorkDir       string
	Timeout       time.Duration
	MaxBytes      int64
	SearchChain   *adapter.SearchChain  // 可选：nil 则不注册 web_search
	FetchChain    *adapter.FetchChain   // 可选：nil 则不注册 web_fetch
	KBStore       KBStore                // 可选：nil 则不注册 kb（知识库 CRUD+检索）
	MemoryStore   MemoryStore            // 可选：nil 则不注册 memory
}

// Build 装配所有工具，返回 []agent.Tool。
// web/kb/memory 工具条件注册（依赖注入时才加入），OS 工具始终注册。
func Build(deps Deps) []agent.Tool {
	tools := []agent.Tool{
		NewBashTool(deps.WorkDir, deps.Timeout, deps.MaxBytes),
		NewReadFileTool(deps.WorkDir, deps.MaxBytes),
		NewWriteFileTool(deps.WorkDir, deps.MaxBytes),
		NewEditFileTool(deps.WorkDir, deps.MaxBytes),
		NewListDirTool(deps.WorkDir, deps.MaxBytes),
		NewGlobTool(deps.WorkDir, deps.MaxBytes),
		NewGrepTool(deps.WorkDir, deps.MaxBytes),
		NewMkdirTool(deps.WorkDir),
	}
	if deps.SearchChain != nil {
		tools = append(tools, NewWebSearchTool(deps.SearchChain, deps.Timeout))
	}
	if deps.FetchChain != nil {
		tools = append(tools, NewWebFetchTool(deps.FetchChain, deps.MaxBytes))
	}
	if deps.KBStore != nil {
		tools = append(tools, NewKBTool(deps.KBStore))
	}
	if deps.MemoryStore != nil {
		tools = append(tools, NewMemoryTool(deps.MemoryStore))
	}
	return tools
}

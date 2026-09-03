// Package tools 提供 Agent 内置工具集。
//
// 工具集（业务无关基础原语，对标 Claude Code / SWE-agent ACI）：
//   - bash       命令执行（GitBash 自适应 + description + timeout）
//   - read_file  文件读取（offset/limit + 行号）
//   - write_file 文件写入（overwrite/append）
//   - edit_file  str_replace 精确编辑（唯一性校验 + 邻近行反馈）
//   - list_dir   目录列表（类型/大小/时间）
//   - glob       文件名模式搜索（** 递归）
//   - grep       内容搜索（正则 + 行号）
//   - mkdir      目录创建
//
// 工具实现 Eino InvokableTool 接口（Info + InvokableRun）。
package tools

import (
	"time"

	"github.com/cloudwego/eino/components/tool"
)

// ToolFactory 组装 Agent 工具集。
type ToolFactory struct {
	workDir     string
	toolTimeout time.Duration
	maxBytes    int64
}

// NewToolFactory 创建工具工厂。
func NewToolFactory(workDir string, timeout time.Duration, maxBytes int64) *ToolFactory {
	return &ToolFactory{workDir: workDir, toolTimeout: timeout, maxBytes: maxBytes}
}

// BuildTools 返回完整工具集。
func (f *ToolFactory) BuildTools() []tool.BaseTool {
	return []tool.BaseTool{
		NewBashTool(f.workDir, f.toolTimeout, f.maxBytes),
		NewAsyncBashTool(f.workDir, f.toolTimeout, f.maxBytes),
		NewReadFileTool(f.workDir, f.maxBytes),
		NewWriteFileTool(f.workDir, f.maxBytes),
		NewEditFileTool(f.workDir, f.maxBytes),
		NewListDirTool(f.workDir, f.maxBytes),
		NewGlobTool(f.workDir, f.maxBytes),
		NewGrepTool(f.workDir, f.maxBytes),
		NewMkdirTool(f.workDir),
	}
}

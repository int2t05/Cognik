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
// 工具实现 Eino InvokableTool（Info + InvokableRun）或 StreamableTool（Info + StreamableRun）接口。
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

// BuildReadOnlyTools 返回只读工具集（供子 Agent 使用）。
func (f *ToolFactory) BuildReadOnlyTools() []tool.BaseTool {
	return []tool.BaseTool{
		NewReadFileTool(f.workDir, f.maxBytes),
		NewGlobTool(f.workDir, f.maxBytes),
		NewGrepTool(f.workDir, f.maxBytes),
		NewListDirTool(f.workDir, f.maxBytes),
	}
}

// WorkDir 返回工作目录。
func (f *ToolFactory) WorkDir() string { return f.workDir }

// MaxBytes 返回截断上限。
func (f *ToolFactory) MaxBytes() int64 { return f.maxBytes }

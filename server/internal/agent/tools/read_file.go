// Package tools 提供 Agent 内置工具集。
// read_file.go：文件读取工具（对标 Claude Code Read）。
//
// 高级特性：offset/limit 行范围读取 + 行号输出（cat -n 风格）。
// 行号让 Agent 精确定位后续 edit_file 的 old_string，避免读取整文件膨胀 token。

package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ReadFileTool 文件读取工具（实现 eino InvokableTool 接口）。
type ReadFileTool struct {
	workDir  string
	maxBytes int64
}

// NewReadFileTool 创建文件读取工具。
func NewReadFileTool(workDir string, maxBytes int64) *ReadFileTool {
	return &ReadFileTool{workDir: workDir, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (r *ReadFileTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_file",
		Desc: "Read a file's content with line numbers from the working directory sandbox. Returns cat -n style output (line number + tab + content). Use offset/limit to read ranges of large files.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Desc:     "Relative path to the file within the working directory",
				Required: true,
			},
			"offset": {
				Type: schema.Integer,
				Desc: "1-based line number to start reading from (default 1). Use for large files.",
			},
			"limit": {
				Type: schema.Integer,
				Desc: "Maximum number of lines to read (default: read to EOF). Use with offset for paging.",
			},
		}),
	}, nil
}

// readFileParams read_file 工具参数。
type readFileParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"` // 1-based，<=0 视为 1
	Limit  int    `json:"limit,omitempty"`  // <=0 视为读到 EOF
}

// InvokableRun 读取文件内容，输出带行号。路径限制在 workDir 沙箱内（防 traversal）。
func (r *ReadFileTool) InvokableRun(_ context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var params readFileParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	fullPath, err := safeJoin(r.workDir, params.Path)
	if err != nil {
		return "", err
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// offset/limit 转 1-based 语义
	offset := params.Offset
	if offset < 1 {
		offset = 1
	}
	limit := params.Limit

	scanner := bufio.NewScanner(f)
	// 放大单行 buffer（默认 64KB，覆盖长行）
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var b []byte
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo < offset {
			continue
		}
		if limit > 0 && lineNo-offset >= limit {
			break
		}
		// cat -n 风格：右对齐行号 + tab + 内容
		line := fmt.Sprintf("%6d\t%s\n", lineNo, scanner.Text())
		b = append(b, line...)
		if int64(len(b)) > r.maxBytes {
			b = append(b, []byte("\n...[truncated]\n")...)
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(b) == 0 {
		// 空文件或 offset 超出行数
		return fmt.Sprintf("(file %s: %d lines, no content in range)", params.Path, lineNo), nil
	}
	return string(b), nil
}

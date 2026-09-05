// Package tools 提供 Agent 内置工具集。
// write_file.go：文件写入工具。
//
// 高级特性：mode 支持 overwrite（覆盖，默认）与 append（追加）。
// 追加模式在文件末尾追加内容（文件不存在则创建）。

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cognik/internal/agent"

	"cognik/internal/agent/llm"
)

// WriteFileTool 文件写入工具（实现 agent.SyncTool 接口）。
type WriteFileTool struct {
	workDir  string
	maxBytes int64
}

// NewWriteFileTool 创建文件写入工具。
func NewWriteFileTool(workDir string, maxBytes int64) *WriteFileTool {
	return &WriteFileTool{workDir: workDir, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (w *WriteFileTool) Info() *llm.ToolInfo {
	return &llm.ToolInfo{
		Name: "write_file",
		Desc: `Write content to a file in the working directory sandbox.
- mode=overwrite (default) replaces the file; mode=append adds to the end (creates if missing).
- Do NOT use for targeted edits — use edit_file (cheaper, safer for small changes).`,
		ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
			"path": {
				Type:     llm.String,
				Desc:     "Relative path to the file within the working directory",
				Required: true,
			},
			"content": {
				Type:     llm.String,
				Desc:     "Content to write to the file",
				Required: true,
			},
			"mode": {
				Type: llm.String,
				Desc: "Write mode: overwrite (default) or append",
				Enum: []string{"overwrite", "append"},
			},
		}),
	}
}

// writeFileParams write_file 工具参数。
type writeFileParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"` // overwrite（默认）/ append
}

// Call 写入文件。内容上限 maxBytes；路径限制在 workDir 沙箱内。
func (w *WriteFileTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params writeFileParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if int64(len(params.Content)) > w.maxBytes {
		return "", fmt.Errorf("content exceeds maxBytes %d", w.maxBytes)
	}
	mode := strings.ToLower(params.Mode)
	if mode == "" {
		mode = "overwrite"
	}
	if mode != "overwrite" && mode != "append" {
		return "", fmt.Errorf("mode must be overwrite or append, got: %s", params.Mode)
	}

	fullPath, err := safeJoin(w.workDir, params.Path)
	if err != nil {
		return "", err
	}
	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("create parent dir: %w", err)
	}

	switch mode {
	case "append":
		f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := f.WriteString(params.Content); err != nil {
			return "", err
		}
		return fmt.Sprintf("appended %d bytes to %s", len(params.Content), params.Path), nil
	default: // overwrite
		if err := os.WriteFile(fullPath, []byte(params.Content), 0644); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(params.Content), params.Path), nil
	}
}

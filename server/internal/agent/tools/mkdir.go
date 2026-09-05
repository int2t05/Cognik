// Package tools 提供 Agent 内置工具集。
// mkdir.go：目录创建工具。
//
// 在 workDir 沙箱内创建目录（含父目录，递归）。

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cognos/internal/agent"

	"cognos/internal/agent/llm"
)

// MkdirTool 目录创建工具。
type MkdirTool struct {
	workDir string
}

// NewMkdirTool 创建目录工具。
func NewMkdirTool(workDir string) *MkdirTool {
	return &MkdirTool{workDir: workDir}
}

// Info 返回工具元信息。
func (m *MkdirTool) Info() *llm.ToolInfo {
	return &llm.ToolInfo{
		Name: "mkdir",
		Desc: `Create a directory (including parents) within the working directory sandbox.
- Use before write_file when the target path's parent does not exist.`,
		ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
			"path": {
				Type:     llm.String,
				Desc:     "Relative directory path to create within the working directory",
				Required: true,
			},
		}),
	}
}

// mkdirParams mkdir 工具参数。
type mkdirParams struct {
	Path string `json:"path"`
}

// Call 创建目录。路径限制在 workDir 沙箱内。
func (m *MkdirTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params mkdirParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	fullPath, err := safeJoin(m.workDir, params.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(m.workDir, fullPath)
	if err != nil {
		rel = params.Path
	}
	return fmt.Sprintf("created directory %s", filepath.ToSlash(rel)), nil
}

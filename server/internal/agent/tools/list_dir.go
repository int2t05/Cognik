// Package tools 提供 Agent 内置工具集。
// list_dir.go：目录列表工具。
//
// 列出 workDir 沙箱内指定目录的条目，含类型/大小/修改时间，目录优先排序。
// 让 Agent 探索文件结构而无需读取整个文件内容。

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cognos/internal/agent"

	"github.com/cloudwego/eino/schema"
)

// ListDirTool 目录列表工具。
type ListDirTool struct {
	workDir  string
	maxBytes int64
}

// NewListDirTool 创建目录列表工具。
func NewListDirTool(workDir string, maxBytes int64) *ListDirTool {
	return &ListDirTool{workDir: workDir, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (l *ListDirTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "list_dir",
		Desc: "List entries in a directory within the working directory sandbox. Returns name, type (file/dir), size, and mod time. Directories sorted first.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Desc:     "Relative directory path within the working directory (default: root).",
			},
		}),
	}
}

// listDirParams list_dir 工具参数。
type listDirParams struct {
	Path string `json:"path"`
}

// dirEntry 目录条目（列表项）。
type dirEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`     // dir / file
	Size    int64  `json:"size"`     // 字节数（目录为 0）
	ModTime string `json:"mod_time"` // 修改时间
}

// Call 列出目录条目。路径限制在 workDir 沙箱内。
func (l *ListDirTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params listDirParams
	_ = json.Unmarshal([]byte(args), &params) // path 可选（空=workDir 根）

	dir := l.workDir
	if params.Path != "" {
		p, err := safeJoin(l.workDir, params.Path)
		if err != nil {
			return "", err
		}
		dir = p
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var out []dirEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		typ := "file"
		if e.IsDir() {
			typ = "dir"
		}
		out = append(out, dirEntry{
			Name:    e.Name(),
			Type:    typ,
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	// 目录优先，再按名称排序
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type == "dir" // 目录在前
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d entries)\n", relPath(l.workDir, dir), len(out))
	for _, e := range out {
		marker := " "
		if e.Type == "dir" {
			marker = "d"
		}
		fmt.Fprintf(&b, "%s %12d %s  %s\n", marker, e.Size, e.ModTime, e.Name)
	}
	return truncate(b.String(), l.maxBytes), nil
}

// relPath 返回相对 workDir 的可读路径。
func relPath(workDir, full string) string {
	rel, err := filepath.Rel(workDir, full)
	if err != nil {
		return full
	}
	if rel == "." {
		return "."
	}
	return rel
}

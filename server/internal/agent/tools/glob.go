// Package tools 提供 Agent 内置工具集。
// glob.go：文件名模式搜索工具。
//
// 用 filepath.Match 模式在 workDir 沙箱内递归匹配文件路径。
// 支持 ** 递归（**/*.go 匹配任意深度）、? 单字符、[abc] 字符集。

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cognos/internal/agent"

	"github.com/cloudwego/eino/schema"
)

// GlobTool 文件名模式搜索工具。
type GlobTool struct {
	workDir  string
	maxBytes int64
}

// NewGlobTool 创建 glob 工具。
func NewGlobTool(workDir string, maxBytes int64) *GlobTool {
	return &GlobTool{workDir: workDir, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (g *GlobTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "glob",
		Desc: "Find files matching a glob pattern within the working directory sandbox. Supports ** for recursive matching (e.g. '**/*.go'), ? single char, [abc] char set.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:     schema.String,
				Desc:     "Glob pattern, e.g. '**/*.go', 'src/*.ts', 'config/*.yaml'",
				Required: true,
			},
		}),
	}
}

// globParams glob 工具参数。
type globParams struct {
	Pattern string `json:"pattern"`
}

// Call 递归匹配文件路径。路径限制在 workDir 沙箱内。
func (g *GlobTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params globParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.Pattern) == "" {
		return "", fmt.Errorf("pattern is required")
	}

	matches, err := globRecursive(g.workDir, params.Pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return fmt.Sprintf("no files matching %q", params.Pattern), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d files matching %q:\n", len(matches), params.Pattern)
	for _, m := range matches {
		fmt.Fprintln(&b, m)
	}
	return truncate(b.String(), g.maxBytes), nil
}

// globRecursive 在 root 下递归匹配 pattern。
// pattern 可含 ** 表示任意深度目录；以相对路径返回匹配项。
func globRecursive(root, pattern string) ([]string, error) {
	// 标准化 pattern 分隔符
	pattern = filepath.ToSlash(pattern)
	// 用 filepath.Walk 遍历 + 逐项 Match
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过不可访问项
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if globMatch(pattern, rel) {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// globMatch 匹配 pattern（支持 ** 递归）。
// ** 展开为任意深度目录前缀，其余用 filepath.Match。
func globMatch(pattern, path string) bool {
	// 处理 ** 前缀：**/*.go 匹配 a/b/c.go
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		// path 任意层级下文件名匹配 suffix
		base := filepath.Base(path)
		if ok, _ := filepath.Match(suffix, base); ok {
			return true
		}
		// 或 path 中某段后缀匹配（a/b/*.go）
		return matchSuffixPath(suffix, path)
	}
	// 中间含 **：src/**/*.go
	if strings.Contains(pattern, "/**/") {
		parts := strings.SplitN(pattern, "/**/", 2)
		prefix := parts[0]
		suffix := parts[1]
		if !strings.HasPrefix(filepath.ToSlash(path), prefix+"/") {
			return false
		}
		return matchSuffixPath(suffix, path)
	}
	// 尾部 **：src/** 匹配 src 下所有
	if strings.HasSuffix(pattern, "/**") {
		prefix := pattern[:len(pattern)-3]
		return strings.HasPrefix(filepath.ToSlash(path), prefix+"/")
	}
	// 无 **：直接 Match
	ok, _ := filepath.Match(pattern, path)
	return ok
}

// matchSuffixPath 检查 path 的文件名或路径尾部是否匹配 suffix 模式。
func matchSuffixPath(suffix, path string) bool {
	base := filepath.Base(path)
	if ok, _ := filepath.Match(suffix, base); ok {
		return true
	}
	// 路径尾部段匹配：suffix=a/*.go，path=x/a/b.go
	path = filepath.ToSlash(path)
	if ok, _ := filepath.Match(suffix, path); ok {
		return true
	}
	return false
}

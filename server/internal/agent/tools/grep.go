// Package tools 提供 Agent 内置工具集。
// grep.go：内容搜索工具。
//
// 在 workDir 沙箱内递归搜索文件内容，返回匹配行 + 文件名 + 行号。
// 支持正则与大小写忽略。让 Agent 定位代码/文本而无需逐文件读取。

package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cognos/internal/agent"

	"github.com/cloudwego/eino/schema"
)

// GrepTool 内容搜索工具。
type GrepTool struct {
	workDir  string
	maxBytes int64
}

// NewGrepTool 创建内容搜索工具。
func NewGrepTool(workDir string, maxBytes int64) *GrepTool {
	return &GrepTool{workDir: workDir, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (g *GrepTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "grep",
		Desc: "Search file contents recursively in the working directory sandbox. Returns matching lines with file path and line number. Supports regex.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:     schema.String,
				Desc:     "Search pattern (regex). Example: 'func \\w+\\(' or 'TODO'",
				Required: true,
			},
			"path": {
				Type: schema.String,
				Desc: "Relative directory to search in (default: working directory root).",
			},
			"include": {
				Type: schema.String,
				Desc: "File glob to include, e.g. '*.go'. Default: all files.",
			},
			"ignore_case": {
				Type: schema.Boolean,
				Desc: "Case-insensitive match (default false).",
			},
		}),
	}
}

// grepParams grep 工具参数。
type grepParams struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Include    string `json:"include,omitempty"`
	IgnoreCase bool   `json:"ignore_case,omitempty"`
}

// grepMatch 单次匹配结果。
type grepMatch struct {
	File   string
	Line   int
	Source string
}

// Call 递归搜索文件内容。路径限制在 workDir 沙箱内。
func (g *GrepTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params grepParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.Pattern) == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// 编译正则（大小写忽略可选）
	pat := params.Pattern
	if params.IgnoreCase {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	searchRoot := g.workDir
	if params.Path != "" {
		p, err := safeJoin(g.workDir, params.Path)
		if err != nil {
			return "", err
		}
		searchRoot = p
	}

	var matches []grepMatch
	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可访问项
		}
		if d.IsDir() {
			return nil
		}
		// include 过滤
		if params.Include != "" {
			matched, _ := filepath.Match(params.Include, d.Name())
			if !matched {
				return nil
			}
		}
		// 二进制/大文件跳过（简单启发式：超过 maxBytes 不搜）
		info, err := d.Info()
		if err != nil || info.Size() > g.maxBytes {
			return nil
		}
		fileMatches, err := grepInFile(path, g.workDir, re)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return fmt.Sprintf("no matches for %q", params.Pattern), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d matches for %q:\n", len(matches), params.Pattern)
	for _, m := range matches {
		fmt.Fprintf(&b, "%s:%d: %s\n", m.File, m.Line, strings.TrimRight(m.Source, "\r\n"))
	}
	return truncate(b.String(), g.maxBytes), nil
}

// grepInFile 在单个文件中搜索，返回相对 workDir 的路径 + 行号 + 行内容。
func grepInFile(fullPath, workDir string, re *regexp.Regexp) ([]grepMatch, error) {
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rel, err := filepath.Rel(workDir, fullPath)
	if err != nil {
		rel = fullPath
	}

	var matches []grepMatch
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if re.MatchString(scanner.Text()) {
			matches = append(matches, grepMatch{File: filepath.ToSlash(rel), Line: lineNo, Source: scanner.Text()})
		}
	}
	return matches, scanner.Err()
}

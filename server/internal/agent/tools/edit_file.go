// Package tools 提供 Agent 内置工具集。
// edit_file.go：文件精确编辑工具。
//
// str_replace 是行业共识的编辑原语（Claude Code / Anthropic API / Aider / SWE-agent / OpenHands / Cursor 全采用）：
//   - 只替换匹配的片段，不重写整文件（省 token、抗行号漂移）
//   - 严格精确匹配（非正则、非模糊）— Claude Code 安全策略
//   - 唯一性校验：非 replace_all 时 old_string 必须唯一出现，否则报错（避免歧义替换）
//   - 失败时显示邻近行（Aider 最佳实践）— 让模型自纠正

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cognos/internal/agent"

	"github.com/cloudwego/eino/schema"
)

// EditFileTool 文件精确编辑工具（实现 agent.SyncTool 接口）。
type EditFileTool struct {
	workDir  string
	maxBytes int64
}

// NewEditFileTool 创建编辑工具。
func NewEditFileTool(workDir string, maxBytes int64) *EditFileTool {
	return &EditFileTool{workDir: workDir, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (e *EditFileTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "edit_file",
		Desc: "Edit a file by replacing an exact string (old_string) with new_string. Stricter and cheaper than rewriting: only the matched fragment changes. old_string must be unique unless replace_all=true. Delete by setting new_string empty.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Desc:     "Relative path to the file within the working directory",
				Required: true,
			},
			"old_string": {
				Type:     schema.String,
				Desc:     "Exact text to find (including whitespace/indentation). Must be unique unless replace_all=true.",
				Required: true,
			},
			"new_string": {
				Type:     schema.String,
				Desc:     "Replacement text. Set empty to delete old_string. Must differ from old_string.",
				Required: true,
			},
			"replace_all": {
				Type: schema.Boolean,
				Desc: "If true, replace all occurrences of old_string. Default false (requires uniqueness).",
			},
		}),
	}
}

// editFileParams edit_file 工具参数。
type editFileParams struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// Call 执行 str_replace 编辑。严格精确匹配 + 唯一性校验 + 邻近行反馈。
func (e *EditFileTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params editFileParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if params.OldString == "" {
		return "", fmt.Errorf("old_string is required")
	}
	if params.OldString == params.NewString {
		return "", fmt.Errorf("new_string must differ from old_string")
	}

	fullPath, err := safeJoin(e.workDir, params.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	content := string(data)
	if int64(len(content)) > e.maxBytes {
		return "", fmt.Errorf("file exceeds maxBytes %d; use read_file with offset/limit first", e.maxBytes)
	}

	// 统计匹配次数
	count := strings.Count(content, params.OldString)
	if count == 0 {
		// Aider 风格：未匹配时返回邻近行提示，助模型自纠正
		return "", fmt.Errorf("old_string not found in %s. %s\n%s",
			params.Path, matchHint(params.OldString, content), nearbyLines(content))
	}
	if count > 1 && !params.ReplaceAll {
		// 唯一性校验：多处匹配且未指定 replace_all → 报错（避免歧义）
		return "", fmt.Errorf("old_string appears %d times in %s; include more context to make it unique, or set replace_all=true", count, params.Path)
	}

	// 执行替换
	var updated string
	if params.ReplaceAll {
		updated = strings.ReplaceAll(content, params.OldString, params.NewString)
	} else {
		updated = strings.Replace(content, params.OldString, params.NewString, 1)
	}

	if err := os.WriteFile(fullPath, []byte(updated), 0644); err != nil {
		return "", err
	}

	occ := count
	if !params.ReplaceAll {
		occ = 1
	}
	return fmt.Sprintf("replaced %d occurrence(s) in %s", occ, params.Path), nil
}

// matchHint 给出匹配提示：检查 old_string 首行是否在文件中单独存在（帮助定位缩进/空白问题）。
func matchHint(oldString, content string) string {
	firstLine := oldString
	if i := strings.IndexByte(oldString, '\n'); i >= 0 {
		firstLine = oldString[:i]
	}
	if strings.Contains(content, firstLine) {
		return "The first line matches, but the full block does not — check trailing whitespace, indentation, or line endings (LF vs CRLF)."
	}
	return "No partial match; verify the text and path are exactly correct."
}

// nearbyLines 返回文件中包含 old_string 首行关键词的上下文行（助模型定位正确目标）。
func nearbyLines(content string) string {
	lines := strings.Split(content, "\n")
	// 取文件中间几行作为上下文样本（避免输出整个文件）
	if len(lines) <= 6 {
		return "File content:\n" + content
	}
	mid := len(lines) / 2
	start := mid - 2
	if start < 0 {
		start = 0
	}
	end := mid + 3
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	b.WriteString("\nNearby lines (for reference):\n")
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, "%6d\t%s\n", i+1, lines[i])
	}
	return b.String()
}

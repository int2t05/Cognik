// Package tools 提供 Agent 内置工具集。
// util.go：工具共享辅助（路径沙箱校验、输出截断）。

package tools

import (
	"fmt"
	"path/filepath"
)

// safeJoin 将相对路径安全拼接到 workDir，防止路径穿越（.. / 绝对路径）。
// 所有文件类工具共享此校验。
func safeJoin(workDir, relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	// 禁止绝对路径与向上穿越
	if filepath.IsAbs(cleaned) || len(cleaned) > 0 && (cleaned == ".." || hasDotDotPrefix(cleaned)) {
		return "", fmt.Errorf("path must be relative within working directory: %s", relPath)
	}
	full := filepath.Join(workDir, cleaned)
	// 二次校验：结果必须在 workDir 下
	rel, err := filepath.Rel(workDir, full)
	if err != nil || len(rel) > 0 && rel == ".." || hasDotDotPrefix(rel) {
		return "", fmt.Errorf("path escapes working directory: %s", relPath)
	}
	return full, nil
}

// hasDotDotPrefix 判断路径是否以 .. 开头（含 ../xxx）。
func hasDotDotPrefix(p string) bool {
	return len(p) >= 2 && p[0] == '.' && p[1] == '.' && (len(p) == 2 || p[2] == '/' || p[2] == '\\')
}

// truncate 截断字符串到 maxBytes 上限，防止工具输出 token 膨胀。
// 所有工具共享此截断逻辑。
func truncate(s string, maxBytes int64) string {
	if int64(len(s)) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...[truncated]"
}

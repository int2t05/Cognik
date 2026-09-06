// Package local 实现本地纯 Go 文档解析降级引擎。
//
// doc.go 实现 doc/xls/ppt 旧格式解析，通过 Python markitdown 子进程转换。
//
// 旧格式（doc/xls/ppt）为二进制私有格式，无纯 Go 库可解析。
// markitdown (Microsoft) 提供统一的 CLI 转换能力，输出 Markdown。
// 旧格式图片提取复杂度极高，此处仅提取文本，Images 返回 nil。
// markitdown 或 Python 不可用时返回错误，提示用户转存为新格式（docx/xlsx/pptx）。
package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// pythonCandidates Python 解释器候选路径，按优先级查找。
var pythonCandidates = []string{"python3", "python"}

// markitdownCheck 用于验证 markitdown 可用性的检查命令。
const markitdownCheck = "import markitdown"

// ParseLegacy 解析 doc/xls/ppt 旧格式文件，通过 Python markitdown 子进程转换为 Markdown。
//
// 旧格式为二进制私有格式，需写临时文件供 markitdown 读取。
// 图片不提取（旧格式图片提取复杂度极高），Images 返回 nil。
// Python 或 markitdown 不可用时返回错误，提示用户转存为新格式。
func ParseLegacy(reader io.Reader, fileType string) (*ParseResult, error) {
	// 1. 查找可用的 Python 解释器
	pythonPath, err := findPython()
	if err != nil {
		return nil, fmt.Errorf("旧格式 %s 解析需要 Python，请将文件转为 %s 格式后上传: %w",
			fileType, strings.ToUpper(fileType)+"x", err)
	}

	// 2. 检查 markitdown 是否安装
	if err := exec.Command(pythonPath, "-c", markitdownCheck).Run(); err != nil {
		return nil, fmt.Errorf("旧格式 %s 解析需要 markitdown，请运行 pip install markitdown 或将文件转为 %s 格式",
			fileType, strings.ToUpper(fileType)+"x")
	}

	// 3. 写临时文件（旧格式需文件路径，不支持 stdin 管道）
	ext := strings.ToLower(strings.TrimSpace(fileType))
	tmpFile, err := os.CreateTemp("", "cognik_parse_*."+ext)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	// 4. 调用 markitdown 转换
	cmd := exec.Command(pythonPath, "-m", "markitdown", tmpPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("markitdown 解析 %s 失败: %w（stderr: %s）",
			fileType, err, truncate(stderr.String(), 300))
	}

	markdown := strings.TrimSpace(stdout.String())
	if markdown == "" {
		return nil, fmt.Errorf("markitdown 解析 %s 结果为空", fileType)
	}

	// 5. 旧格式不提取图片
	return &ParseResult{Markdown: markdown}, nil
}

// findPython 按优先级查找可用的 Python 解释器路径。
func findPython() (string, error) {
	for _, name := range pythonCandidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("未找到 Python 解释器（尝试过: %s）", strings.Join(pythonCandidates, ", "))
}

// truncate 截断字符串到指定长度，超出加省略号。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

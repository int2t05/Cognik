// Package tools 提供 Agent 内置工具集。
// bash.go：bash 命令执行工具。
//
// 高级特性：
//   - 平台自适应 bash 二进制（Windows 默认 GitBash，OPSMIND_AGENT_BASH_BIN 可覆盖）
//   - description 参数强制意图声明
//   - timeout 参数可覆盖默认（上限 10min）
//   - workDir sandbox、stdout+stderr 截断、exit_code 始终返回（不抛 error，agent 自判断）

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"opsmind/internal/agent"

	"github.com/cloudwego/eino/schema"
)

// bashMaxTimeout 单次命令超时上限。
const bashMaxTimeout = 10 * time.Minute

// BashTool bash 命令执行工具（实现 agent.SyncTool 接口）。
type BashTool struct {
	workDir  string
	timeout  time.Duration
	maxBytes int64
	bashBin  string // 解析后的 bash 二进制路径
}

// NewBashTool 创建 bash 工具。自动探测平台 bash 二进制（Windows 用 GitBash）。
func NewBashTool(workDir string, timeout time.Duration, maxBytes int64) *BashTool {
	return &BashTool{
		workDir:  workDir,
		timeout:  timeout,
		maxBytes: maxBytes,
		bashBin:  resolveBashBin(),
	}
}

// Info 返回工具元信息。
func (b *BashTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "bash",
		Desc: "Execute a shell command and return stdout+stderr. Use for system checks, diagnostics, text processing. Always state the purpose in description. The working directory is the agent sandbox.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:     schema.String,
				Desc:     "The shell command to execute",
				Required: true,
			},
			"description": {
				Type: schema.String,
				Desc: "Short description of why this command is run (5-10 words). Forces intent clarity before execution.",
			},
			"timeout": {
				Type: schema.Integer,
				Desc: fmt.Sprintf("Timeout in milliseconds (default %d, max %d). Override for long-running commands.", int(b.timeout.Milliseconds()), int(bashMaxTimeout.Milliseconds())),
			},
		}),
	}
}

// bashParams bash 工具参数。
type bashParams struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Timeout     int    `json:"timeout,omitempty"` // 毫秒，覆盖默认
}

// Call 执行 bash 命令。失败也返回字符串（含 exit_code），不返回 error——agent 看 exit_code。
func (b *BashTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params bashParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.Command) == "" {
		return "", fmt.Errorf("command is required")
	}

	// 超时：参数覆盖默认，上限 bashMaxTimeout
	timeout := b.timeout
	if params.Timeout > 0 {
		t := time.Duration(params.Timeout) * time.Millisecond
		if t > bashMaxTimeout {
			t = bashMaxTimeout
		}
		timeout = t
	}
	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(tCtx, b.bashBin, "-c", params.Command)
	cmd.Dir = b.workDir
	// 强制 UTF-8 输出（Python/其他程序在 Windows 下默认 GBK 致 emoji 乱码）。
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8", "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// 超时单独标识（exit_code + timeout 信息）
	if tCtx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("exit_code=%d\nerror=timeout after %s\nstdout=%s", exitCode, timeout, truncate(stdout.String(), b.maxBytes)), nil
	}
	if err != nil && exitCode == 0 {
		return fmt.Sprintf("exit_code=%d\nerror=%s\nstdout=%s", exitCode, err.Error(), truncate(stdout.String(), b.maxBytes)), nil
	}

	result := fmt.Sprintf("exit_code=%d\nstdout=%s", exitCode, truncate(stdout.String(), b.maxBytes))
	if stderr.Len() > 0 {
		result += "\n--- stderr ---\n" + truncate(stderr.String(), b.maxBytes)
	}
	return result, nil
}

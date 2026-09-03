// Package tools 提供 Agent 内置工具集。
// async_bash.go：异步 bash 工具（StreamableTool 接口）。
//
// 与普通 BashTool（InvokableTool）的区别：
// - 普通工具：阻塞执行，Agent 等待结果
// - 异步工具：流式输出 stdout/stderr，Agent 实时看到命令输出（长命令不阻塞 SSE 流）
// - 实现 Eino StreamableTool 接口：StreamableRun 返回 StreamReader[string]

package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// AsyncBashTool 异步 bash 命令执行工具（实现 eino StreamableTool 接口）。
// 流式返回 stdout，让 Agent 实时看到命令输出。
type AsyncBashTool struct {
	workDir  string
	timeout  time.Duration
	maxBytes int64
	bashBin  string
}

// NewAsyncBashTool 创建异步 bash 工具。
func NewAsyncBashTool(workDir string, timeout time.Duration, maxBytes int64) *AsyncBashTool {
	return &AsyncBashTool{
		workDir:  workDir,
		timeout:  timeout,
		maxBytes: maxBytes,
		bashBin:  resolveBashBin(),
	}
}

// Info 返回工具元信息。
func (b *AsyncBashTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "async_bash",
		Desc: "Execute a shell command with streaming output. Use for long-running commands where you want to see output in real-time. Returns stdout/stderr chunks as they arrive.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:     schema.String,
				Desc:     "The shell command to execute",
				Required: true,
			},
			"description": {
				Type: schema.String,
				Desc: "Short description of why this command is run (5-10 words).",
			},
		}),
	}, nil
}

// StreamableRun 执行 bash 命令，流式返回 stdout。
func (b *AsyncBashTool) StreamableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (*schema.StreamReader[string], error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.Command) == "" {
		return nil, fmt.Errorf("command is required")
	}

	tCtx, cancel := context.WithTimeout(ctx, b.timeout)

	cmd := exec.CommandContext(tCtx, b.bashBin, "-c", params.Command)
	cmd.Dir = b.workDir

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("启动命令失败: %w", err)
	}

	// 创建 StreamReader
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer cancel()

		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var totalBytes int64
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			totalBytes += int64(len(line))
			if totalBytes > b.maxBytes {
				ch <- "\n...[output truncated at %d bytes]\n"
				break
			}
			select {
			case ch <- line:
			case <-tCtx.Done():
				ch <- fmt.Sprintf("\n[timeout after %s]\n", b.timeout)
				return
			}
		}

		// stderr
		errScanner := bufio.NewScanner(stderrPipe)
		errScanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for errScanner.Scan() {
			line := "--- stderr ---\n" + errScanner.Text() + "\n"
			totalBytes += int64(len(line))
			if totalBytes > b.maxBytes {
				break
			}
			select {
			case ch <- line:
			case <-tCtx.Done():
				return
			}
		}

		// 等待命令完成
		err := cmd.Wait()
		if err != nil {
			exitCode := -1
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
			ch <- fmt.Sprintf("\nexit_code=%d\n", exitCode)
		} else {
			ch <- fmt.Sprintf("\nexit_code=0\n")
		}
	}()

	// 将 chan string 转为 *schema.StreamReader[string]
	reader, writer := schema.Pipe[string](64)
	go func() {
		defer writer.Close()
		for s := range ch {
			if !writer.Send(s, nil) {
				return
			}
		}
	}()
	return reader, nil
}

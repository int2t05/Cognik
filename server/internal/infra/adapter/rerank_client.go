// Package adapter 提供外部服务的适配层。
package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// 接口定义
// =============================================================================

// Reranker 定义 cross-encoder 重排序接口，解耦实现细节。
type Reranker interface {
	// Rerank 对候选 passages 按与 query 的相关性重新排序。
	//
	// 返回按分数降序排列的 passage ID 序列及对应分数。
	// 调用失败时 Pipeline 降级为原始排序。
	Rerank(ctx context.Context, query string, passages []RerankPassage) (*RerankResult, error)
}

// =============================================================================
// 请求/响应类型
// =============================================================================

// RerankPassage 候选文档片段。
type RerankPassage struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// RerankResult 重排序结果。
type RerankResult struct {
	Order  []int     `json:"order"`  // 按分数降序的 passage ID
	Scores []float64 `json:"scores"` // 对应相关性分数（[0,1]）
}

// =============================================================================
// 子进程实现
// =============================================================================

// SubprocessReranker 通过子进程调用 Python cross-encoder 推理。
// 通信协议为 stdin/stdout JSON Lines；stdin 写入串行化，stdout 单 goroutine 分发。
// 子进程崩溃时通知等待请求返回错误，调用方降级为原始排序。
type SubprocessReranker struct {
	cmd        *exec.Cmd
	pythonPath string // 保存用于自动重启
	scriptPath string
	stdin      io.WriteCloser
	mu         sync.Mutex                      // 保护 stdin 写入
	pending    map[string]chan *rerankResponse // reqID → 响应 channel
	pendingMu  sync.Mutex
	reqSeq     int
	closed     atomic.Bool
}

// rerankRequest JSON Lines 输入格式。
type rerankRequest struct {
	ReqID    string          `json:"req_id"`
	Query    string          `json:"query"`
	Passages []RerankPassage `json:"passages"`
}

// rerankResponse JSON Lines 输出格式。
type rerankResponse struct {
	ReqID  string    `json:"req_id"`
	Order  []int     `json:"order"`
	Scores []float64 `json:"scores"`
	Error  string    `json:"error,omitempty"`
}

// NewSubprocessReranker 创建并启动 Python 推理子进程，启动失败返回 nil。
func NewSubprocessReranker(pythonPath, scriptPath string) *SubprocessReranker {
	r := &SubprocessReranker{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		pending:    make(map[string]chan *rerankResponse),
	}

	if err := r.start(pythonPath, scriptPath); err != nil {
		slog.Error("rerank 子进程启动失败，重排序将降级跳过", "error", err)
		return nil
	}

	slog.Info("rerank 子进程已启动", "python", pythonPath, "script", scriptPath)
	return r
}

// start 启动子进程并开始读取 stdout。
func (r *SubprocessReranker) start(pythonPath, scriptPath string) error {
	r.cmd = exec.Command(pythonPath, "-u", scriptPath) // -u 禁用 Python 输出缓冲
	r.cmd.Stderr = os.Stderr                           // Python 日志直接输出到 stderr

	stdin, err := r.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建 stdin pipe 失败: %w", err)
	}
	r.stdin = stdin

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("启动子进程失败: %w", err)
	}

	// 后台读取 stdout 分发响应
	go r.readStdout(stdout)
	// 监控子进程退出并重启
	go r.monitorProcess()

	return nil
}

// readStdout 读取子进程 stdout，解析 JSON Lines 并路由到等待的请求。
func (r *SubprocessReranker) readStdout(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		var resp rerankResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// 截断长行后打日志（100 字符足够诊断格式错误）
			preview := line
			if len(preview) > 100 {
				preview = preview[:100]
			}
			slog.Warn("rerank 子进程返回非 JSON 行，跳过", "line", preview)
			continue
		}

		r.pendingMu.Lock()
		ch, ok := r.pending[resp.ReqID]
		if ok {
			delete(r.pending, resp.ReqID)
		}
		r.pendingMu.Unlock()

		if ok {
			ch <- &resp
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Error("rerank 子进程 stdout 读取失败", "error", err)
	}
}

// monitorProcess 监控子进程退出，通知等待请求并尝试重启。
func (r *SubprocessReranker) monitorProcess() {
	err := r.cmd.Wait()
	slog.Warn("rerank 子进程已退出，将自动重启", "error", err)

	r.pendingMu.Lock()
	r.closed.Store(true)
	for id, ch := range r.pending {
		ch <- &rerankResponse{Error: "rerank 子进程已退出"}
		delete(r.pending, id)
	}
	r.pendingMu.Unlock()

	// 等待模型文件稳定后尝试重启
	time.Sleep(3 * time.Second)
	if restartErr := r.start(r.pythonPath, r.scriptPath); restartErr != nil {
		slog.Error("rerank 子进程重启失败，需人工恢复", "error", restartErr)
	} else {
		r.closed.Store(false)
		slog.Info("rerank 子进程已自动重启")
	}
}

// Rerank 执行重排序，ctx 取消/超时返回错误供调用方降级。
func (r *SubprocessReranker) Rerank(ctx context.Context, query string, passages []RerankPassage) (*RerankResult, error) {
	if r == nil || r.closed.Load() {
		return nil, nil // 降级：返回 nil 让调用方用原排序
	}

	if len(passages) <= 1 {
		order := make([]int, len(passages))
		for i := range order {
			order[i] = passages[i].ID
		}
		return &RerankResult{Order: order, Scores: []float64{1.0}}, nil
	}

	// 注册请求 ID 与响应 channel
	r.pendingMu.Lock()
	r.reqSeq++
	reqID := fmt.Sprintf("%d", r.reqSeq)
	respCh := make(chan *rerankResponse, 1)
	r.pending[reqID] = respCh
	r.pendingMu.Unlock()

	defer func() {
		r.pendingMu.Lock()
		delete(r.pending, reqID)
		r.pendingMu.Unlock()
	}()

	req := rerankRequest{
		ReqID:    reqID,
		Query:    query,
		Passages: passages,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化 rerank 请求失败: %w", err)
	}

	// 写 stdin（线程安全）
	r.mu.Lock()
	_, err = fmt.Fprintf(r.stdin, "%s\n", string(reqJSON))
	r.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("写入 rerank 子进程失败: %w", err)
	}

	// 内部 30s 超时（防止子进程挂起时无 ctx deadline 永久阻塞）
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	select {
	case <-timeoutCtx.Done():
		return nil, timeoutCtx.Err()
	case resp := <-respCh:
		if resp.Error != "" {
			return nil, fmt.Errorf("rerank 子进程错误: %s", resp.Error)
		}
		return &RerankResult{
			Order:  resp.Order,
			Scores: resp.Scores,
		}, nil
	}
}

// Close 优雅关闭子进程：关 stdin 通知退出，再发 SIGTERM 兜底。
func (r *SubprocessReranker) Close() error {
	if r == nil || r.closed.Load() {
		return nil
	}
	r.closed.Store(true)

	if r.stdin != nil {
		r.stdin.Close()
	}
	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

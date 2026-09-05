// agent/task.go：后台任务单元 + 注册表。
//
// AsyncTool.Dispatch 创建 *Task；Loop 通过 waitForAny 等待完成，
// 注入 user notification 恢复 LLM 循环（fire-and-forget + completion-notification）。
package agent

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
)

// 任务状态常量。
const (
	TaskRunning  = "running"
	TaskDone     = "done"
	TaskFailed   = "failed"
	TaskCancelled = "cancelled"
)

// 任务类型（Cognik 仅用 subagent/bash）。
const (
	taskKindSubagent = "subagent"
	taskKindBash     = "bash"
)

// Task 后台任务单元。AsyncTool.Dispatch 创建，Loop 通过 waitForAny 等待完成。
type Task struct {
	ID         string             // 任务 ID（前缀+随机，如 a-4f3a2b1c）
	ToolCallID string             // 原始 tool_use_id（前端通知配对用，dispatch 后由 Loop 设置）
	Status     string             // running | done | failed | cancelled
	Result     string             // 最终结果（完成时设置）
	Err        error              // 错误（失败/取消时设置）
	done       chan struct{}      // 完成信号（close 触发 waitForAny 返回）
	cancel     context.CancelFunc // 取消函数（子 ctx，链到父）
	once       sync.Once          // markDone/Cancel 幂等保证
}

// newTask 创建任务（done channel + cancel）。
func newTask(kind string, cancel context.CancelFunc) *Task {
	return &Task{
		ID:     generateTaskID(kind),
		Status: TaskRunning,
		done:   make(chan struct{}),
		cancel: cancel,
	}
}

// markDone 标记任务完成（幂等，close done channel）。
// 后台 goroutine 完成时调用，触发 Loop 的 waitForAny 返回。
func (t *Task) markDone(result string, err error) {
	t.once.Do(func() {
		t.Result = result
		t.Err = err
		if err != nil {
			t.Status = TaskFailed
		} else {
			t.Status = TaskDone
		}
		close(t.done)
	})
}

// Cancel 取消任务（幂等）。调用子 ctx cancel + 标记 cancelled。
func (t *Task) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
	t.once.Do(func() {
		t.Status = TaskCancelled
		t.Err = context.Canceled
		close(t.done)
	})
}

// generateTaskID 生成任务 ID（前缀 + 8 位随机十六进制）。
func generateTaskID(kind string) string {
	prefix := "x"
	switch kind {
	case taskKindSubagent:
		prefix = "a"
	case taskKindBash:
		prefix = "b"
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%x", prefix, b)
}

// TaskRegistry 后台任务注册表（ID→*Task）。
// 供前端状态查询/取消 API（未来扩展）；当前主要用于生命周期跟踪。
type TaskRegistry struct {
	tasks sync.Map
}

// NewTaskRegistry 创建注册表。
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{}
}

// Register 注册任务。
func (r *TaskRegistry) Register(t *Task) {
	r.tasks.Store(t.ID, t)
}

// Get 按 ID 查询任务。
func (r *TaskRegistry) Get(id string) (*Task, bool) {
	v, ok := r.tasks.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Task), true
}

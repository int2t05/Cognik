// Package task 提供 Agent 异步任务管理。
// manager.go：TaskManager 创建/执行/查询/取消任务。

package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"opsmind/internal/agent"
	"opsmind/internal/infra/runtime"
)

// TaskInput 任务输入参数。
type TaskInput struct {
	Question string `json:"question"` // Agent 问题
}

// TaskOutput 任务输出结果。
type TaskOutput struct {
	Answer  string `json:"answer"`  // Agent 回答
	Parts   int    `json:"parts"`   // parts 数量
}

// TaskManager 管理异步任务生命周期。
type TaskManager struct {
	store        TaskStore
	agentRunner  *agent.AgentRunner
	gateway      *runtime.Gateway[any] // 事件发布（可选）
	cancels      sync.Map              // taskID → context.CancelFunc
}

// NewTaskManager 创建任务管理器。
func NewTaskManager(store TaskStore, runner *agent.AgentRunner) *TaskManager {
	return &TaskManager{
		store:       store,
		agentRunner: runner,
	}
}

// CreateTask 创建任务（pending 状态）。
func (m *TaskManager) CreateTask(ctx context.Context, threadID, userID int64, taskType, input string) (*Task, error) {
	task := &Task{
		ThreadID: threadID,
		UserID:   userID,
		Type:     taskType,
		Status:   StatusPending,
		Input:    input,
	}
	if err := m.store.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}
	return task, nil
}

// ExecuteTask 后台执行任务（detached goroutine）。
func (m *TaskManager) ExecuteTask(taskID int64, question string) {
	// 标记为 running
	ctx := context.Background()
	task, err := m.store.Get(ctx, taskID, 0) // 后台执行，不校验 userID
	if err != nil {
		slog.Error("任务不存在", "task_id", taskID, "error", err)
		return
	}

	task.Status = StatusRunning
	_ = m.store.Update(ctx, task)

	// 独立 ctx（不受请求取消影响）
	gctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	m.cancels.Store(taskID, cancel)
	defer func() {
		m.cancels.Delete(taskID)
	}()

	// 解析输入
	var taskInput TaskInput
	_ = json.Unmarshal([]byte(task.Input), &taskInput)

	// 执行 Agent
	agentEvents, err := m.agentRunner.Stream(gctx, []*schema.Message{schema.UserMessage(taskInput.Question)})
	if err != nil {
		task.Status = StatusFailed
		task.Error = err.Error()
		_ = m.store.Update(ctx, task)
		return
	}

	// 收集结果
	var answer string
	for evt := range agentEvents {
		if evt.Type == agent.EventToken {
			answer += evt.Content
		}
		if evt.Type == agent.EventError {
			task.Status = StatusFailed
			task.Error = evt.Error
			_ = m.store.Update(ctx, task)
			return
		}
	}

	// 完成
	output, _ := json.Marshal(TaskOutput{Answer: answer})
	task.Status = StatusCompleted
	task.Output = string(output)
	_ = m.store.Update(ctx, task)
}

// GetTask 查询任务状态。
func (m *TaskManager) GetTask(ctx context.Context, taskID, userID int64) (*Task, error) {
	return m.store.Get(ctx, taskID, userID)
}

// ListTasks 列出线程的任务。
func (m *TaskManager) ListTasks(ctx context.Context, threadID int64) ([]Task, error) {
	return m.store.List(ctx, threadID)
}

// CancelTask 取消任务。
func (m *TaskManager) CancelTask(ctx context.Context, taskID, userID int64) error {
	task, err := m.store.Get(ctx, taskID, userID)
	if err != nil {
		return fmt.Errorf("任务不存在")
	}
	if task.Status != StatusRunning && task.Status != StatusPending {
		return fmt.Errorf("任务已完成，无法取消")
	}

	// 触发取消
	if cancel, ok := m.cancels.Load(taskID); ok {
		cancel.(context.CancelFunc)()
	}

	task.Status = StatusCancelled
	return m.store.Update(ctx, task)
}

// CleanupStale 清理残留的 running 任务。
func (m *TaskManager) CleanupStale(ctx context.Context) error {
	_, err := m.store.CleanupStale(ctx)
	return err
}

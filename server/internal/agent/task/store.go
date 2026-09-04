// Package task 提供 Agent 异步任务管理。
// store.go：TaskStore 接口 + SQLite 实现。

package task

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TaskStore 任务存储接口。
type TaskStore interface {
	Create(ctx context.Context, task *Task) error
	Get(ctx context.Context, taskID, userID int64) (*Task, error)
	List(ctx context.Context, threadID int64) ([]Task, error)
	Update(ctx context.Context, task *Task) error
	CleanupStale(ctx context.Context) (int64, error)
}

// SQLiteTaskStore SQLite 实现（复用 agent SQLite 连接）。
type SQLiteTaskStore struct {
	db *gorm.DB
}

// NewSQLiteTaskStore 创建 SQLite 任务存储。
func NewSQLiteTaskStore(db *gorm.DB) *SQLiteTaskStore {
	return &SQLiteTaskStore{db: db}
}

// Create 创建任务。
func (s *SQLiteTaskStore) Create(ctx context.Context, task *Task) error {
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = task.CreatedAt
	return s.db.WithContext(ctx).Create(task).Error
}

// Get 获取任务（含归属校验）。
func (s *SQLiteTaskStore) Get(ctx context.Context, taskID, userID int64) (*Task, error) {
	var task Task
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// List 列出线程的任务。
func (s *SQLiteTaskStore) List(ctx context.Context, threadID int64) ([]Task, error) {
	var tasks []Task
	if err := s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// Update 更新任务（status/output/error）。
func (s *SQLiteTaskStore) Update(ctx context.Context, task *Task) error {
	task.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Model(&Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status": task.Status,
		"output": task.Output,
		"error":  task.Error,
	}).Error
}

// CleanupStale 清理启动时残留的 running 任务（标记为 failed）。
func (s *SQLiteTaskStore) CleanupStale(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Model(&Task{}).Where("status = ?", StatusRunning).Update("status", StatusFailed)
	return result.RowsAffected, result.Error
}

// 确保 SQLiteTaskStore 实现 TaskStore 接口。
var _ TaskStore = (*SQLiteTaskStore)(nil)

// NotFoundError 任务不存在错误。
func NotFoundError(taskID int64) error {
	return fmt.Errorf("任务 %d 不存在", taskID)
}

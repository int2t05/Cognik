// Package store 提供 Agent 对话数据的独立存储（SQLite）。
// store.go：ChatStore 接口 + SQLite 实现。

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"opsmind/internal/agent/task"
)

// ChatStore Agent 对话数据存储接口（与业务 PostgreSQL 隔离）。
type ChatStore interface {
	ListThreads(ctx context.Context, userID int64) ([]Thread, error)
	CreateThread(ctx context.Context, userID int64, title string) (*Thread, error)
	GetThread(ctx context.Context, threadID, userID int64) (*Thread, error)
	DeleteThread(ctx context.Context, threadID, userID int64) error
	UpdateThread(ctx context.Context, thread *Thread) error

	SaveMessage(ctx context.Context, msg *Message) error
	ListMessages(ctx context.Context, threadID int64) ([]Message, error)
	UpdateMessage(ctx context.Context, msg *Message) error
	GetMessage(ctx context.Context, messageID, threadID int64) (*Message, error)
	CleanupStale(ctx context.Context) (int64, error)
}

// SQLiteStore SQLite 实现。DB 文件 data/agent.db。
type SQLiteStore struct {
	db *gorm.DB
}

// NewSQLiteStore 创建 SQLite 存储。dbPath 为 SQLite 文件路径；用 ":memory:" 则纯内存。
// 注意：:memory: 模式下 GORM 连接池的每个连接看到独立空库，需用 shared cache 模式。
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dsn := dbPath
	if dbPath == ":memory:" {
		dsn = "file::memory:?cache=shared" // 共享缓存，所有连接看到同一内存库
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	if err := db.AutoMigrate(&Thread{}, &Message{}, &task.Task{}); err != nil {
		return nil, fmt.Errorf("SQLite 迁移失败: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Close 关闭底层 DB 连接。
func (s *SQLiteStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// ListThreads 列出用户的对话线程（按更新时间倒序）。
func (s *SQLiteStore) ListThreads(ctx context.Context, userID int64) ([]Thread, error) {
	var threads []Thread
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC").Find(&threads).Error; err != nil {
		return nil, err
	}
	return threads, nil
}

// CreateThread 创建新对话线程。
func (s *SQLiteStore) CreateThread(ctx context.Context, userID int64, title string) (*Thread, error) {
	if title == "" {
		title = "新对话"
	}
	now := time.Now()
	thread := &Thread{
		UserID:    userID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(thread).Error; err != nil {
		return nil, err
	}
	return thread, nil
}

// GetThread 获取线程（含归属校验）。
func (s *SQLiteStore) GetThread(ctx context.Context, threadID, userID int64) (*Thread, error) {
	var thread Thread
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", threadID, userID).First(&thread).Error; err != nil {
		return nil, err
	}
	return &thread, nil
}

// DeleteThread 删除线程及其全部消息。
func (s *SQLiteStore) DeleteThread(ctx context.Context, threadID, userID int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", threadID, userID).Delete(&Thread{}).Error; err != nil {
			return err
		}
		return tx.Where("thread_id = ?", threadID).Delete(&Message{}).Error
	})
}

// UpdateThread 更新线程（标题/时间）。
func (s *SQLiteStore) UpdateThread(ctx context.Context, thread *Thread) error {
	thread.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Model(&Thread{}).Where("id = ?", thread.ID).Updates(map[string]any{
		"title":      thread.Title,
		"updated_at": thread.UpdatedAt,
	}).Error
}

// SaveMessage 保存消息（新增或更新）。
func (s *SQLiteStore) SaveMessage(ctx context.Context, msg *Message) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	return s.db.WithContext(ctx).Save(msg).Error
}

// ListMessages 列出线程的消息（按时间正序）。
func (s *SQLiteStore) ListMessages(ctx context.Context, threadID int64) ([]Message, error) {
	var messages []Message
	if err := s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at ASC, id ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

// UpdateMessage 更新消息（parts/status/error）。
func (s *SQLiteStore) UpdateMessage(ctx context.Context, msg *Message) error {
	return s.db.WithContext(ctx).Model(&Message{}).Where("id = ?", msg.ID).Updates(map[string]any{
		"parts":  msg.Parts,
		"status": msg.Status,
		"error":  msg.Error,
	}).Error
}

// GetMessage 获取单条消息（含线程归属校验）。
func (s *SQLiteStore) GetMessage(ctx context.Context, messageID, threadID int64) (*Message, error) {
	var msg Message
	if err := s.db.WithContext(ctx).Where("id = ? AND thread_id = ?", messageID, threadID).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

// CleanupStale 清理启动时残留的 generating 状态消息（上次异常退出遗留）。
func (s *SQLiteStore) CleanupStale(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Model(&Message{}).Where("status = ?", MessageStatusGenerating).
		Update("status", MessageStatusFailed)
	return result.RowsAffected, result.Error
}

// PartsToJSON 将 parts 数组序列化为 JSON 字符串（存 Message.Parts）。
func PartsToJSON(parts []MessagePart) (string, error) {
	data, err := json.Marshal(parts)
	if err != nil {
		return "[]", err
	}
	return string(data), nil
}

// ParseParts 从 JSON 字符串解析 parts 数组。
func ParseParts(partsJSON string) ([]MessagePart, error) {
	var parts []MessagePart
	if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
		return nil, err
	}
	return parts, nil
}

// 确保 SQLiteStore 实现 ChatStore 接口（编译时校验）。
var _ ChatStore = (*SQLiteStore)(nil)

// UpsertMessage 使用 upsert 语义保存消息（若存在则更新，避免主键冲突）。
func (s *SQLiteStore) UpsertMessage(ctx context.Context, msg *Message) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"parts", "status", "error"}),
	}).Create(msg).Error
}

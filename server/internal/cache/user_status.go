// Package cache 提供后端内存缓存基础设施。
//
// UserStatusCache 缓存用户冻结/正常状态，减少高并发下每个 API 请求的 DB 查询。
// TTL 默认 30 秒，缓存未命中时回退到 DB 查询。
package cache

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

// UserStatusCache 用户状态内存缓存。
//
// 为什么用 sync.RWMutex + map 而非 sync.Map：
// sync.Map 适合读多写少且 key 集合稳定的场景，
// 但用户状态缓存的 key 集合动态变化且需要 TTL 清理，普通 map + RWMutex 更直观。
type UserStatusCache struct {
	db      *gorm.DB
	mu      sync.RWMutex
	entries map[int64]statusEntry
	ttl     time.Duration
}

type statusEntry struct {
	status    int
	expiresAt time.Time
}

// NewUserStatusCache 创建用户状态缓存。
// ttl 为缓存有效期，默认 30 秒。
func NewUserStatusCache(db *gorm.DB, ttl time.Duration) *UserStatusCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &UserStatusCache{
		db:      db,
		entries: make(map[int64]statusEntry),
		ttl:     ttl,
	}
}

// GetStatus 获取用户状态（1=正常, 2=冻结）。
// 优先读缓存，未命中则查 DB 并写入缓存。
func (c *UserStatusCache) GetStatus(ctx context.Context, userID int64) (int, error) {
	c.mu.RLock()
	e, ok := c.entries[userID]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		return e.status, nil
	}

	var status int
	err := c.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		Select("status").
		Scan(&status).Error
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	c.entries[userID] = statusEntry{status: status, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return status, nil
}

// Invalidate 使指定用户的缓存失效（冻结/恢复操作后调用）。
func (c *UserStatusCache) Invalidate(userID int64) {
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()
}

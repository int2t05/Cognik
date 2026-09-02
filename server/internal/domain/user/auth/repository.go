// Package auth 封装认证领域的内存数据存储层。
//
// repository.go 管理登录限流器与 token 黑名单——两者均为内存级数据结构，
// MVP 阶段单实例部署足够，避免引入 Redis 等额外依赖。
package auth

import (
	"time"
)

// loginFailRecord 记录单个用户的登录失败信息。
//
// 使用滑动窗口计数：firstAt 为窗口起始时间，count 为窗口内失败次数。
type loginFailRecord struct {
	count   int
	firstAt time.Time
}

// loginRateLimiter 基于内存的登录失败限流器。
//
// 限制策略：同一用户名在 window 内连续失败 maxFails 次后，后续尝试直接拒绝。
// 成功登录会清除该用户的失败记录。
type loginRateLimiter struct {
	attempts map[string]*loginFailRecord
	maxFails int
	window   time.Duration
}

func newLoginRateLimiter() *loginRateLimiter {
	return &loginRateLimiter{
		attempts: make(map[string]*loginFailRecord),
		maxFails: 5,
		window:   15 * time.Minute,
	}
}

// allowLogin 检查是否允许该用户名尝试登录。
func (r *loginRateLimiter) allowLogin(username string) error {
	rec, exists := r.attempts[username]
	if !exists {
		return nil
	}

	if time.Since(rec.firstAt) > r.window {
		delete(r.attempts, username)
		return nil
	}

	if rec.count >= r.maxFails {
		return AppError{Code: 10003, Message: "登录失败次数过多，请15分钟后再试"}
	}
	return nil
}

// recordFail 记录一次登录失败。
func (r *loginRateLimiter) recordFail(username string) {
	rec, exists := r.attempts[username]
	if !exists || time.Since(rec.firstAt) > r.window {
		r.attempts[username] = &loginFailRecord{count: 1, firstAt: time.Now()}
		return
	}
	rec.count++
}

// recordSuccess 登录成功后清除失败记录。
func (r *loginRateLimiter) recordSuccess(username string) {
	delete(r.attempts, username)
}

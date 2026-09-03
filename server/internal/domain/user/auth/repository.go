// Package auth 认证领域内存数据存储（登录限流器、token 黑名单）。
package auth

import (
	"fmt"
	"time"
)

// loginFailRecord 登录失败记录（滑动窗口计数）。
type loginFailRecord struct {
	count   int
	firstAt time.Time
}

// loginRateLimiter 内存登录限流器（窗口内失败超限后拒绝，成功清除记录）。
type loginRateLimiter struct {
	attempts map[string]*loginFailRecord
	maxFails int
	window   time.Duration
}

func newLoginRateLimiter() *loginRateLimiter {
	return &loginRateLimiter{
		attempts: make(map[string]*loginFailRecord),
		maxFails: 5,
		window:   3 * time.Minute,
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
		return AppError{Code: 10003, Message: fmt.Sprintf("登录失败次数过多，请%d分钟后再试", int(r.window.Minutes()))}
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

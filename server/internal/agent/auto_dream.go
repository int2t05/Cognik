// Package agent 提供自建 ReAct 循环与工具接口。
//
// auto_dream.go：跨会话记忆复盘 forked agent。
// 参考 Claude Code AutoDream：双门触发（时间 + 会话数）+ 锁 + 4 阶段 prompt + 游标追踪。
// 合并重复记忆、删除矛盾、更新 MEMORY.md 索引。
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cognik/internal/agent/llm"
)

// AutoDream 跨会话记忆复盘 agent。
type AutoDream struct {
	memoryRoot string                  // 记忆根目录（如 storage/memory/）
	modelGetter func() *llm.ChatModel
	minHours   int                     // 时间门：距上次复盘 ≥ N 小时
	minSessions int                    // 会话数门：新增会话 ≥ N 个
	maxTurns    int                     // forked agent 最大步数
	lockTTL     time.Duration          // 锁过期时间（PID 复用保护）
}

// NewAutoDream 创建复盘 agent。
func NewAutoDream(memoryRoot string, modelGetter func() *llm.ChatModel) *AutoDream {
	return &AutoDream{
		memoryRoot:  memoryRoot,
		modelGetter: modelGetter,
		minHours:    24,
		minSessions: 5,
		maxTurns:    10,
		lockTTL:     60 * time.Minute,
	}
}

// MaybeConsolidate 检查双门，通过则执行复盘。
// 应在定时器（如每 10 分钟）或会话结束时调用。
func (a *AutoDream) MaybeConsolidate(ctx context.Context) error {
	if a == nil || a.modelGetter == nil {
		return nil
	}

	lockPath := filepath.Join(a.memoryRoot, "global", ".consolidate-lock")

	// Gate 1: 时间门——锁文件 mtime 距今 ≥ minHours（1 次 stat，最便宜）
	last, ok := a.lastConsolidated(lockPath)
	if ok && time.Since(last) < time.Duration(a.minHours)*time.Hour {
		return nil // 时间未到
	}

	// Gate 2: 会话数门——新会话数 ≥ minSessions（1 次目录扫描）
	sessionsDir := filepath.Join(a.memoryRoot, "sessions")
	count, err := a.countNewSessions(sessionsDir, last)
	if err != nil || count < a.minSessions {
		return nil // 会话数不足
	}

	// Gate 3: 锁——无其他进程在复盘
	if !a.acquireLock(lockPath) {
		return nil // 已有进程在复盘
	}

	slog.Info("AutoDream 复盘触发", "new_sessions", count, "last_consolidated", last)

	// 执行复盘（forked agent）
	err = a.consolidate(ctx)

	// 更新锁 mtime（无论成功失败都更新，避免重复触发；失败时游标前进但可下次重试）
	a.touchLock(lockPath)

	if err != nil {
		slog.Warn("AutoDream 复盘失败", "error", err)
		return err
	}

	slog.Info("AutoDream 复盘完成")
	return nil
}

// consolidate 执行 4 阶段复盘 prompt（forked agent）。
func (a *AutoDream) consolidate(ctx context.Context) error {
	instruction := a.buildConsolidationPrompt()

	// 读取当前 global MEMORY.md 作为上下文
	globalDir := filepath.Join(a.memoryRoot, "global")
	memoryIndex := ""
	if data, err := os.ReadFile(filepath.Join(globalDir, "MEMORY.md")); err == nil {
		memoryIndex = string(data)
	}

	// 列出现有记忆条目
	var existing []string
	if entries, err := os.ReadDir(globalDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && e.Name() != "MEMORY.md" && e.Name() != ".consolidate-lock" {
				existing = append(existing, e.Name())
			}
		}
	}

	input := []*llm.Message{
		llm.SystemMessage(instruction),
		llm.UserMessage(fmt.Sprintf("现有记忆索引：\n%s\n\n现有记忆文件：\n%s\n\n请执行 4 阶段复盘。",
			memoryIndex, strings.Join(existing, "\n"))),
	}

	_, err := RunForkedAgentSimple(ctx, a.modelGetter, instruction, input, a.maxTurns)
	return err
}

// buildConsolidationPrompt 构造 4 阶段复盘 prompt（参考 Claude Code consolidationPrompt.ts）。
func (a *AutoDream) buildConsolidationPrompt() string {
	return `你是记忆复盘 agent。执行 4 阶段跨会话记忆合并。

## Phase 1 — Orient
浏览现有记忆目录，读 MEMORY.md 索引，了解已有记忆条目。

## Phase 2 — Gather
收集新增的 session 记忆，检测与现有记忆的矛盾或重复。

## Phase 3 — Consolidate
合并重复记忆（而非创建近似副本），转换相对日期为绝对日期，删除矛盾事实。只基于现有记忆合并，不编造未记录的事实。

## Phase 4 — Prune
更新 MEMORY.md 索引保持 ≤200 行，移除过时指针，精简冗长条目。`
}

// lastConsolidated 返回锁文件的 mtime（即上次复盘时间）。
func (a *AutoDream) lastConsolidated(lockPath string) (time.Time, bool) {
	info, err := os.Stat(lockPath)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// countNewSessions 统计 mtime 在 last 之后的会话目录数。
func (a *AutoDream) countNewSessions(sessionsDir string, last time.Time) (int, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(last) {
			count++
		}
	}
	return count, nil
}

// acquireLock 尝试获取锁（PID 文件）。已存在且未过期则失败。
func (a *AutoDream) acquireLock(lockPath string) bool {
	if data, err := os.ReadFile(lockPath); err == nil {
		// 检查锁是否过期
		if info, err := os.Stat(lockPath); err == nil {
			if time.Since(info.ModTime()) > a.lockTTL {
				// 锁过期，可以抢
			} else if pid, err := strconv.Atoi(string(data)); err == nil {
				// PID 仍存活则不抢（简化：假设存活）
				_ = pid
				return false
			}
		}
	}
	// 写入当前 PID
	pid := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(lockPath, []byte(pid), 0644); err != nil {
		return false
	}
	return true
}

// touchLock 更新锁文件 mtime（游标前进）。
func (a *AutoDream) touchLock(lockPath string) {
	now := time.Now()
	if err := os.Chtimes(lockPath, now, now); err != nil {
		// 文件不存在则创建
		_ = os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644)
	}
}

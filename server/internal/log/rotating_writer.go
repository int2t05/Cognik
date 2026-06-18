// Package log 提供后端日志基础设施。
//
// 使用方式：
//
//	cleanup, _ := log.Init("../logs")
//	defer cleanup()
//	// 之后所有模块直接用 slog.Info/Warn/Error 即可
//
// Init 一次性完成全局 slog 配置：
//   - JSON 格式输出到 stdout + 日志文件
//   - 日志文件命名 log-YYYY-MM-DD-HH-MM-SS.log
//   - 单文件超过 10MB 自动切换到新文件
//
// 保留策略：最多保留 7 个日志文件，超过则在创建新文件时删除最旧的文件。
//
// 为什么不是独立日志框架：
// slog 是 Go 1.21+ 标准库，零依赖。本包只负责「日志写到哪里」和「文件怎么旋转」，
// 所有日志内容由调用方通过 slog 标准 API 产生。
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxSize  = 10 * 1024 * 1024 // 10MB
	defaultMaxFiles = 7                 // 最多保留 7 个日志文件
)

// Init 初始化日志系统。
//
// dir 为日志文件目录，不存在则自动创建。
// 返回 cleanup 函数，进程退出前调用以关闭当前日志文件。
//
// 初始化失败时返回 error，调用方可降级为仅 stdout 输出。
func Init(dir string) (cleanup func(), err error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录 %s 失败: %w", dir, err)
	}

	w := &rotatingWriter{dir: dir, maxSize: defaultMaxSize, maxFiles: defaultMaxFiles}
	if err := w.open(); err != nil {
		return nil, err
	}

	// JSON 格式输出到 stdout + 日志文件
	slog.SetDefault(slog.New(slog.NewJSONHandler(
		io.MultiWriter(os.Stdout, w),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	return func() { w.Close() }, nil
}

// ─── 旋转文件写入器（包内私有）─────────────────────────────────────────

// rotatingWriter 是线程安全的旋转文件 io.Writer。
//
// 写入前检查当前文件大小，超过 maxSize 则关闭旧文件、创建新文件。
// 新文件名格式：log-YYYY-MM-DD-HH-MM-SS.log，文件系统排序即时间顺序。
type rotatingWriter struct {
	dir      string
	maxSize  int64
	maxFiles int
	mu       sync.Mutex
	file     *os.File
	currSize int64
}

func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currSize+int64(len(p)) > w.maxSize {
		if err := w.open(); err != nil {
			return 0, fmt.Errorf("切换日志文件失败: %w", err)
		}
	}

	n, err = w.file.Write(p)
	w.currSize += int64(n)
	return n, err
}

// open 关闭当前文件（如有），创建新日志文件，并清理超出保留数的旧文件。
func (w *rotatingWriter) open() error {
	if w.file != nil {
		w.file.Close()
	}

	name := fmt.Sprintf("log-%s.log", time.Now().Format("2006-01-02-15-04-05"))
	f, err := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件 %s 失败: %w", name, err)
	}

	stat, _ := f.Stat()
	w.file = f
	w.currSize = stat.Size()

	// 清理：保留最近 maxFiles 个日志文件
	w.prune()
	return nil
}

// prune 删除超出 maxFiles 保留数的旧日志文件（按文件名时间排序，旧在前）。
func (w *rotatingWriter) prune() {
	entries, err := os.ReadDir(w.dir)
	if err != nil || len(entries) <= w.maxFiles {
		return
	}

	var logs []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "log-") && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, e)
		}
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].Name() < logs[j].Name() })

	for i := 0; i < len(logs)-w.maxFiles; i++ {
		os.Remove(filepath.Join(w.dir, logs[i].Name()))
	}
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

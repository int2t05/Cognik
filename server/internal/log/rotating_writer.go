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
//   - 日志文件按日期命名：log-YYYY-MM-DD.log
//   - 同一天内续写同一文件（进程重启也不会创建新文件）
//   - 单文件超过 10MB 自动切换到编号后缀：log-YYYY-MM-DD.2.log、log-YYYY-MM-DD.3.log …
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
// 写入前检查当前文件大小，超过 maxSize 则切换到新文件。
// 文件命名规则：
//   - 基础名 log-YYYY-MM-DD.log（当天第一个文件）
//   - 超 10MB 后 → log-YYYY-MM-DD.2.log、log-YYYY-MM-DD.3.log …
//   - 同一天内进程重启直接续写已有文件（O_APPEND）
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

// open 切换到下一个可用的日志文件。
//
// 命名策略：
//  1. 优先续写当天的 log-YYYY-MM-DD.log（若存在且未超 maxSize）
//  2. 若当天文件已超 maxSize，寻找下一个可用编号 log-YYYY-MM-DD.N.log
//  3. 同一天内进程多次重启，O_APPEND 自动追加到已有文件末尾
func (w *rotatingWriter) open() error {
	if w.file != nil {
		w.file.Close()
	}

	today := time.Now().Format("2006-01-02")
	base := fmt.Sprintf("log-%s", today)

	// 计算当天已有多少个文件（用于 size 超限后的编号续写）
	name := base + ".log"
	if stat, err := os.Stat(filepath.Join(w.dir, name)); err == nil && stat.Size() >= w.maxSize {
		// 当天主文件已满，寻找下一个可用编号
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s.%d.log", base, i)
			if _, err := os.Stat(filepath.Join(w.dir, candidate)); os.IsNotExist(err) {
				name = candidate
				break
			}
			// 如果编号 N 文件也存在且未满，续写它（进程重启场景）
			if st, err := os.Stat(filepath.Join(w.dir, candidate)); err == nil && st.Size() < w.maxSize {
				name = candidate
				break
			}
		}
	}

	f, err := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件 %s 失败: %w", name, err)
	}

	stat, _ := f.Stat()
	w.file = f
	w.currSize = stat.Size()

	// 清理：保留最近 maxFiles 个日志文件（按文件名排序，日期优先）
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

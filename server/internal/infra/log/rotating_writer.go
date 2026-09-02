// Package log 提供基于 slog 的日志基础设施（JSON 输出 stdout + 按日期/大小旋转的文件）。
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
	defaultMaxFiles = 7                // 最多保留 7 个日志文件
)

// Init 初始化日志系统，dir 为日志目录，返回 cleanup 关闭函数。
func Init(dir string) (cleanup func(), err error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录 %s 失败: %w", dir, err)
	}

	w := &rotatingWriter{dir: dir, maxSize: defaultMaxSize, maxFiles: defaultMaxFiles}
	if err := w.open(); err != nil {
		return nil, err
	}

	// JSON 输出到 stdout + 文件
	slog.SetDefault(slog.New(slog.NewJSONHandler(
		io.MultiWriter(os.Stdout, w),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	return func() { w.Close() }, nil
}

// ─── 旋转文件写入器（包内私有）─────────────────────────────────────────

// rotatingWriter 线程安全的旋转文件 io.Writer，超 maxSize 自动切换编号文件。
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

// open 切换到下一个可用日志文件（续写当天未满文件或创建编号新文件）。
func (w *rotatingWriter) open() error {
	if w.file != nil {
		w.file.Close()
	}

	today := time.Now().Format("2006-01-02")
	base := fmt.Sprintf("log-%s", today)

	// 主文件已满时寻找下一个可用编号
	name := base + ".log"
	if stat, err := os.Stat(filepath.Join(w.dir, name)); err == nil && stat.Size() >= w.maxSize {
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s.%d.log", base, i)
			if _, err := os.Stat(filepath.Join(w.dir, candidate)); os.IsNotExist(err) {
				name = candidate
				break
			}
			// 编号 N 文件未满则续写（进程重启场景）
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

	// 保留最近 maxFiles 个文件
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

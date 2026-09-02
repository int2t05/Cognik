// Package runtime 提供运行时基础设施。
// GenerationHub 管理进行中的流式生成：与 HTTP ctx 解耦（断开不影响落库），重连凭 since 回放续传；泛型 T + setSeq 回调避免反向依赖 service 层。
package runtime

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrGenerationInProgress 表示同一会话已有进行中的生成（一问一答语义）。
var ErrGenerationInProgress = errors.New("该会话已有进行中的生成")

// generationGracePeriod 生成完成后缓冲保留时长，覆盖完成瞬间重连的客户端。
const generationGracePeriod = 30 * time.Second

// subChanBuffer 订阅通道缓冲；慢订阅者写满即丢弃（可凭 since 重连补回），保证生成不被阻塞。
const subChanBuffer = 256

type generation[T any] struct {
	mu        sync.Mutex
	buffer    []T
	finished  bool
	subs      map[int]chan T
	nextSubID int
	cancel    context.CancelFunc
}

// GenerationHub 按 sessionID 管理所有进行中的生成。T 为流式事件类型。
type GenerationHub[T any] struct {
	mu     sync.RWMutex
	gen    map[int64]*generation[T]
	setSeq func(evt T, seq int) T
}

// NewGenerationHub 创建实例。setSeq 在 Publish 内对齐事件 Seq 与缓冲下标，由调用方注入。
func NewGenerationHub[T any](setSeq func(evt T, seq int) T) *GenerationHub[T] {
	return &GenerationHub[T]{gen: make(map[int64]*generation[T]), setSeq: setSeq}
}

// Start 登记一个新生成；若该会话已有未完成的生成则拒绝。
func (h *GenerationHub[T]) Start(sessionID int64, cancel context.CancelFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if g, ok := h.gen[sessionID]; ok && !g.finished {
		return ErrGenerationInProgress
	}
	h.gen[sessionID] = &generation[T]{
		buffer: make([]T, 0, 64),
		subs:   make(map[int]chan T),
		cancel: cancel,
	}
	return nil
}

// Publish 追加事件到缓冲并扇出给订阅者（非阻塞）。Seq 由缓冲下标决定，保证与回放位置对齐。
func (h *GenerationHub[T]) Publish(sessionID int64, evt T) {
	g := h.get(sessionID)
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	evt = h.setSeq(evt, len(g.buffer))
	g.buffer = append(g.buffer, evt)
	for id, ch := range g.subs {
		select {
		case ch <- evt:
		default:
			// 订阅者太慢：丢弃它，关闭并移除；它可凭 since 重连补回。
			close(ch)
			delete(g.subs, id)
		}
	}
}

// Subscribe 回放 buffer[since:] 并注册实时通道（同一锁内完成，避免回放与注册间漏事件）。
// ok=false 表示会话无活跃生成。
func (h *GenerationHub[T]) Subscribe(sessionID int64, since int) (replay []T, ch <-chan T, unsub func(), ok bool) {
	g := h.get(sessionID)
	if g == nil {
		return nil, nil, nil, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if since < 0 {
		since = 0
	}
	if since > len(g.buffer) {
		since = len(g.buffer)
	}
	replay = append([]T(nil), g.buffer[since:]...)
	// 已结束的生成：只回放，不注册通道，返回一个已关闭的空通道。
	if g.finished {
		closed := make(chan T)
		close(closed)
		return replay, closed, func() {}, true
	}
	out := make(chan T, subChanBuffer)
	id := g.nextSubID
	g.nextSubID++
	g.subs[id] = out
	unsub = func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		if c, exists := g.subs[id]; exists {
			close(c)
			delete(g.subs, id)
		}
	}
	return replay, out, unsub, true
}

// Finish 标记生成结束并关闭订阅通道；宽限期后才删除缓冲，覆盖完成瞬间断线的重连。
func (h *GenerationHub[T]) Finish(sessionID int64) {
	g := h.get(sessionID)
	if g == nil {
		return
	}
	g.mu.Lock()
	g.finished = true
	for id, ch := range g.subs {
		close(ch)
		delete(g.subs, id)
	}
	g.mu.Unlock()

	time.AfterFunc(generationGracePeriod, func() {
		h.mu.Lock()
		if cur, ok := h.gen[sessionID]; ok && cur == g {
			delete(h.gen, sessionID)
		}
		h.mu.Unlock()
	})
}

// Cancel 触发生成的 cancel()；不调 Finish——生成 goroutine 感知 ctx 取消后自行 Finish 并写 error 事件。
func (h *GenerationHub[T]) Cancel(sessionID int64) bool {
	g := h.get(sessionID)
	if g == nil {
		return false
	}
	g.mu.Lock()
	finished := g.finished
	cancel := g.cancel
	g.mu.Unlock()
	if finished || cancel == nil {
		return false
	}
	cancel()
	return true
}

// Active 报告该会话是否有未结束的生成（前端进入会话时判断是否续传）。
func (h *GenerationHub[T]) Active(sessionID int64) bool {
	g := h.get(sessionID)
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.finished
}

// get 线程安全地获取 generation（只读锁，允许并发订阅）。
func (h *GenerationHub[T]) get(sessionID int64) *generation[T] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.gen[sessionID]
}

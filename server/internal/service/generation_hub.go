// Package service —— GenerationHub 接管「进行中」的流式生成所有权。
//
// 为什么需要它：原实现把生成绑在 HTTP 请求 ctx 上，客户端一断开（导航/刷新）
// 生成即停止且不落库。Hub 把生成与请求解耦——请求只是订阅者，断开不影响生成；
// 重连可凭 since 回放缓冲实现真实断点续传。单实例内存实现，不依赖外部中间件。
package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrGenerationInProgress 表示同一会话已有进行中的生成（一问一答语义）。
var ErrGenerationInProgress = errors.New("该会话已有进行中的生成")

// generationGracePeriod —— 生成完成后缓冲保留时长，覆盖「刚完成那一刻刷新」的客户端。
const generationGracePeriod = 30 * time.Second

// subChanBuffer —— 每个订阅通道的缓冲，慢订阅者写满即被丢弃（可凭 since 重连补回），
// 保证生成本身永不被订阅者阻塞。
const subChanBuffer = 256

type generation struct {
	mu        sync.Mutex
	buffer    []StreamEvent
	finished  bool
	subs      map[int]chan StreamEvent
	nextSubID int
	cancel    context.CancelFunc
}

// GenerationHub 按 sessionID 管理所有进行中的生成。
type GenerationHub struct {
	mu  sync.RWMutex
	gen map[int64]*generation
}

// NewGenerationHub 创建 GenerationHub 实例。
// 为什么不用 sync.Map：需要原子性「读 + 写」操作（检查是否存在再插入），
// RWMutex + map 比 sync.Map 在这种模式下更清晰、更易推理。
func NewGenerationHub() *GenerationHub {
	return &GenerationHub{gen: make(map[int64]*generation)}
}

// Start 登记一个新生成；若该会话已有未完成的生成则拒绝。
// msgID 预留给后续任务（持久化时关联消息行），本任务不使用但保留接口。
func (h *GenerationHub) Start(sessionID, msgID int64, cancel context.CancelFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if g, ok := h.gen[sessionID]; ok && !g.finished {
		return ErrGenerationInProgress
	}
	h.gen[sessionID] = &generation{
		buffer: make([]StreamEvent, 0, 64),
		subs:   make(map[int]chan StreamEvent),
		cancel: cancel,
	}
	return nil
}

// Publish 追加事件到缓冲并扇出给所有订阅者（非阻塞）。Seq 由缓冲下标决定。
// 为什么用 buffer 下标而非外部计数器：Seq 必须和回放位置完全对齐，用 len(buffer)
// 赋值再 append 可保证两者严格同步，不存在竞态下的偏移。
func (h *GenerationHub) Publish(sessionID int64, evt StreamEvent) {
	g := h.get(sessionID)
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	evt.Seq = len(g.buffer)
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

// Subscribe 先在同一把锁内回放 buffer[since:]，再注册新通道接后续实时事件。
// ok=false 表示该会话无活跃（或已过宽限期被清理的）生成。
//
// 为什么回放和注册必须在同一把锁内完成：
// 若先读完 buffer 再释放锁再注册，中间窗口内 Publish 的事件会同时写入 buffer
// 和旧订阅者列表，新订阅者既回放不到（snapshot 已旧）又接不到实时推送，造成漏事件。
func (h *GenerationHub) Subscribe(sessionID int64, since int) (replay []StreamEvent, ch <-chan StreamEvent, unsub func(), ok bool) {
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
	replay = append([]StreamEvent(nil), g.buffer[since:]...)
	// 已结束的生成：只回放，不注册通道，返回一个已关闭的空通道。
	if g.finished {
		closed := make(chan StreamEvent)
		close(closed)
		return replay, closed, func() {}, true
	}
	out := make(chan StreamEvent, subChanBuffer)
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

// Finish 标记生成结束，关闭所有订阅通道；宽限期后从 map 删除缓冲。
// 为什么用 time.AfterFunc 而非立即删除：
// 客户端刚好在生成完成那一刻断线时，宽限期内重连仍可回放完整内容；
// 立即删除会导致这类客户端拿不到结尾的 done 事件。
func (h *GenerationHub) Finish(sessionID int64) {
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

// Cancel 调用生成的 cancel()，使其 goroutine 经 gctx.Done() 退出。
// 为什么不在 Cancel 中调用 Finish：生成 goroutine 感知到 ctx 取消后会自行调用
// Finish 并写入 error 事件；Cancel 只负责触发信号，保持单一职责。
func (h *GenerationHub) Cancel(sessionID int64) bool {
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
func (h *GenerationHub) Active(sessionID int64) bool {
	g := h.get(sessionID)
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.finished
}

// get 线程安全地获取 generation（只读锁，允许并发订阅）。
func (h *GenerationHub) get(sessionID int64) *generation {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.gen[sessionID]
}

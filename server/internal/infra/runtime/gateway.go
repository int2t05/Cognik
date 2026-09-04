// Package runtime 提供运行时基础设施。
//
// Gateway 是订阅渠道制网关：Agent 运行（生产者）与 HTTP 客户端（订阅者）解耦。
// 对齐 LangGraph Server（channel+worker 分离 / join_stream(last_event_id) / on_disconnect=continue）、
// Mastra Durable（PubSub topic + cache 重放 / observe(runId,offset)）、
// OpenAI Responses background（sequence_number + starting_after 游标）。
//
// 生产者 Publish 事件到 runID 渠道；订阅者凭 since 游标 Subscribe，先回放缓冲再接实时流，
// 断线重连不丢事件。调用方传入 detached ctx 的 cancel，网关在 Finish 宽限期内保留缓冲供重连。
package runtime

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrRunInProgress 表示同一 run 已有进行中的生成（一问一答语义）。
var ErrRunInProgress = errors.New("该运行已有进行中的生成")

const (
	// runGracePeriod 生成完成后缓冲保留时长，覆盖完成瞬间断线的客户端重连。
	runGracePeriod = 30 * time.Second
	// subChanBuffer 订阅通道缓冲；慢订阅者写满即丢弃（可凭 since 重连补回），保证生成不被阻塞。
	subChanBuffer = 256
	// eventStoreCap 单 run 事件缓冲上限（环形覆盖最旧事件，防止长生成内存膨胀）。
	eventStoreCap = 1024
)

// eventStore 有界环形缓冲 + seq 游标重放（对齐 OpenAI sequence_number / LangGraph Last-Event-ID）。
// count 限防内存膨胀；调用方凭 since=seq 重连回放 buffer[since:]。
type eventStore[E any] struct {
	buffer []E // 预分配定长环形（len==cap==eventStoreCap），用 head+len 跟踪有效区间
	head   int // 下一个写入位置
	len    int // 已写入事件数（持续增长；buffer 只保留最近 cap 个）
}

// append 追加事件并返回其 seq（缓冲下标对齐，保证回放位置）。
// buffer 预分配定长，写入覆盖最旧事件（环形）；seq 按 len 单调增长。
func (s *eventStore[E]) append(evt E) int {
	seq := s.len
	s.buffer[s.head] = evt // 覆盖最旧（环形）
	s.head = (s.head + 1) % cap(s.buffer)
	s.len++
	return seq
}

// replay 返回 seq >= since 的事件（从缓冲中按顺序回放）。
// since 语义：buffer[since:] 包含 since 本身；since=0 回放全部。
func (s *eventStore[E]) replay(since int) []E {
	if since < 0 {
		since = 0
	}
	if since >= s.len {
		return nil
	}
	// 缓冲可能已覆盖最旧事件：oldest 为现存最旧事件的 seq
	oldest := s.len - cap(s.buffer)
	if oldest < 0 {
		oldest = 0
	}
	start := since
	if start < oldest {
		start = oldest // 游标过期：从现存最旧开始（调用方可选发 resync）
	}
	if start >= s.len {
		return nil
	}
	count := s.len - start
	out := make([]E, 0, count)
	for i := start; i < s.len; i++ {
		// 第 i 个事件在环形 buffer 中的下标（head 指向最新+1）
		idx := (s.head - s.len + i + cap(s.buffer)) % cap(s.buffer)
		out = append(out, s.buffer[idx])
	}
	return out
}

// run 单次 Agent 运行的事件流。Broadcaster（实时 fan-out）与 EventStore（重放缓冲）职责分离。
type run[E any] struct {
	mu        sync.Mutex
	store     eventStore[E]   // 有界环形缓冲（重放）
	subs      map[int]chan E  // 订阅者通道（实时 fan-out）
	nextSubID int
	cancel    context.CancelFunc // 取消 detached 生成
	finished  bool
}

// Gateway 订阅渠道制网关。E 为事件类型（泛型，不绑业务）。
// runID 为 string（通用，chat 用 sessionID 字符串；未来 MCP/深度研究流用各自 ID）。
type Gateway[E any] struct {
	mu     sync.RWMutex
	runs   map[string]*run[E]
	setSeq func(evt E, seq int) E // 注入 seq（缓冲下标对齐，保证回放位置）
}

// NewGateway 创建网关。setSeq 在 Publish 内对齐事件 Seq 与缓冲下标，由调用方注入。
func NewGateway[E any](setSeq func(evt E, seq int) E) *Gateway[E] {
	return &Gateway[E]{runs: make(map[string]*run[E]), setSeq: setSeq}
}

// Start 登记一个 run（生产者即将开始）；若该 run 已有未完成的生成则拒绝。
func (g *Gateway[E]) Start(runID string, cancel context.CancelFunc) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r, ok := g.runs[runID]; ok && !r.finished {
		return ErrRunInProgress
	}
	g.runs[runID] = &run[E]{
		store: eventStore[E]{buffer: make([]E, eventStoreCap)}, // 定长环形缓冲（len==cap，索引覆盖写入）
		subs:  make(map[int]chan E),
		cancel: cancel,
	}
	return nil
}

// Publish 生产者发布事件：追加缓冲（setSeq 对齐下标）+ 非阻塞 fan-out（慢订阅者满则丢弃，可重连补回）。
func (g *Gateway[E]) Publish(runID string, evt E) {
	r := g.get(runID)
	if r == nil {
		return
	}
	r.mu.Lock()
	// setSeq 对齐：seq = 当前缓冲长度（下标），保证 Subscribe(since) 回放位置正确
	evt = g.setSeq(evt, r.store.len)
	r.store.append(evt)
	for id, ch := range r.subs {
		select {
		case ch <- evt:
		default:
			// 订阅者太慢：丢弃它，关闭并移除；它可凭 since 重连补回。
			close(ch)
			delete(r.subs, id)
		}
	}
	r.mu.Unlock()
}

// Subscribe 订阅者凭 since 游标 join：原子回放 buffer[since:] + 注册实时通道（同一锁内，避免回放与注册间漏事件）。
// ok=false 表示 run 不存在。
func (g *Gateway[E]) Subscribe(runID string, since int) (replay []E, ch <-chan E, unsub func(), ok bool) {
	r := g.get(runID)
	if r == nil {
		return nil, nil, nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	replay = r.store.replay(since)
	// 已结束的 run：只回放，不注册通道，返回已关闭的空通道。
	if r.finished {
		closed := make(chan E)
		close(closed)
		return replay, closed, func() {}, true
	}
	out := make(chan E, subChanBuffer)
	id := r.nextSubID
	r.nextSubID++
	r.subs[id] = out
	unsub = func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if c, exists := r.subs[id]; exists {
			close(c)
			delete(r.subs, id)
		}
	}
	return replay, out, unsub, true
}

// Finish 标记 run 结束并关闭订阅通道；宽限期后才删除缓冲，覆盖完成瞬间断线的重连。
func (g *Gateway[E]) Finish(runID string) {
	r := g.get(runID)
	if r == nil {
		return
	}
	r.mu.Lock()
	r.finished = true
	for id, ch := range r.subs {
		close(ch)
		delete(r.subs, id)
	}
	r.mu.Unlock()

	time.AfterFunc(runGracePeriod, func() {
		g.mu.Lock()
		if cur, ok := g.runs[runID]; ok && cur == r {
			delete(g.runs, runID)
		}
		g.mu.Unlock()
	})
}

// Cancel 触发 run 的 cancel()；不调 Finish——生成 goroutine 感知 ctx 取消后自行 Finish 并写 error 事件。
func (g *Gateway[E]) Cancel(runID string) bool {
	r := g.get(runID)
	if r == nil {
		return false
	}
	r.mu.Lock()
	finished := r.finished
	cancel := r.cancel
	r.mu.Unlock()
	if finished || cancel == nil {
		return false
	}
	cancel()
	return true
}

// Active 报告该 run 是否有未结束的生成（前端进入会话时判断是否续传）。
func (g *Gateway[E]) Active(runID string) bool {
	r := g.get(runID)
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.finished
}

// get 线程安全地获取 run（只读锁，允许并发订阅）。
func (g *Gateway[E]) get(runID string) *run[E] {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.runs[runID]
}

# 可续传流式对话 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让门户端智能问答的流式生成脱离 HTTP 请求生命周期——导航离开、刷新、重开标签都不丢失用户消息和 AI 回复，并能真实断点续传，支持多会话并行。

**Architecture:** 后端新增内存 `GenerationHub` 接管「进行中」的生成所有权；`ChatService.StreamChat` 立即落库用户消息、在 `context.Background()` 跑生成、完成时落库 assistant 消息。HTTP 请求（POST 发问 / GET 续传）降级为 Hub 的订阅者，断开不影响生成。前端把流式状态从页面内 hook 提升到 portal 布局层的全局 `ChatStreamProvider`，跨路由保活，并在进入会话时按 `status=generating` 自动续传。

**Tech Stack:** Go / Gin / GORM；Next.js(App Router) / React / TypeScript / SWR；SSE。

## Global Constraints

- 三层架构：Handler → Service → Repository，禁止跨层；Handler 只调 Service 方法，`GenerationHub` 归 Service 层持有。
- 统一响应格式 `pkg/response`；错误码 `pkg/errcode`（AI 失败 20001 / RAG 失败 20002）。
- 测试在 `server/tests/` 外部 `_test` 包，**禁止 mock**，依赖真实模块；集成测试 `-tags=integration`，跨包串行 `-p 1`。
- 中文注释（文件头说明为什么存在，关键函数说明为什么这样实现）。
- 中文 commit message，格式 `类型: 描述`；提交署名 scutmmq，**不带任何 AI 署名**；**不自动 push**。
- 单实例假设：Hub 用内存，不引入 Redis。
- 生成超时 120s；生成完成后 Hub 缓冲宽限期 30s。

---

## File Structure

**后端：**
- `server/internal/service/generation_hub.go` — **新**：`GenerationHub` 与 `generation`，纯内存、并发安全、与 DB/HTTP 解耦。
- `server/internal/service/llm_service.go` — **改**：`StreamEvent` 增加 `Seq int` 字段。
- `server/internal/model/chat.go` — **改**：`ChatMessage` 增加 `Status` 字段。
- `server/internal/repository/chat.go` — **改**：新增 `CreateMessage`/`UpdateMessage`/`MarkGeneratingFailed`。
- `server/internal/service/chat_service.go` — **改**：注入 hub；重写 `StreamChat`；新增 `ResumeStream`/`CancelGeneration`/`CleanupStaleGenerating`；扩接口。
- `server/internal/handler/chat.go` — **改**：流写入抽公共函数；`StreamChatMessage` 改为订阅；新增 `ResumeStream`/`CancelGeneration`。
- `server/internal/dto/response/chat.go` — **改**：消息响应增加 `status`。
- `server/internal/router/portal.go` — **改**：注册 `GET .../stream`、`POST .../cancel`。
- `server/cmd/main.go` — **改**：构造 hub 注入 ChatService；启动时调用 `CleanupStaleGenerating`。

**前端：**
- `web/src/contexts/ChatStreamProvider.tsx` — **新**：全局 store + Provider + `useChatStreamStore`。
- `web/src/app/portal/layout.tsx` — **改**：用 Provider 包裹。
- `web/src/lib/api/chat.ts` — **改**：新增 `cancelGeneration`；导出续传/流 URL 辅助。
- `web/src/app/portal/chat/page.tsx` — **改**：从 store 读流；进入会话自动续传；停止按钮调 cancel。
- `web/src/hooks/useChatStream.ts` — **删/废弃**：逻辑迁入 Provider（保留薄封装或移除引用）。

**测试：**
- `server/tests/service/generation_hub_test.go` — **新**：Hub 单元/并发（无需 DB）。
- `server/tests/service/chat_stream_test.go` — **新**：集成（立即落库 user、断开仍完成、启动清理）。
- `web/e2e/chat-resume.spec.ts` — **新**：Playwright E2E。

---

## Task 1: GenerationHub（纯内存，先回放后注册、非阻塞扇出）

**Files:**
- Create: `server/internal/service/generation_hub.go`
- Test: `server/tests/service/generation_hub_test.go`

**Interfaces:**
- Consumes: `service.StreamEvent`（已存在于 `llm_service.go`）。
- Produces:
  - `func NewGenerationHub() *GenerationHub`
  - `func (h *GenerationHub) Start(sessionID, msgID int64, cancel context.CancelFunc) error` — 已存在进行中则返回 `ErrGenerationInProgress`
  - `func (h *GenerationHub) Publish(sessionID int64, evt StreamEvent)` — 内部给 evt 赋递增 Seq
  - `func (h *GenerationHub) Subscribe(sessionID int64, since int) (replay []StreamEvent, ch <-chan StreamEvent, unsub func(), ok bool)`
  - `func (h *GenerationHub) Finish(sessionID int64)`
  - `func (h *GenerationHub) Cancel(sessionID int64) bool`
  - `func (h *GenerationHub) Active(sessionID int64) bool`
  - `var ErrGenerationInProgress = errors.New("该会话已有进行中的生成")`

- [ ] **Step 1: 写失败测试 — 先回放后注册不漏事件 + 多订阅者**

创建 `server/tests/service/generation_hub_test.go`：

```go
// Package service_test 验证 GenerationHub 的并发与续传语义。
// 为什么不 mock：Hub 是纯内存结构，直接用真实实例最接近运行时行为。
package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"opsmind/internal/service"
)

func drain(ch <-chan service.StreamEvent, n int, d time.Duration) []service.StreamEvent {
	out := []service.StreamEvent{}
	timeout := time.After(d)
	for len(out) < n {
		select {
		case e, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, e)
		case <-timeout:
			return out
		}
	}
	return out
}

// 先 Publish 若干，再 Subscribe(since=2)，必须回放 2..尾 且与实时无缺号。
func TestHub_ReplayThenLiveNoGap(t *testing.T) {
	h := service.NewGenerationHub()
	if err := h.Start(1, 100, func() {}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 3; i++ {
		h.Publish(1, service.StreamEvent{Type: "token", Content: "x"})
	}
	replay, ch, unsub, ok := h.Subscribe(1, 2)
	if !ok {
		t.Fatal("Subscribe 应成功")
	}
	defer unsub()
	if len(replay) != 1 || replay[0].Seq != 2 {
		t.Fatalf("回放应为 [seq=2]，得到 %+v", replay)
	}
	h.Publish(1, service.StreamEvent{Type: "token", Content: "y"})
	live := drain(ch, 1, time.Second)
	if len(live) != 1 || live[0].Seq != 3 {
		t.Fatalf("实时应为 seq=3，得到 %+v", live)
	}
}

// 多订阅者都收到全量实时事件。
func TestHub_MultipleSubscribers(t *testing.T) {
	h := service.NewGenerationHub()
	_ = h.Start(2, 200, func() {})
	_, chA, ua, _ := h.Subscribe(2, 0)
	_, chB, ub, _ := h.Subscribe(2, 0)
	defer ua()
	defer ub()
	h.Publish(2, service.StreamEvent{Type: "token", Content: "a"})
	if a := drain(chA, 1, time.Second); len(a) != 1 {
		t.Fatal("A 应收到事件")
	}
	if b := drain(chB, 1, time.Second); len(b) != 1 {
		t.Fatal("B 应收到事件")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./tests/service/ -run TestHub_ -v`
Expected: 编译失败（`NewGenerationHub` 等未定义）。

- [ ] **Step 3: 实现 GenerationHub**

创建 `server/internal/service/generation_hub.go`：

```go
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

// graceperiod —— 生成完成后缓冲保留时长，覆盖「刚完成那一刻刷新」的客户端。
const generationGracePeriod = 30 * time.Second

// subBuffer —— 每个订阅通道的缓冲，慢订阅者写满即被丢弃（可凭 since 重连补回），
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

func NewGenerationHub() *GenerationHub {
	return &GenerationHub{gen: make(map[int64]*generation)}
}

// Start 登记一个新生成；若该会话已有未完成的生成则拒绝。
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

func (h *GenerationHub) get(sessionID int64) *generation {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.gen[sessionID]
}
```

- [ ] **Step 4: 加并发/取消/Finish 测试**

在 `generation_hub_test.go` 追加：

```go
// 慢订阅者（不消费）不得阻塞生成：Publish 远超 buffer 容量仍快速返回。
func TestHub_SlowSubscriberNotBlocking(t *testing.T) {
	h := service.NewGenerationHub()
	_ = h.Start(3, 300, func() {})
	_, _, unsub, _ := h.Subscribe(3, 0) // 拿到通道但从不读
	defer unsub()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.Publish(3, service.StreamEvent{Type: "token", Content: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("慢订阅者阻塞了生成")
	}
}

// 同会话重复 Start 返回 ErrGenerationInProgress。
func TestHub_DuplicateStart(t *testing.T) {
	h := service.NewGenerationHub()
	_ = h.Start(4, 400, func() {})
	if err := h.Start(4, 401, func() {}); err != service.ErrGenerationInProgress {
		t.Fatalf("应返回 ErrGenerationInProgress，得到 %v", err)
	}
}

// Cancel 触发 cancel 回调；Finish 后 Active=false。
func TestHub_CancelAndFinish(t *testing.T) {
	h := service.NewGenerationHub()
	var mu sync.Mutex
	canceled := false
	_ = h.Start(5, 500, func() { mu.Lock(); canceled = true; mu.Unlock() })
	if !h.Cancel(5) {
		t.Fatal("Cancel 应返回 true")
	}
	mu.Lock()
	c := canceled
	mu.Unlock()
	if !c {
		t.Fatal("cancel 回调未被调用")
	}
	h.Finish(5)
	if h.Active(5) {
		t.Fatal("Finish 后 Active 应为 false")
	}
}

// 用 -race 跑：Subscribe/Publish/Unsubscribe 并发无数据竞争。
func TestHub_ConcurrentRace(t *testing.T) {
	h := service.NewGenerationHub()
	_ = h.Start(6, 600, func() {})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ch, unsub, ok := h.Subscribe(6, 0)
			if !ok {
				return
			}
			go func() { for range ch {} }()
			time.Sleep(10 * time.Millisecond)
			unsub()
		}()
	}
	for i := 0; i < 200; i++ {
		h.Publish(6, service.StreamEvent{Type: "token"})
	}
	wg.Wait()
	h.Finish(6)
}
```

- [ ] **Step 5: 运行全部 Hub 测试（含 -race）**

Run: `cd server && go test ./tests/service/ -run TestHub_ -race -v`
Expected: PASS（5 个测试全过，无 race 警告）。

- [ ] **Step 6: 提交**

```bash
git add server/internal/service/generation_hub.go server/tests/service/generation_hub_test.go
git commit -m "feat: 新增 GenerationHub 管理可续传流式生成"
```

---

## Task 2: StreamEvent.Seq + ChatMessage.Status + 响应 DTO

**Files:**
- Modify: `server/internal/service/llm_service.go`（`StreamEvent` 加 `Seq`）
- Modify: `server/internal/model/chat.go`（`ChatMessage` 加 `Status`）
- Modify: `server/internal/dto/response/chat.go`（消息响应加 `status`）

**Interfaces:**
- Produces: `StreamEvent.Seq int`；`ChatMessage.Status string`；消息状态常量 `MessageStatusGenerating/Completed/Failed`。

- [ ] **Step 1: 给 StreamEvent 加 Seq 字段**

`server/internal/service/llm_service.go`，在 `StreamEvent` 结构体内 `Type` 之后加：

```go
	Seq      int             `json:"seq"`                // 生成内单调递增序号，用于断点续传去重
```

- [ ] **Step 2: 给 ChatMessage 加 Status 字段 + 常量**

`server/internal/model/chat.go`，在 `ChatMessage` 的 `Confidence` 字段后加：

```go
	Status          string         `gorm:"type:varchar(16);not null;default:completed" json:"status"` // generating|completed|failed
```

并在文件内（types 区）加常量：

```go
// 消息生成状态：generating 表示后台仍在生成（刷新后据此续传），
// completed/failed 为终态。默认 completed 兼容历史数据。
const (
	MessageStatusGenerating = "generating"
	MessageStatusCompleted  = "completed"
	MessageStatusFailed     = "failed"
)
```

- [ ] **Step 3: 响应 DTO 暴露 status**

`server/internal/dto/response/chat.go`，在表示单条消息的结构体（如 `ChatMessageResponse`）内加字段：

```go
	Status string `json:"status"`
```

并在该结构体的组装处（同文件内由 `model.ChatMessage` 映射的地方）补 `Status: m.Status,`。
（若 `GetChatDetail` 在 `chat_service.go` 内手工组装消息，则在 Task 3 一并补；此步只确保 DTO 字段存在。）

- [ ] **Step 4: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译通过。

- [ ] **Step 5: 提交**

```bash
git add server/internal/service/llm_service.go server/internal/model/chat.go server/internal/dto/response/chat.go
git commit -m "feat: 消息增加生成状态字段与流事件序号"
```

---

## Task 3: Repository 新增消息单写/更新/清理

**Files:**
- Modify: `server/internal/repository/chat.go`
- Test: `server/tests/service/chat_stream_test.go`（本任务先写 repo 集成测试）

**Interfaces:**
- Produces（`*ChatRepo` 方法）：
  - `func (r *ChatRepo) CreateMessage(ctx context.Context, m *model.ChatMessage) error`（写入并回填 `m.ID`）
  - `func (r *ChatRepo) UpdateMessage(ctx context.Context, m *model.ChatMessage) error`（按 ID 全量更新）
  - `func (r *ChatRepo) MarkGeneratingFailed(ctx context.Context) (int64, error)`（启动清理，返回影响行数）

- [ ] **Step 1: 写失败的集成测试**

创建 `server/tests/service/chat_stream_test.go`：

```go
//go:build integration

// Package service_test 验证 ChatRepo 的消息写入/更新/清理。
package service_test

import (
	"context"
	"testing"

	"opsmind/internal/model"
	"opsmind/tests/testutil" // 复用现有测试基建：DB 连接 + 清理
)

func TestChatRepo_CreateUpdateAndMarkFailed(t *testing.T) {
	db := testutil.NewTestDB(t) // 既有约定：返回迁移好的 *gorm.DB；若实际工具名不同，按既有测试改
	repo := newChatRepoForTest(db) // 见 Step 3 的构造说明
	ctx := context.Background()

	// 准备一个会话（满足外键）
	sess := &model.ChatSession{KBID: 1, UserID: 1, Title: "t"}
	if err := db.Create(sess).Error; err != nil {
		t.Fatalf("建会话: %v", err)
	}

	m := &model.ChatMessage{SessionID: sess.ID, Role: "assistant", Content: "", Status: model.MessageStatusGenerating}
	if err := repo.CreateMessage(ctx, m); err != nil || m.ID == 0 {
		t.Fatalf("CreateMessage 应回填 ID: err=%v id=%d", err, m.ID)
	}

	m.Content = "完整答案"
	m.Status = model.MessageStatusCompleted
	if err := repo.UpdateMessage(ctx, m); err != nil {
		t.Fatalf("UpdateMessage: %v", err)
	}

	// 再造一条残留 generating，验证清理
	stale := &model.ChatMessage{SessionID: sess.ID, Role: "assistant", Content: "半截", Status: model.MessageStatusGenerating}
	_ = repo.CreateMessage(ctx, stale)
	n, err := repo.MarkGeneratingFailed(ctx)
	if err != nil || n < 1 {
		t.Fatalf("MarkGeneratingFailed 应清理至少 1 行: n=%d err=%v", n, err)
	}
}
```

> 注：`testutil.NewTestDB` 与 `newChatRepoForTest` 按本仓库 `tests/` 既有写法对齐（参考其他 `tests/service` 或 `tests/database` 用例的 DB 初始化与 repo 构造方式）。若既有用例直接 `repository.NewChatRepo(db)`，则用之。

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./tests/service/ -run TestChatRepo_CreateUpdateAndMarkFailed -tags=integration -v`
Expected: 编译失败（方法未定义）。

- [ ] **Step 3: 实现三个 repo 方法**

`server/internal/repository/chat.go` 追加（沿用文件内既有 `r.db.WithContext(ctx)` 风格）：

```go
// CreateMessage 单条写入消息并回填自增 ID。
// 为什么单写：可续传方案要在生成开始时先建一条 generating 的 assistant 消息，
// 拿到 ID 后于完成时再 UpdateMessage 回填内容，区别于一次性 CreateBatch。
func (r *ChatRepo) CreateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// UpdateMessage 按主键全量更新一条消息（含 Status/Content/Sources 等）。
func (r *ChatRepo) UpdateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{ID: m.ID}).
		Select("content", "sources", "pipeline_metrics", "confidence", "status").
		Updates(m).Error
}

// MarkGeneratingFailed 将所有残留 generating 消息标记为 failed。
// 为什么需要：内存 Hub 在服务重启后丢失进行中的生成，避免前端永久卡「生成中」。
func (r *ChatRepo) MarkGeneratingFailed(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("status = ?", model.MessageStatusGenerating).
		Update("status", model.MessageStatusFailed)
	return res.RowsAffected, res.Error
}
```

- [ ] **Step 4: 运行测试通过**

Run: `cd server && go test ./tests/service/ -run TestChatRepo_CreateUpdateAndMarkFailed -tags=integration -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add server/internal/repository/chat.go server/tests/service/chat_stream_test.go
git commit -m "feat: ChatRepo 支持消息单写/更新/残留清理"
```

---

## Task 4: ChatService 改造（注入 hub、重写 StreamChat、续传/取消/清理）

**Files:**
- Modify: `server/internal/service/chat_service.go`

**Interfaces:**
- Consumes: `GenerationHub`（Task 1）；`ChatRepo.CreateMessage/UpdateMessage/MarkGeneratingFailed`（Task 3）；`StreamEvent.Seq`、`model.MessageStatus*`（Task 2）。
- Produces:
  - `ChatService` 增字段 `hub *GenerationHub`；构造函数加参数。
  - `func (s *ChatService) StreamChat(ctx, sessionID, question, userID, routeCount, rerankCount) (replay []StreamEvent, ch <-chan StreamEvent, unsub func(), err error)` —— **签名变更**。
  - `func (s *ChatService) ResumeStream(ctx, sessionID, userID int64, since int) (replay []StreamEvent, ch <-chan StreamEvent, unsub func(), err error)`
  - `func (s *ChatService) CancelGeneration(ctx, sessionID, userID int64) error`
  - `func (s *ChatService) CleanupStaleGenerating(ctx) error`
  - 消费接口新增方法：`CreateMessage`、`UpdateMessage`、`MarkGeneratingFailed`。

- [ ] **Step 1: 扩展 Service 依赖接口 + 构造函数注入 hub**

在 `chat_service.go` 顶部的 chatRepo 消费接口里追加方法：

```go
	CreateMessage(ctx context.Context, m *model.ChatMessage) error
	UpdateMessage(ctx context.Context, m *model.ChatMessage) error
	MarkGeneratingFailed(ctx context.Context) (int64, error)
```

`ChatService` 结构体加字段 `hub *GenerationHub`；构造函数（如 `NewChatService(...)`）增加 `hub *GenerationHub` 参数并赋值。

- [ ] **Step 2: 重写 StreamChat —— 立即落库 user、建 generating assistant、后台生成、返回订阅**

将现有 `StreamChat` 主体替换为（保留前段校验与历史加载、`buildRAGOptions`）：

```go
// StreamChat 发起一次新生成：立即落库用户消息、建 generating 的 assistant 消息，
// 在 context.Background() 跑生成（脱离请求 ctx，客户端断开不影响），
// 返回 Hub 订阅（replay+实时）。完成时由后台 goroutine 落库 assistant 终稿。
func (s *ChatService) StreamChat(ctx context.Context, sessionID int64, question string, userID int64, routeCount, rerankCount int) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	// —— 前段不变：校验 question/llmService/chatRepo、FindByID、归属校验、加载 history/ragHistory、buildRAGOptions ——
	// （沿用现有代码，直到拿到 session、history、opts）

	// 立即落库用户消息（解决导航/刷新后用户消息丢失）
	if err := s.chatRepo.CreateMessage(ctx, &model.ChatMessage{
		SessionID: sessionID, Role: "user", Content: question, Status: model.MessageStatusCompleted,
	}); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "保存用户消息失败"}
	}

	// 建 generating 的 assistant 占位消息，拿到 msgID
	assistant := &model.ChatMessage{SessionID: sessionID, Role: "assistant", Content: "", Status: model.MessageStatusGenerating}
	if err := s.chatRepo.CreateMessage(ctx, assistant); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建回复占位失败"}
	}

	// 脱离请求 ctx：用 background + 独立超时
	gctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	if err := s.hub.Start(sessionID, assistant.ID, cancel); err != nil {
		cancel()
		// 标记占位失败，避免残留 generating
		assistant.Status = model.MessageStatusFailed
		_ = s.chatRepo.UpdateMessage(context.Background(), assistant)
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	go s.runGeneration(gctx, sessionID, assistant.ID, question, session.KBID, opts, history)

	replay, ch, unsub, ok := s.hub.Subscribe(sessionID, 0)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "订阅生成失败"}
	}
	return replay, ch, unsub, nil
}

// runGeneration 在后台跑 RAG 管道，逐事件 Publish 到 Hub；完成/失败时落库并 Finish。
func (s *ChatService) runGeneration(gctx context.Context, sessionID, msgID int64, question string, kbID int64, opts any, history []adapter.ChatMessage) {
	defer s.hub.Finish(sessionID)

	llmEvents, err := s.llmService.StreamChat(gctx, question, kbID, opts, history)
	if err != nil {
		s.hub.Publish(sessionID, StreamEvent{Type: "error", Error: err.Error()})
		s.failAssistant(msgID)
		return
	}

	var answer string
	for evt := range llmEvents {
		if evt.Type == "token" {
			answer += evt.Content
		}
		if evt.Type == "done" && evt.Metadata != nil {
			srcJSON, _ := json.Marshal(evt.Metadata.Sources)
			pipelineJSON, _ := json.Marshal(evt.Metadata.Pipeline)
			_ = s.chatRepo.UpdateSession(context.Background(), &model.ChatSession{
				ID: sessionID, Answer: evt.Metadata.Answer, Sources: srcJSON,
				Confidence: evt.Metadata.Confidence, DurationMs: evt.Metadata.DurationMS,
			})
			_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
				ID: msgID, Content: evt.Metadata.Answer, Sources: srcJSON,
				PipelineMetrics: pipelineJSON, Confidence: evt.Metadata.Confidence,
				Status: model.MessageStatusCompleted,
			})
			evt.Metadata.SessionID = sessionID
			evt.Metadata.Question = question
			evt.Metadata.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		if evt.Type == "error" {
			s.failAssistant(msgID)
		}
		s.hub.Publish(sessionID, evt)
	}

	// gctx 被取消（用户停止/超时）：落库当前已生成内容为 failed
	if gctx.Err() != nil {
		_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
			ID: msgID, Content: answer, Status: model.MessageStatusFailed,
		})
		s.hub.Publish(sessionID, StreamEvent{Type: "error", Error: "生成已停止"})
	}
}

func (s *ChatService) failAssistant(msgID int64) {
	_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{ID: msgID, Status: model.MessageStatusFailed})
}
```

> `opts any` 仅为示意：实际类型用 `buildRAGOptions` 的真实返回类型替换（在本文件可见，直接照搬）。

- [ ] **Step 3: 实现 ResumeStream / CancelGeneration / CleanupStaleGenerating**

```go
// ResumeStream 续传：校验会话归属后从 since 订阅 Hub。无活跃生成则返回 ErrNotFound。
func (s *ChatService) ResumeStream(ctx context.Context, sessionID, userID int64, since int) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrForbidden, Message: "无权访问该会话"}
	}
	replay, ch, unsub, ok := s.hub.Subscribe(sessionID, since)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "无进行中的生成"}
	}
	return replay, ch, unsub, nil
}

// CancelGeneration 校验归属后真正取消后端生成。
func (s *ChatService) CancelGeneration(ctx context.Context, sessionID, userID int64) error {
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权访问该会话"}
	}
	if !s.hub.Cancel(sessionID) {
		return errcode.AppError{Code: errcode.ErrParam, Message: "无进行中的生成"}
	}
	return nil
}

// CleanupStaleGenerating 启动时把残留 generating 标记 failed。
func (s *ChatService) CleanupStaleGenerating(ctx context.Context) error {
	_, err := s.chatRepo.MarkGeneratingFailed(ctx)
	return err
}
```

- [ ] **Step 4: 同步 GetChatDetail 返回 status**

在 `GetChatDetail` 组装消息处，给每条消息补 `Status: m.Status`（Task 2 已加 DTO 字段）。

- [ ] **Step 5: 编译**

Run: `cd server && go build ./...`
Expected: 通过（handler 调用处会暂时不匹配——Task 5 修）。若 handler 报错，先继续 Task 5 再统一编译。

- [ ] **Step 6: 提交**

```bash
git add server/internal/service/chat_service.go
git commit -m "feat: StreamChat 脱离请求生命周期并支持续传/取消"
```

---

## Task 5: Handler + 路由 + main.go 接线

**Files:**
- Modify: `server/internal/handler/chat.go`
- Modify: `server/internal/router/portal.go`
- Modify: `server/cmd/main.go`

**Interfaces:**
- Consumes: `ChatService.StreamChat/ResumeStream/CancelGeneration/CleanupStaleGenerating`、`NewGenerationHub`。
- Produces: `ResumeStream`、`CancelGeneration` 两个 handler；两条新路由；main 中 hub 构造与启动清理。

- [ ] **Step 1: 抽公共 SSE 写出函数**

`server/internal/handler/chat.go` 加：

```go
// writeStream 把订阅到的 replay + 实时事件写到 SSE 客户端。
// 客户端断开时通过 unsub 退订（不影响后端生成），由 ctx.Done() 感知断开。
func writeStream(c *gin.Context, replay []service.StreamEvent, ch <-chan service.StreamEvent, unsub func()) {
	defer unsub()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrUnknown, "流式不被支持")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	for _, evt := range replay {
		_ = writeSSEEvent(c.Writer, evt)
	}
	flusher.Flush()
	for {
		select {
		case <-c.Request.Context().Done():
			return // 客户端断开：退订，生成继续
		case evt, ok := <-ch:
			if !ok {
				return
			}
			_ = writeSSEEvent(c.Writer, evt)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 2: 改写 StreamChatMessage + 新增 ResumeStream/CancelGeneration**

```go
// StreamChatMessage 发起新问题（POST）。生成已脱离本请求，断开不影响。
func (h *ChatHandler) StreamChatMessage(c *gin.Context) {
	sessionID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	userID := c.GetInt64("user_id")
	var req request.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败")
		return
	}
	replay, ch, unsub, err := h.svc.StreamChat(c.Request.Context(), sessionID, req.Question, userID, req.RouteCount, req.RerankCount)
	if err != nil {
		response.AutoError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// ResumeStream 续传（GET ?since=N）：刷新/重连后接上进行中的生成。
func (h *ChatHandler) ResumeStream(c *gin.Context) {
	sessionID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	userID := c.GetInt64("user_id")
	since, _ := strconv.Atoi(c.DefaultQuery("since", "0"))
	replay, ch, unsub, err := h.svc.ResumeStream(c.Request.Context(), sessionID, userID, since)
	if err != nil {
		response.AutoError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// CancelGeneration 真正停止后端生成（POST）。
func (h *ChatHandler) CancelGeneration(c *gin.Context) {
	sessionID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	userID := c.GetInt64("user_id")
	if err := h.svc.CancelGeneration(c.Request.Context(), sessionID, userID); err != nil {
		response.AutoError(c, err)
		return
	}
	response.Success(c, nil)
}
```

> `response.AutoError`/`response.Error`/`c.GetInt64("user_id")` 按本仓库既有 handler 写法对齐（参考同文件其他方法如何取 userID 与返回错误）。

- [ ] **Step 3: 注册路由**

`server/internal/router/portal.go`，在 chat-sessions 分组内、`StreamChatMessage` 那行附近加：

```go
		rg.GET("/chat-sessions/:id/stream", h.Chat.ResumeStream)    // 续传（断点重连）
		rg.POST("/chat-sessions/:id/cancel", h.Chat.CancelGeneration) // 停止生成
```

- [ ] **Step 4: main.go 构造 hub + 注入 + 启动清理**

`server/cmd/main.go`：在构造 ChatService 处先建 hub 并传入，初始化后执行清理：

```go
	genHub := service.NewGenerationHub()
	chatService := service.NewChatService(/* …既有依赖…, */ genHub)
	// …
	if err := chatService.CleanupStaleGenerating(context.Background()); err != nil {
		slog.Warn("启动清理残留 generating 失败", "err", err)
	}
```

> `NewChatService` 的既有参数列表照搬，只在末尾加 `genHub`。

- [ ] **Step 5: 全量编译 + 既有测试**

Run: `cd server && go build ./... && go vet ./...`
Expected: 通过。
Run: `cd server && go test ./tests/service/ -run 'TestHub_|TestChatRepo_' -tags=integration -p 1 -v`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add server/internal/handler/chat.go server/internal/router/portal.go server/cmd/main.go
git commit -m "feat: 新增续传/取消接口并接线 GenerationHub"
```

---

## Task 6: 后端集成验证（断开仍完成 + 启动清理）

**Files:**
- Modify: `server/tests/service/chat_stream_test.go`（追加端到端集成用例）

**Interfaces:**
- Consumes: `ChatService.StreamChat`、`hub`、真实 `llmService`/embedding（依测试环境）。

- [ ] **Step 1: 追加「断开订阅后生成仍落库完成」测试**

> 该用例需要真实 LLM/embedding 可用的集成环境。若 CI 无 LLM，则用一个返回固定事件的轻量 llmService 实现（仍不算 mock——它是真实接口实现，行为确定），构造一个最小 ChatService。优先复用既有 `tests/service` 中构造 ChatService 的方式。

```go
//go:build integration

func TestStreamChat_SurvivesClientDisconnect(t *testing.T) {
	svc, hub, db, sessionID, userID := newChatServiceForTest(t) // 见下方说明
	_ = hub
	ctx := context.Background()

	replay, ch, unsub, err := svc.StreamChat(ctx, sessionID, "如何重置VPN密码", userID, 0, 0)
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}
	_ = replay
	// 立即退订模拟客户端断开
	unsub()

	// 轮询 DB 直到 assistant 消息变 completed（生成在后台继续）
	deadline := time.Now().Add(30 * time.Second)
	var done bool
	for time.Now().Before(deadline) {
		var m model.ChatMessage
		db.Where("session_id = ? AND role = ?", sessionID, "assistant").
			Order("id desc").First(&m)
		if m.Status == model.MessageStatusCompleted && m.Content != "" {
			done = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !done {
		t.Fatal("断开后生成未在后台完成落库")
	}
	// 同时应已落库 user 消息
	var userCnt int64
	db.Model(&model.ChatMessage{}).Where("session_id = ? AND role = ?", sessionID, "user").Count(&userCnt)
	if userCnt < 1 {
		t.Fatal("用户消息未立即落库")
	}
	_ = ch
}
```

> `newChatServiceForTest` 按本仓库既有集成用例风格构造：真实 DB + 真实 adapter + `NewGenerationHub()`，并预置一个属于 `userID` 的会话与一篇已发布文章（或复用 seed）。

- [ ] **Step 2: 追加启动清理用例**

```go
//go:build integration

func TestCleanupStaleGenerating(t *testing.T) {
	svc, _, db, sessionID, _ := newChatServiceForTest(t)
	db.Create(&model.ChatMessage{SessionID: sessionID, Role: "assistant", Content: "半截", Status: model.MessageStatusGenerating})
	if err := svc.CleanupStaleGenerating(context.Background()); err != nil {
		t.Fatalf("清理: %v", err)
	}
	var cnt int64
	db.Model(&model.ChatMessage{}).Where("status = ?", model.MessageStatusGenerating).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("仍有 %d 条 generating 未清理", cnt)
	}
}
```

- [ ] **Step 3: 运行集成测试**

Run: `cd server && go test ./tests/service/ -run 'TestStreamChat_|TestCleanupStaleGenerating' -tags=integration -p 1 -v`
Expected: PASS（需 PG + 可用 LLM/embedding；本地用当前 DeepSeek+本地向量栈即可）。

- [ ] **Step 4: 提交**

```bash
git add server/tests/service/chat_stream_test.go
git commit -m "test: 验证断开后生成仍完成与启动清理"
```

---

## Task 7: 前端全局 ChatStreamProvider + API 辅助

**Files:**
- Create: `web/src/contexts/ChatStreamProvider.tsx`
- Modify: `web/src/lib/api/chat.ts`
- Modify: `web/src/app/portal/layout.tsx`

**Interfaces:**
- Produces:
  - `ChatStreamProvider`（组件）
  - `useChatStreamStore(): ChatStreamStore`，含 `getStream(id)`、`send(sessionId|null, kbId, question)`、`resume(sessionId, since)`、`cancel(sessionId)`
  - `cancelGeneration(id)`、`streamUrl(id)`、`resumeUrl(id, since)`（`lib/api/chat.ts`）

- [ ] **Step 1: API 辅助**

`web/src/lib/api/chat.ts` 追加：

```ts
const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
// 直连后端（SSE 绕过 Next rewrite，避免 Turbopack POST 代理问题）
export const streamUrl = (id: number) => `${API}/api/v1/portal/chat-sessions/${id}/stream`;
export const resumeUrl = (id: number, since: number) => `${streamUrl(id)}?since=${since}`;
export function cancelGeneration(id: number) {
  return apiFetch<null>(`/api/v1/portal/chat-sessions/${id}/cancel`, { method: 'POST' });
}
```

- [ ] **Step 2: 实现 Provider（store 跨路由保活）**

创建 `web/src/contexts/ChatStreamProvider.tsx`：

```tsx
'use client';
// ChatStreamProvider —— 把流式状态从聊天页面提升到 portal 布局层。
// 为什么：原状态在页面内 hook，导航离开即卸载丢失。提升到布局层后跨路由保活，
// 配合后端续传，实现「离开/刷新不丢、多会话并行」。
import { createContext, useContext, useRef, useState, useCallback } from 'react';
import { streamUrl, resumeUrl, cancelGeneration, createSession } from '@/lib/api/chat';

export interface ChatMessage {
  id: string; role: 'user' | 'assistant' | 'system'; content: string;
  sources?: { doc_name: string; chunk_content: string; confidence: number }[];
  confidence?: number; status?: string; createdAt: string;
}
interface PipelineStep { id: string; label: string; duration_ms?: number; }
interface SessionStream {
  messages: ChatMessage[]; status: 'idle' | 'streaming' | 'error';
  lastSeq: number; pipelineSteps: PipelineStep[]; currentStep: string | null;
}
interface Store {
  getStream(id: number): SessionStream | undefined;
  setMessages(id: number, msgs: ChatMessage[]): void;
  send(sessionId: number | null, kbId: number, question: string, token: string,
       onError: (m: string) => void): Promise<number | null>;
  resume(id: number, since: number, token: string): void;
  cancel(id: number): Promise<void>;
}

const Ctx = createContext<Store | null>(null);
export const useChatStreamStore = () => {
  const v = useContext(Ctx);
  if (!v) throw new Error('useChatStreamStore 必须在 ChatStreamProvider 内');
  return v;
};

export function ChatStreamProvider({ children }: { children: React.ReactNode }) {
  const [streams, setStreams] = useState<Record<number, SessionStream>>({});
  const controllers = useRef<Record<number, AbortController>>({});

  const patch = useCallback((id: number, f: (s: SessionStream) => SessionStream) => {
    setStreams((prev) => {
      const cur = prev[id] ?? { messages: [], status: 'idle', lastSeq: -1, pipelineSteps: [], currentStep: null };
      return { ...prev, [id]: f(cur) };
    });
  }, []);

  // 共用：消费一个 SSE Response 流，按 seq 去重，更新 store
  const consume = useCallback(async (id: number, resp: Response, onError?: (m: string) => void) => {
    if (!resp.ok || !resp.body) { onError?.(`HTTP ${resp.status}`); patch(id, s => ({ ...s, status: 'error' })); return; }
    patch(id, s => ({ ...s, status: 'streaming' }));
    const reader = resp.body.getReader();
    const dec = new TextDecoder();
    let buf = ''; let acc = '';
    const ensureAssistant = () => patch(id, s => {
      const last = s.messages[s.messages.length - 1];
      if (last?.role === 'assistant' && last.status === 'generating') return s;
      return { ...s, messages: [...s.messages, { id: `a-${Date.now()}`, role: 'assistant', content: '', status: 'generating', createdAt: new Date().toISOString() }] };
    });
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const lines = buf.split('\n'); buf = lines.pop() || '';
      for (const ln of lines) {
        if (!ln.startsWith('data: ')) continue;
        let evt: any; try { evt = JSON.parse(ln.slice(6)); } catch { continue; }
        // seq 去重：只接受比已消费更大的
        let skip = false;
        patch(id, s => { if (evt.seq <= s.lastSeq) { skip = true; return s; } return { ...s, lastSeq: evt.seq }; });
        if (skip) continue;
        if (evt.type === 'step') { ensureAssistant(); patch(id, s => ({ ...s, currentStep: evt.label, pipelineSteps: [...s.pipelineSteps, { id: evt.id, label: evt.label }] })); }
        else if (evt.type === 'token') { ensureAssistant(); acc += evt.content; patch(id, s => ({ ...s, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, content: acc } : m) })); }
        else if (evt.type === 'done') { const meta = evt.metadata; patch(id, s => ({ ...s, status: 'idle', currentStep: null, messages: s.messages.map((m, i) => i === s.messages.length - 1 ? { ...m, content: meta.answer || acc, sources: meta.sources, confidence: meta.confidence, status: 'completed' } : m), pipelineSteps: meta.pipeline?.steps || s.pipelineSteps })); }
        else if (evt.type === 'error') { patch(id, s => ({ ...s, status: 'error', currentStep: null })); onError?.(evt.error || '生成失败'); }
      }
    }
  }, [patch]);

  const getStream = useCallback((id: number) => streams[id], [streams]);
  const setMessages = useCallback((id: number, msgs: ChatMessage[]) => patch(id, s => ({ ...s, messages: msgs, lastSeq: -1 })), [patch]);

  const send: Store['send'] = useCallback(async (sessionId, kbId, question, token, onError) => {
    let sid = sessionId;
    if (!sid) { const r = await createSession(kbId, question.slice(0, 50)); sid = r.session_id; }
    patch(sid, s => ({ ...s, lastSeq: -1, pipelineSteps: [], messages: [...s.messages, { id: `u-${Date.now()}`, role: 'user', content: question, createdAt: new Date().toISOString() }] }));
    const ctrl = new AbortController(); controllers.current[sid] = ctrl;
    try {
      const resp = await fetch(streamUrl(sid), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: JSON.stringify({ question }), signal: ctrl.signal });
      await consume(sid, resp, onError);
    } catch (e: any) { if (e?.name !== 'AbortError') { onError(e?.message || '请求失败'); patch(sid!, s => ({ ...s, status: 'error' })); } }
    return sid;
  }, [patch, consume]);

  const resume: Store['resume'] = useCallback(async (id, since, token) => {
    const ctrl = new AbortController(); controllers.current[id] = ctrl;
    try {
      const resp = await fetch(resumeUrl(id, since), { headers: { Authorization: `Bearer ${token}` }, signal: ctrl.signal });
      if (resp.status === 404) return; // 无活跃生成
      await consume(id, resp);
    } catch (e: any) { if (e?.name !== 'AbortError') { /* 续传失败静默，详情已有完整消息 */ } }
  }, [consume]);

  const cancel: Store['cancel'] = useCallback(async (id) => {
    await cancelGeneration(id);
    controllers.current[id]?.abort();
  }, []);

  return <Ctx.Provider value={{ getStream, setMessages, send, resume, cancel }}>{children}</Ctx.Provider>;
}
```

- [ ] **Step 3: 在 portal 布局包裹 Provider**

`web/src/app/portal/layout.tsx`：

```tsx
'use client';
import { PortalLayout as PortalLayoutUI } from '@/components/layout/PortalLayout';
import { ChatStreamProvider } from '@/contexts/ChatStreamProvider';

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return (
    <ChatStreamProvider>
      <PortalLayoutUI>{children}</PortalLayoutUI>
    </ChatStreamProvider>
  );
}
```

- [ ] **Step 4: 类型检查**

Run: `cd web && npm run lint`
Expected: 无类型/lint 错误（Provider 未被页面消费也应通过编译）。

- [ ] **Step 5: 提交**

```bash
git add web/src/contexts/ChatStreamProvider.tsx web/src/lib/api/chat.ts web/src/app/portal/layout.tsx
git commit -m "feat: 新增全局 ChatStreamProvider 跨路由保活流式状态"
```

---

## Task 8: 聊天页消费 store + 进入会话自动续传 + 停止按钮

**Files:**
- Modify: `web/src/app/portal/chat/page.tsx`
- Modify/Delete: `web/src/hooks/useChatStream.ts`（移除页面对它的依赖）

**Interfaces:**
- Consumes: `useChatStreamStore`、`getChatDetail`（返回消息含 `status`）、`cancel`。

- [ ] **Step 1: 页面改用 store**

`web/src/app/portal/chat/page.tsx`：
- 删除 `const { messages, streaming, ... } = useChatStream(...)`，改为：

```tsx
import { useChatStreamStore } from '@/contexts/ChatStreamProvider';
// …
const store = useChatStreamStore();
const stream = sessionId ? store.getStream(sessionId) : undefined;
const messages = stream?.messages ?? [];
const streaming = stream?.status === 'streaming';
const pipelineSteps = stream?.pipelineSteps ?? [];
const currentStep = stream?.currentStep ?? null;
```

- `handleSend` 改为：

```tsx
const handleSend = async (text?: string) => {
  const question = (text || input).trim();
  if (!question || !selectedKB || !token) return;
  if (isTokenExpired(token)) { toast.error('登录已过期，请刷新页面'); return; }
  setInput('');
  const wasNew = !sessionId;
  const sid = await store.send(sessionId, selectedKB, question, token, (m) => toast.error(m));
  if (sid) { setSessionId(sid); if (wasNew) mutateSessions(); }
};
```

- [ ] **Step 2: 进入会话时填充历史 + 自动续传**

替换 `handleSelectSession` 的加载逻辑（仍调 `getChatDetail`），并在选中会话后判断续传：

```tsx
const handleSelectSession = async (id: number) => {
  if (id === sessionId) return;
  setSessionId(id);
  setFeedbackMap({});
  try {
    const detail = await getChatDetail(id);
    const msgs = ((detail.messages ?? []) as ApiChatMessage[]).map((m) => ({
      id: String(m.id), role: m.role, content: m.content,
      sources: m.sources, confidence: m.confidence, status: m.status,
      createdAt: m.created_at,
    }));
    store.setMessages(id, msgs);
    // 最后一条 assistant 仍 generating → 续传（从头回放缓冲）
    const last = msgs[msgs.length - 1];
    if (last?.role === 'assistant' && last.status === 'generating' && token) {
      store.resume(id, 0, token);
    }
  } catch { toast.error('加载会话失败'); }
};
```

（`ApiChatMessage` 接口加 `status?: string;`。）

- [ ] **Step 3: 停止按钮调后端 cancel**

在流式进行中（`streaming`）渲染「停止生成」按钮：

```tsx
{streaming && sessionId && (
  <AppleButton variant="utility" onClick={() => store.cancel(sessionId)}>停止生成</AppleButton>
)}
```

- [ ] **Step 4: 移除对 useChatStream 的引用**

删除 `import { useChatStream } from '@/hooks/useChatStream'` 及残留用法。`useChatStream.ts` 可删除（确认无其他引用：`grep -rn useChatStream web/src`）。`ChatMessage` 类型从 Provider 导出处引入。

- [ ] **Step 5: 类型检查 + 构建**

Run: `cd web && npm run lint && npm run build`
Expected: 通过。

- [ ] **Step 6: 提交**

```bash
git add web/src/app/portal/chat/page.tsx web/src/hooks/useChatStream.ts
git commit -m "feat: 聊天页改用全局流 store 并支持续传与停止生成"
```

---

## Task 9: Playwright E2E（离开/刷新/并行/停止）

**Files:**
- Create: `web/e2e/chat-resume.spec.ts`

**Interfaces:**
- Consumes: 运行中的本地栈（DeepSeek LLM + 本地 embedding，已有 1 篇已发布 VPN 文章）。

- [ ] **Step 1: 写 E2E（四条路径）**

创建 `web/e2e/chat-resume.spec.ts`（按本仓库既有 Playwright 配置/登录辅助对齐；下为行为骨架，需补登录与选择知识库的既有步骤）：

```ts
import { test, expect } from '@playwright/test';

const login = async (page) => { /* 复用既有登录：reporter1/Reporter@123 → /portal/chat */ };

test('离开功能页再回来，回复不丢失', async ({ page }) => {
  await login(page);
  // 选知识库 + 发问
  await page.getByPlaceholder(/输入问题/).fill('如何解决 VPN 连接超时的问题？');
  await page.keyboard.press('Enter');
  await expect(page.getByText(/查询改写|向量检索/)).toBeVisible();
  // 切到「我的申告」再切回
  await page.getByRole('link', { name: /我的申告/ }).click();
  await page.getByRole('link', { name: /智能问答/ }).click();
  // 回复仍在或已完成
  await expect(page.getByText(/VPN/)).toBeVisible({ timeout: 30000 });
});

test('刷新页面后续传，最终出现完整答案', async ({ page }) => {
  await login(page);
  await page.getByPlaceholder(/输入问题/).fill('如何解决 VPN 连接超时的问题？');
  await page.keyboard.press('Enter');
  await expect(page.getByText(/查询改写|向量检索/)).toBeVisible();
  await page.reload();
  // 进入该会话后应续传/展示完整答案
  await expect(page.getByText(/VPN/)).toBeVisible({ timeout: 30000 });
});

test('停止生成后刷新不再继续', async ({ page }) => {
  await login(page);
  await page.getByPlaceholder(/输入问题/).fill('请详细说明 VPN 的所有排查步骤');
  await page.keyboard.press('Enter');
  await page.getByRole('button', { name: '停止生成' }).click();
  await page.reload();
  await expect(page.getByRole('button', { name: '停止生成' })).toHaveCount(0);
});
```

- [ ] **Step 2: 运行 E2E**

Run: `cd web && npx playwright test e2e/chat-resume.spec.ts`
Expected: 3 个用例通过（需本地栈在跑）。

- [ ] **Step 3: 提交**

```bash
git add web/e2e/chat-resume.spec.ts
git commit -m "test: 新增可续传对话 E2E"
```

---

## 收尾

- 全量回归：`cd server && go test ./tests/... -tags=integration -p 1`；`cd web && npm run build`。
- 对照设计文档第 6 节错误矩阵逐项手测一遍（尤其「服务重启后残留 generating → failed」：重启 server 后刷新会话不应卡「生成中」）。
- 按 submit-pr skill：分支 `feat/resumable-chat-streaming`，提交署名 scutmmq、无 AI 署名，**等用户确认后**再 push 到 fork 并开 PR。

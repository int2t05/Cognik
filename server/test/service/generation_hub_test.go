// Package service_test 验证 GenerationHub 的并发与续传语义。
// 为什么不 mock：Hub 是纯内存结构，直接用真实实例最接近运行时行为。
package service_test

import (
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

// 生成已 Finish 后(宽限期内)Subscribe 应成功：全量回放 + 通道已关闭、无实时事件。
func TestHub_SubscribeAfterFinish(t *testing.T) {
	h := service.NewGenerationHub()
	_ = h.Start(7, 700, func() {})
	h.Publish(7, service.StreamEvent{Type: "token", Content: "a"})
	h.Publish(7, service.StreamEvent{Type: "done"})
	h.Finish(7)

	replay, ch, unsub, ok := h.Subscribe(7, 0)
	if !ok {
		t.Fatal("宽限期内对已结束生成 Subscribe 应返回 ok=true")
	}
	defer unsub()
	if len(replay) != 2 {
		t.Fatalf("应回放 2 个事件，得到 %d", len(replay))
	}
	if _, open := <-ch; open {
		t.Fatal("已结束生成返回的通道应为已关闭状态")
	}
}

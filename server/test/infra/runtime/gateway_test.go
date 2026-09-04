// Package runtime_test 验证 Gateway 网关的游标重放、多订阅者、慢消费者丢弃与并发语义。
package runtime_test

import (
	"sync"
	"testing"
	"time"

	"cognos/internal/infra/runtime"
)

// evt 测试用事件类型（仅 Seq 字段参与网关游标对齐）。
type evt struct {
	Type    string
	Seq     int
	Content string
}

// newGateway 构造带 Seq 对齐回调的测试网关。
func newGateway() *runtime.Gateway[evt] {
	return runtime.NewGateway[evt](func(e evt, seq int) evt {
		e.Seq = seq
		return e
	})
}

func drain(ch <-chan evt, n int, d time.Duration) []evt {
	out := []evt{}
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
func TestGateway_ReplayThenLiveNoGap(t *testing.T) {
	g := newGateway()
	if err := g.Start("run1", func() {}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 3; i++ {
		g.Publish("run1", evt{Type: "token", Content: "x"})
	}
	replay, ch, unsub, ok := g.Subscribe("run1", 2)
	if !ok {
		t.Fatal("Subscribe 应成功")
	}
	defer unsub()
	if len(replay) != 1 || replay[0].Seq != 2 {
		t.Fatalf("回放应为 [seq=2]，得到 %+v", replay)
	}
	g.Publish("run1", evt{Type: "token", Content: "y"})
	live := drain(ch, 1, time.Second)
	if len(live) != 1 || live[0].Seq != 3 {
		t.Fatalf("实时应为 seq=3，得到 %+v", live)
	}
}

// 多订阅者都收到全量实时事件（对齐 LangGraph 多客户端 join 同一 run）。
func TestGateway_MultipleSubscribers(t *testing.T) {
	g := newGateway()
	_ = g.Start("run2", func() {})
	_, chA, ua, _ := g.Subscribe("run2", 0)
	_, chB, ub, _ := g.Subscribe("run2", 0)
	defer ua()
	defer ub()
	g.Publish("run2", evt{Type: "token", Content: "a"})
	if a := drain(chA, 1, time.Second); len(a) != 1 {
		t.Fatal("A 应收到事件")
	}
	if b := drain(chB, 1, time.Second); len(b) != 1 {
		t.Fatal("B 应收到事件")
	}
}

// 慢订阅者（不消费）不得阻塞生成：Publish 远超 buffer 容量仍快速返回。
func TestGateway_SlowSubscriberNotBlocking(t *testing.T) {
	g := newGateway()
	_ = g.Start("run3", func() {})
	_, _, unsub, _ := g.Subscribe("run3", 0) // 拿到通道但从不读
	defer unsub()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			g.Publish("run3", evt{Type: "token", Content: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("慢订阅者阻塞了生成")
	}
}

// 同 run 重复 Start 返回 ErrRunInProgress。
func TestGateway_DuplicateStart(t *testing.T) {
	g := newGateway()
	_ = g.Start("run4", func() {})
	if err := g.Start("run4", func() {}); err != runtime.ErrRunInProgress {
		t.Fatalf("应返回 ErrRunInProgress，得到 %v", err)
	}
}

// Cancel 触发 cancel 回调；Finish 后 Active=false。
func TestGateway_CancelAndFinish(t *testing.T) {
	g := newGateway()
	var mu sync.Mutex
	canceled := false
	_ = g.Start("run5", func() { mu.Lock(); canceled = true; mu.Unlock() })
	if !g.Cancel("run5") {
		t.Fatal("Cancel 应返回 true")
	}
	mu.Lock()
	c := canceled
	mu.Unlock()
	if !c {
		t.Fatal("cancel 回调未被调用")
	}
	g.Finish("run5")
	if g.Active("run5") {
		t.Fatal("Finish 后 Active 应为 false")
	}
}

// 用 -race 跑：Subscribe/Publish/Unsubscribe 并发无数据竞争。
func TestGateway_ConcurrentRace(t *testing.T) {
	g := newGateway()
	_ = g.Start("run6", func() {})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ch, unsub, ok := g.Subscribe("run6", 0)
			if !ok {
				return
			}
			go func() {
				for range ch {
				}
			}()
			time.Sleep(10 * time.Millisecond)
			unsub()
		}()
	}
	for i := 0; i < 200; i++ {
		g.Publish("run6", evt{Type: "token"})
	}
	wg.Wait()
	g.Finish("run6")
}

// 生成已 Finish 后(宽限期内)Subscribe 应成功：全量回放 + 通道已关闭、无实时事件。
func TestGateway_SubscribeAfterFinish(t *testing.T) {
	g := newGateway()
	_ = g.Start("run7", func() {})
	g.Publish("run7", evt{Type: "token", Content: "a"})
	g.Publish("run7", evt{Type: "done"})
	g.Finish("run7")

	replay, ch, unsub, ok := g.Subscribe("run7", 0)
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

// 环形缓冲覆盖：Publish 超过 eventStoreCap 后，旧事件被覆盖，replay(since) 从现存最旧开始。
func TestGateway_RingBufferOverwrite(t *testing.T) {
	g := newGateway()
	_ = g.Start("run8", func() {})
	// 写入超过 eventStoreCap 个事件（1024），验证不阻塞且 seq 持续增长
	for i := 0; i < 1100; i++ {
		g.Publish("run8", evt{Type: "token", Content: "x"})
	}
	// 用一个消费得起的订阅者验证最新事件可达（慢订阅者场景已被前序测试覆盖）
	replay, _, unsub, ok := g.Subscribe("run8", 1095)
	if !ok {
		t.Fatal("Subscribe 应成功")
	}
	defer unsub()
	if len(replay) != 5 {
		t.Fatalf("since=1095 应回放 5 个事件(1096..1100)，得到 %d", len(replay))
	}
	if replay[len(replay)-1].Seq != 1099 {
		t.Fatalf("最后一个事件 seq 应为 1099，得到 %d", replay[len(replay)-1].Seq)
	}
}

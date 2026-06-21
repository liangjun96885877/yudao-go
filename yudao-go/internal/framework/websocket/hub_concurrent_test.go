// Package websocket Hub 并发安全单测。
//
// 配合 -race 跑可检测出 conns/subs/channels 三张 map 在多 goroutine 下的数据竞争:
//   go test ./internal/framework/websocket -race -run TestHubConcurrent
//
// 构造 Conn 时刻意把 ws=nil,只调 register/subscribe/unsubscribe/pushLocal 这些
// 不触碰底层 websocket.Conn 的方法,避免对真实网络的依赖。
package websocket

import (
	"sync"
	"sync/atomic"
	"testing"
)

// newTestConn 构造一个不带真实 websocket.Conn 的测试连接。
// 不能调 close()(会 ws.Close() 的 nil panic),其余路径都安全。
func newTestConn(h *Hub, userID int64) *Conn {
	return &Conn{
		userID:   userID,
		tenantID: 1,
		ws:       nil, // 不触发 close 即可
		send:     make(chan []byte, 1024),
		done:     make(chan struct{}),
		channels: make(map[string]struct{}),
		owner:    h,
	}
}

// TestHubConcurrent_SubUnsubPush 并发对同一频道反复 sub/unsub + 推消息,
// 既验证无数据竞争(-race),也验证收尾后状态干净(subs 内该 channel 被回收)。
//
// 每个测试连接配一个 drain goroutine 持续排干 send 通道,避免缓冲打满后
// trySend 触发 close → 真实 ws.Conn 为 nil 的 panic。
func TestHubConcurrent_SubUnsubPush(t *testing.T) {
	h := NewHub(nil) // rdb=nil,本测试不走 Broadcast/Redis
	const (
		nConn = 20
		iters = 200
	)
	channel := RecordChannel(1, "concurrency_test", 42)

	conns := make([]*Conn, nConn)
	for i := 0; i < nConn; i++ {
		conns[i] = newTestConn(h, int64(i+1))
		h.register(conns[i])
	}

	// 每条连接配一个排干协程,在 stopDrain 关闭前持续消费 send,
	// 保证 trySend 永远走"成功送入 channel"分支,不会触发 close。
	stopDrain := make(chan struct{})
	var drainWg sync.WaitGroup
	for _, c := range conns {
		drainWg.Add(1)
		c := c
		go func() {
			defer drainWg.Done()
			for {
				select {
				case <-c.send:
				case <-stopDrain:
					// 收尾:再非阻塞排一次空,避免主测试 wg.Wait 后还有残留消息。
					for {
						select {
						case <-c.send:
						default:
							return
						}
					}
				}
			}
		}()
	}

	var wg sync.WaitGroup
	// 一半连接反复 sub/unsub,另一半反复推消息。
	for i, c := range conns {
		wg.Add(1)
		c := c
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				if i%2 == 0 {
					h.subscribe(c, channel)
					h.unsubscribe(c, channel)
				} else {
					h.pushLocal(channel, []byte("ping"))
				}
			}
		}()
	}
	wg.Wait()
	close(stopDrain)
	drainWg.Wait()

	// 收尾:所有连接都 unsubscribe 一次,channel 应从 subs 中被清理掉。
	for _, c := range conns {
		h.unsubscribe(c, channel)
	}
	h.mu.RLock()
	_, exists := h.subs[channel]
	h.mu.RUnlock()
	if exists {
		t.Fatalf("subs[%q] 应在最后一个订阅者退出后被回收", channel)
	}
}

// TestHubConcurrent_RegisterUnregister 并发 register/unregister 不同连接,
// 检验 conns 表的并发安全 + 索引完整(每条 conn 退出后无残留)。
func TestHubConcurrent_RegisterUnregister(t *testing.T) {
	h := NewHub(nil)
	const N = 100

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			c := newTestConn(h, int64(i%10+1)) // 10 个 user,每个 10 连接
			h.register(c)
			// 给每条连接订阅 2 个 channel
			h.subscribe(c, "ch-a")
			h.subscribe(c, RecordChannel(1, "tt", int64(i)))
			h.unregister(c) // 直接 unregister(close 走 ws,这里跳过)
		}()
	}
	wg.Wait()

	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.conns) != 0 {
		t.Fatalf("收尾后 conns 应为空, 实际 %d 个 user 残留", len(h.conns))
	}
	if len(h.subs) != 0 {
		t.Fatalf("收尾后 subs 应为空, 实际 %d 个 channel 残留", len(h.subs))
	}
}

// TestHubConcurrent_PushLocalDropOnFull 验证 trySend 慢消费者保护:
// send 缓冲塞满后 trySend 不阻塞,且后续 pushLocal 不阻塞 Hub。
// 这里只验"不会卡死"(用 1024 次 push 喂一个不消费的 conn,在 race 下也应 sub-second 返回)。
func TestHubConcurrent_PushLocalDropOnFull(t *testing.T) {
	h := NewHub(nil)
	c := newTestConn(h, 1)
	// send 缓冲是 sendBuffer(包内常量),手动捏一个小缓冲连接更贴近压力场景。
	c.send = make(chan []byte, 4)
	h.register(c)
	defer h.unregister(c)
	h.subscribe(c, "tight")

	// trySend 在满时若 done 未关闭会调用 close → ws.Close() nil panic。
	// 为避免 panic,这里主动先 close done,使 trySend 走到 `<-c.done` 分支直接返回。
	close(c.done)

	var dropped atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 1024; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			before := len(c.send)
			h.pushLocal("tight", []byte("x"))
			if len(c.send) == before {
				dropped.Add(1)
			}
		}()
	}
	wg.Wait()
	// done 已关闭,trySend 全部走 done 分支直接返回,dropped 应接近全量。
	if dropped.Load() == 0 {
		t.Fatalf("done 关闭后所有 push 应被丢弃,实际丢弃 %d", dropped.Load())
	}
}

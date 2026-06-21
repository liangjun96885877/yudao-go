// 多实例 WebSocket Hub 跨进程消息分发测试。
//
// 模拟生产多实例部署场景:
//   - hub1 和 hub2 是两个独立 Hub(代表两个 server 实例),共用一个真实 Redis;
//   - 客户端 conn 注册到 hub2,订阅 record 频道;
//   - hub1 调 Broadcast → Redis Pub/Sub `chatter:ws` → hub2 通过 runRedisSubscriber
//     收到 fanout message → 经 pushLocal 推到本地 conn.send。
//
// 跑: CGO_ENABLED=1 go test -race -count=1 ./internal/framework/websocket \
//      -run TestHub_CrossInstance
package websocket

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const crossInstanceRedisAddr = "127.0.0.1:16381"

func openCrossInstanceRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:        crossInstanceRedisAddr,
		DialTimeout: 800 * time.Millisecond,
	})
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		_ = rdb.Close()
		t.Skipf("跳过多实例 WS 测试: Redis 不可用 (%v)", err)
	}
	return rdb
}

// gracefulShutdown 不依赖 Hub.Close (会 close 所有 conn 触发 nil ws panic),
// 而是手动关闭 redis 订阅 + 等 wg。
//
// 实测 Hub 没暴露这种"不关连接"的退出接口,这里直接放任 goroutine 退出:
// 测试结束时 Redis 客户端 Close → Subscriber pubsub.Channel 关闭 → runRedisSubscriber 返回。
func gracefulShutdown(t *testing.T, h *Hub, conns []*Conn, rdb *redis.Client) {
	t.Helper()
	for _, c := range conns {
		h.unregister(c)
	}
	close(h.closed) // 停 runRedisSubscriber
	_ = rdb.Close()
	doneCh := make(chan struct{})
	go func() { h.wg.Wait(); close(doneCh) }()
	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Log("warn: hub wg.Wait 超时,goroutine 可能滞留")
	}
}

// TestHub_CrossInstanceBroadcast 单条消息跨实例分发。
func TestHub_CrossInstanceBroadcast(t *testing.T) {
	// 两个独立 Redis 客户端,模拟两进程
	rdb1 := openCrossInstanceRedis(t)
	rdb2 := openCrossInstanceRedis(t)

	hub1 := NewHub(rdb1)
	hub2 := NewHub(rdb2)
	hub1.Start()
	hub2.Start()
	// 给 runRedisSubscriber 充分时间订阅 Redis 频道
	time.Sleep(100 * time.Millisecond)

	conn := newTestConn(hub2, 42)
	hub2.register(conn)
	channel := RecordChannel(1, "cross_instance_test", time.Now().UnixNano())
	hub2.subscribe(conn, channel)
	defer gracefulShutdown(t, hub2, []*Conn{conn}, rdb2)
	defer gracefulShutdown(t, hub1, nil, rdb1)

	payload := []byte(`{"type":"timeline.new","item":{}}`)
	if err := hub1.Broadcast(context.Background(), []string{channel}, payload); err != nil {
		t.Fatalf("hub1.Broadcast: %v", err)
	}

	select {
	case msg := <-conn.send:
		if !bytes.Equal(msg, payload) {
			t.Fatalf("payload 不一致:\n want=%q\n got =%q", payload, msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("超时未收到跨实例广播")
	}
}

// TestHub_CrossInstanceMultiChannel 一次 Broadcast 多频道,验证 fanoutMessage.Channels
// 中的每个频道都能命中对应 conn。
func TestHub_CrossInstanceMultiChannel(t *testing.T) {
	rdb1 := openCrossInstanceRedis(t)
	rdb2 := openCrossInstanceRedis(t)
	hub1 := NewHub(rdb1)
	hub2 := NewHub(rdb2)
	hub1.Start()
	hub2.Start()
	time.Sleep(100 * time.Millisecond)

	connA := newTestConn(hub2, 101)
	connB := newTestConn(hub2, 102)
	hub2.register(connA)
	hub2.register(connB)
	chA := fmt.Sprintf("test:multi:A:%d", time.Now().UnixNano())
	chB := fmt.Sprintf("test:multi:B:%d", time.Now().UnixNano())
	hub2.subscribe(connA, chA)
	hub2.subscribe(connB, chB)
	defer gracefulShutdown(t, hub2, []*Conn{connA, connB}, rdb2)
	defer gracefulShutdown(t, hub1, nil, rdb1)

	payload := []byte(`{"type":"timeline.new"}`)
	if err := hub1.Broadcast(context.Background(), []string{chA, chB}, payload); err != nil {
		t.Fatalf("Broadcast: %v", err)
	}

	deadline := time.After(2 * time.Second)
	gotA, gotB := false, false
	for !(gotA && gotB) {
		select {
		case <-connA.send:
			gotA = true
		case <-connB.send:
			gotB = true
		case <-deadline:
			t.Fatalf("超时: gotA=%v gotB=%v", gotA, gotB)
		}
	}
}

// TestHub_CrossInstanceUserChannel 验证个人通知频道:
// hub2 上的 conn 注册时自动绑 UserChannel,hub1 推到该 userChannel,conn 收到。
func TestHub_CrossInstanceUserChannel(t *testing.T) {
	rdb1 := openCrossInstanceRedis(t)
	rdb2 := openCrossInstanceRedis(t)
	hub1 := NewHub(rdb1)
	hub2 := NewHub(rdb2)
	hub1.Start()
	hub2.Start()
	time.Sleep(100 * time.Millisecond)

	userID := int64(time.Now().UnixNano() % 100000)
	conn := newTestConn(hub2, userID)
	hub2.register(conn) // register 内部自动 subscribe(UserChannel(...))
	defer gracefulShutdown(t, hub2, []*Conn{conn}, rdb2)
	defer gracefulShutdown(t, hub1, nil, rdb1)

	userCh := UserChannel(conn.tenantID, userID)
	payload := []byte(`{"type":"notification.new"}`)
	if err := hub1.Broadcast(context.Background(), []string{userCh}, payload); err != nil {
		t.Fatalf("Broadcast: %v", err)
	}
	select {
	case <-conn.send:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("跨实例个人频道推送超时")
	}
}

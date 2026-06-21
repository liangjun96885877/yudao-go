// Package websocket 提供 WebSocket 连接与订阅管理，支持多实例 Redis fan-out。
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"yudao-go/internal/framework/logger"
)

// wsRedisChannel 是多实例广播所用的 Redis Pub/Sub 频道。
const wsRedisChannel = "chatter:ws"

// fanoutMessage 是跨实例广播的消息体。
type fanoutMessage struct {
	Channels []string        `json:"channels"`
	Payload  json.RawMessage `json:"payload"`
}

// Hub 管理所有 WebSocket 连接与频道订阅关系。
// 并发安全：conns 与 subs 两张表由同一把 RWMutex 保护。
// 多实例：推送经 Redis Pub/Sub 广播，每个实例向本地命中的连接下发。
type Hub struct {
	rdb *redis.Client

	mu    sync.RWMutex
	conns map[int64]map[*Conn]struct{}  // userID -> 连接集合
	subs  map[string]map[*Conn]struct{} // channel -> 连接集合

	wg        sync.WaitGroup
	closed    chan struct{}
	closeOnce sync.Once
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rdb:    rdb,
		conns:  make(map[int64]map[*Conn]struct{}),
		subs:   make(map[string]map[*Conn]struct{}),
		closed: make(chan struct{}),
	}
}

// RecordChannel 返回某业务记录的频道名。
func RecordChannel(tenantID int64, bizType string, bizID int64) string {
	return fmt.Sprintf("record:%d:%s:%d", tenantID, bizType, bizID)
}

// UserChannel 返回某用户的个人频道名（用于通知推送）。
func UserChannel(tenantID, userID int64) string {
	return fmt.Sprintf("user:%d:%d", tenantID, userID)
}

// Start 启动跨实例广播订阅协程。
func (h *Hub) Start() {
	h.wg.Add(1)
	go h.runRedisSubscriber()
}

// register 登记连接，并自动订阅其个人频道。
func (h *Hub) register(c *Conn) {
	userCh := UserChannel(c.tenantID, c.userID)
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.conns[c.userID]
	if set == nil {
		set = make(map[*Conn]struct{})
		h.conns[c.userID] = set
	}
	set[c] = struct{}{}
	h.addSubLocked(c, userCh)
}

// unregister 将连接从所有表中摘除。幂等。
func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.conns[c.userID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, c.userID)
		}
	}
	for ch := range c.channels {
		if set, ok := h.subs[ch]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.subs, ch)
			}
		}
	}
	c.channels = make(map[string]struct{})
}

// addSubLocked 把连接加入频道订阅。调用方必须持有 h.mu 写锁。
func (h *Hub) addSubLocked(c *Conn, channel string) {
	set := h.subs[channel]
	if set == nil {
		set = make(map[*Conn]struct{})
		h.subs[channel] = set
	}
	set[c] = struct{}{}
	c.channels[channel] = struct{}{}
}

func (h *Hub) subscribe(c *Conn, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.addSubLocked(c, channel)
}

func (h *Hub) unsubscribe(c *Conn, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.subs[channel]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.subs, channel)
		}
	}
	delete(c.channels, channel)
}

// Broadcast 把消息经 Redis 广播到指定频道（所有实例均会收到并向本地连接下发）。
func (h *Hub) Broadcast(ctx context.Context, channels []string, payload []byte) error {
	if len(channels) == 0 {
		return nil
	}
	data, err := json.Marshal(fanoutMessage{Channels: channels, Payload: payload})
	if err != nil {
		return err
	}
	return h.rdb.Publish(ctx, wsRedisChannel, data).Err()
}

// pushLocal 向本实例中订阅了某频道的连接下发消息。
func (h *Hub) pushLocal(channel string, msg []byte) {
	h.mu.RLock()
	targets := make([]*Conn, 0, len(h.subs[channel]))
	for c := range h.subs[channel] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	// 在锁外发送，避免慢连接阻塞 Hub。
	for _, c := range targets {
		c.trySend(msg)
	}
}

// runRedisSubscriber 订阅 Redis 广播频道，把消息下发到本地连接。
func (h *Hub) runRedisSubscriber() {
	defer h.wg.Done()
	pubsub := h.rdb.Subscribe(context.Background(), wsRedisChannel)
	defer func() { _ = pubsub.Close() }()
	ch := pubsub.Channel()
	for {
		select {
		case <-h.closed:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var fm fanoutMessage
			if err := json.Unmarshal([]byte(msg.Payload), &fm); err != nil {
				logger.L().Error("websocket: bad fanout message", "error", err)
				continue
			}
			for _, channel := range fm.Channels {
				h.pushLocal(channel, fm.Payload)
			}
		}
	}
}

// Close 关闭广播订阅与所有连接，用于进程优雅退出。
func (h *Hub) Close() {
	h.closeOnce.Do(func() { close(h.closed) })
	h.mu.RLock()
	all := make([]*Conn, 0)
	for _, set := range h.conns {
		for c := range set {
			all = append(all, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.close()
	}
	h.wg.Wait()
}

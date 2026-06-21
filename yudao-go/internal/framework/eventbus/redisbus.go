package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"yudao-go/internal/framework/logger"
)

const (
	streamPrefix    = "chatter:stream:" // 每个主题一个 Stream
	readBlock       = 2 * time.Second   // XReadGroup 阻塞时长
	readCount       = 16                // 单次读取条数
	reclaimInterval = 30 * time.Second  // 回收待确认消息的周期
	reclaimMinIdle  = 60 * time.Second  // 消息空闲多久后可被回收重投
	maxDeliveries   = 5                 // 最大投递次数，超出进入死信
)

func streamKey(topic string) string { return streamPrefix + topic }

// safeCall 调用处理器并隔离 panic。
func safeCall(ctx context.Context, h Handler, e DomainEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return h(ctx, e)
}

// RedisStreamBus 是基于 Redis Streams 的分布式事件总线。
//   - 同一消费组内每条事件只被一个实例处理一次；
//   - 处理失败的消息保留在 PEL，由 reclaim 协程在空闲超时后重投；
//   - 投递次数超过上限的「毒消息」进入死信日志并 ACK，避免无限循环。
type RedisStreamBus struct {
	rdb      *redis.Client
	codec    *Codec
	group    string
	consumer string

	mu       sync.RWMutex
	handlers map[string][]Handler
	started  bool

	wg        sync.WaitGroup
	closed    chan struct{}
	closeOnce sync.Once
}

// NewRedisStreamBus 创建总线。group 为消费组名（多实例共享），consumer 为实例唯一名。
func NewRedisStreamBus(rdb *redis.Client, codec *Codec, group, consumer string) *RedisStreamBus {
	return &RedisStreamBus{
		rdb: rdb, codec: codec, group: group, consumer: consumer,
		handlers: make(map[string][]Handler),
		closed:   make(chan struct{}),
	}
}

func (b *RedisStreamBus) Subscribe(topic string, h Handler) error {
	if h == nil {
		return errors.New("eventbus: nil handler")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		return errors.New("eventbus: subscribe after start")
	}
	b.handlers[topic] = append(b.handlers[topic], h)
	return nil
}

func (b *RedisStreamBus) Publish(ctx context.Context, e DomainEvent) error {
	env, err := b.codec.Encode(e)
	if err != nil {
		return err
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(e.Topic()),
		Values: map[string]any{"data": data},
		MaxLen: 1_000_000, // 近似裁剪，限制 Stream 内存
		Approx: true,
	}).Err()
}

func (b *RedisStreamBus) Start() error {
	b.mu.Lock()
	b.started = true
	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}
	b.mu.Unlock()

	ctx := context.Background()
	for _, topic := range topics {
		key := streamKey(topic)
		// 创建消费组（MKSTREAM 确保 Stream 存在）；组已存在则忽略。
		if err := b.rdb.XGroupCreateMkStream(ctx, key, b.group, "0").Err(); err != nil {
			if !strings.Contains(err.Error(), "BUSYGROUP") {
				return fmt.Errorf("eventbus: create group for %s: %w", key, err)
			}
		}
		b.wg.Add(1)
		go b.consumeLoop(topic, key)
	}
	b.wg.Add(1)
	go b.reclaimLoop(topics)
	return nil
}

func (b *RedisStreamBus) consumeLoop(topic, key string) {
	defer b.wg.Done()
	for {
		select {
		case <-b.closed:
			return
		default:
		}
		res, err := b.rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Group:    b.group,
			Consumer: b.consumer,
			Streams:  []string{key, ">"},
			Count:    readCount,
			Block:    readBlock,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue // 阻塞超时，无新消息
			}
			select {
			case <-b.closed:
				return
			default:
			}
			logger.L().Error("eventbus: XReadGroup error", "topic", topic, "error", err)
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				b.dispatch(topic, key, msg)
			}
		}
	}
}

// dispatch 处理一条消息：成功或毒消息 → XACK；可重试失败 → 保留在 PEL。
func (b *RedisStreamBus) dispatch(topic, key string, msg redis.XMessage) {
	raw, _ := msg.Values["data"].(string)
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logger.L().Error("eventbus: bad envelope, dropping", "id", msg.ID, "error", err)
		b.ack(key, msg.ID)
		return
	}
	event, err := b.codec.Decode(env)
	if err != nil {
		logger.L().Error("eventbus: decode failed, dropping", "topic", topic, "id", msg.ID, "error", err)
		b.ack(key, msg.ID)
		return
	}
	b.mu.RLock()
	hs := b.handlers[topic]
	b.mu.RUnlock()

	failed := false
	for _, h := range hs {
		if invokeErr := safeCall(context.Background(), h, event); invokeErr != nil {
			failed = true
			logger.L().Error("eventbus: handler failed",
				"topic", topic, "event_id", env.EventID, "error", invokeErr)
		}
	}
	if !failed {
		b.ack(key, msg.ID) // 失败则保留在 PEL，由 reclaimLoop 重投
	}
}

func (b *RedisStreamBus) ack(key, id string) {
	if err := b.rdb.XAck(context.Background(), key, b.group, id).Err(); err != nil {
		logger.L().Error("eventbus: XAck failed", "id", id, "error", err)
	}
}

// reclaimLoop 周期性回收长时间未确认的消息重新投递。
func (b *RedisStreamBus) reclaimLoop(topics []string) {
	defer b.wg.Done()
	ticker := time.NewTicker(reclaimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.closed:
			return
		case <-ticker.C:
			for _, topic := range topics {
				b.reclaimTopic(topic, streamKey(topic))
			}
		}
	}
}

func (b *RedisStreamBus) reclaimTopic(topic, key string) {
	ctx := context.Background()
	pending, err := b.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: key, Group: b.group,
		Idle: reclaimMinIdle, Start: "-", End: "+", Count: 64,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.L().Error("eventbus: XPendingExt failed", "topic", topic, "error", err)
		return
	}
	for _, p := range pending {
		if p.RetryCount > maxDeliveries {
			// 毒消息：死信并 ACK，避免无限重投。
			logger.L().Error("eventbus: message exceeded max deliveries, dead-lettering",
				"topic", topic, "id", p.ID, "retries", p.RetryCount)
			b.ack(key, p.ID)
			continue
		}
		claimed, err := b.rdb.XClaim(ctx, &redis.XClaimArgs{
			Stream: key, Group: b.group, Consumer: b.consumer,
			MinIdle: reclaimMinIdle, Messages: []string{p.ID},
		}).Result()
		if err != nil {
			logger.L().Error("eventbus: XClaim failed", "id", p.ID, "error", err)
			continue
		}
		for _, msg := range claimed {
			b.dispatch(topic, key, msg)
		}
	}
}

func (b *RedisStreamBus) Stop(ctx context.Context) error {
	b.closeOnce.Do(func() { close(b.closed) })
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

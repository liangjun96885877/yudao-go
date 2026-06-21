// Package eventbus 定义事件总线抽象。移植标准：业务只依赖本抽象，禁止直连 MQ SDK。
package eventbus

import (
	"context"
	"time"
)

// DomainEvent 是所有领域事件的统一接口。
type DomainEvent interface {
	EventID() string       // 全局唯一 ID，用于消费幂等
	Topic() string         // 事件主题，用于路由
	AggregateType() string // 聚合类型，如 "chatter_timeline"
	AggregateID() int64    // 聚合根 ID
	OccurredAt() time.Time // 事件发生时间
}

// Handler 处理一个领域事件。返回 error 仅记录日志（inproc 实现不重试）。
type Handler func(ctx context.Context, e DomainEvent) error

// Bus 是事件总线。当前提供进程内实现；后续可替换为 Kafka / RabbitMQ。
type Bus interface {
	// Publish 异步发布事件。调用方 context 取消不影响已入队事件的处理。
	Publish(ctx context.Context, e DomainEvent) error
	// Subscribe 订阅某主题。须在 Start 之前完成订阅。
	Subscribe(topic string, h Handler) error
	// Start 启动后台处理协程。
	Start() error
	// Stop 优雅停止：停止收新事件，处理完队列后返回；受 ctx 超时约束。
	Stop(ctx context.Context) error
}

// BaseEvent 可被具体事件结构体嵌入，复用公共字段实现。
type BaseEvent struct {
	ID      string
	Tp      string
	AggType string
	AggID   int64
	At      time.Time
}

func (e BaseEvent) EventID() string       { return e.ID }
func (e BaseEvent) Topic() string         { return e.Tp }
func (e BaseEvent) AggregateType() string { return e.AggType }
func (e BaseEvent) AggregateID() int64    { return e.AggID }
func (e BaseEvent) OccurredAt() time.Time { return e.At }

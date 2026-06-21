package service

import (
	"context"

	"yudao-go/internal/framework/eventbus"
)

// EventSink 接收领域事件。生产实现为事务发件箱（outbox.Outbox），
// 使业务写入与事件落库在同一事务内原子提交。
// 应用服务依赖此抽象，不直接依赖 infrastructure。
type EventSink interface {
	Append(ctx context.Context, e eventbus.DomainEvent) error
}

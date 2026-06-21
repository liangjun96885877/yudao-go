package outbox

import (
	"context"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/framework/orm"
)

// Outbox 把领域事件写入发件箱表。
// 调用方须在业务事务内调用 Append，使业务写入与事件落库原子提交。
type Outbox struct {
	tx    *orm.TxManager
	codec *eventbus.Codec
}

func NewOutbox(tx *orm.TxManager, codec *eventbus.Codec) *Outbox {
	return &Outbox{tx: tx, codec: codec}
}

// Append 将事件编码并写入发件箱。复用 ctx 中的事务连接（须在 tx.Do 内调用）。
func (o *Outbox) Append(ctx context.Context, e eventbus.DomainEvent) error {
	env, err := o.codec.Encode(e)
	if err != nil {
		return err
	}
	po := &OutboxPO{
		TenantID:      contextx.TenantID(ctx),
		EventID:       env.EventID,
		Topic:         env.Topic,
		AggregateType: env.AggregateType,
		AggregateID:   env.AggregateID,
		Payload:       string(env.Payload),
		Status:        StatusPending,
	}
	return o.tx.DB(ctx).Create(po).Error
}

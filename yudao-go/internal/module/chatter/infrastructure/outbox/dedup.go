package outbox

import (
	"context"

	"gorm.io/gorm/clause"

	"yudao-go/internal/framework/orm"
)

// Deduplicator 基于 chatter_consumed_event 表实现消费幂等。
type Deduplicator struct {
	tx *orm.TxManager
}

func NewDeduplicator(tx *orm.TxManager) *Deduplicator {
	return &Deduplicator{tx: tx}
}

// MarkConsumed 尝试登记 (consumerGroup, eventID) 已消费。
// 返回 true 表示首次消费应继续处理；false 表示重复事件应跳过。
// 须在消费者事务内调用：事务回滚时该登记一并回滚，事件可被重新处理。
func (d *Deduplicator) MarkConsumed(ctx context.Context, group, eventID string) (bool, error) {
	po := &ConsumedEventPO{ConsumerGroup: group, EventID: eventID}
	res := d.tx.DB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(po)
	if res.Error != nil {
		return false, res.Error
	}
	// RowsAffected==1 表示成功插入（首次）；==0 表示主键冲突（重复）。
	return res.RowsAffected == 1, nil
}

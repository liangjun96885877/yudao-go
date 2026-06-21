// Package outbox 实现事务发件箱模式：业务写入与事件发布在同一事务内原子完成，
// 再由 Relay 异步、可靠地投递到事件总线。
package outbox

import "time"

// 发件箱记录状态。
const (
	StatusPending   int8 = 0 // 待投递
	StatusPublished int8 = 1 // 已投递
	StatusFailed    int8 = 2 // 投递失败（超过重试上限或无法解码）
)

// OutboxPO 对应 chatter_event_outbox。
type OutboxPO struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID      int64      `gorm:"column:tenant_id"`
	EventID       string     `gorm:"column:event_id"`
	Topic         string     `gorm:"column:topic"`
	AggregateType string     `gorm:"column:aggregate_type"`
	AggregateID   int64      `gorm:"column:aggregate_id"`
	Payload       string     `gorm:"column:payload"`
	Status        int8       `gorm:"column:status"`
	RetryCount    int        `gorm:"column:retry_count"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
}

func (OutboxPO) TableName() string { return "chatter_event_outbox" }

// ConsumedEventPO 对应 chatter_consumed_event，用于消费幂等。
// 无 tenant_id 字段，故不受多租户插件影响。
type ConsumedEventPO struct {
	ConsumerGroup string    `gorm:"column:consumer_group;primaryKey"`
	EventID       string    `gorm:"column:event_id;primaryKey"`
	ConsumedAt    time.Time `gorm:"column:consumed_at;autoCreateTime"`
}

func (ConsumedEventPO) TableName() string { return "chatter_consumed_event" }

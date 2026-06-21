package outbox

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"gorm.io/gorm/clause"

	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/orm"
)

const (
	defaultRelayInterval = time.Second
	defaultRelayBatch    = 100
	defaultMaxRetry      = 10
)

// Relay 周期性地把发件箱中待投递的事件投递到事件总线。
// 多实例并发安全：取数使用 FOR UPDATE SKIP LOCKED。
type Relay struct {
	tx    *orm.TxManager
	bus   eventbus.Bus
	codec *eventbus.Codec

	interval  time.Duration
	batchSize int
	maxRetry  int

	closed    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func NewRelay(tx *orm.TxManager, bus eventbus.Bus, codec *eventbus.Codec) *Relay {
	return &Relay{
		tx: tx, bus: bus, codec: codec,
		interval: defaultRelayInterval, batchSize: defaultRelayBatch, maxRetry: defaultMaxRetry,
		closed: make(chan struct{}),
	}
}

// Start 启动后台投递协程。
func (r *Relay) Start() {
	r.wg.Add(1)
	go r.loop()
}

func (r *Relay) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.closed:
			return
		case <-ticker.C:
			if err := r.dispatch(context.Background()); err != nil {
				logger.L().Error("outbox relay dispatch error", "error", err)
			}
		}
	}
}

// dispatch 取一批待投递事件，逐条投递并更新状态，全程在一个事务内。
func (r *Relay) dispatch(ctx context.Context) error {
	return r.tx.Do(ctx, func(ctx context.Context) error {
		var rows []*OutboxPO
		err := r.tx.DB(ctx).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", StatusPending).
			Order("id ASC").Limit(r.batchSize).Find(&rows).Error
		if err != nil {
			return err
		}
		for _, row := range rows {
			r.deliver(ctx, row)
		}
		return nil
	})
}

// deliver 投递单条发件箱记录并更新其状态。
func (r *Relay) deliver(ctx context.Context, row *OutboxPO) {
	updates := map[string]any{}
	event, decErr := r.codec.Decode(eventbus.Envelope{
		Topic: row.Topic, Payload: json.RawMessage(row.Payload),
	})
	switch {
	case decErr != nil:
		// 无法解码：标记失败，避免无限重试。
		logger.L().Error("outbox: decode failed", "event_id", row.EventID, "error", decErr)
		updates["status"] = StatusFailed
	default:
		// 用干净的 context 投递：不能携带 Relay 事务句柄，
		// 否则异步事件处理器会误用这个已提交/关闭的事务。
		if pubErr := r.bus.Publish(context.Background(), event); pubErr != nil {
			row.RetryCount++
			updates["retry_count"] = row.RetryCount
			if row.RetryCount >= r.maxRetry {
				updates["status"] = StatusFailed
				logger.L().Error("outbox: publish exhausted retries",
					"event_id", row.EventID, "error", pubErr)
			}
		} else {
			updates["status"] = StatusPublished
			updates["published_at"] = time.Now()
		}
	}
	if err := r.tx.DB(ctx).Model(&OutboxPO{}).
		Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		logger.L().Error("outbox: update row failed", "id", row.ID, "error", err)
	}
}

// Stop 优雅停止投递协程。
func (r *Relay) Stop(ctx context.Context) error {
	r.closeOnce.Do(func() { close(r.closed) })
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

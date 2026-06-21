package consumer

import (
	"context"
	"sync"
	"testing"
	"time"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/infrastructure/outbox"
	"yudao-go/internal/module/chatter/infrastructure/persistence"
	"yudao-go/internal/pkg/idgen"
)

// TestConcurrent_EventDeduplication 验证事件幂等去重:
// N 个 goroutine 并发投递同一 event_id 的 RecordUpdated → 只生成 1 条 chatter_timeline,
// 由 chatter_consumed_event(consumer_group, event_id) 唯一键兜底。
//
// 跑: go test ./internal/module/chatter/infrastructure/consumer -race -run TestConcurrent
func TestConcurrent_EventDeduplication(t *testing.T) {
	db := openTestDB(t)
	tx := orm.NewTxManager(db)
	c := NewTimelineConsumer(
		persistence.NewTimelineRepo(tx),
		persistence.NewCommentRepo(tx),
		persistence.NewFollowerRepo(tx),
		persistence.NewNotificationRepo(tx),
		outbox.NewDeduplicator(tx),
		NopNotifier{},
		tx,
	)

	bizID := time.Now().UnixNano()
	// 关键:同一 event_id 被多个 goroutine 同时处理。
	evt := event.RecordUpdated{
		Base:  event.NewBase(idgen.UUID(), event.TopicRecordUpdated, "test_dedup", bizID),
		Ref:   model.BizRef{TenantID: 1, BizType: "test_dedup", BizID: bizID},
		Actor: model.Actor{Type: model.ActorUser, ID: 1, Name: "tester"},
		Changes: []model.FieldChange{
			{Field: "Name", Label: "名称", OldValue: "草稿", NewValue: "已审批", ValueType: "string"},
		},
	}
	defer func() {
		db.Unscoped().Where("biz_type = ?", "test_dedup").Delete(&persistence.TimelinePO{})
		db.Unscoped().Where("biz_type = ?", "test_dedup").Delete(&persistence.AuditLogPO{})
	}()

	const N = 10
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- c.onRecordUpdated(context.Background(), evt)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发处理不应报错: %v", err)
		}
	}

	var timelineCnt, auditCnt int64
	db.Table("chatter_timeline").
		Where("biz_type = ? AND biz_id = ?", "test_dedup", bizID).
		Count(&timelineCnt)
	db.Table("chatter_audit_log").
		Where("biz_type = ? AND biz_id = ?", "test_dedup", bizID).
		Count(&auditCnt)

	if timelineCnt != 1 {
		t.Fatalf("事件去重失效: 期望 1 条 timeline, 实际 %d", timelineCnt)
	}
	if auditCnt != 1 {
		t.Fatalf("审计明细应只随成功那次写一份: 期望 1 条, 实际 %d", auditCnt)
	}
}

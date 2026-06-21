package consumer

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/infrastructure/outbox"
	"yudao-go/internal/module/chatter/infrastructure/persistence"
	"yudao-go/internal/pkg/idgen"
)

// 开发库 DSN（deploy/docker-compose.yml 的 yudaogo-dev）。
const testDSN = "root:123456@tcp(127.0.0.1:13306)/yudao_go?charset=utf8mb4&parseTime=True&loc=Local"

// openTestDB 连接开发库；不可用时跳过测试。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("跳过集成测试：数据库不可用 (%v)", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		t.Skipf("跳过集成测试：数据库 ping 失败")
	}
	if err := orm.RegisterPlugins(db); err != nil {
		t.Fatalf("注册 ORM 插件失败: %v", err)
	}
	return db
}

// TestRecordUpdated_BuildsTimelineAndAuditLogs 验证字段变更审计链路：
// RecordUpdated 事件 → 时间线条目 + 审计明细，且重复事件幂等。
func TestRecordUpdated_BuildsTimelineAndAuditLogs(t *testing.T) {
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

	bizID := time.Now().UnixNano() // 保证业务记录唯一
	evt := event.RecordUpdated{
		Base:  event.NewBase(idgen.UUID(), event.TopicRecordUpdated, "erp_order", bizID),
		Ref:   model.BizRef{TenantID: 1, BizType: "erp_order", BizID: bizID},
		Actor: model.Actor{Type: model.ActorUser, ID: 1, Name: "tester"},
		Changes: []model.FieldChange{
			{Field: "Name", Label: "名称", OldValue: "草稿", NewValue: "已审批", ValueType: "string"},
			{Field: "Amount", Label: "金额", OldValue: "100", NewValue: "200", ValueType: "int"},
		},
	}

	if err := c.onRecordUpdated(context.Background(), evt); err != nil {
		t.Fatalf("处理 RecordUpdated 失败: %v", err)
	}

	var timelineCount, auditCount int64
	db.Table("chatter_timeline").
		Where("biz_type = ? AND biz_id = ? AND event_type = 'update'", "erp_order", bizID).
		Count(&timelineCount)
	if timelineCount != 1 {
		t.Fatalf("期望 1 条时间线，实际 %d", timelineCount)
	}
	db.Table("chatter_audit_log").
		Where("biz_type = ? AND biz_id = ?", "erp_order", bizID).
		Count(&auditCount)
	if auditCount != 2 {
		t.Fatalf("期望 2 条审计明细，实际 %d", auditCount)
	}

	// 幂等：重复处理同一事件不应产生新行。
	if err := c.onRecordUpdated(context.Background(), evt); err != nil {
		t.Fatalf("重复处理失败: %v", err)
	}
	db.Table("chatter_timeline").
		Where("biz_type = ? AND biz_id = ?", "erp_order", bizID).
		Count(&timelineCount)
	if timelineCount != 1 {
		t.Fatalf("幂等失效：期望仍为 1 条时间线，实际 %d", timelineCount)
	}
}

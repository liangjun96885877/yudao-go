// 通知扇出性能基准:模拟一条事件需要给 N 个关注者发通知的全链路:
//   followers.ListByBiz (一次 SELECT) → 内存过滤 + 去重 → notifications.CreateBatch (一次批量 INSERT)
//
// 不灌大规模数据,每个基准函数自带 N 个 follower 的 fixture,跑完 b.Cleanup 删干净。
//
// 跑:
//   CGO_ENABLED=1 go test -bench=BenchmarkFanout -benchmem -benchtime=2s \
//     ./internal/module/chatter/infrastructure/consumer
package consumer

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/infrastructure/outbox"
	"yudao-go/internal/module/chatter/infrastructure/persistence"
)

func openBenchDB(b *testing.B) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Skipf("跳过基准: DB 不可用 (%v)", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		b.Skipf("跳过基准: DB ping 失败")
	}
	if err := orm.RegisterPlugins(db); err != nil {
		b.Fatalf("注册 ORM 插件失败: %v", err)
	}
	return db
}

var fanoutBenchCounter atomic.Int64

func benchFanout(b *testing.B, nFollowers int) {
	db := openBenchDB(b)
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

	bizType := fmt.Sprintf("bench_fanout_%d_%d", time.Now().UnixNano(), fanoutBenchCounter.Add(1))
	bizID := time.Now().UnixNano()
	ctx := contextx.WithTenantID(context.Background(), 1)
	followerRepo := persistence.NewFollowerRepo(tx)

	// 灌 N 个关注者,每个 user_id 不同
	for i := 0; i < nFollowers; i++ {
		f := &model.Follower{
			Ref:      model.BizRef{TenantID: 1, BizType: bizType, BizID: bizID},
			UserID:   int64(1000000 + i),
			UserName: fmt.Sprintf("fan-%d", i),
			Reason:   model.FollowManual,
		}
		if err := followerRepo.Add(ctx, f); err != nil {
			b.Fatalf("seed follower %d: %v", i, err)
		}
	}
	b.Cleanup(func() {
		db.Unscoped().Where("biz_type = ?", bizType).Delete(&persistence.FollowerPO{})
		db.Unscoped().Where("biz_type = ?", bizType).Delete(&persistence.NotificationPO{})
	})

	// 准备一条 timeline(只用其 Ref/Actor/EventType/ID,不真存 DB)
	// 注意:扇出取 Actor.ID 用于过滤自己,这里设为 -1 保证 N 个 follower 全部入选。
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tl := &model.Timeline{
			ID:        int64(i + 1), // 假 id,反正只用在 notification.TimelineID
			Ref:       model.BizRef{TenantID: 1, BizType: bizType, BizID: bizID},
			EventType: model.EventComment,
			Actor:     model.Actor{Type: model.ActorUser, ID: -1, Name: "tester"},
			Summary:   "bench",
		}
		_, err := c.fanout(ctx, tl, &notifySpec{
			notifyType: model.NotifyComment, title: "test fanout",
		})
		if err != nil {
			b.Fatalf("fanout err: %v", err)
		}
	}
}

func BenchmarkFanout_10(b *testing.B)   { benchFanout(b, 10) }
func BenchmarkFanout_100(b *testing.B)  { benchFanout(b, 100) }
func BenchmarkFanout_1000(b *testing.B) { benchFanout(b, 1000) }

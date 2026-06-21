// chatter Feed 性能基准。
//
// 灌入 100 / 1k / 10k 条 timeline + 同等规模的 audit_log,
// 测 TimelineService.Feed 的完整调用(含审计明细 + flag + replyCount)。
//
// 跑:
//   CGO_ENABLED=1 go test -bench=BenchmarkFeed -benchmem -benchtime=3s \
//     ./internal/module/chatter/application/service
//
// DB 不可用时自动 Skip(基准函数 Skip 后报告 0 ns,不影响其他测试)。
package service

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
	"yudao-go/internal/module/chatter/infrastructure/persistence"
)

const benchDSN = "root:123456@tcp(127.0.0.1:13306)/yudao_go?charset=utf8mb4&parseTime=True&loc=Local"

func openBenchDB(b *testing.B) *gorm.DB {
	b.Helper()
	// 关 gorm 日志,基准跑数据库不打 SQL log
	db, err := gorm.Open(mysql.Open(benchDSN), &gorm.Config{
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

// uniqueBenchSuffix 每次 Benchmark 唯一后缀,避免多次跑数据混淆。
var benchCounter atomic.Int64

func uniqueBenchSuffix() string {
	return fmt.Sprintf("%d_%d", time.Now().UnixNano(), benchCounter.Add(1))
}

// seedTimelines 灌入 n 条 timeline + 每条 1 个 audit_log。
// 一半 update / 一半 comment(模拟真实混合流)。返回 bizType + bizID。
func seedTimelines(b *testing.B, db *gorm.DB, n int) (string, int64) {
	b.Helper()
	bizType := "bench_feed_" + uniqueBenchSuffix()
	bizID := time.Now().UnixNano()
	ctx := contextx.WithTenantID(context.Background(), 1)
	tx := orm.NewTxManager(db)
	repo := persistence.NewTimelineRepo(tx)

	for i := 0; i < n; i++ {
		eventType := model.EventUpdate
		if i%2 == 0 {
			eventType = model.EventComment
		}
		tl := &model.Timeline{
			Ref:       model.BizRef{TenantID: 1, BizType: bizType, BizID: bizID},
			EventType: eventType,
			Summary:   fmt.Sprintf("seed-%d", i),
			Actor:     model.Actor{Type: model.ActorUser, ID: 1, Name: "tester"},
			EventID:   fmt.Sprintf("bench-evt-%s-%d", bizType, i),
		}
		if err := repo.Save(ctx, tl); err != nil {
			b.Fatalf("seed timeline %d: %v", i, err)
		}
		// update 类型补 1 条 audit_log(模拟真实数据)
		if eventType == model.EventUpdate {
			al := &model.AuditLog{
				Ref:        tl.Ref,
				TimelineID: tl.ID,
				Change: model.FieldChange{
					Field: "Name", Label: "名称",
					OldValue: "old", NewValue: "new",
					ValueType: "string",
				},
			}
			if err := repo.SaveAuditLogs(ctx, []*model.AuditLog{al}); err != nil {
				b.Fatalf("seed audit %d: %v", i, err)
			}
		}
	}
	b.Cleanup(func() {
		db.Unscoped().Where("biz_type = ?", bizType).Delete(&persistence.TimelinePO{})
		db.Unscoped().Where("biz_type = ?", bizType).Delete(&persistence.AuditLogPO{})
	})
	return bizType, bizID
}

func benchFeed(b *testing.B, n int) {
	db := openBenchDB(b)
	tx := orm.NewTxManager(db)
	bizType, bizID := seedTimelines(b, db, n)

	svc := NewTimelineService(
		persistence.NewTimelineRepo(tx),
		persistence.NewCommentRepo(tx),
		tx, AllowAll{},
	)
	// 用 1 号用户上下文跑(super admin 走读)
	ctx := contextx.WithTenantID(context.Background(), 1)
	ctx = contextx.WithUserID(ctx, 1)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		page, err := svc.Feed(ctx, bizType, bizID, 0, 20)
		if err != nil {
			b.Fatalf("Feed err: %v", err)
		}
		if len(page.List) == 0 && n > 0 {
			b.Fatalf("Feed 空结果(seed %d 条)", n)
		}
	}
}

func BenchmarkFeed_100(b *testing.B)   { benchFeed(b, 100) }
func BenchmarkFeed_1000(b *testing.B)  { benchFeed(b, 1000) }
func BenchmarkFeed_10000(b *testing.B) { benchFeed(b, 10000) }

// 多租户读隔离集成测试 —— 验证 ORM 多租户插件在并发查询下不漏不串。
//
// 跑: go test ./internal/module/chatter/infrastructure/persistence -run TestTenantIsolation
package persistence

import (
	"context"
	"sync"
	"testing"
	"time"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
)

// withTenant 构造一个带租户 + 用户的 ctx,模拟登录态。
func withTenant(tenantID int64) context.Context {
	ctx := context.Background()
	ctx = contextx.WithTenantID(ctx, tenantID)
	ctx = contextx.WithUserID(ctx, 1)
	return ctx
}

// TestTenantIsolation_TimelineFeed 验证:
//   - 用租户 A 的 ctx 写入若干 timeline → tenant_id 自动填 A;
//   - 用租户 A 的 ctx 查询 → 只能看到 A 的,看不到 B 的;
//   - 反向同理。
func TestTenantIsolation_TimelineFeed(t *testing.T) {
	db := openConcurrentTestDB(t)
	tx := orm.NewTxManager(db)
	repo := NewTimelineRepo(tx)

	const (
		tenantA int64 = 9001
		tenantB int64 = 9002
	)
	bizID := time.Now().UnixNano()
	ref := model.BizRef{BizType: "test_tenant_iso", BizID: bizID}

	defer db.Unscoped().Where("biz_type = ?", "test_tenant_iso").Delete(&TimelinePO{})

	// A 写 3 条
	for i := 0; i < 3; i++ {
		tl := &model.Timeline{
			Ref:       model.BizRef{BizType: ref.BizType, BizID: ref.BizID, TenantID: tenantA},
			EventType: model.EventComment, Summary: "A 的评论",
			EventID: "evt-a-" + idForTest(i),
		}
		if err := repo.Save(withTenant(tenantA), tl); err != nil {
			t.Fatalf("A 写入失败: %v", err)
		}
	}
	// B 写 5 条
	for i := 0; i < 5; i++ {
		tl := &model.Timeline{
			Ref:       model.BizRef{BizType: ref.BizType, BizID: ref.BizID, TenantID: tenantB},
			EventType: model.EventComment, Summary: "B 的评论",
			EventID: "evt-b-" + idForTest(i),
		}
		if err := repo.Save(withTenant(tenantB), tl); err != nil {
			t.Fatalf("B 写入失败: %v", err)
		}
	}

	// A 查 → 只 3 条
	asA, err := repo.PageByBiz(withTenant(tenantA), ref, 0, 100)
	if err != nil {
		t.Fatalf("A 查询失败: %v", err)
	}
	if len(asA) != 3 {
		t.Fatalf("A 应只读到 3 条,实际 %d", len(asA))
	}
	for _, tl := range asA {
		if tl.Summary != "A 的评论" {
			t.Fatalf("A 读到了非 A 的数据: %+v", tl)
		}
	}

	// B 查 → 只 5 条
	asB, err := repo.PageByBiz(withTenant(tenantB), ref, 0, 100)
	if err != nil {
		t.Fatalf("B 查询失败: %v", err)
	}
	if len(asB) != 5 {
		t.Fatalf("B 应只读到 5 条,实际 %d", len(asB))
	}
	for _, tl := range asB {
		if tl.Summary != "B 的评论" {
			t.Fatalf("B 读到了非 B 的数据: %+v", tl)
		}
	}
}

// TestTenantIsolation_ConcurrentQueries 100 个 goroutine 一半 A 一半 B 并发查询,
// 任何一个 ctx 都不应错读到对方租户的数据(防 plugin 状态串)。
func TestTenantIsolation_ConcurrentQueries(t *testing.T) {
	db := openConcurrentTestDB(t)
	tx := orm.NewTxManager(db)
	repo := NewTimelineRepo(tx)

	const (
		tenantA int64 = 9101
		tenantB int64 = 9102
	)
	bizID := time.Now().UnixNano()
	ref := model.BizRef{BizType: "test_tenant_concur", BizID: bizID}

	defer db.Unscoped().Where("biz_type = ?", "test_tenant_concur").Delete(&TimelinePO{})

	// 各写 10 条
	for i := 0; i < 10; i++ {
		_ = repo.Save(withTenant(tenantA), &model.Timeline{
			Ref:       model.BizRef{BizType: ref.BizType, BizID: ref.BizID, TenantID: tenantA},
			EventType: model.EventComment, Summary: "A",
			EventID: "concur-a-" + idForTest(i),
		})
		_ = repo.Save(withTenant(tenantB), &model.Timeline{
			Ref:       model.BizRef{BizType: ref.BizType, BizID: ref.BizID, TenantID: tenantB},
			EventType: model.EventComment, Summary: "B",
			EventID: "concur-b-" + idForTest(i),
		})
	}

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan string, N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			var (
				ctx  context.Context
				want string
			)
			if i%2 == 0 {
				ctx, want = withTenant(tenantA), "A"
			} else {
				ctx, want = withTenant(tenantB), "B"
			}
			items, err := repo.PageByBiz(ctx, ref, 0, 100)
			if err != nil {
				errs <- err.Error()
				return
			}
			if len(items) != 10 {
				errs <- "数量不对"
				return
			}
			for _, it := range items {
				if it.Summary != want {
					errs <- "串租户: 期望 " + want + " 实际 " + it.Summary
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("并发租户隔离破裂: %s", msg)
	}
}

// TestTenantIsolation_NoContextSkipped 不带 tenant 上下文时,插件应跳过(返回全量),
// 这是设计上的保护:避免误把 0 号租户当真实租户去过滤。
// 用 IgnoreTenant 显式表达"我知道在裸读"。
func TestTenantIsolation_NoContextSkipped(t *testing.T) {
	db := openConcurrentTestDB(t)
	tx := orm.NewTxManager(db)
	repo := NewTimelineRepo(tx)

	const tenantA int64 = 9201
	bizID := time.Now().UnixNano()
	ref := model.BizRef{BizType: "test_tenant_skip", BizID: bizID}

	defer db.Unscoped().Where("biz_type = ?", "test_tenant_skip").Delete(&TimelinePO{})

	_ = repo.Save(withTenant(tenantA), &model.Timeline{
		Ref:       model.BizRef{BizType: ref.BizType, BizID: ref.BizID, TenantID: tenantA},
		EventType: model.EventComment, Summary: "A",
		EventID: "skip-a-" + idForTest(0),
	})

	// 无 tenant ctx → 插件 tid==0 直接跳过,等价 IgnoreTenant
	items, err := repo.PageByBiz(context.Background(), ref, 0, 100)
	if err != nil {
		t.Fatalf("裸 ctx 查询失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("裸 ctx 应不加 tenant 过滤,期望 1 条,实际 %d", len(items))
	}
}

// idForTest 生成稳定但唯一的 eventID 后缀,避免与并发跑的其他测试冲突。
func idForTest(i int) string {
	return time.Now().Format("20060102150405.000000") + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	const digits = "0123456789"
	var b [16]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = digits[i%10]
		i /= 10
	}
	return string(b[pos:])
}

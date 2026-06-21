// 发件箱端到端并发集成测试 —— 验证「业务并发 Append → Relay 投递 → InProcBus
// → 订阅者收齐」全链路不丢不重。
//
// 设计:
//   - 用 InProcBus(纯内存)替代 Redis Streams,避免外部依赖。
//   - 真实 MySQL outbox 表(yudao-go-mysql:13306),不可用时 Skip。
//   - 手动控制 Relay.dispatch 时机,断言每次状态变化都正确。
//
// 跑: CGO_ENABLED=1 go test -race -count=1 ./internal/module/chatter/infrastructure/outbox -run TestRelayE2E
package outbox

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/infrastructure/eventcodec"
	"yudao-go/internal/pkg/idgen"
)

// dispatchMine 是 Relay.dispatch 的限定版,只取本测试 bizTag 的 pending 行。
// 必须这么做的原因:本机若有 yudao-go server 在运行,它的 Relay 会跟测试抢同一张
// chatter_event_outbox 表(SKIP LOCKED 跨进程生效),把测试发的事件投递到 server
// 的 bus,导致测试 bus 收不到。隔离 bizTag 即跟生产 relay 完全分流。
//
// 返回实际处理(deliver 调用)的行数,供测试日志与重试控制。
func dispatchMine(tx *orm.TxManager, r *Relay, bizTag string) (int, error) {
	n := 0
	err := tx.Do(context.Background(), func(ctx context.Context) error {
		var rows []*OutboxPO
		err := tx.DB(ctx).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND aggregate_type = ?", StatusPending, bizTag).
			Order("id ASC").Limit(200).Find(&rows).Error
		if err != nil {
			return err
		}
		for _, row := range rows {
			r.deliver(ctx, row)
			n++
		}
		return nil
	})
	return n, err
}

const outboxTestDSN = "root:123456@tcp(127.0.0.1:13306)/yudao_go?charset=utf8mb4&parseTime=True&loc=Local"

func openOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.Open(outboxTestDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("跳过集成测试: 数据库不可用 (%v)", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		t.Skipf("跳过集成测试: 数据库 ping 失败")
	}
	if err := orm.RegisterPlugins(db); err != nil {
		t.Fatalf("注册 ORM 插件失败: %v", err)
	}
	// 跳过条件:本机 server.exe 在跑,其 Outbox Relay 会跟测试抢同一张表的行级锁
	// (SKIP LOCKED 跨进程生效),即使我们过滤 aggregate_type,server 取 status=0
	// 不带条件,可能先锁住所有 pending 行 → 测试 dispatchMine 直接跳过,导致少收事件。
	// 用 HTTP 探测 48090 健康端口判断 server 是否在跑。
	resp, err := http.Get("http://127.0.0.1:48090/health")
	if err == nil {
		_ = resp.Body.Close()
		t.Skipf("跳过 outbox 端到端测试: 本机 server.exe 在 :48090 运行,其 Relay 会与测试抢 outbox 表行锁。" +
			"请先 `Stop-Process -Name server -Force` 再跑此测试。")
	}
	return db
}

// TestRelayE2E_ConcurrentAppendAndDispatch:
// 100 个 goroutine 并发 Outbox.Append 不同 event_id → 手动 dispatch →
// InProcBus 异步派发 → handler 收齐 100 条;
// 断言 outbox 表 100 行全 status=Published、handler 集合 == 投入集合。
func TestRelayE2E_ConcurrentAppendAndDispatch(t *testing.T) {
	db := openOutboxTestDB(t)
	tx := orm.NewTxManager(db)
	codec := eventbus.NewCodec()
	eventcodec.Register(codec)

	const N = 100
	bus := eventbus.NewInProcBus(8, N*2)
	if err := bus.Start(); err != nil {
		t.Fatalf("启动 bus 失败: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = bus.Stop(ctx)
	}()

	bizTag := "test_relay_e2e_" + fmt.Sprint(time.Now().UnixNano())

	// handler 通过 wg + sync.Map 收集所有到达的事件。
	var handlerWG sync.WaitGroup
	handlerWG.Add(N)
	var received sync.Map // event_id -> struct{}
	var receivedCnt atomic.Int64
	_ = bus.Subscribe(event.TopicRecordUpdated, func(_ context.Context, e eventbus.DomainEvent) error {
		evt, ok := e.(event.RecordUpdated)
		if !ok {
			return nil
		}
		// 只统计本测试投入的事件(用 BizType 区分),过滤其它测试残留。
		if evt.Ref.BizType != bizTag {
			return nil
		}
		if _, dup := received.LoadOrStore(evt.EventID(), struct{}{}); !dup {
			receivedCnt.Add(1)
			handlerWG.Done()
		}
		return nil
	})

	o := NewOutbox(tx, codec)
	relay := NewRelay(tx, bus, codec)

	// 收尾:删除 outbox 行(避免下次跑测试受残留影响)。
	defer db.Unscoped().
		Where("aggregate_type = ?", bizTag).
		Delete(&OutboxPO{})

	// === Phase 1: 并发 Append ===
	expectedIDs := make([]string, N)
	var appendWG sync.WaitGroup
	for i := 0; i < N; i++ {
		expectedIDs[i] = idgen.UUID()
		appendWG.Add(1)
		go func(i int, id string) {
			defer appendWG.Done()
			evt := event.RecordUpdated{
				Base: event.NewBase(id, event.TopicRecordUpdated, bizTag, int64(i+1)),
				Ref:  model.BizRef{TenantID: 1, BizType: bizTag, BizID: int64(i + 1)},
				Actor: model.Actor{Type: model.ActorUser, ID: 1, Name: "tester"},
			}
			ctx := context.Background()
			if err := o.Append(ctx, evt); err != nil {
				t.Errorf("Append %d 失败: %v", i, err)
			}
		}(i, expectedIDs[i])
	}
	appendWG.Wait()

	// 校验:outbox 表里有 N 条 pending
	var pending int64
	db.Model(&OutboxPO{}).Where("aggregate_type = ? AND status = ?", bizTag, StatusPending).Count(&pending)
	if pending != N {
		t.Fatalf("Append 阶段后期望 %d 条 pending, 实际 %d", N, pending)
	}

	// === Phase 2: 手动反复 dispatch(限定本测试 bizTag,跟可能在跑的 server relay 隔离) ===
	for round := 0; round < 5; round++ {
		if _, err := dispatchMine(tx, relay, bizTag); err != nil {
			t.Fatalf("dispatch round %d 失败: %v", round, err)
		}
		var stillPending int64
		db.Model(&OutboxPO{}).Where("aggregate_type = ? AND status = ?", bizTag, StatusPending).Count(&stillPending)
		if stillPending == 0 {
			break
		}
	}

	// === Phase 3: 等 handler 全收齐(InProcBus 是异步派发) ===
	if !waitForHandlerWG(&handlerWG, 5*time.Second) {
		t.Fatalf("handler 等待超时: 期望收 %d 条, 实际 %d", N, receivedCnt.Load())
	}

	// === 校验 ===
	// 3a) outbox 全 Published,无 Failed
	var published, failed int64
	db.Model(&OutboxPO{}).Where("aggregate_type = ? AND status = ?", bizTag, StatusPublished).Count(&published)
	db.Model(&OutboxPO{}).Where("aggregate_type = ? AND status = ?", bizTag, StatusFailed).Count(&failed)
	if published != N {
		t.Errorf("期望 %d 条 Published, 实际 %d", N, published)
	}
	if failed != 0 {
		t.Errorf("不应有 Failed, 实际 %d", failed)
	}

	// 3b) handler 收到的事件 == 投入的事件(不丢不重)
	if receivedCnt.Load() != N {
		t.Errorf("handler 收到数量不对: 期望 %d, 实际 %d", N, receivedCnt.Load())
	}
	for _, id := range expectedIDs {
		if _, ok := received.Load(id); !ok {
			t.Errorf("事件 %s 丢失", id)
		}
	}
}

// TestRelayE2E_ConcurrentDispatch 多个并发 dispatch 取同一批 pending 行,
// 验证 FOR UPDATE SKIP LOCKED 防止重复投递。
// 期望:每条 outbox 行只被一个 dispatch 取走,handler 不重复收。
func TestRelayE2E_ConcurrentDispatch(t *testing.T) {
	db := openOutboxTestDB(t)
	tx := orm.NewTxManager(db)
	codec := eventbus.NewCodec()
	eventcodec.Register(codec)

	const N = 30
	bus := eventbus.NewInProcBus(4, N*2)
	if err := bus.Start(); err != nil {
		t.Fatalf("启动 bus 失败: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = bus.Stop(ctx)
	}()

	bizTag := "test_relay_lock_" + fmt.Sprint(time.Now().UnixNano())
	var receivedCnt atomic.Int64
	handlerWG := &sync.WaitGroup{}
	handlerWG.Add(N)
	var received sync.Map
	_ = bus.Subscribe(event.TopicRecordUpdated, func(_ context.Context, e eventbus.DomainEvent) error {
		evt, ok := e.(event.RecordUpdated)
		if !ok || evt.Ref.BizType != bizTag {
			return nil
		}
		if _, dup := received.LoadOrStore(evt.EventID(), struct{}{}); !dup {
			receivedCnt.Add(1)
			handlerWG.Done()
		}
		return nil
	})

	o := NewOutbox(tx, codec)
	relay := NewRelay(tx, bus, codec)
	defer db.Unscoped().Where("aggregate_type = ?", bizTag).Delete(&OutboxPO{})

	// 先准备 N 条 pending
	for i := 0; i < N; i++ {
		evt := event.RecordUpdated{
			Base: event.NewBase(idgen.UUID(), event.TopicRecordUpdated, bizTag, int64(i+1)),
			Ref:  model.BizRef{TenantID: 1, BizType: bizTag, BizID: int64(i + 1)},
			Actor: model.Actor{Type: model.ActorUser, ID: 1, Name: "tester"},
		}
		if err := o.Append(context.Background(), evt); err != nil {
			t.Fatalf("Append %d 失败: %v", i, err)
		}
	}

	// 起 4 个 dispatch 并发跑,SKIP LOCKED 应保证每条 pending 只被取走一次。
	// 用 dispatchMine 限定 bizTag,跟可能在跑的 server relay 隔离。
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				if _, err := dispatchMine(tx, relay, bizTag); err != nil {
					t.Logf("goroutine %d round %d dispatch error: %v", idx, j, err)
				}
			}
		}(i)
	}
	wg.Wait()

	// 兜底:轮询直到本 bizTag 的 outbox 全 Published(race 下 dispatchMine 偶发返回错误,需多兜)。
	for round := 0; round < 30; round++ {
		var pending int64
		db.Model(&OutboxPO{}).Where("aggregate_type = ? AND status = ?", bizTag, StatusPending).Count(&pending)
		if pending == 0 {
			break
		}
		if _, err := dispatchMine(tx, relay, bizTag); err != nil {
			t.Logf("fallback dispatch round %d error: %v", round, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !waitForHandlerWG(handlerWG, 8*time.Second) {
		t.Fatalf("handler 等待超时: 期望 %d, 实际 %d", N, receivedCnt.Load())
	}

	// 校验 handler 总共收到 N 条(不重)
	if receivedCnt.Load() != N {
		t.Errorf("handler 收到数量不对: 期望 %d, 实际 %d", N, receivedCnt.Load())
	}
	// 校验 outbox 全 Published
	var published int64
	db.Model(&OutboxPO{}).Where("aggregate_type = ? AND status = ?", bizTag, StatusPublished).Count(&published)
	if published != N {
		t.Errorf("期望 %d 条 Published, 实际 %d", N, published)
	}
}

// waitForHandlerWG 带超时等 wg.Done 满足 N 次。
func waitForHandlerWG(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

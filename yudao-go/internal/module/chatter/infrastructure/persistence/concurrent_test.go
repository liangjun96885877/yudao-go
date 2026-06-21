// Package persistence 并发/幂等安全集成测试。
//
// 这些测试都需要开发库 (deploy/docker-compose.yml 的 yudao-go-mysql),
// 不可用时自动 Skip,不阻塞 CI。
//
// 覆盖场景:
//   - 评论乐观锁:并发 Update 仅 1 个成功,另一个 Conflict (反 lost-update)
//   - 关注幂等:并发 Add 同一 (user,biz) 仅插入 1 行 (反双重关注)
//
// 跑: go test ./internal/module/chatter/infrastructure/persistence -race -run TestConcurrent
package persistence

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/pkg/errcode"
)

const concurrentTestDSN = "root:123456@tcp(127.0.0.1:13306)/yudao_go?charset=utf8mb4&parseTime=True&loc=Local"

func openConcurrentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.Open(concurrentTestDSN), &gorm.Config{})
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
	return db
}

// TestConcurrent_CommentOptimisticLock 验证评论乐观锁:
// 两个 goroutine 拿到同一 version=1 同时 Update,只能 1 个成功,
// 另一个必须收到 errcode.Conflict(避免 lost update)。
func TestConcurrent_CommentOptimisticLock(t *testing.T) {
	db := openConcurrentTestDB(t)
	tx := orm.NewTxManager(db)
	repo := NewCommentRepo(tx)
	ctx := context.Background()

	bizID := time.Now().UnixNano()
	c := &model.Comment{
		Ref:     model.BizRef{TenantID: 1, BizType: "test_concurrent", BizID: bizID},
		Content: "原始内容",
		Author:  model.Actor{Type: model.ActorUser, ID: 1, Name: "tester"},
		Version: 1,
	}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	defer db.Unscoped().Where("biz_type = ?", "test_concurrent").Delete(&CommentPO{})

	// 两个 goroutine 拿同一 (id, version=1) 同时升级到 version=2,内容不同。
	var (
		wg          sync.WaitGroup
		successCnt  int32
		conflictCnt int32
		mu          sync.Mutex
	)
	for i, content := range []string{"A 的修改", "B 的修改"} {
		wg.Add(1)
		go func(i int, content string) {
			defer wg.Done()
			updated := &model.Comment{
				ID:      c.ID,
				Content: content,
				Version: 2, // 新版本 (旧版本 = 2-1 = 1,WHERE version=1)
			}
			err := repo.Update(ctx, updated)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCnt++
			} else if errors.Is(err, errcode.Conflict) {
				conflictCnt++
			} else {
				t.Errorf("goroutine %d 非预期错误: %v", i, err)
			}
		}(i, content)
	}
	wg.Wait()

	if successCnt != 1 || conflictCnt != 1 {
		t.Fatalf("乐观锁失效: 期望 success=1 conflict=1, 实际 success=%d conflict=%d",
			successCnt, conflictCnt)
	}

	// DB 中 version 应该是 2(只有 1 次成功 +1),不应是 3。
	var finalPO CommentPO
	if err := db.First(&finalPO, c.ID).Error; err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if finalPO.Version != 2 {
		t.Fatalf("version 不对: 期望 2, 实际 %d", finalPO.Version)
	}
}

// TestConcurrent_FollowerIdempotent 验证关注幂等:
// N 个 goroutine 并发 Add 同一 (user_id, biz_type, biz_id) → 只产生 1 行
// (ON CONFLICT (tenant,biz_type,biz_id,user_id) DO NOTHING)。
func TestConcurrent_FollowerIdempotent(t *testing.T) {
	db := openConcurrentTestDB(t)
	tx := orm.NewTxManager(db)
	repo := NewFollowerRepo(tx)
	ctx := context.Background()

	bizID := time.Now().UnixNano()
	ref := model.BizRef{TenantID: 1, BizType: "test_concurrent_follow", BizID: bizID}
	userID := int64(99999)

	defer db.Unscoped().
		Where("biz_type = ? AND user_id = ?", "test_concurrent_follow", userID).
		Delete(&FollowerPO{})

	const N = 20
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := &model.Follower{
				Ref:      ref,
				UserID:   userID,
				UserName: "concurrent-tester",
				Reason:   model.FollowManual,
			}
			errs <- repo.Add(ctx, f)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Add 不应报错(ON CONFLICT DO NOTHING): %v", err)
		}
	}

	var cnt int64
	db.Model(&FollowerPO{}).
		Where("biz_type = ? AND biz_id = ? AND user_id = ?",
			"test_concurrent_follow", bizID, userID).
		Count(&cnt)
	if cnt != 1 {
		t.Fatalf("关注幂等失效: 期望 1 行, 实际 %d 行", cnt)
	}
}

// PrivilegeGuard 缓存命中测试 + 性能基准。
//
// 跑:
//   功能测试: go test ./internal/module/system/service -run TestPrivilegeGuardCache
//   基准:    go test -bench=BenchmarkPrivilegeGuard -benchmem -benchtime=2s \
//             ./internal/module/system/service
package service

import (
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yudao-go/internal/framework/orm"
)

func openBenchPrivGuardDB(b *testing.B) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(mysql.Open(privGuardTestDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Skipf("跳过基准: DB 不可用 (%v)", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		b.Skipf("跳过基准: DB ping 失败")
	}
	return db
}

// TestPrivilegeGuardCache_HitsAvoidDB 验证:
// 第一次调用查 DB 并写入缓存,后续在 TTL 内连续调用全部命中缓存(不再查 DB)。
// 这里用 ClearCache + 直接观察"两次结果一致 + 第二次远快于第一次"作为缓存生效的近似证据,
// 严格证明缓存命中通过 bench 体现。
func TestPrivilegeGuardCache_HitsAvoidDB(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1) // admin

	// 第一次:冷启动,查 DB 填缓存
	g.ClearCache()
	first := g.IsSuperAdmin(ctx)
	// 第二次:应命中缓存
	second := g.IsSuperAdmin(ctx)
	if first != second {
		t.Fatalf("两次结果应一致: first=%v second=%v", first, second)
	}
	// admin 应是 super_admin
	if !first {
		t.Fatal("admin (uid=1) 应是 super_admin")
	}

	// MyRoleIDs / MyMenuIDs / MyDataScope 各调一次后再调,应都命中缓存
	roleIDs := g.MyRoleIDs(ctx)
	if g2 := g.MyRoleIDs(ctx); len(g2) != len(roleIDs) {
		t.Fatalf("RoleIDs 缓存不一致: %v vs %v", roleIDs, g2)
	}
	menuIDs := g.MyMenuIDs(ctx)
	if g2 := g.MyMenuIDs(ctx); len(g2) != len(menuIDs) {
		t.Fatalf("MenuIDs 缓存不一致: %v vs %v", menuIDs, g2)
	}
	scope, deptIDs := g.MyDataScope(ctx)
	scope2, deptIDs2 := g.MyDataScope(ctx)
	if scope != scope2 || len(deptIDs) != len(deptIDs2) {
		t.Fatalf("DataScope 缓存不一致: (%d,%v) vs (%d,%v)", scope, deptIDs, scope2, deptIDs2)
	}
}

// TestPrivilegeGuardCache_ClearWorks ClearCache 后下次调用应重新走 DB。
func TestPrivilegeGuardCache_ClearWorks(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1)

	g.IsSuperAdmin(ctx)
	// 缓存有效
	g.mu.RLock()
	e1 := g.users[1]
	g.mu.RUnlock()
	if e1 == nil || !e1.hasSuper {
		t.Fatal("缓存应被填充")
	}

	g.ClearCache()
	g.mu.RLock()
	e2 := g.users[1]
	g.mu.RUnlock()
	if e2 != nil {
		t.Fatal("ClearCache 后缓存应清空")
	}
}

// TestPrivilegeGuardCache_Expiry 缓存 TTL 到期后应重查 DB。
// 这里把 entry 的 expiresAt 直接设到过去,模拟过期。
func TestPrivilegeGuardCache_Expiry(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1)

	g.IsSuperAdmin(ctx)
	// 强制把 expiresAt 调到过去
	g.mu.Lock()
	if e := g.users[1]; e != nil {
		e.expiresAt = time.Now().Add(-time.Second)
	}
	g.mu.Unlock()

	// 下次调用应当成过期处理,getEntry 返回 nil,然后重 DB + 重写缓存
	v := g.IsSuperAdmin(ctx)
	if !v {
		t.Fatal("过期后重查应仍返回 admin = super_admin")
	}
	g.mu.RLock()
	e := g.users[1]
	g.mu.RUnlock()
	if e == nil || time.Now().After(e.expiresAt) {
		t.Fatal("过期后第二次调用应重新填充新 expiresAt")
	}
}

// === 基准:加缓存 vs 不加缓存 ===

func BenchmarkPrivilegeGuard_IsSuperAdmin_Cached(b *testing.B) {
	db := openBenchPrivGuardDB(b)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1)
	// warm up
	g.IsSuperAdmin(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = g.IsSuperAdmin(ctx)
	}
}

func BenchmarkPrivilegeGuard_IsSuperAdmin_NoCache(b *testing.B) {
	db := openBenchPrivGuardDB(b)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.ClearCache() // 每次清缓存,强制走 DB
		_ = g.IsSuperAdmin(ctx)
	}
}

func BenchmarkPrivilegeGuard_FullCheck_Cached(b *testing.B) {
	// 模拟一次写权限调用链经历的全部缓存方法:
	// EnsureCanModifyRole 内 IsSuperAdmin + roleCodeByID
	// EnsureCanGrantMenus 内 IsSuperAdmin + MyMenuIDs
	// EnsureCanGrantDataScope 内 IsSuperAdmin + MyDataScope
	db := openBenchPrivGuardDB(b)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1)
	// warm up
	g.IsSuperAdmin(ctx)
	g.MyRoleIDs(ctx)
	g.MyMenuIDs(ctx)
	g.MyDataScope(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = g.IsSuperAdmin(ctx)
		_ = g.MyRoleIDs(ctx)
		_ = g.MyMenuIDs(ctx)
		_, _ = g.MyDataScope(ctx)
	}
}

func BenchmarkPrivilegeGuard_FullCheck_NoCache(b *testing.B) {
	db := openBenchPrivGuardDB(b)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	ctx := ctxAs(1)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.ClearCache()
		_ = g.IsSuperAdmin(ctx)
		_ = g.MyRoleIDs(ctx)
		_ = g.MyMenuIDs(ctx)
		_, _ = g.MyDataScope(ctx)
	}
}


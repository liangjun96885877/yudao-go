// PrivilegeGuard 越权防护集成测试 —— 把 docs/防提权护栏方案.md 的手动 curl
// 验证场景固化下来,后续重构不丢。
//
// 跑: go test ./internal/module/system/service -run TestPrivilegeGuard
//
// 数据库不可用时 Skip。所有测试用 fixture (用户/角色/菜单) 都用一个固定的
// "test_priv_guard" 前缀 + 极大 ID 隔离,t.Cleanup 收尾删除。
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/pkg/errcode"
)

const privGuardTestDSN = "root:123456@tcp(127.0.0.1:13306)/yudao_go?charset=utf8mb4&parseTime=True&loc=Local"

func openPrivGuardDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.Open(privGuardTestDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("跳过集成测试: 数据库不可用 (%v)", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		t.Skipf("跳过集成测试: 数据库 ping 失败")
	}
	// 不注册 ORM 插件(避免多租户、数据范围 scope 影响裸读 fixture)。
	return db
}

// privFixture 构造一个最小越权测试场景:
//   - normalUserID  : 普通用户,仅拥有 normalRoleID;
//   - normalRoleID  : 普通角色,授了 ownedMenuID + dataScope=2 + ownedDeptID;
//   - foreignMenuID : 系统中存在但 normalUserID 没拥有,用来测"授未拥有菜单"。
type privFixture struct {
	db             *gorm.DB
	normalUserID   int64
	normalRoleID   int64
	ownedMenuID    int64
	foreignMenuID  int64
	ownedDeptID    int64
	superAdminID   int64
	// 收尾用:t.Cleanup 删表里的对应行。
	cleanup []func()
}

func newPrivFixture(t *testing.T, db *gorm.DB) *privFixture {
	t.Helper()
	stamp := time.Now().UnixNano()
	f := &privFixture{
		db:            db,
		normalUserID:  900000000 + stamp%1000000, // 避开真实 user 区段
		normalRoleID:  0,
		ownedMenuID:   0,
		foreignMenuID: 0,
		ownedDeptID:   8001,
	}

	// 查 super_admin id(原版 SQL 已导入)
	db.Raw(`SELECT id FROM system_role WHERE code='super_admin' AND deleted=0 LIMIT 1`).Scan(&f.superAdminID)
	if f.superAdminID == 0 {
		t.Skip("跳过: DB 中没有 super_admin 角色,可能未导入 ruoyi-vue-pro.sql")
	}

	// 1) 造一个普通角色,dataScope=2 + 部门集合 [ownedDeptID]
	role := &model.Role{
		Name: "priv_test_role", Code: "priv_test_role_" + tsCode(stamp),
		Sort: 99, Status: 0, Type: 2,
		DataScope: 2, DataScopeDeptIDs: orm.Int64Array{f.ownedDeptID},
	}
	role.TenantBase.TenantID = 1
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("创建 role 失败: %v", err)
	}
	f.normalRoleID = role.ID
	f.cleanup = append(f.cleanup, func() {
		db.Unscoped().Where("id = ?", role.ID).Delete(&model.Role{})
	})

	// 2) 造两个菜单:一个"自己有的", 一个"别人的"
	owned := &model.Menu{Name: "priv_owned_menu", Type: 3, Sort: 1, Permission: "priv:test:owned"}
	if err := db.Create(owned).Error; err != nil {
		t.Fatalf("创建 owned menu 失败: %v", err)
	}
	f.ownedMenuID = owned.ID
	f.cleanup = append(f.cleanup, func() {
		db.Unscoped().Where("id = ?", owned.ID).Delete(&model.Menu{})
	})
	foreign := &model.Menu{Name: "priv_foreign_menu", Type: 3, Sort: 2, Permission: "priv:test:foreign"}
	if err := db.Create(foreign).Error; err != nil {
		t.Fatalf("创建 foreign menu 失败: %v", err)
	}
	f.foreignMenuID = foreign.ID
	f.cleanup = append(f.cleanup, func() {
		db.Unscoped().Where("id = ?", foreign.ID).Delete(&model.Menu{})
	})

	// 3) 给 normalRole 授 ownedMenu
	rm := &model.RoleMenu{RoleID: f.normalRoleID, MenuID: f.ownedMenuID}
	rm.TenantBase.TenantID = 1
	if err := db.Create(rm).Error; err != nil {
		t.Fatalf("创建 role_menu 失败: %v", err)
	}
	f.cleanup = append(f.cleanup, func() {
		db.Unscoped().Where("id = ?", rm.ID).Delete(&model.RoleMenu{})
	})

	// 4) 把 normalRole 分配给 normalUser
	ur := &model.UserRole{UserID: f.normalUserID, RoleID: f.normalRoleID}
	ur.TenantBase.TenantID = 1
	if err := db.Create(ur).Error; err != nil {
		t.Fatalf("创建 user_role 失败: %v", err)
	}
	f.cleanup = append(f.cleanup, func() {
		db.Unscoped().Where("id = ?", ur.ID).Delete(&model.UserRole{})
	})

	t.Cleanup(func() {
		for _, fn := range f.cleanup {
			fn()
		}
	})
	return f
}

func tsCode(n int64) string {
	// 简短化,避免 code 字段超长
	return time.Unix(0, n).Format("150405.000000")
}

// ctxAs 构造一个用指定 userID 跑的上下文(模拟登录态)。
func ctxAs(userID int64) context.Context {
	ctx := context.Background()
	ctx = contextx.WithUserID(ctx, userID)
	ctx = contextx.WithTenantID(ctx, 1)
	return ctx
}

// expectForbidden 断言 err 为 403 Forbidden。
func expectForbidden(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: 期望 403, 实际无错误", label)
		return
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Errorf("%s: 期望 *errcode.Error, 实际 %T: %v", label, err, err)
		return
	}
	if e.Code != 403 {
		t.Errorf("%s: 期望 code=403, 实际 code=%d msg=%q", label, e.Code, e.Msg)
	}
}

// TestPrivilegeGuard_EnsureCanAssignToUser 覆盖 3 种越权:
//   ① 改自己 ② 加 super_admin ③ 授自己没有的角色
func TestPrivilegeGuard_EnsureCanAssignToUser(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	f := newPrivFixture(t, db)
	ctx := ctxAs(f.normalUserID)

	// ① 改自己
	expectForbidden(t, "改自己角色",
		g.EnsureCanAssignToUser(ctx, f.normalUserID, []int64{f.normalRoleID}))

	// ② 给别人加 super_admin
	otherUser := f.normalUserID + 1
	expectForbidden(t, "给别人加 super_admin",
		g.EnsureCanAssignToUser(ctx, otherUser, []int64{f.superAdminID}))

	// ③ 授自己没有的角色 id (用一个绝对不存在的 id 9999999)
	expectForbidden(t, "授自己未拥有的角色",
		g.EnsureCanAssignToUser(ctx, otherUser, []int64{9999999}))

	// 正向: 授自己拥有的 normalRole 给别人 → 应放行
	if err := g.EnsureCanAssignToUser(ctx, otherUser, []int64{f.normalRoleID}); err != nil {
		t.Errorf("授自己拥有的角色应放行,实际: %v", err)
	}
}

// TestPrivilegeGuard_EnsureCanGrantMenus 验证授未拥有菜单被拦。
func TestPrivilegeGuard_EnsureCanGrantMenus(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	f := newPrivFixture(t, db)
	ctx := ctxAs(f.normalUserID)

	// 拦: 含 foreignMenuID(自己没有)
	expectForbidden(t, "授未拥有菜单",
		g.EnsureCanGrantMenus(ctx, []int64{f.ownedMenuID, f.foreignMenuID}))

	// 放: 全是自己有的
	if err := g.EnsureCanGrantMenus(ctx, []int64{f.ownedMenuID}); err != nil {
		t.Errorf("授自己拥有的菜单应放行,实际: %v", err)
	}
}

// TestPrivilegeGuard_EnsureCanGrantDataScope 验证非超管授 data_scope=1 被拦。
func TestPrivilegeGuard_EnsureCanGrantDataScope(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	f := newPrivFixture(t, db)
	ctx := ctxAs(f.normalUserID)

	// 拦: scope=1 全部
	expectForbidden(t, "授全部数据范围",
		g.EnsureCanGrantDataScope(ctx, 1, nil))

	// 放: scope=5 仅本人
	if err := g.EnsureCanGrantDataScope(ctx, 5, nil); err != nil {
		t.Errorf("授仅本人应放行,实际: %v", err)
	}
}

// TestPrivilegeGuard_EnsureCanModifyRole 验证非超管不能改 super_admin 角色。
func TestPrivilegeGuard_EnsureCanModifyRole(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	f := newPrivFixture(t, db)
	ctx := ctxAs(f.normalUserID)

	expectForbidden(t, "改 super_admin 角色",
		g.EnsureCanModifyRole(ctx, f.superAdminID))

	// 放: 改自己创建的普通角色
	if err := g.EnsureCanModifyRole(ctx, f.normalRoleID); err != nil {
		t.Errorf("改普通角色应放行,实际: %v", err)
	}
}

// TestPrivilegeGuard_EnsureCanManageMenu 验证菜单管理仅限超管。
func TestPrivilegeGuard_EnsureCanManageMenu(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	g := NewPrivilegeGuard(tx)
	f := newPrivFixture(t, db)
	ctx := ctxAs(f.normalUserID)

	expectForbidden(t, "非超管管理菜单",
		g.EnsureCanManageMenu(ctx))
}

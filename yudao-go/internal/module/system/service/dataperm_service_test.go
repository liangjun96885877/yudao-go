// 数据范围(data permission)正确性集成测试。
//
// 覆盖:
//   - 多角色并集:本部门 + 指定部门集合 → DeptIDs 取并集去重
//   - scope=4 子树 BFS:用户 dept 是 root,DeptIDs 含全部后代
//   - All 短路:任一角色为「全部」或 super_admin → All=true,DeptIDs 空
//   - GORM 插件 SQL 拼接:DeptIDs + SelfOnly 双条件用 OR 合并
//   - 无任何范围保护:DataPerm 全空 → 注入 `1 = 0` 防漏读
//
// 跑: go test ./internal/module/system/service -run TestDataPerm
package service

import (
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/system/model"
)

// dataPermFixture 造一个最小可测的部门树 + 角色 + 用户 fixture。
//
// 部门树:
//
//	rootDept (id=8200)
//	├── childA (id=8201)
//	│   └── grandchild (id=8203)
//	└── childB (id=8202)
//
// 用户挂在 childA;另一个独立部门 foreignDept (id=8210) 不在子树里。
type dataPermFixture struct {
	db             *gorm.DB
	userID         int64
	userDeptID     int64 // childA
	rootDeptID     int64
	childADeptID   int64
	childBDeptID   int64
	grandchildID   int64
	foreignDeptID  int64
	roleAllID      int64 // scope=1 全部
	roleDeptListID int64 // scope=2 指定部门 [foreignDeptID]
	roleSelfDeptID int64 // scope=3 本部门
	roleSubtreeID  int64 // scope=4 本部门及以下
	roleSelfOnlyID int64 // scope=5 仅本人
	cleanup        []func()
}

func newDataPermFixture(t *testing.T, db *gorm.DB) *dataPermFixture {
	t.Helper()
	stamp := time.Now().UnixNano()
	f := &dataPermFixture{db: db, userID: 940000000 + stamp%1000000}

	// === 部门树 ===
	makeDept := func(name string, parent, id int64) int64 {
		d := &model.Dept{Name: name, ParentID: parent, Sort: 1, Status: 0}
		d.TenantBase.TenantID = 1
		if id != 0 {
			d.ID = id
		}
		if err := db.Create(d).Error; err != nil {
			t.Fatalf("create dept %q: %v", name, err)
		}
		f.cleanup = append(f.cleanup, func() {
			db.Unscoped().Where("id = ?", d.ID).Delete(&model.Dept{})
		})
		return d.ID
	}
	f.rootDeptID = makeDept("dp_root", 0, 0)
	f.childADeptID = makeDept("dp_childA", f.rootDeptID, 0)
	f.childBDeptID = makeDept("dp_childB", f.rootDeptID, 0)
	f.grandchildID = makeDept("dp_grandchild", f.childADeptID, 0)
	f.foreignDeptID = makeDept("dp_foreign", 0, 0)
	f.userDeptID = f.childADeptID

	// === 用户 (落在 system_users,只为 Resolve 能查到 dept_id) ===
	// 不依赖 model.User 整套(避免必填字段),裸 SQL 插。nickname 是 NOT NULL 无默认。
	uname := "dp_test_user_" + tsCode(stamp)
	res := db.Exec(`INSERT INTO system_users(tenant_id, dept_id, username, nickname, password, status, deleted, creator, updater, create_time, update_time)
		VALUES(?, ?, ?, ?, '', 0, 0, '0', '0', NOW(), NOW())`,
		1, f.userDeptID, uname, uname)
	if res.Error != nil {
		t.Fatalf("create user: %v", res.Error)
	}
	// 取刚插入 user_id
	db.Raw("SELECT id FROM system_users WHERE username = ? LIMIT 1", uname).Scan(&f.userID)
	f.cleanup = append(f.cleanup, func() {
		db.Exec("DELETE FROM system_users WHERE id = ?", f.userID)
	})

	// === 5 个角色 ===
	makeRole := func(name string, scope int8, deptIDsJSON string) int64 {
		var arr orm.Int64Array
		if deptIDsJSON != "" {
			_ = arr.Scan(deptIDsJSON)
		}
		r := &model.Role{
			Name: name, Code: name + "_" + tsCode(stamp),
			Sort: 1, Status: 0, Type: 2,
			DataScope: scope, DataScopeDeptIDs: arr,
		}
		r.TenantBase.TenantID = 1
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("create role %q: %v", name, err)
		}
		f.cleanup = append(f.cleanup, func() {
			db.Unscoped().Where("id = ?", r.ID).Delete(&model.Role{})
		})
		return r.ID
	}
	f.roleAllID = makeRole("dp_role_all", 1, "")
	f.roleDeptListID = makeRole("dp_role_list", 2, jsonArr(f.foreignDeptID))
	f.roleSelfDeptID = makeRole("dp_role_dept", 3, "")
	f.roleSubtreeID = makeRole("dp_role_subtree", 4, "")
	f.roleSelfOnlyID = makeRole("dp_role_self", 5, "")

	t.Cleanup(func() {
		// 反向清理,先清 user_role 再清 fixture(role/dept 用 ON DELETE NO ACTION,只能手动)
		db.Exec("DELETE FROM system_user_role WHERE user_id = ?", f.userID)
		for i := len(f.cleanup) - 1; i >= 0; i-- {
			f.cleanup[i]()
		}
	})
	return f
}

// jsonArr 生成 [a,b,c] 形式的 JSON 字符串。
func jsonArr(ids ...int64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(itoa64(id))
	}
	b.WriteByte(']')
	return b.String()
}
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = '0' + byte(n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// 把指定角色绑到 fixture 的用户上,t.Cleanup 自动清。
func (f *dataPermFixture) assignRoles(t *testing.T, roleIDs ...int64) {
	t.Helper()
	for _, rid := range roleIDs {
		ur := &model.UserRole{UserID: f.userID, RoleID: rid}
		ur.TenantBase.TenantID = 1
		if err := f.db.Create(ur).Error; err != nil {
			t.Fatalf("assign role %d: %v", rid, err)
		}
		t.Cleanup(func() {
			f.db.Unscoped().Where("id = ?", ur.ID).Delete(&model.UserRole{})
		})
	}
}

func int64Set(arr []int64) map[int64]bool {
	m := make(map[int64]bool, len(arr))
	for _, v := range arr {
		m[v] = true
	}
	return m
}

func sortedInt64(arr []int64) []int64 {
	out := make([]int64, len(arr))
	copy(out, arr)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// openDataPermPluginDB 与 openPrivGuardDB 同样的 DSN,但**注册了完整 ORM 插件**,
// 用于验证 dataperm 插件的 SQL 拼接行为。
func openDataPermPluginDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openPrivGuardDB(t) // 复用 connection (skip 逻辑一致)
	if err := orm.RegisterPlugins(db); err != nil {
		t.Fatalf("注册 ORM 插件失败: %v", err)
	}
	return db
}

// === 测试用例 ===

// TestDataPerm_Resolve_MultiRoleUnion 多角色取并集:本部门(scope=3) + 指定部门(scope=2)
// → DeptIDs = {userDept(childA), foreignDept}。
func TestDataPerm_Resolve_MultiRoleUnion(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newDataPermFixture(t, db)
	f.assignRoles(t, f.roleSelfDeptID, f.roleDeptListID)

	svc := NewDataPermService(tx)
	dp, err := svc.Resolve(ctxAs(f.userID), f.userID)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if dp.All {
		t.Fatalf("不应该 All=true")
	}
	got := int64Set(dp.DeptIDs)
	if len(got) != 2 || !got[f.userDeptID] || !got[f.foreignDeptID] {
		t.Fatalf("DeptIDs 并集错误: 期望 {%d, %d}, 实际 %v",
			f.userDeptID, f.foreignDeptID, sortedInt64(dp.DeptIDs))
	}
}

// TestDataPerm_Resolve_SubtreeBFS scope=4 应展开后代:
// 用户在 childA → DeptIDs 应含 childA 及其孙 grandchild。
func TestDataPerm_Resolve_SubtreeBFS(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newDataPermFixture(t, db)
	f.assignRoles(t, f.roleSubtreeID)

	svc := NewDataPermService(tx)
	dp, err := svc.Resolve(ctxAs(f.userID), f.userID)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	got := int64Set(dp.DeptIDs)
	if !got[f.childADeptID] || !got[f.grandchildID] {
		t.Fatalf("子树未展开: 期望含 childA(%d) + grandchild(%d), 实际 %v",
			f.childADeptID, f.grandchildID, sortedInt64(dp.DeptIDs))
	}
	if got[f.childBDeptID] {
		t.Fatalf("不应包含兄弟节点 childB(%d): %v", f.childBDeptID, sortedInt64(dp.DeptIDs))
	}
	if got[f.foreignDeptID] {
		t.Fatalf("不应包含无关部门 foreign(%d): %v", f.foreignDeptID, sortedInt64(dp.DeptIDs))
	}
}

// TestDataPerm_Resolve_AllShortcut 任一角色为「全部」→ All=true 立即短路。
func TestDataPerm_Resolve_AllShortcut(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newDataPermFixture(t, db)
	// 同时挂 scope=4 + scope=1,scope=1 应短路,DeptIDs 空。
	f.assignRoles(t, f.roleSubtreeID, f.roleAllID)

	svc := NewDataPermService(tx)
	dp, err := svc.Resolve(ctxAs(f.userID), f.userID)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if !dp.All {
		t.Fatalf("scope=1 应短路设 All=true, 实际 %+v", dp)
	}
	if len(dp.DeptIDs) != 0 {
		t.Fatalf("All 时 DeptIDs 应为空, 实际 %v", dp.DeptIDs)
	}
}

// TestDataPerm_Resolve_SelfOnly scope=5 → SelfOnly=true, DeptIDs 空。
func TestDataPerm_Resolve_SelfOnly(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newDataPermFixture(t, db)
	f.assignRoles(t, f.roleSelfOnlyID)

	svc := NewDataPermService(tx)
	dp, err := svc.Resolve(ctxAs(f.userID), f.userID)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if dp.All {
		t.Fatalf("scope=5 不应 All=true")
	}
	if !dp.SelfOnly {
		t.Fatalf("scope=5 应 SelfOnly=true")
	}
	if len(dp.DeptIDs) != 0 {
		t.Fatalf("仅本人时 DeptIDs 应为空, 实际 %v", dp.DeptIDs)
	}
}

// TestDataPerm_PluginSQL_DeptAndSelf 验证 GORM 插件 SQL 拼接:
// DeptIDs=[10,20] + SelfOnly=true → WHERE (dept_id IN (10,20) OR creator = '99')
// 用 DryRun 抓 SQL 而非真实插数据,避免污染。
func TestDataPerm_PluginSQL_DeptAndSelf(t *testing.T) {
	db := openDataPermPluginDB(t)
	ctx := ctxAs(99)
	ctx = contextx.WithDataPerm(ctx, &contextx.DataPerm{
		DeptIDs: []int64{10, 20}, SelfOnly: true, UserID: 99,
	})

	// system_users 有 dept_id + creator 两列,目标条件应都生效。
	sess := db.Session(&gorm.Session{DryRun: true}).WithContext(ctx)
	stmt := sess.Model(&model.User{}).Where("status = 0").Find(&[]model.User{}).Statement
	sql := stmt.SQL.String()

	if !strings.Contains(sql, "dept_id") || !strings.Contains(sql, "creator") {
		t.Fatalf("SQL 缺 dept_id 或 creator 条件: %s", sql)
	}
	if !strings.Contains(sql, " OR ") {
		t.Fatalf("SQL 缺 OR 拼接 (DeptIDs + SelfOnly 应 OR 合并): %s", sql)
	}
}

// TestDataPerm_PluginSQL_EmptyPermShortcut DeptIDs 空 + SelfOnly 假 → 注入 `1 = 0`,
// 这是兜底安全保护:数据权限解析失败/为空时不能漏读全表。
func TestDataPerm_PluginSQL_EmptyPermShortcut(t *testing.T) {
	db := openDataPermPluginDB(t)
	ctx := ctxAs(99)
	ctx = contextx.WithDataPerm(ctx, &contextx.DataPerm{UserID: 99}) // 全空

	sess := db.Session(&gorm.Session{DryRun: true}).WithContext(ctx)
	stmt := sess.Model(&model.User{}).Find(&[]model.User{}).Statement
	sql := stmt.SQL.String()

	if !strings.Contains(sql, "1 = 0") {
		t.Fatalf("空权限应注入 `1 = 0` 保护, 实际 SQL: %s", sql)
	}
}

// TestDataPerm_PluginSQL_AllSkipped All=true 应完全不追加 WHERE 子句。
func TestDataPerm_PluginSQL_AllSkipped(t *testing.T) {
	db := openDataPermPluginDB(t)
	ctx := ctxAs(99)
	ctx = contextx.WithDataPerm(ctx, &contextx.DataPerm{All: true, UserID: 99})

	sess := db.Session(&gorm.Session{DryRun: true}).WithContext(ctx)
	stmt := sess.Model(&model.User{}).Find(&[]model.User{}).Statement
	sql := stmt.SQL.String()

	if strings.Contains(sql, "1 = 0") {
		t.Fatalf("All 时不应注入 `1 = 0`: %s", sql)
	}
	// 不严格断言"无 dept_id 条件"(SELECT * 自然会出现 dept_id 列),只确认无 OR 拼接的过滤子句。
	if strings.Contains(sql, "(dept_id IN") || strings.Contains(sql, "(`dept_id` IN") {
		t.Fatalf("All 时不应追加 dept_id 过滤: %s", sql)
	}
}

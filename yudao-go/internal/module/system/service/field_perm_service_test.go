// 字段权限 FieldPermService 集成测试 —— 关键场景:
//   - Resolve 多角色取最宽松 (plain > mask > hide)
//   - Save 全量重写,默认 mask 不落库
//   - ListByRole 回显配置
//
// 跑: go test ./internal/module/system/service -run TestFieldPermService
package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/system/model"
)

// fieldPermFixture:1 个用户 + 2 个角色,每个角色给同一字段不同 action。
type fieldPermFixture struct {
	db      *gorm.DB
	userID  int64
	roleA   int64 // 给 system_user:mobile 配 hide
	roleB   int64 // 给 system_user:mobile 配 plain;给 system_user:email 配 mask(不落库)
	cleanup []func()
}

func newFieldPermFixture(t *testing.T, db *gorm.DB) *fieldPermFixture {
	t.Helper()
	stamp := time.Now().UnixNano()
	suf := tsCode(stamp)
	f := &fieldPermFixture{db: db, userID: 920000000 + stamp%1000000}

	makeRole := func(name string) int64 {
		r := &model.Role{Name: name, Code: name + "_" + suf, Sort: 1, Status: 0, Type: 2}
		r.TenantBase.TenantID = 1
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("create role %s: %v", name, err)
		}
		f.cleanup = append(f.cleanup, func() {
			db.Unscoped().Where("id = ?", r.ID).Delete(&model.Role{})
		})
		return r.ID
	}
	f.roleA = makeRole("fp_test_roleA")
	f.roleB = makeRole("fp_test_roleB")

	// user 分别挂两个角色
	for _, rid := range []int64{f.roleA, f.roleB} {
		ur := &model.UserRole{UserID: f.userID, RoleID: rid}
		ur.TenantBase.TenantID = 1
		if err := db.Create(ur).Error; err != nil {
			t.Fatalf("create user_role: %v", err)
		}
		urID := ur.ID
		f.cleanup = append(f.cleanup, func() {
			db.Unscoped().Where("id = ?", urID).Delete(&model.UserRole{})
		})
	}

	t.Cleanup(func() {
		// 清残留的 role_field_perm
		db.Unscoped().Where("role_id IN ?", []int64{f.roleA, f.roleB}).
			Delete(&model.RoleFieldPerm{})
		for i := len(f.cleanup) - 1; i >= 0; i-- {
			f.cleanup[i]()
		}
	})
	return f
}

// TestFieldPermService_Resolve_MaxAction 多角色合并:同一字段不同 action 取最宽松。
//   - roleA:mobile=hide
//   - roleB:mobile=plain
//   - 期望:Resolve 后 mobile=plain (plain > hide)
func TestFieldPermService_Resolve_MaxAction(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newFieldPermFixture(t, db)
	svc := NewFieldPermService(tx)

	// 写两条规则:同一字段不同 action
	for _, row := range []model.RoleFieldPerm{
		{RoleID: f.roleA, BizType: "system_user", Field: "mobile", Action: "hide"},
		{RoleID: f.roleB, BizType: "system_user", Field: "mobile", Action: "plain"},
		// 额外加一条:roleB 给 email 配 hide(单角色,验证落库即生效)
		{RoleID: f.roleB, BizType: "system_user", Field: "email", Action: "hide"},
	} {
		row.TenantBase.TenantID = 1
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create field_perm: %v", err)
		}
	}

	fp, err := svc.Resolve(ctxAs(f.userID), f.userID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := fp.Action("system_user", "mobile"); got != "plain" {
		t.Errorf("mobile 多角色合并应 plain (>hide), 实际 %q", got)
	}
	if got := fp.Action("system_user", "email"); got != "hide" {
		t.Errorf("email 应 hide(单角色), 实际 %q", got)
	}
	// 未配置的字段 → 默认 mask
	if got := fp.Action("system_user", "idcard"); got != "mask" {
		t.Errorf("未配置字段应默认 mask, 实际 %q", got)
	}
}

// TestFieldPermService_Save_MaskNotPersisted Save 全量重写,
// action="" 或 "mask" 视为默认不落库(节省存储)。
func TestFieldPermService_Save_MaskNotPersisted(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newFieldPermFixture(t, db)
	svc := NewFieldPermService(tx)

	items := []model.RoleFieldPerm{
		{BizType: "system_user", Field: "mobile", Action: "plain"}, // 落
		{BizType: "system_user", Field: "email", Action: "hide"},   // 落
		{BizType: "system_user", Field: "idcard", Action: "mask"},  // 默认,不落
		{BizType: "system_user", Field: "remark", Action: ""},      // 空,不落
	}
	if err := svc.Save(ctxAs(f.userID), f.roleA, items); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := svc.ListByRole(ctxAs(f.userID), f.roleA)
	if err != nil {
		t.Fatalf("ListByRole: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("默认 mask 与空应被跳过,期望 2 条入库, 实际 %d: %v", len(got), got)
	}
	if got["system_user:mobile"] != "plain" || got["system_user:email"] != "hide" {
		t.Errorf("落库内容不对: %v", got)
	}
	if _, ok := got["system_user:idcard"]; ok {
		t.Errorf("mask 不应落库,但出现在结果里")
	}

	// 再 Save 一次空集合,全量重写 → 全清空
	if err := svc.Save(ctxAs(f.userID), f.roleA, nil); err != nil {
		t.Fatalf("Save empty: %v", err)
	}
	got, _ = svc.ListByRole(ctxAs(f.userID), f.roleA)
	if len(got) != 0 {
		t.Errorf("Save nil 后应清空, 实际 %v", got)
	}
}

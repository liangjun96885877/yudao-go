// 操作权限 HasPermission 集成测试 + RequirePermissionByPath 中间件端到端测试。
//
// 跑: go test ./internal/module/system/service -run TestPermissionService
//     go test ./internal/module/system/service -run TestRequirePermissionByPath
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/security"
	"yudao-go/internal/module/system/model"
)

// permFixture 造一个最小操作权限测试场景:
//   - permCode "perm_test:owned:create" 在 menu 表登记,挂到 role,role 给 normalUser
//   - permCode "perm_test:foreign:create" 在 menu 表登记,**不挂任何人**
//   - permCode "perm_test:undef:xxx" **完全不存在**
type permFixture struct {
	db          *gorm.DB
	normalUser  int64
	superUser   int64 // 用既有 admin (id=1) 充当超管,不创建新的
	role        int64
	ownedMenu   int64
	foreignMenu int64
	ownedCode   string
	foreignCode string
	undefCode   string
	cleanup     []func()
}

func newPermFixture(t *testing.T, db *gorm.DB) *permFixture {
	t.Helper()
	stamp := time.Now().UnixNano()
	codeSuffix := tsCode(stamp)
	f := &permFixture{
		db:          db,
		normalUser:  930000000 + stamp%1000000,
		superUser:   1, // admin (已在 SQL 种子,挂 super_admin)
		ownedCode:   "perm_test:owned_" + codeSuffix + ":create",
		foreignCode: "perm_test:foreign_" + codeSuffix + ":create",
		undefCode:   "perm_test:undef_" + codeSuffix + ":anything",
	}

	// 1) 普通角色
	r := &model.Role{Name: "perm_test_role", Code: "perm_test_role_" + codeSuffix, Sort: 1, Status: 0, Type: 2}
	r.TenantBase.TenantID = 1
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	f.role = r.ID
	f.cleanup = append(f.cleanup, func() { db.Unscoped().Where("id = ?", r.ID).Delete(&model.Role{}) })

	// 2) 两个菜单:owned (挂到 role) + foreign (不挂)
	owned := &model.Menu{Name: "perm_owned", Type: 3, Sort: 1, Permission: f.ownedCode, Status: 0}
	if err := db.Create(owned).Error; err != nil {
		t.Fatalf("create owned menu: %v", err)
	}
	f.ownedMenu = owned.ID
	f.cleanup = append(f.cleanup, func() { db.Unscoped().Where("id = ?", owned.ID).Delete(&model.Menu{}) })

	foreign := &model.Menu{Name: "perm_foreign", Type: 3, Sort: 2, Permission: f.foreignCode, Status: 0}
	if err := db.Create(foreign).Error; err != nil {
		t.Fatalf("create foreign menu: %v", err)
	}
	f.foreignMenu = foreign.ID
	f.cleanup = append(f.cleanup, func() { db.Unscoped().Where("id = ?", foreign.ID).Delete(&model.Menu{}) })

	// 3) 给 role 挂 owned menu
	rm := &model.RoleMenu{RoleID: f.role, MenuID: f.ownedMenu}
	rm.TenantBase.TenantID = 1
	if err := db.Create(rm).Error; err != nil {
		t.Fatalf("create role_menu: %v", err)
	}
	f.cleanup = append(f.cleanup, func() { db.Unscoped().Where("id = ?", rm.ID).Delete(&model.RoleMenu{}) })

	// 4) 给 normalUser 挂 role
	ur := &model.UserRole{UserID: f.normalUser, RoleID: f.role}
	ur.TenantBase.TenantID = 1
	if err := db.Create(ur).Error; err != nil {
		t.Fatalf("create user_role: %v", err)
	}
	f.cleanup = append(f.cleanup, func() { db.Unscoped().Where("id = ?", ur.ID).Delete(&model.UserRole{}) })

	t.Cleanup(func() {
		for i := len(f.cleanup) - 1; i >= 0; i-- {
			f.cleanup[i]()
		}
	})
	return f
}

// TestPermissionService_HasPermission 覆盖 4 个核心场景:
//   ① 普通用户有该权限码 → true
//   ② 普通用户无该权限码 → false
//   ③ super_admin 短路 → true
//   ④ 匿名(userID=0) → false
func TestPermissionService_HasPermission(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newPermFixture(t, db)
	svc := NewPermissionService(tx)

	// ① 普通用户拥有 ownedCode
	if !svc.HasPermission(ctxAs(f.normalUser), f.ownedCode) {
		t.Errorf("普通用户应拥有 %s", f.ownedCode)
	}

	// ② 普通用户不拥有 foreignCode(挂在别处)
	if svc.HasPermission(ctxAs(f.normalUser), f.foreignCode) {
		t.Errorf("普通用户不应拥有 %s", f.foreignCode)
	}

	// ③ super_admin (id=1) 短路
	if !svc.HasPermission(ctxAs(f.superUser), f.foreignCode) {
		t.Errorf("super_admin 应放行任意权限码")
	}
	if !svc.HasPermission(ctxAs(f.superUser), "perm_test:totally:made-up:xyz") {
		t.Errorf("super_admin 应放行不存在的码")
	}

	// ④ 匿名(无 userID 上下文) → false
	if svc.HasPermission(context.Background(), f.ownedCode) {
		t.Errorf("匿名调用应返回 false")
	}
}

// TestPermissionService_IsDefinedPermission 验证已定义码识别 + 60s 缓存:
// 第一次 reload,第二次走缓存。
func TestPermissionService_IsDefinedPermission(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newPermFixture(t, db)
	svc := NewPermissionService(tx)

	// 已登记的码
	if !svc.IsDefinedPermission(f.ownedCode) {
		t.Errorf("%s 应被识别为已定义", f.ownedCode)
	}
	if !svc.IsDefinedPermission(f.foreignCode) {
		t.Errorf("%s 应被识别为已定义", f.foreignCode)
	}

	// 未登记的码
	if svc.IsDefinedPermission(f.undefCode) {
		t.Errorf("%s 不应被识别为已定义(从未插入 menu 表)", f.undefCode)
	}

	// 并发读不出 race(同时 reload)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.IsDefinedPermission(f.ownedCode)
			_ = svc.IsDefinedPermission(f.undefCode)
		}()
	}
	wg.Wait()
}

// pathPermAdapter 把 PermissionService 适配为 PathPermissionChecker。
type pathPermAdapter struct{ svc *PermissionService }

func (a *pathPermAdapter) HasPermission(ctx context.Context, code string) bool {
	return a.svc.HasPermission(ctx, code)
}
func (a *pathPermAdapter) IsDefinedPermission(code string) bool {
	return a.svc.IsDefinedPermission(code)
}

// TestRequirePermissionByPath_EndToEnd 起 httptest gin 引擎,挂 RequirePermissionByPath,
// 覆盖中间件的 4 个分支:
//   ① 路径推导的码未定义 → 宽松放行(不强制)
//   ② 路径推导的码已定义 + 用户拥有 → 放行
//   ③ 路径推导的码已定义 + 用户无 → 403
//   ④ super_admin 任何路径都放行
func TestRequirePermissionByPath_EndToEnd(t *testing.T) {
	db := openPrivGuardDB(t)
	tx := orm.NewTxManager(db)
	f := newPermFixture(t, db)
	svc := NewPermissionService(tx)
	adapter := &pathPermAdapter{svc: svc}

	gin.SetMode(gin.ReleaseMode)

	// 给路由用 normalUser 上下文跑
	makeReq := func(userID int64, path string) *httptest.ResponseRecorder {
		r := gin.New()
		// 装一个 fake auth: 把 userID 注入 ctx
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(contextx.WithUserID(c.Request.Context(), userID))
			c.Next()
		})
		r.Use(security.RequirePermissionByPath(adapter))
		// 注册所有要测的路径,handler 直接 200。
		paths := []string{
			"/admin-api/perm_test/owned_" + tsCode(time.Now().UnixNano()) + "/create",
			path,
		}
		_ = paths
		r.Any("/*any", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
		})
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// ① 推导码未在 menu 表登记 → 宽松放行(不强制)
	w := makeReq(f.normalUser, "/admin-api/nonexistent_module/something_"+tsCode(time.Now().UnixNano())+"/create")
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Errorf("未定义权限码应宽松放行, 实际: %s", w.Body.String())
	}

	// 计算 ownedCode 对应的路径(模块:业务:操作 → /admin-api/模块/业务/操作)
	ownedPath := "/admin-api/" + strings.ReplaceAll(f.ownedCode, ":", "/")
	foreignPath := "/admin-api/" + strings.ReplaceAll(f.foreignCode, ":", "/")

	// ② 推导码已定义 + 用户拥有 → 放行
	w = makeReq(f.normalUser, ownedPath)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Errorf("拥有权限应放行 %s, 实际: %s", ownedPath, w.Body.String())
	}

	// ③ 推导码已定义 + 用户无 → 403
	w = makeReq(f.normalUser, foreignPath)
	if !strings.Contains(w.Body.String(), `"code":403`) {
		t.Errorf("无权限应 403 %s, 实际: %s", foreignPath, w.Body.String())
	}

	// ④ super_admin 任何路径都放行(包括 foreignPath)
	w = makeReq(f.superUser, foreignPath)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Errorf("super_admin 应放行任意路径, 实际: %s", w.Body.String())
	}
}

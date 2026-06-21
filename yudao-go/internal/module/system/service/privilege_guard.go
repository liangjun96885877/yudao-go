package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/pkg/errcode"
)

// privilegeGuardCacheTTL 是 PrivilegeGuard 用户级缓存有效期。
// 60s 与 PermissionService.IsDefinedPermission 一致;角色变化容忍度同等。
// 角色调整后,最多 60s 后才对当前用户生效——业务可接受(yudao 原版亦无即时失效)。
const privilegeGuardCacheTTL = 60 * time.Second

// userCacheEntry 缓存某用户的全部权限相关派生数据,统一过期。
// 一次填充 4 个字段(IsSuperAdmin / RoleIDs / MenuIDs / DataScope)既能减少 DB 往返,
// 也避免半填充半空状态。
type userCacheEntry struct {
	expiresAt    time.Time
	isSuperAdmin bool
	roleIDs      []int64 // nil 表示无角色;非 nil 表示已加载
	menuIDs      []int64
	scope        int8 // 1/2/5,0 表示未加载
	deptIDs      []int64
	// 标记位:不同方法首次访问时按需加载,避免一次性查 4 个 SQL。
	hasSuper bool
	hasRoles bool
	hasMenus bool
	hasScope bool
}

// PrivilegeGuard 防提权护栏：保证「非超管能授出的 ≤ 自己已拥有的」(Axelor 风格)。
// 所有写权限接口在动手前都应过这一关。
//
// 缓存策略:
//   - 用户级:IsSuperAdmin / MyRoleIDs / MyMenuIDs / MyDataScope 走 60s TTL 内存缓存;
//   - 全局:superAdminRoleID(几乎不变)用 atomic 缓存;
//   - roleCodeByID:不缓存(很少调用,roleID 维度爆炸)。
type PrivilegeGuard struct {
	tx *orm.TxManager

	mu    sync.RWMutex
	users map[int64]*userCacheEntry // uid -> entry

	superRoleID atomic.Int64
	superRoleAt atomic.Int64 // unix nano
}

func NewPrivilegeGuard(tx *orm.TxManager) *PrivilegeGuard {
	return &PrivilegeGuard{
		tx:    tx,
		users: make(map[int64]*userCacheEntry),
	}
}

// ClearCache 清空全部缓存。供测试与角色变更后强一致诉求(可选)调用。
func (g *PrivilegeGuard) ClearCache() {
	g.mu.Lock()
	g.users = make(map[int64]*userCacheEntry)
	g.mu.Unlock()
	g.superRoleID.Store(0)
	g.superRoleAt.Store(0)
}

// getEntry 返回 uid 的有效缓存项(未过期);过期或不存在返回 nil。
func (g *PrivilegeGuard) getEntry(uid int64) *userCacheEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e := g.users[uid]
	if e == nil || time.Now().After(e.expiresAt) {
		return nil
	}
	return e
}

// putField 更新 uid 缓存项的某个字段;过期时新建。
func (g *PrivilegeGuard) putField(uid int64, fill func(*userCacheEntry)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.users[uid]
	if e == nil || time.Now().After(e.expiresAt) {
		e = &userCacheEntry{expiresAt: time.Now().Add(privilegeGuardCacheTTL)}
		g.users[uid] = e
	}
	fill(e)
}

// IsSuperAdmin 当前用户是否拥有 super_admin 角色。命中缓存避免重复查 DB。
func (g *PrivilegeGuard) IsSuperAdmin(ctx context.Context) bool {
	uid := contextx.UserID(ctx)
	if uid == 0 {
		return false
	}
	if e := g.getEntry(uid); e != nil && e.hasSuper {
		return e.isSuperAdmin
	}
	var cnt int64
	g.tx.DB(ctx).Raw(`SELECT COUNT(*) FROM system_role r
		JOIN system_user_role ur ON ur.role_id = r.id
		WHERE ur.user_id = ? AND r.code = 'super_admin' AND r.status = 0 AND r.deleted = 0`,
		uid).Scan(&cnt)
	is := cnt > 0
	g.putField(uid, func(e *userCacheEntry) { e.isSuperAdmin = is; e.hasSuper = true })
	return is
}

// MyRoleIDs 当前用户拥有的角色编号集合。命中缓存避免重复查 DB。
func (g *PrivilegeGuard) MyRoleIDs(ctx context.Context) []int64 {
	uid := contextx.UserID(ctx)
	if uid == 0 {
		return nil
	}
	if e := g.getEntry(uid); e != nil && e.hasRoles {
		return e.roleIDs
	}
	var ids []int64
	g.tx.DB(ctx).Raw(`SELECT role_id FROM system_user_role
		WHERE user_id = ? AND deleted = 0`, uid).Scan(&ids)
	g.putField(uid, func(e *userCacheEntry) { e.roleIDs = ids; e.hasRoles = true })
	return ids
}

// MyMenuIDs 当前用户的角色集合下能访问的全部菜单编号(去重)。命中缓存避免重复查 DB。
func (g *PrivilegeGuard) MyMenuIDs(ctx context.Context) []int64 {
	uid := contextx.UserID(ctx)
	if uid == 0 {
		return nil
	}
	if e := g.getEntry(uid); e != nil && e.hasMenus {
		return e.menuIDs
	}
	var ids []int64
	g.tx.DB(ctx).Raw(`SELECT DISTINCT rm.menu_id FROM system_role_menu rm
		JOIN system_user_role ur ON ur.role_id = rm.role_id
		WHERE ur.user_id = ? AND rm.deleted = 0`, uid).Scan(&ids)
	g.putField(uid, func(e *userCacheEntry) { e.menuIDs = ids; e.hasMenus = true })
	return ids
}

// MyDataScope 当前用户的最宽数据范围（多角色取并集）。命中缓存避免重复查 DB(同时省去部门树 BFS)。
// 返回归一化后的 (scope, deptIDs)：
//   - 1=全部(任一角色拥有 scope=1)
//   - 5=仅本人(只有 scope=5、无部门集合)
//   - 2=自定义部门(其余,deptIDs 为已展开的部门集合,含 scope 3/4 的子树展开)
func (g *PrivilegeGuard) MyDataScope(ctx context.Context) (int8, []int64) {
	uid := contextx.UserID(ctx)
	if uid == 0 {
		return 5, nil
	}
	if e := g.getEntry(uid); e != nil && e.hasScope {
		return e.scope, e.deptIDs
	}
	dp, err := NewDataPermService(g.tx).Resolve(ctx, uid)
	if err != nil || dp == nil {
		g.putField(uid, func(e *userCacheEntry) { e.scope = 5; e.deptIDs = nil; e.hasScope = true })
		return 5, nil
	}
	var scope int8
	var depts []int64
	switch {
	case dp.All:
		scope = 1
	case dp.SelfOnly && len(dp.DeptIDs) == 0:
		scope = 5
	default:
		scope = 2
		depts = dp.DeptIDs
	}
	g.putField(uid, func(e *userCacheEntry) { e.scope = scope; e.deptIDs = depts; e.hasScope = true })
	return scope, depts
}

// superAdminRoleID 查 super_admin 角色编号。原子缓存,几乎不变。
func (g *PrivilegeGuard) superAdminRoleID(ctx context.Context) int64 {
	if id := g.superRoleID.Load(); id != 0 &&
		time.Now().UnixNano()-g.superRoleAt.Load() < int64(privilegeGuardCacheTTL) {
		return id
	}
	var id int64
	g.tx.DB(ctx).Raw(`SELECT id FROM system_role WHERE code = 'super_admin' AND deleted = 0 LIMIT 1`).Scan(&id)
	if id != 0 {
		g.superRoleID.Store(id)
		g.superRoleAt.Store(time.Now().UnixNano())
	}
	return id
}

// roleCodeByID 查角色编码（用于判断是否是 super_admin）。
func (g *PrivilegeGuard) roleCodeByID(ctx context.Context, roleID int64) string {
	var code string
	g.tx.DB(ctx).Raw(`SELECT code FROM system_role WHERE id = ? AND deleted = 0`, roleID).Scan(&code)
	return code
}

// =====================  Ensure*  =====================

// EnsureCanAssignToUser 校验「给某用户分配角色」的合法性。
//   - 不能改自己的角色;
//   - 非超管不能分配 super_admin;
//   - 非超管只能授出自己拥有的角色子集。
func (g *PrivilegeGuard) EnsureCanAssignToUser(ctx context.Context, targetUserID int64, roleIDs []int64) error {
	if targetUserID == contextx.UserID(ctx) {
		return errcode.Forbidden.WithMsg("禁止越权：不能修改自己的角色")
	}
	if g.IsSuperAdmin(ctx) {
		return nil
	}
	superID := g.superAdminRoleID(ctx)
	mine := toSetInt64(g.MyRoleIDs(ctx))
	for _, rid := range roleIDs {
		if rid == superID {
			return errcode.Forbidden.WithMsg("禁止越权：非超管不能分配「超级管理员」角色")
		}
		if !mine[rid] {
			return errcode.Forbidden.WithMsgf("禁止越权：不能分配自己未拥有的角色 (roleId=%d)", rid)
		}
	}
	return nil
}

// EnsureCanModifyRole 校验「能否修改/删除某角色」(菜单权限、数据范围、字段权限、删改 等都用)。
// 非超管不能动 super_admin 角色。
func (g *PrivilegeGuard) EnsureCanModifyRole(ctx context.Context, roleID int64) error {
	if g.IsSuperAdmin(ctx) {
		return nil
	}
	if g.roleCodeByID(ctx, roleID) == "super_admin" {
		return errcode.Forbidden.WithMsg("禁止越权：非超管不能修改「超级管理员」角色")
	}
	return nil
}

// EnsureCanGrantMenus 校验「能否把这些菜单授给角色」。
// 非超管不能授出自己没有的菜单(防造超级角色)。
func (g *PrivilegeGuard) EnsureCanGrantMenus(ctx context.Context, menuIDs []int64) error {
	if g.IsSuperAdmin(ctx) {
		return nil
	}
	mine := toSetInt64(g.MyMenuIDs(ctx))
	for _, mid := range menuIDs {
		if !mine[mid] {
			return errcode.Forbidden.WithMsgf("禁止越权：不能授出自己未拥有的菜单 (menuId=%d)", mid)
		}
	}
	return nil
}

// EnsureCanGrantDataScope 校验「能否把这个数据范围授给角色」。
// 规则:
//   - 自己若拥有「全部」(scope=1) → 任意范围可授;
//   - 目标 scope=1(全部) → 仅超管/自己也是全部可授;
//   - 目标 scope=2(自定义部门) → 目标 deptIDs 必须是自己有效部门集合的子集;
//   - 目标 scope=3/4(本部门 / 及以下) → 自己若是「仅本人」(5) 则拒,否则放行;
//   - 目标 scope=5(仅本人) → 始终放行(最窄)。
func (g *PrivilegeGuard) EnsureCanGrantDataScope(ctx context.Context, scope int8, deptIDs []int64) error {
	if g.IsSuperAdmin(ctx) {
		return nil
	}
	myScope, myDepts := g.MyDataScope(ctx)
	if myScope == 1 {
		return nil // 自己「全部」 → 任意可授
	}
	switch scope {
	case 1:
		return errcode.Forbidden.WithMsg("禁止越权：非超管不能授出「全部数据」范围")
	case 2:
		if myScope == 5 {
			return errcode.Forbidden.WithMsg("禁止越权：你只能看本人数据，不能授出部门范围")
		}
		mine := toSetInt64(myDepts)
		for _, did := range deptIDs {
			if !mine[did] {
				return errcode.Forbidden.WithMsgf("禁止越权：不能授出自己未拥有的部门 (deptId=%d)", did)
			}
		}
		return nil
	case 3, 4:
		if myScope == 5 {
			return errcode.Forbidden.WithMsg("禁止越权：你只能看本人数据，不能授出「本部门」类范围")
		}
		return nil
	case 5:
		return nil
	default:
		return nil
	}
}

// EnsureCanManageMenu 校验「能否管理菜单」(create/update/delete)。
// 菜单是系统结构,且修改 permission 字段可直接改变权限码 → 非超管禁止。
func (g *PrivilegeGuard) EnsureCanManageMenu(ctx context.Context) error {
	if g.IsSuperAdmin(ctx) {
		return nil
	}
	return errcode.Forbidden.WithMsg("禁止越权：菜单管理仅限超级管理员")
}

// EnsureCanSaveRoleFieldPerm 校验「能否给角色设字段权限」。
// 非超管:不能改超管角色;不能把自己只能打码/占位符 的字段配为明文。
func (g *PrivilegeGuard) EnsureCanSaveRoleFieldPerm(ctx context.Context, roleID int64, items []struct{ BizType, Field, Action string }) error {
	if err := g.EnsureCanModifyRole(ctx, roleID); err != nil {
		return err
	}
	if g.IsSuperAdmin(ctx) {
		return nil
	}
	fp := contextx.FieldPermOf(ctx)
	for _, it := range items {
		if it.Action == "plain" && fp.Action(it.BizType, it.Field) != "plain" {
			return errcode.Forbidden.WithMsgf("禁止越权：不能将字段 %s.%s 配为「明文」(你自己也看不到明文)", it.BizType, it.Field)
		}
	}
	return nil
}

// EnsureNotSelf 阻止删自己/禁用自己。
func (g *PrivilegeGuard) EnsureNotSelf(ctx context.Context, userID int64, action string) error {
	if userID == contextx.UserID(ctx) {
		return errcode.Forbidden.WithMsgf("禁止越权：不能%s自己", action)
	}
	return nil
}

func toSetInt64(xs []int64) map[int64]bool {
	m := make(map[int64]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

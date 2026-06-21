// Package contextx 统一管理跨层透传的上下文值（租户、用户、链路 ID）。
// 移植标准：租户/用户/TraceID 只经 context.Context 传递，禁止全局变量或 goroutine 局部变量。
package contextx

import "context"

type ctxKey int

const (
	keyTenantID ctxKey = iota
	keyUserID
	keyUserName
	keyTraceID
	keyIgnoreTenant
	keyDataPerm
	keyIgnoreDataPerm
	keyFieldPerm
)

// DataPerm 是请求级数据权限上下文（横切能力 #6）。
// 多角色取并集：任一角色为「全部」即 All=true。
type DataPerm struct {
	All      bool    // 可见全部数据（scope=1 或超级管理员）
	DeptIDs  []int64 // 可见部门并集（scope 2/3/4）
	SelfOnly bool    // 含「仅本人」（scope=5）
	UserID   int64   // 当前用户编号
}

// --- 租户 ---

func WithTenantID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}

// TenantID 返回当前租户编号；未设置时返回 0。
func TenantID(ctx context.Context) int64 {
	v, _ := ctx.Value(keyTenantID).(int64)
	return v
}

// WithIgnoreTenant 标记忽略多租户过滤（用于跨租户系统操作）。
func WithIgnoreTenant(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyIgnoreTenant, true)
}

func IgnoreTenant(ctx context.Context) bool {
	v, _ := ctx.Value(keyIgnoreTenant).(bool)
	return v
}

// --- 用户 ---

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

func UserID(ctx context.Context) int64 {
	v, _ := ctx.Value(keyUserID).(int64)
	return v
}

func WithUserName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, keyUserName, name)
}

func UserName(ctx context.Context) string {
	v, _ := ctx.Value(keyUserName).(string)
	return v
}

// --- 数据权限 ---

func WithDataPerm(ctx context.Context, dp *DataPerm) context.Context {
	return context.WithValue(ctx, keyDataPerm, dp)
}

// DataPermOf 返回当前数据权限；未设置时返回 nil（表示不过滤）。
func DataPermOf(ctx context.Context) *DataPerm {
	v, _ := ctx.Value(keyDataPerm).(*DataPerm)
	return v
}

// WithIgnoreDataPerm 标记忽略数据权限过滤（用于系统级查询）。
func WithIgnoreDataPerm(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyIgnoreDataPerm, true)
}

func IgnoreDataPerm(ctx context.Context) bool {
	v, _ := ctx.Value(keyIgnoreDataPerm).(bool)
	return v
}

// --- 字段权限 / 数据脱敏 ---

// FieldPerm 是请求级字段权限：决定每个敏感字段输出 明文/打码/占位符。
type FieldPerm struct {
	All     bool              // 超级管理员 → 全部明文
	Actions map[string]string // 键 "bizType:field" → plain/mask/hide
}

// Action 返回字段的有效脱敏动作；未配置时默认 mask（安全默认）。
func (f *FieldPerm) Action(bizType, field string) string {
	if f == nil {
		return "mask"
	}
	if f.All {
		return "plain"
	}
	if a := f.Actions[bizType+":"+field]; a != "" {
		return a
	}
	return "mask"
}

// WithFieldPerm 将字段权限写入 context。
func WithFieldPerm(ctx context.Context, fp *FieldPerm) context.Context {
	return context.WithValue(ctx, keyFieldPerm, fp)
}

// FieldPermOf 返回当前请求的字段权限；未设置时返回 nil。
func FieldPermOf(ctx context.Context) *FieldPerm {
	v, _ := ctx.Value(keyFieldPerm).(*FieldPerm)
	return v
}

// --- 链路 ---

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTraceID, id)
}

func TraceID(ctx context.Context) string {
	v, _ := ctx.Value(keyTraceID).(string)
	return v
}

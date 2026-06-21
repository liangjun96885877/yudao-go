// Package security 提供认证鉴权能力。移植标准：鉴权统一在此，业务模块禁止自实现。
package security

import "context"

// 用户类型常量。
const (
	UserTypeAdmin  int8 = 1 // 后台管理员
	UserTypeMember int8 = 2 // 会员（C 端用户）
)

// LoginUser 是当前登录用户上下文。
type LoginUser struct {
	ID       int64
	TenantID int64
	UserType int8
	Username string
	Scopes   []string // 授权范围
}

type loginUserKey struct{}

// WithLoginUser 将登录用户写入 context。
func WithLoginUser(ctx context.Context, u *LoginUser) context.Context {
	return context.WithValue(ctx, loginUserKey{}, u)
}

// CurrentUser 返回当前登录用户；未登录时返回 nil。
func CurrentUser(ctx context.Context) *LoginUser {
	u, _ := ctx.Value(loginUserKey{}).(*LoginUser)
	return u
}

// TokenValidator 校验访问令牌。由 system 模块实现并注入。
type TokenValidator interface {
	Validate(ctx context.Context, token string) (*LoginUser, error)
}

// PermissionChecker 校验当前用户是否拥有指定权限码。由 system 模块实现并注入。
type PermissionChecker interface {
	HasPermission(ctx context.Context, code string) bool
}

// PathPermissionChecker 在 PermissionChecker 之上增加「权限码是否已定义」查询，
// 供按路径推导权限码的中间件做宽松校验（未定义对应权限码的接口不强制）。
type PathPermissionChecker interface {
	PermissionChecker
	IsDefinedPermission(code string) bool
}

package security

import (
	"strings"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/pkg/errcode"
)

// extractToken 从 Authorization 头（Bearer）或 token 查询参数中提取令牌。
func extractToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
		return strings.TrimSpace(h)
	}
	return c.Query("token")
}

// Auth 认证中间件：校验令牌，将 LoginUser 及租户/用户标识写入 context。横切能力 #7。
func Auth(v TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			web.Fail(c, errcode.Unauthorized)
			c.Abort()
			return
		}
		user, err := v.Validate(c.Request.Context(), token)
		if err != nil || user == nil {
			web.Fail(c, errcode.Unauthorized)
			c.Abort()
			return
		}
		ctx := WithLoginUser(c.Request.Context(), user)
		ctx = contextx.WithTenantID(ctx, user.TenantID)
		ctx = contextx.WithUserID(ctx, user.ID)
		ctx = contextx.WithUserName(ctx, user.Username)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequirePermission 权限校验中间件，挂在需要鉴权的路由分组上。
// 权限码沿用 yudao 约定：模块:业务:操作，如 chatter:comment:create。
func RequirePermission(checker PermissionChecker, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checker.HasPermission(c.Request.Context(), code) {
			web.Fail(c, errcode.Forbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePermissionByPath 按请求路径推导权限码并校验（横切能力 #7）。
// 宽松模式：推导出的权限码若未在系统中定义，则不强制（避免误锁未配权限的接口）。
func RequirePermissionByPath(checker PathPermissionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := derivePermCode(c.Request.URL.Path)
		if code == "" || !checker.IsDefinedPermission(code) {
			c.Next()
			return
		}
		if !checker.HasPermission(c.Request.Context(), code) {
			web.Fail(c, errcode.Forbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// derivePermCode 把 /admin-api/{模块}/{业务}/{动作} 推导为权限码 模块:业务:动作。
// 动作做归一：page/list/get/simple-list → query，delete-list → delete。
func derivePermCode(path string) string {
	p, ok := strings.CutPrefix(path, "/admin-api/")
	if !ok {
		return ""
	}
	segs := strings.Split(strings.Trim(p, "/"), "/")
	if len(segs) < 2 {
		return ""
	}
	action := segs[len(segs)-1]
	switch action {
	case "page", "list", "get", "simple-list", "list-all-simple", "export-excel":
		action = "query"
	case "delete-list":
		action = "delete"
	}
	return strings.Join(segs[:len(segs)-1], ":") + ":" + action
}

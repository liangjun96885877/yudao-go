// Package system 是系统管理模块的组合根。
package system

import (
	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/security"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/module/system/rest"
	"yudao-go/internal/module/system/service"
)

// Module 持有 system 模块的对外注册入口。
type Module struct {
	handler    *rest.Handler
	sys        *rest.SysHandler
	user       *rest.UserHandler
	dict       *rest.DictHandler
	tenant     *rest.TenantHandler
	notify     *rest.NotifyHandler
	mail       *rest.MailHandler
	sms        *rest.SmsHandler
	oauth2        *rest.OAuth2Handler
	perm          *rest.PermissionHandler
	roleFieldPerm *rest.RoleFieldPermHandler
	operateLog    gin.HandlerFunc
	token         *service.TokenService
	dataPerm      *service.DataPermService
	permSvc       *service.PermissionService
	fieldPerm     *service.FieldPermService
}

// New 装配 system 模块。
func New(tx *orm.TxManager) *Module {
	r := repo.New(tx)
	tokenSvc := service.NewTokenService(r)
	authSvc := service.NewAuthService(r, tokenSvc)
	return &Module{
		handler:    rest.NewHandler(authSvc),
		sys:        rest.NewSysHandler(tx),
		user:       rest.NewUserHandler(tx),
		dict:       rest.NewDictHandler(tx),
		tenant:     rest.NewTenantHandler(tx),
		notify:     rest.NewNotifyHandler(tx),
		mail:       rest.NewMailHandler(tx),
		sms:        rest.NewSmsHandler(tx),
		oauth2:        rest.NewOAuth2Handler(tx),
		perm:          rest.NewPermissionHandler(tx),
		roleFieldPerm: rest.NewRoleFieldPermHandler(tx),
		operateLog:    rest.NewOperateLogMiddleware(tx),
		token:         tokenSvc,
		dataPerm:      service.NewDataPermService(tx),
		permSvc:       service.NewPermissionService(tx),
		fieldPerm:     service.NewFieldPermService(tx),
	}
}

// PermissionChecker 返回操作权限校验器，供按路径鉴权的中间件使用。
func (m *Module) PermissionChecker() security.PathPermissionChecker { return m.permSvc }

// FieldPermMiddleware 返回字段权限中间件：解析当前用户对各敏感字段的
// 明文/打码/占位符 动作并写入 context，供 web.Success 脱敏使用。
func (m *Module) FieldPermMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := contextx.UserID(c.Request.Context())
		if uid != 0 {
			if fp, err := m.fieldPerm.Resolve(c.Request.Context(), uid); err == nil {
				c.Request = c.Request.WithContext(contextx.WithFieldPerm(c.Request.Context(), fp))
			}
		}
		c.Next()
	}
}

// DataPermMiddleware 返回数据权限中间件：认证后解析当前用户的数据范围并写入 context。
func (m *Module) DataPermMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := contextx.UserID(c.Request.Context())
		if uid != 0 {
			if dp, err := m.dataPerm.Resolve(c.Request.Context(), uid); err == nil {
				c.Request = c.Request.WithContext(contextx.WithDataPerm(c.Request.Context(), dp))
			}
		}
		c.Next()
	}
}

// TokenValidator 返回令牌校验器，供认证中间件使用。
func (m *Module) TokenValidator() security.TokenValidator { return m.token }

// OperateLogMiddleware 返回操作日志中间件，挂在需认证的 API 分组上。
func (m *Module) OperateLogMiddleware() gin.HandlerFunc { return m.operateLog }

// SetAuditor 注入 chatter 字段审计器，启用用户与角色变更的时间线记录。
func (m *Module) SetAuditor(a rest.Auditor) {
	m.user.SetAuditor(a)
	m.sys.SetAuditor(a)
	m.perm.SetAuditor(a)
	m.roleFieldPerm.SetAuditor(a)
}

// RegisterPublic 注册免认证路由。
func (m *Module) RegisterPublic(g *gin.RouterGroup) { m.handler.RegisterPublic(g) }

// RegisterAuthed 注册需认证路由。
func (m *Module) RegisterAuthed(g *gin.RouterGroup) {
	m.handler.RegisterAuthed(g)
	m.sys.Register(g)
	m.user.Register(g)
	m.dict.Register(g)
	m.tenant.Register(g)
	m.notify.Register(g)
	m.mail.Register(g)
	m.sms.Register(g)
	m.oauth2.Register(g)
	m.perm.Register(g)
	m.roleFieldPerm.Register(g)
}

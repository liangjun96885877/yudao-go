// Package rest 是 system 模块的 HTTP 接入层。
package rest

import (
	"strings"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/dto"
	"yudao-go/internal/module/system/service"
	"yudao-go/internal/pkg/errcode"
)

// Handler 提供 system 模块的 HTTP 处理函数。
type Handler struct {
	auth *service.AuthService
}

func NewHandler(auth *service.AuthService) *Handler { return &Handler{auth: auth} }

// RegisterPublic 注册免认证接口（登录、刷新令牌、租户解析）。
func (h *Handler) RegisterPublic(g *gin.RouterGroup) {
	g.POST("/system/auth/login", h.login)
	g.POST("/system/auth/refresh-token", h.refreshToken)
	g.GET("/system/tenant/get-id-by-name", h.tenantIDByName)
	g.GET("/system/tenant/get-by-website", h.tenantByWebsite)
}

// tenantByWebsite 按域名获取租户：暂不支持域名路由，返回空（前端回退默认租户名）。
func (h *Handler) tenantByWebsite(c *gin.Context) {
	web.Success[any](c, nil)
}

// refreshToken 用刷新令牌换取新的访问令牌。
func (h *Handler) refreshToken(c *gin.Context) {
	resp, err := h.auth.RefreshToken(c.Request.Context(), c.Query("refreshToken"))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, resp)
}

// RegisterAuthed 注册需认证接口。
func (h *Handler) RegisterAuthed(g *gin.RouterGroup) {
	g.GET("/system/auth/get-permission-info", h.getPermissionInfo)
	g.POST("/system/auth/logout", h.logout)
	g.GET("/system/dict-data/simple-list", h.dictSimpleList)
}

func (h *Handler) login(c *gin.Context) {
	var req dto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		web.Fail(c, errcode.BadRequest)
		return
	}
	resp, err := h.auth.Login(c.Request.Context(), &req, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, resp)
}

func (h *Handler) tenantIDByName(c *gin.Context) {
	id, err := h.auth.TenantIDByName(c.Request.Context(), c.Query("name"))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) getPermissionInfo(c *gin.Context) {
	resp, err := h.auth.GetPermissionInfo(c.Request.Context())
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, resp)
}

func (h *Handler) logout(c *gin.Context) {
	if err := h.auth.Logout(c.Request.Context(), bearerToken(c)); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) dictSimpleList(c *gin.Context) {
	data, err := h.auth.ListDictData(c.Request.Context())
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, data)
}

// bearerToken 从 Authorization 头提取令牌。
func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return strings.TrimSpace(h)
}

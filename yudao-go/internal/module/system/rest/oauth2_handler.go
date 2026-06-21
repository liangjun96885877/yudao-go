package rest

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// OAuth2Handler 提供 OAuth 2.0「客户端管理」「令牌管理」接口。
type OAuth2Handler struct {
	client *repo.CRUD[model.OAuth2Client]
	token  *repo.CRUD[model.OAuth2AccessToken]
	tx     *orm.TxManager
}

func NewOAuth2Handler(tx *orm.TxManager) *OAuth2Handler {
	return &OAuth2Handler{
		client: repo.NewCRUD[model.OAuth2Client](tx),
		token:  repo.NewCRUD[model.OAuth2AccessToken](tx),
		tx:     tx,
	}
}

func (h *OAuth2Handler) Register(g *gin.RouterGroup) {
	g.GET("/system/oauth2-client/page", h.clientPage)
	g.GET("/system/oauth2-client/get", h.clientGet)
	g.POST("/system/oauth2-client/create", h.clientCreate)
	g.PUT("/system/oauth2-client/update", h.clientUpdate)
	g.DELETE("/system/oauth2-client/delete", h.clientDelete)
	g.DELETE("/system/oauth2-client/delete-list", h.clientDeleteList)
	g.GET("/system/oauth2-token/page", h.tokenPage)
	g.DELETE("/system/oauth2-token/delete", h.tokenDelete)
}

// ===== OAuth2 客户端 =====

func (h *OAuth2Handler) clientPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, status := c.Query("name"), c.Query("status")
	list, total, err := h.client.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = eqIf(q, "status", status)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *OAuth2Handler) clientGet(c *gin.Context) {
	m, err := h.client.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *OAuth2Handler) clientCreate(c *gin.Context) {
	var m model.OAuth2Client
	if !bind(c, &m) {
		return
	}
	if err := h.client.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *OAuth2Handler) clientUpdate(c *gin.Context) {
	var m model.OAuth2Client
	if !bind(c, &m) {
		return
	}
	// 显式字段映射：避免 GORM 结构体更新跳过 status=0 等零值。
	respondOK(c, h.client.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"client_id": m.ClientID, "secret": m.Secret, "name": m.Name,
		"logo": m.Logo, "description": m.Description, "status": m.Status,
		"access_token_validity_seconds":  m.AccessTokenValiditySeconds,
		"refresh_token_validity_seconds": m.RefreshTokenValiditySeconds,
		"redirect_uris":          m.RedirectURIs,
		"authorized_grant_types": m.AuthorizedGrantTypes,
		"scopes":                 m.Scopes,
		"auto_approve_scopes":    m.AutoApproveScopes,
		"authorities":            m.Authorities,
		"resource_ids":           m.ResourceIDs,
		"additional_information": m.AdditionalInformation,
	}))
}

func (h *OAuth2Handler) clientDelete(c *gin.Context) {
	respondOK(c, h.client.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *OAuth2Handler) clientDeleteList(c *gin.Context) {
	respondOK(c, h.client.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== OAuth2 令牌 =====

func (h *OAuth2Handler) tokenPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	userID, userType := c.Query("userId"), c.Query("userType")
	clientID := c.Query("clientId")
	list, total, err := h.token.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = eqIf(q, "user_id", userID)
			q = eqIf(q, "user_type", userType)
			q = likeIf(q, "client_id", clientID)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

// tokenDelete 按 accessToken 删除（即强制下线）一个令牌。
func (h *OAuth2Handler) tokenDelete(c *gin.Context) {
	at := c.Query("accessToken")
	if at == "" {
		web.Fail(c, errcode.BadRequest.WithMsg("accessToken 不能为空"))
		return
	}
	err := h.tx.DB(c.Request.Context()).Model(&model.OAuth2AccessToken{}).
		Where("access_token = ? AND deleted = 0", at).
		Update("deleted", orm.Bit(true)).Error
	respondOK(c, err)
}

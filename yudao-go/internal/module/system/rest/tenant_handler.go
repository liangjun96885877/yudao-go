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

// TenantHandler 提供租户、租户套餐、登录日志、操作日志接口。
type TenantHandler struct {
	tenant  *repo.CRUD[model.Tenant]
	tpkg    *repo.CRUD[model.TenantPackage]
	loginLg *repo.CRUD[model.LoginLog]
	operLg  *repo.CRUD[model.OperateLog]
}

func NewTenantHandler(tx *orm.TxManager) *TenantHandler {
	return &TenantHandler{
		tenant:  repo.NewCRUD[model.Tenant](tx),
		tpkg:    repo.NewCRUD[model.TenantPackage](tx),
		loginLg: repo.NewCRUD[model.LoginLog](tx),
		operLg:  repo.NewCRUD[model.OperateLog](tx),
	}
}

func (h *TenantHandler) Register(g *gin.RouterGroup) {
	// 租户（simple-list 已在 DictHandler 注册）
	g.GET("/system/tenant/page", h.tenantPage)
	g.GET("/system/tenant/get", h.tenantGet)
	g.POST("/system/tenant/create", h.tenantCreate)
	g.PUT("/system/tenant/update", h.tenantUpdate)
	g.DELETE("/system/tenant/delete", h.tenantDelete)
	g.DELETE("/system/tenant/delete-list", h.tenantDeleteList)
	// 租户套餐
	g.GET("/system/tenant-package/page", h.tpkgPage)
	g.GET("/system/tenant-package/simple-list", h.tpkgSimpleList)
	g.GET("/system/tenant-package/get", h.tpkgGet)
	g.POST("/system/tenant-package/create", h.tpkgCreate)
	g.PUT("/system/tenant-package/update", h.tpkgUpdate)
	g.DELETE("/system/tenant-package/delete", h.tpkgDelete)
	g.DELETE("/system/tenant-package/delete-list", h.tpkgDeleteList)
	// 审计日志（只读）
	g.GET("/system/login-log/page", h.loginLogPage)
	g.GET("/system/operate-log/page", h.operateLogPage)
}

// ===== 租户 =====

func (h *TenantHandler) tenantPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, status := c.Query("name"), c.Query("status")
	list, total, err := h.tenant.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = eqIf(q, "status", status)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *TenantHandler) tenantGet(c *gin.Context) {
	m, err := h.tenant.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *TenantHandler) tenantCreate(c *gin.Context) {
	var m model.Tenant
	if !bind(c, &m) {
		return
	}
	if err := h.tenant.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *TenantHandler) tenantUpdate(c *gin.Context) {
	var m model.Tenant
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.tenant.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "contact_name": m.ContactName, "contact_mobile": m.ContactMobile,
		"status": m.Status, "websites": m.Website, "package_id": m.PackageID,
		"expire_time": m.ExpireTime, "account_count": m.AccountCount,
	}))
}

func (h *TenantHandler) tenantDelete(c *gin.Context) {
	respondOK(c, h.tenant.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *TenantHandler) tenantDeleteList(c *gin.Context) {
	respondOK(c, h.tenant.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 租户套餐 =====

// tpkgVO 在套餐字段外附带菜单 ID 数组。
type tpkgVO struct {
	*model.TenantPackage
	MenuIDList []int64 `json:"menuIds"`
}

// tpkgReq 是套餐创建/修改请求。
type tpkgReq struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Status  int8    `json:"status"`
	Remark  string  `json:"remark"`
	MenuIDs []int64 `json:"menuIds"`
}

func (h *TenantHandler) tpkgPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, status := c.Query("name"), c.Query("status")
	list, total, err := h.tpkg.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = eqIf(q, "status", status)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *TenantHandler) tpkgSimpleList(c *gin.Context) {
	list, err := h.tpkg.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0").Order("id ASC")
	})
	respond(c, list, err)
}

func (h *TenantHandler) tpkgGet(c *gin.Context) {
	m, err := h.tpkg.Get(c.Request.Context(), qID(c))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if m == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	web.Success(c, &tpkgVO{TenantPackage: m, MenuIDList: parsePostIDs(m.MenuIDs)})
}

func (h *TenantHandler) tpkgCreate(c *gin.Context) {
	var req tpkgReq
	if !bind(c, &req) {
		return
	}
	m := &model.TenantPackage{
		Name: req.Name, Status: req.Status, Remark: req.Remark,
		MenuIDs: marshalPostIDs(req.MenuIDs),
	}
	if err := h.tpkg.Create(c.Request.Context(), m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *TenantHandler) tpkgUpdate(c *gin.Context) {
	var req tpkgReq
	if !bind(c, &req) {
		return
	}
	respondOK(c, h.tpkg.UpdateFields(c.Request.Context(), req.ID, map[string]any{
		"name": req.Name, "status": req.Status, "remark": req.Remark,
		"menu_ids": marshalPostIDs(req.MenuIDs),
	}))
}

func (h *TenantHandler) tpkgDelete(c *gin.Context) {
	respondOK(c, h.tpkg.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *TenantHandler) tpkgDeleteList(c *gin.Context) {
	respondOK(c, h.tpkg.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 审计日志 =====

func (h *TenantHandler) loginLogPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	username, userIP := c.Query("username"), c.Query("userIp")
	list, total, err := h.loginLg.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "username", username)
			q = likeIf(q, "user_ip", userIP)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *TenantHandler) operateLogPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	typ, userID := c.Query("type"), c.Query("userId")
	list, total, err := h.operLg.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "type", typ)
			q = eqIf(q, "user_id", userID)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

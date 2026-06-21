package rest

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
)

// DictHandler 提供字典管理 CRUD 与若干杂项接口。
type DictHandler struct {
	dictType *repo.CRUD[model.DictType]
	dictData *repo.CRUD[model.DictData]
	tenant   *repo.CRUD[model.Tenant]
}

func NewDictHandler(tx *orm.TxManager) *DictHandler {
	return &DictHandler{
		dictType: repo.NewCRUD[model.DictType](tx),
		dictData: repo.NewCRUD[model.DictData](tx),
		tenant:   repo.NewCRUD[model.Tenant](tx),
	}
}

func (h *DictHandler) Register(g *gin.RouterGroup) {
	// 字典类型
	g.GET("/system/dict-type/page", h.typePage)
	g.GET("/system/dict-type/simple-list", h.typeSimpleList)
	g.GET("/system/dict-type/get", h.typeGet)
	g.POST("/system/dict-type/create", h.typeCreate)
	g.PUT("/system/dict-type/update", h.typeUpdate)
	g.DELETE("/system/dict-type/delete", h.typeDelete)
	g.DELETE("/system/dict-type/delete-list", h.typeDeleteList)
	// 字典数据
	g.GET("/system/dict-data/page", h.dataPage)
	g.GET("/system/dict-data/get", h.dataGet)
	g.POST("/system/dict-data/create", h.dataCreate)
	g.PUT("/system/dict-data/update", h.dataUpdate)
	g.DELETE("/system/dict-data/delete", h.dataDelete)
	g.DELETE("/system/dict-data/delete-list", h.dataDeleteList)
	// 杂项
	g.GET("/system/tenant/simple-list", h.tenantSimpleList)
}

// ===== 字典类型 =====

func (h *DictHandler) typePage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, typ, status := c.Query("name"), c.Query("type"), c.Query("status")
	list, total, err := h.dictType.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = likeIf(q, "type", typ)
			q = eqIf(q, "status", status)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *DictHandler) typeSimpleList(c *gin.Context) {
	list, err := h.dictType.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0").Order("id ASC")
	})
	respond(c, list, err)
}

func (h *DictHandler) typeGet(c *gin.Context) {
	m, err := h.dictType.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *DictHandler) typeCreate(c *gin.Context) {
	var m model.DictType
	if !bind(c, &m) {
		return
	}
	if err := h.dictType.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *DictHandler) typeUpdate(c *gin.Context) {
	var m model.DictType
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.dictType.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "type": m.Type, "status": m.Status, "remark": m.Remark,
	}))
}

func (h *DictHandler) typeDelete(c *gin.Context) {
	respondOK(c, h.dictType.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *DictHandler) typeDeleteList(c *gin.Context) {
	respondOK(c, h.dictType.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 字典数据 =====

func (h *DictHandler) dataPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	label, dictType, status := c.Query("label"), c.Query("dictType"), c.Query("status")
	list, total, err := h.dictData.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "label", label)
			q = eqIf(q, "dict_type", dictType)
			q = eqIf(q, "status", status)
			return q.Order("dict_type ASC, sort ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *DictHandler) dataGet(c *gin.Context) {
	m, err := h.dictData.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *DictHandler) dataCreate(c *gin.Context) {
	var m model.DictData
	if !bind(c, &m) {
		return
	}
	if err := h.dictData.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *DictHandler) dataUpdate(c *gin.Context) {
	var m model.DictData
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.dictData.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"sort": m.Sort, "label": m.Label, "value": m.Value, "dict_type": m.DictType,
		"status": m.Status, "color_type": m.ColorType, "css_class": m.CSSClass, "remark": m.Remark,
	}))
}

func (h *DictHandler) dataDelete(c *gin.Context) {
	respondOK(c, h.dictData.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *DictHandler) dataDeleteList(c *gin.Context) {
	respondOK(c, h.dictData.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 杂项 =====

func (h *DictHandler) tenantSimpleList(c *gin.Context) {
	list, err := h.tenant.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Order("id ASC")
	})
	respond(c, list, err)
}


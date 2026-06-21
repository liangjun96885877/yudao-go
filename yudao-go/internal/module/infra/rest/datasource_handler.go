package rest

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// DataSourceHandler 提供基础设施「数据源配置」CRUD 接口。
type DataSourceHandler struct {
	ds *sysrepo.CRUD[model.DataSourceConfig]
}

func NewDataSourceHandler(tx *orm.TxManager) *DataSourceHandler {
	return &DataSourceHandler{ds: sysrepo.NewCRUD[model.DataSourceConfig](tx)}
}

func (h *DataSourceHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/data-source-config/list", h.list)
	g.GET("/infra/data-source-config/get", h.get)
	g.POST("/infra/data-source-config/create", h.create)
	g.PUT("/infra/data-source-config/update", h.update)
	g.DELETE("/infra/data-source-config/delete", h.del)
	g.DELETE("/infra/data-source-config/delete-list", h.delList)
}

func (h *DataSourceHandler) list(c *gin.Context) {
	list, err := h.ds.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Order("id ASC")
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, list)
}

func (h *DataSourceHandler) get(c *gin.Context) {
	m, err := h.ds.Get(c.Request.Context(), qID(c))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if m == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	web.Success(c, m)
}

func (h *DataSourceHandler) create(c *gin.Context) {
	var m model.DataSourceConfig
	if !bind(c, &m) {
		return
	}
	if err := h.ds.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *DataSourceHandler) update(c *gin.Context) {
	var m model.DataSourceConfig
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.ds.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "url": m.URL, "username": m.Username, "password": m.Password,
	}))
}

func (h *DataSourceHandler) del(c *gin.Context) {
	respondOK(c, h.ds.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *DataSourceHandler) delList(c *gin.Context) {
	respondOK(c, h.ds.SoftDelete(c.Request.Context(), qIDs(c)))
}

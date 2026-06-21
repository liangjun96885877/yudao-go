// Package rest 是 infra 模块的 HTTP 接入层。
package rest

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// ConfigHandler 提供基础设施「配置管理」CRUD 接口。
type ConfigHandler struct {
	config *sysrepo.CRUD[model.Config]
}

func NewConfigHandler(tx *orm.TxManager) *ConfigHandler {
	return &ConfigHandler{config: sysrepo.NewCRUD[model.Config](tx)}
}

func (h *ConfigHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/config/page", h.page)
	g.GET("/infra/config/get", h.get)
	g.GET("/infra/config/get-value-by-key", h.getValueByKey)
	g.POST("/infra/config/create", h.create)
	g.PUT("/infra/config/update", h.update)
	g.DELETE("/infra/config/delete", h.del)
	g.DELETE("/infra/config/delete-list", h.delList)
}

func (h *ConfigHandler) page(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, key, typ := c.Query("name"), c.Query("key"), c.Query("type")
	list, total, err := h.config.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = likeIf(q, "config_key", key)
			q = eqIf(q, "type", typ)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *ConfigHandler) get(c *gin.Context) {
	m, err := h.config.Get(c.Request.Context(), qID(c))
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

// getValueByKey 按配置键返回配置值。
func (h *ConfigHandler) getValueByKey(c *gin.Context) {
	key := c.Query("key")
	list, err := h.config.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("config_key = ?", key).Limit(1)
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if len(list) == 0 {
		web.Success[any](c, nil)
		return
	}
	web.Success(c, list[0].Value)
}

func (h *ConfigHandler) create(c *gin.Context) {
	var m model.Config
	if !bind(c, &m) {
		return
	}
	if err := h.config.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *ConfigHandler) update(c *gin.Context) {
	var m model.Config
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.config.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"category": m.Category, "name": m.Name, "value": m.Value,
		"visible": m.Visible, "remark": m.Remark,
	}))
}

func (h *ConfigHandler) del(c *gin.Context) {
	respondOK(c, h.config.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *ConfigHandler) delList(c *gin.Context) {
	respondOK(c, h.config.SoftDelete(c.Request.Context(), qIDs(c)))
}

// --- 辅助函数 ---

func bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		web.Fail(c, errcode.BadRequest.WithMsg("请求参数不正确"))
		return false
	}
	return true
}

func respondOK(c *gin.Context, err error) {
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, true)
}

func qID(c *gin.Context) int64 {
	id, _ := strconv.ParseInt(c.Query("id"), 10, 64)
	return id
}

// qInt64 读取指定名称的 int64 查询参数。
func qInt64(c *gin.Context, name string) int64 {
	v, _ := strconv.ParseInt(c.Query(name), 10, 64)
	return v
}

func qIDs(c *gin.Context) []int64 {
	var ids []int64
	for _, s := range strings.Split(c.Query("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func likeIf(q *gorm.DB, col, val string) *gorm.DB {
	if val != "" {
		return q.Where(col+" LIKE ?", "%"+val+"%")
	}
	return q
}

func eqIf(q *gorm.DB, col, val string) *gorm.DB {
	if val != "" {
		return q.Where(col+" = ?", val)
	}
	return q
}

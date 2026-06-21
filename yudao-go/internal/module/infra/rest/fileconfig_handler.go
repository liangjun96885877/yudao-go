package rest

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// FileConfigHandler 提供「文件配置」CRUD 接口。
type FileConfigHandler struct {
	cfg *sysrepo.CRUD[model.FileConfig]
	tx  *orm.TxManager
}

func NewFileConfigHandler(tx *orm.TxManager) *FileConfigHandler {
	return &FileConfigHandler{cfg: sysrepo.NewCRUD[model.FileConfig](tx), tx: tx}
}

func (h *FileConfigHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/file-config/page", h.page)
	g.GET("/infra/file-config/get", h.get)
	g.GET("/infra/file-config/test", h.test)
	g.POST("/infra/file-config/create", h.create)
	g.PUT("/infra/file-config/update", h.update)
	g.PUT("/infra/file-config/update-master", h.updateMaster)
	g.DELETE("/infra/file-config/delete", h.del)
	g.DELETE("/infra/file-config/delete-list", h.delList)
}

// fileConfigReq 是文件配置创建 / 修改请求（config 为存储相关 JSON）。
type fileConfigReq struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name"`
	Storage int8            `json:"storage"`
	Remark  string          `json:"remark"`
	Config  json.RawMessage `json:"config"`
}

// fileConfigVO 在配置字段外把 config 以 JSON 对象返回。
type fileConfigVO struct {
	*model.FileConfig
	Config json.RawMessage `json:"config"`
}

func (h *FileConfigHandler) page(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, storage := c.Query("name"), c.Query("storage")
	list, total, err := h.cfg.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = eqIf(q, "storage", storage)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *FileConfigHandler) get(c *gin.Context) {
	m, err := h.cfg.Get(c.Request.Context(), qID(c))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if m == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	cfg := json.RawMessage(m.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	web.Success(c, &fileConfigVO{FileConfig: m, Config: cfg})
}

func (h *FileConfigHandler) create(c *gin.Context) {
	var req fileConfigReq
	if !bind(c, &req) {
		return
	}
	m := &model.FileConfig{
		Name: req.Name, Storage: req.Storage, Remark: req.Remark,
		Config: string(req.Config),
	}
	if err := h.cfg.Create(c.Request.Context(), m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *FileConfigHandler) update(c *gin.Context) {
	var req fileConfigReq
	if !bind(c, &req) {
		return
	}
	respondOK(c, h.cfg.UpdateFields(c.Request.Context(), req.ID, map[string]any{
		"name": req.Name, "storage": req.Storage,
		"remark": req.Remark, "config": string(req.Config),
	}))
}

// updateMaster 把指定配置设为主配置（其余取消主），同一事务内完成。
func (h *FileConfigHandler) updateMaster(c *gin.Context) {
	id := qID(c)
	err := h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		db := h.tx.DB(ctx)
		if err := db.Model(&model.FileConfig{}).
			Where("deleted = 0").Update("master", orm.Bit(false)).Error; err != nil {
			return err
		}
		return db.Model(&model.FileConfig{}).
			Where("id = ?", id).Update("master", orm.Bit(true)).Error
	})
	respondOK(c, err)
}

// test 测试存储配置：DB 存储无需外部连通性测试，返回提示。
func (h *FileConfigHandler) test(c *gin.Context) {
	web.Success(c, "配置可用（当前为数据库存储，无需连通性测试）")
}

func (h *FileConfigHandler) del(c *gin.Context) {
	respondOK(c, h.cfg.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *FileConfigHandler) delList(c *gin.Context) {
	respondOK(c, h.cfg.SoftDelete(c.Request.Context(), qIDs(c)))
}

package rest

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
	"yudao-go/internal/pkg/idgen"
)

// FileHandler 提供「文件管理」接口。文件内容存于 infra_file_content（DB 存储）。
type FileHandler struct {
	files   *sysrepo.CRUD[model.File]
	content *sysrepo.CRUD[model.FileContent]
}

func NewFileHandler(tx *orm.TxManager) *FileHandler {
	return &FileHandler{
		files:   sysrepo.NewCRUD[model.File](tx),
		content: sysrepo.NewCRUD[model.FileContent](tx),
	}
}

// Register 注册需认证接口。
func (h *FileHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/file/page", h.page)
	g.POST("/infra/file/upload", h.upload)
	g.DELETE("/infra/file/delete", h.del)
	g.DELETE("/infra/file/delete-list", h.delList)
}

// RegisterPublic 注册免认证的文件内容服务（供 <img> 等直接访问）。
func (h *FileHandler) RegisterPublic(g *gin.RouterGroup) {
	g.GET("/infra/file/content", h.serveContent)
}

func (h *FileHandler) page(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, typ := c.Query("name"), c.Query("type")
	list, total, err := h.files.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = likeIf(q, "type", typ)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

// upload 接收 multipart 文件，内容存入 DB，返回可访问 URL。
func (h *FileHandler) upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		web.Fail(c, errcode.BadRequest.WithMsg("缺少文件"))
		return
	}
	src, err := fh.Open()
	if err != nil {
		web.FailErr(c, err)
		return
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(src)
	if err != nil {
		web.FailErr(c, err)
		return
	}

	path := idgen.UUID() + filepath.Ext(fh.Filename)
	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/admin-api/infra/file/content?path=%s", scheme, c.Request.Host, path)

	ctx := c.Request.Context()
	if err := h.content.Create(ctx, &model.FileContent{Path: path, Content: data}); err != nil {
		web.FailErr(c, err)
		return
	}
	file := &model.File{
		Name: fh.Filename, Path: path, URL: url,
		Type: contentType, Size: len(data),
	}
	if err := h.files.Create(ctx, file); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, url)
}

// serveContent 按 path 返回文件内容。
func (h *FileHandler) serveContent(c *gin.Context) {
	path := c.Query("path")
	contents, err := h.content.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("path = ?", path).Limit(1)
	})
	if err != nil || len(contents) == 0 {
		c.Status(404)
		return
	}
	contentType := "application/octet-stream"
	if files, _ := h.files.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("path = ?", path).Limit(1)
	}); len(files) > 0 && files[0].Type != "" {
		contentType = files[0].Type
	}
	c.Data(200, contentType, contents[0].Content)
}

func (h *FileHandler) del(c *gin.Context) {
	respondOK(c, h.files.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *FileHandler) delList(c *gin.Context) {
	respondOK(c, h.files.SoftDelete(c.Request.Context(), qIDs(c)))
}

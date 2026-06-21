// Package rest 是 myerp 模块的 HTTP 接入层。
//
// 端点:分类 6 + 属性 6 + 产品 6 + 单位 6 + 批次 5 + 出入库 2。
//
// 安全:
//   - 请求体大小 由全局中间件控制(BUG #8 防 DoS)
//   - findById null → 业务 404 而非 code=0 data=null(BUG #7)
//   - 错误统一走 web.FailErr 转 ServiceError
package rest

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/application/service"
	"yudao-go/internal/module/myerp/domain/repository"
	"yudao-go/internal/pkg/errcode"
)

// Handler 聚合 myerp 应用服务,提供 HTTP 处理函数。
type Handler struct {
	category  *service.CategoryService
	attribute *service.AttributeService
	product   *service.ProductService
	uom       *service.UomService
	batch     *service.BatchService
	stockMove *service.StockMoveService
	template  *service.TemplateService
}

func NewHandler(
	cat *service.CategoryService,
	attr *service.AttributeService,
	prod *service.ProductService,
	uom *service.UomService,
	batch *service.BatchService,
	stockMove *service.StockMoveService,
	template *service.TemplateService,
) *Handler {
	return &Handler{
		category: cat, attribute: attr, product: prod, uom: uom,
		batch: batch, stockMove: stockMove, template: template,
	}
}

// Register 在 /admin-api 分组下挂载 myerp 全部接口。
func (h *Handler) Register(group *gin.RouterGroup) {
	g := group.Group("/myerp")

	// 分类(6)
	g.GET("/category/page", h.categoryPage)
	g.GET("/category/tree", h.categoryTree)
	g.GET("/category/get", h.categoryGet)
	g.POST("/category/create", h.categoryCreate)
	g.PUT("/category/update", h.categoryUpdate)
	g.DELETE("/category/delete", h.categoryDelete)

	// 属性(6)
	g.GET("/attribute/page", h.attributePage)
	g.GET("/attribute/get", h.attributeGet)
	g.GET("/attribute/list-by-category", h.attributeListByCategory)
	g.POST("/attribute/create", h.attributeCreate)
	g.PUT("/attribute/update", h.attributeUpdate)
	g.DELETE("/attribute/delete", h.attributeDelete)

	// 产品(5 + 动态筛选)
	g.GET("/product/page", h.productPage)
	g.GET("/product/get", h.productGet)
	g.GET("/product/search", h.productSearch)
	g.POST("/product/create", h.productCreate)
	g.PUT("/product/update", h.productUpdate)
	g.DELETE("/product/delete", h.productDelete)

	// 单位(5 + 全量下拉)
	g.GET("/uom/page", h.uomPage)
	g.GET("/uom/list-all", h.uomListAll)
	g.GET("/uom/get", h.uomGet)
	g.POST("/uom/create", h.uomCreate)
	g.PUT("/uom/update", h.uomUpdate)
	g.DELETE("/uom/delete", h.uomDelete)

	// 批次(5)
	g.GET("/batch/page", h.batchPage)
	g.GET("/batch/get", h.batchGet)
	g.POST("/batch/create", h.batchCreate)
	g.PUT("/batch/update", h.batchUpdate)
	g.DELETE("/batch/delete", h.batchDelete)

	// 出入库流水(列表 + 记账)
	g.GET("/stock-move/page", h.stockMovePage)
	g.POST("/stock-move/create", h.stockMoveCreate)

	// 产品模板(SPU)
	g.GET("/template/page", h.templatePage)
	g.GET("/template/get", h.templateGet)
	g.GET("/template/list-variants", h.templateListVariants)
	g.POST("/template/create", h.templateCreate)
	g.PUT("/template/update", h.templateUpdate)
	g.DELETE("/template/delete", h.templateDelete)
	g.POST("/template/generate-variants", h.templateGenerateVariants)
}

// === Template ===

func (h *Handler) templatePage(c *gin.Context) {
	q := repository.TemplateQuery{
		PageNo:     parseInt(c.Query("pageNo"), 1),
		PageSize:   parseInt(c.Query("pageSize"), 10),
		Name:       c.Query("name"),
		Code:       c.Query("code"),
		CategoryID: parseInt64Ptr(c.Query("categoryId")),
		Status:     parseInt8Ptr(c.Query("status")),
	}
	res, err := h.template.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, res)
}

func (h *Handler) templateGet(c *gin.Context) {
	v, err := h.template.Get(c.Request.Context(), parseInt64(c.Query("id")))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

func (h *Handler) templateListVariants(c *gin.Context) {
	v, err := h.template.ListVariants(c.Request.Context(), parseInt64(c.Query("templateId")))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

func (h *Handler) templateCreate(c *gin.Context) {
	var req dto.TemplateSaveReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.template.Create(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) templateUpdate(c *gin.Context) {
	var req dto.TemplateSaveReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.template.Update(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) templateDelete(c *gin.Context) {
	if err := h.template.Delete(c.Request.Context(), parseInt64(c.Query("id"))); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) templateGenerateVariants(c *gin.Context) {
	var req dto.GenerateVariantsReq
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.template.GenerateVariants(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, resp)
}

// === Batch ===

func (h *Handler) batchPage(c *gin.Context) {
	q := repository.BatchQuery{
		PageNo:    parseInt(c.Query("pageNo"), 1),
		PageSize:  parseInt(c.Query("pageSize"), 10),
		ProductID: parseInt64Ptr(c.Query("productId")),
		BatchNo:   c.Query("batchNo"),
		Status:    parseInt8Ptr(c.Query("status")),
	}
	res, err := h.batch.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, res)
}

func (h *Handler) batchGet(c *gin.Context) {
	v, err := h.batch.Get(c.Request.Context(), parseInt64(c.Query("id")))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

func (h *Handler) batchCreate(c *gin.Context) {
	var req dto.BatchSaveReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.batch.Create(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) batchUpdate(c *gin.Context) {
	var req dto.BatchSaveReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.batch.Update(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) batchDelete(c *gin.Context) {
	if err := h.batch.Delete(c.Request.Context(), parseInt64(c.Query("id"))); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// === StockMove ===

func (h *Handler) stockMovePage(c *gin.Context) {
	q := repository.StockMoveQuery{
		PageNo:    parseInt(c.Query("pageNo"), 1),
		PageSize:  parseInt(c.Query("pageSize"), 10),
		ProductID: parseInt64Ptr(c.Query("productId")),
		BatchID:   parseInt64Ptr(c.Query("batchId")),
		MoveType:  parseInt8Ptr(c.Query("moveType")),
	}
	res, err := h.stockMove.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, res)
}

func (h *Handler) stockMoveCreate(c *gin.Context) {
	var req dto.StockMoveReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.stockMove.Record(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

// === Uom ===

func (h *Handler) uomPage(c *gin.Context) {
	q := repository.UomQuery{
		PageNo:   parseInt(c.Query("pageNo"), 1),
		PageSize: parseInt(c.Query("pageSize"), 10),
		Name:     c.Query("name"),
		Code:     c.Query("code"),
		Category: c.Query("category"),
		Status:   parseInt8Ptr(c.Query("status")),
	}
	res, err := h.uom.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, res)
}

func (h *Handler) uomListAll(c *gin.Context) {
	res, err := h.uom.ListAll(c.Request.Context())
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, res)
}

func (h *Handler) uomGet(c *gin.Context) {
	v, err := h.uom.Get(c.Request.Context(), parseInt64(c.Query("id")))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

func (h *Handler) uomCreate(c *gin.Context) {
	var req dto.UomSaveReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.uom.Create(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) uomUpdate(c *gin.Context) {
	var req dto.UomSaveReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uom.Update(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) uomDelete(c *gin.Context) {
	if err := h.uom.Delete(c.Request.Context(), parseInt64(c.Query("id"))); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// === 通用工具 ===

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		web.Fail(c, errcode.BadRequest.WithMsg("请求参数不正确: "+err.Error()))
		return false
	}
	return true
}

func parseInt(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func parseInt8Ptr(s string) *int8 {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	v := int8(n)
	return &v
}

func parseInt64Ptr(s string) *int64 {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// === Category ===

func (h *Handler) categoryPage(c *gin.Context) {
	q := repository.CategoryQuery{
		PageNo:   parseInt(c.Query("pageNo"), 1),
		PageSize: parseInt(c.Query("pageSize"), 10),
		Name:     c.Query("name"),
		Code:     c.Query("code"),
		Status:   parseInt8Ptr(c.Query("status")),
	}
	page, err := h.category.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, page)
}

func (h *Handler) categoryTree(c *gin.Context) {
	list, err := h.category.Tree(c.Request.Context())
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, list)
}

func (h *Handler) categoryGet(c *gin.Context) {
	id := parseInt64(c.Query("id"))
	v, err := h.category.Get(c.Request.Context(), id)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

func (h *Handler) categoryCreate(c *gin.Context) {
	var req dto.CategoryCreateReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.category.Create(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) categoryUpdate(c *gin.Context) {
	var req dto.CategoryUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.category.Update(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) categoryDelete(c *gin.Context) {
	id := parseInt64(c.Query("id"))
	if err := h.category.Delete(c.Request.Context(), id); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// === Attribute ===

func (h *Handler) attributePage(c *gin.Context) {
	q := repository.AttributeQuery{
		PageNo:     parseInt(c.Query("pageNo"), 1),
		PageSize:   parseInt(c.Query("pageSize"), 10),
		Name:       c.Query("name"),
		Code:       c.Query("code"),
		CategoryID: parseInt64Ptr(c.Query("categoryId")),
		Status:     parseInt8Ptr(c.Query("status")),
	}
	page, err := h.attribute.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, page)
}

func (h *Handler) attributeGet(c *gin.Context) {
	id := parseInt64(c.Query("id"))
	v, err := h.attribute.Get(c.Request.Context(), id)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

func (h *Handler) attributeListByCategory(c *gin.Context) {
	catID := parseInt64(c.Query("categoryId"))
	list, err := h.attribute.ListByCategory(c.Request.Context(), catID)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, list)
}

func (h *Handler) attributeCreate(c *gin.Context) {
	var req dto.AttributeSaveReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.attribute.Create(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) attributeUpdate(c *gin.Context) {
	var req dto.AttributeSaveReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.attribute.Update(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) attributeDelete(c *gin.Context) {
	id := parseInt64(c.Query("id"))
	if err := h.attribute.Delete(c.Request.Context(), id); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// === Product ===

func (h *Handler) productPage(c *gin.Context) {
	q := repository.ProductQuery{
		PageNo:     parseInt(c.Query("pageNo"), 1),
		PageSize:   parseInt(c.Query("pageSize"), 10),
		Name:       c.Query("name"),
		Code:       c.Query("code"),
		BarCode:    c.Query("barCode"),
		CategoryID: parseInt64Ptr(c.Query("categoryId")),
		TemplateID: parseInt64Ptr(c.Query("templateId")), // 传 -1 → 仅独立 SKU
		Status:     parseInt8Ptr(c.Query("status")),
	}
	page, err := h.product.Page(c.Request.Context(), q)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, page)
}

func (h *Handler) productGet(c *gin.Context) {
	id := parseInt64(c.Query("id"))
	v, err := h.product.Get(c.Request.Context(), id)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, v)
}

// productSearch 动态属性筛选,?categoryId=1&attr_brand=Apple&attr_size=6.1
func (h *Handler) productSearch(c *gin.Context) {
	q := repository.ProductQuery{
		PageNo:     parseInt(c.Query("pageNo"), 1),
		PageSize:   parseInt(c.Query("pageSize"), 10),
		CategoryID: parseInt64Ptr(c.Query("categoryId")),
	}
	if q.CategoryID == nil {
		web.Fail(c, errcode.BadRequest.WithMsg("categoryId 必填"))
		return
	}
	rawFilters := make(map[string]string)
	for key, vals := range c.Request.URL.Query() {
		if !strings.HasPrefix(key, "attr_") || len(vals) == 0 || vals[0] == "" {
			continue
		}
		rawFilters[strings.TrimPrefix(key, "attr_")] = vals[0]
	}
	filters, err := h.product.ResolveAttrFiltersFromQuery(c.Request.Context(), *q.CategoryID, rawFilters)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	page, err := h.product.SearchByAttrFilter(c.Request.Context(), q, filters)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, page)
}

func (h *Handler) productCreate(c *gin.Context) {
	var req dto.ProductSaveReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.product.Create(c.Request.Context(), &req)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, id)
}

func (h *Handler) productUpdate(c *gin.Context) {
	var req dto.ProductSaveReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.product.Update(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) productDelete(c *gin.Context) {
	id := parseInt64(c.Query("id"))
	if err := h.product.Delete(c.Request.Context(), id); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

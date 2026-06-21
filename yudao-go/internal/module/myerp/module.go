// Package myerp 是自用 ERP(EAV 模型)模块的组合根。
// 参考一个 go-zero 微服务原型(service/myerp)移植并重新设计,
// 在本项目以 DDD 4 层架构挂主进程的 /admin-api/myerp/*。
package myerp

import (
	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/service"
	"yudao-go/internal/module/myerp/infrastructure/persistence"
	"yudao-go/internal/module/myerp/interfaces/rest"
)

// Module 持有 myerp 的对外注册入口。
type Module struct {
	handler  *rest.Handler
	catSvc   *service.CategoryService
	attrSvc  *service.AttributeService
	prodSvc  *service.ProductService
	uomSvc   *service.UomService
	batchSvc *service.BatchService
	moveSvc  *service.StockMoveService
	tplSvc   *service.TemplateService
}

// SetAuditor 注入 chatter 字段审计器(组合根调用,可选)。
// 注入后:Update 调用会自动产生 chatter 时间线条目,显示在详情页右侧。
func (m *Module) SetAuditor(a service.Auditor) {
	m.catSvc.SetAuditor(a)
	m.attrSvc.SetAuditor(a)
	m.prodSvc.SetAuditor(a)
	m.uomSvc.SetAuditor(a)
}

// New 装配 myerp 模块的全部依赖。
func New(tx *orm.TxManager) *Module {
	// 仓储
	catRepo := persistence.NewCategoryRepo(tx)
	attrRepo := persistence.NewAttributeRepo(tx)
	optRepo := persistence.NewAttributeOptionRepo(tx)
	prodRepo := persistence.NewProductRepo(tx)
	valRepo := persistence.NewAttrValueRepo(tx)
	uomRepo := persistence.NewUomRepo(tx)
	prodUomRepo := persistence.NewProductUomRepo(tx)
	batchRepo := persistence.NewBatchRepo(tx)
	moveRepo := persistence.NewStockMoveRepo(tx)
	tplRepo := persistence.NewTemplateRepo(tx)
	tplLineRepo := persistence.NewTemplateAttributeLineRepo(tx)

	// 应用服务
	catSvc := service.NewCategoryService(catRepo, prodRepo, tx)
	attrSvc := service.NewAttributeService(attrRepo, optRepo, catRepo, tx)
	prodSvc := service.NewProductService(prodRepo, valRepo, attrRepo, optRepo, catRepo, uomRepo, prodUomRepo, tplRepo, tx)
	uomSvc := service.NewUomService(uomRepo, prodUomRepo, tx)
	batchSvc := service.NewBatchService(batchRepo, prodRepo, uomRepo, tx)
	moveSvc := service.NewStockMoveService(moveRepo, batchRepo, prodRepo, tx)
	tplSvc := service.NewTemplateService(tplRepo, tplLineRepo, attrRepo, optRepo, catRepo, uomRepo, prodRepo, valRepo, tx)

	return &Module{
		handler:  rest.NewHandler(catSvc, attrSvc, prodSvc, uomSvc, batchSvc, moveSvc, tplSvc),
		catSvc:   catSvc,
		attrSvc:  attrSvc,
		prodSvc:  prodSvc,
		uomSvc:   uomSvc,
		batchSvc: batchSvc,
		moveSvc:  moveSvc,
		tplSvc:   tplSvc,
	}
}

// RegisterRoutes 在 /admin-api 分组下挂载 myerp 的全部接口。
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.Register(group)
}

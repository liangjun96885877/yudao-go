// Package repository 定义 myerp 领域仓储接口。实现位于 infrastructure/persistence。
package repository

import (
	"context"

	"yudao-go/internal/module/myerp/domain/model"
)

// CategoryQuery 分类分页查询条件。
type CategoryQuery struct {
	PageNo, PageSize int
	Name, Code       string
	Status           *int8
}

// AttributeQuery 属性分页查询条件。
type AttributeQuery struct {
	PageNo, PageSize int
	CategoryID       *int64
	Name, Code       string
	Status           *int8
}

// ProductQuery 产品分页查询条件。
type ProductQuery struct {
	PageNo, PageSize int
	CategoryID       *int64
	TemplateID       *int64 // 按模板筛变体;传 -1 表示「无模板的独立 SKU」
	Name, Code, BarCode string
	Status           *int8
}

// UomQuery 单位分页查询条件。
type UomQuery struct {
	PageNo, PageSize int
	Name, Code, Category string
	Status           *int8
}

// TemplateQuery 产品模板分页查询条件。
type TemplateQuery struct {
	PageNo, PageSize int
	CategoryID       *int64
	Name, Code       string
	Status           *int8
}

// BatchQuery 批次分页查询条件。
type BatchQuery struct {
	PageNo, PageSize int
	ProductID        *int64
	BatchNo          string
	Status           *int8
}

// StockMoveQuery 出入库流水分页查询条件。
type StockMoveQuery struct {
	PageNo, PageSize int
	ProductID        *int64
	BatchID          *int64
	MoveType         *int8
}

// CategoryRepository 分类仓储。
type CategoryRepository interface {
	Create(ctx context.Context, c *model.Category) error
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetByID(ctx context.Context, id int64) (*model.Category, error)
	GetByCode(ctx context.Context, code string) (*model.Category, error) // 唯一性预检
	DeleteByID(ctx context.Context, id int64) error
	Page(ctx context.Context, q CategoryQuery) ([]*model.Category, int64, error)
	ListAll(ctx context.Context) ([]*model.Category, error) // 用于树构建
	HasChildren(ctx context.Context, id int64) (bool, error)
	// AncestorChain 从 id 向上追到根,返回完整祖先链(含自己,根在最后)。
	// 同时用于循环引用检测:Update 时新 parentId 不能在自己的祖先链里。
	AncestorChain(ctx context.Context, id int64) ([]int64, error)
}

// AttributeRepository 属性仓储。
type AttributeRepository interface {
	Create(ctx context.Context, a *model.Attribute) error
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetByID(ctx context.Context, id int64) (*model.Attribute, error)
	DeleteByID(ctx context.Context, id int64) error
	Page(ctx context.Context, q AttributeQuery) ([]*model.Attribute, int64, error)
	// ListByCategoryIDs 按分类链批量取属性(用于继承合并去重)。
	ListByCategoryIDs(ctx context.Context, categoryIDs []int64) ([]*model.Attribute, error)
}

// AttributeOptionRepository 属性枚举值仓储。
type AttributeOptionRepository interface {
	CreateBatch(ctx context.Context, items []*model.AttributeOption) error
	DeleteByAttribute(ctx context.Context, attributeID int64) error
	ListByAttributeIDs(ctx context.Context, attributeIDs []int64) ([]*model.AttributeOption, error)
}

// ProductRepository 产品仓储。
type ProductRepository interface {
	Create(ctx context.Context, p *model.Product) error
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetByID(ctx context.Context, id int64) (*model.Product, error)
	GetByCode(ctx context.Context, code string) (*model.Product, error)
	DeleteByID(ctx context.Context, id int64) error
	Page(ctx context.Context, q ProductQuery) ([]*model.Product, int64, error)
	CountByCategory(ctx context.Context, categoryID int64) (int64, error)
	CountByTemplate(ctx context.Context, templateID int64) (int64, error)
	ListByTemplate(ctx context.Context, templateID int64) ([]*model.Product, error)
	// AddStock 在产品库存合计上做增量(入正出负),用于流水记账投影。
	AddStock(ctx context.Context, id int64, deltaBase, deltaAux string) error
}

// AttrValueRepository EAV 值仓储。
type AttrValueRepository interface {
	UpsertBatch(ctx context.Context, productID int64, items []*model.ProductAttrValue) error
	DeleteByProduct(ctx context.Context, productID int64) error
	ListByProductIDs(ctx context.Context, productIDs []int64) ([]*model.ProductAttrValue, error)
	// FindProductIDsByAttrFilters 按属性值过滤产品 ID 集合(动态属性筛选)。
	// filters: attributeID -> 期望值;多个属性 AND。
	FindProductIDsByAttrFilters(ctx context.Context, tenantID, categoryID int64, filters map[int64]string) ([]int64, error)
}

// UomRepository 单位字典仓储。
type UomRepository interface {
	Create(ctx context.Context, u *model.Uom) error
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetByID(ctx context.Context, id int64) (*model.Uom, error)
	GetByCode(ctx context.Context, code string) (*model.Uom, error)
	DeleteByID(ctx context.Context, id int64) error
	Page(ctx context.Context, q UomQuery) ([]*model.Uom, int64, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*model.Uom, error) // 批量取(回填名称)
	ListAll(ctx context.Context) ([]*model.Uom, error)               // 下拉选择用
}

// ProductUomRepository 产品多单位换算仓储。
type ProductUomRepository interface {
	UpsertBatch(ctx context.Context, productID int64, items []*model.ProductUom) error
	DeleteByProduct(ctx context.Context, productID int64) error
	ListByProductIDs(ctx context.Context, productIDs []int64) ([]*model.ProductUom, error)
	CountByUom(ctx context.Context, uomID int64) (int64, error) // 删单位前检查是否被引用
}

// TemplateRepository 产品模板(SPU)仓储。
type TemplateRepository interface {
	Create(ctx context.Context, t *model.ProductTemplate) error
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetByID(ctx context.Context, id int64) (*model.ProductTemplate, error)
	GetByCode(ctx context.Context, code string) (*model.ProductTemplate, error)
	DeleteByID(ctx context.Context, id int64) error
	Page(ctx context.Context, q TemplateQuery) ([]*model.ProductTemplate, int64, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*model.ProductTemplate, error)
}

// TemplateAttributeLineRepository 模板区分属性仓储。
type TemplateAttributeLineRepository interface {
	UpsertBatch(ctx context.Context, templateID int64, items []*model.TemplateAttributeLine) error
	DeleteByTemplate(ctx context.Context, templateID int64) error
	ListByTemplateIDs(ctx context.Context, templateIDs []int64) ([]*model.TemplateAttributeLine, error)
}

// BatchRepository 产品批次仓储。
type BatchRepository interface {
	Create(ctx context.Context, b *model.ProductBatch) error
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetByID(ctx context.Context, id int64) (*model.ProductBatch, error)
	GetByNo(ctx context.Context, productID int64, batchNo string) (*model.ProductBatch, error)
	DeleteByID(ctx context.Context, id int64) error
	Page(ctx context.Context, q BatchQuery) ([]*model.ProductBatch, int64, error)
	CountByProduct(ctx context.Context, productID int64) (int64, error) // 删产品前检查
	// AddStock 在批次结存上做增量(入正出负),用于流水记账。
	AddStock(ctx context.Context, id int64, deltaBase, deltaAux string) error
}

// StockMoveRepository 出入库流水仓储(append-only)。
type StockMoveRepository interface {
	Create(ctx context.Context, m *model.StockMove) error
	Page(ctx context.Context, q StockMoveQuery) ([]*model.StockMove, int64, error)
}

// Package persistence 是 myerp 持久化层:PO 定义 + PO↔领域模型转换 + 仓储实现。
package persistence

import (
	"time"

	"yudao-go/internal/framework/orm"
)

// CategoryPO 对应 myerp_category。
type CategoryPO struct {
	orm.TenantModel
	Name               string  `gorm:"column:name"`
	ParentID           int64   `gorm:"column:parent_id"`
	Code               string  `gorm:"column:code"`
	Sort               int     `gorm:"column:sort"`
	Status             int8    `gorm:"column:status"`
	InheritParentAttrs orm.Bit `gorm:"column:inherit_parent_attrs"`
	Description        string  `gorm:"column:description"`
}

func (CategoryPO) TableName() string { return "myerp_category" }

// AttributePO 对应 myerp_attribute。
type AttributePO struct {
	orm.TenantModel
	CategoryID   int64    `gorm:"column:category_id"`
	Code         string   `gorm:"column:code"`
	Name         string   `gorm:"column:name"`
	InputType    string   `gorm:"column:input_type"`
	Unit         string   `gorm:"column:unit"`
	Required     orm.Bit  `gorm:"column:required"`
	Searchable   orm.Bit  `gorm:"column:searchable"`
	ShowInList   orm.Bit  `gorm:"column:show_in_list"`
	IsVariant    bool     `gorm:"column:is_variant"`
	MinValue     *string  `gorm:"column:min_value"` // DECIMAL → 字符串避免精度
	MaxValue     *string  `gorm:"column:max_value"`
	MinLength    *int     `gorm:"column:min_length"`
	MaxLength    int      `gorm:"column:max_length"`
	Regex        string   `gorm:"column:regex"`
	DefaultValue string   `gorm:"column:default_value"`
	Sort         int      `gorm:"column:sort"`
	Status       int8     `gorm:"column:status"`
	Description  string   `gorm:"column:description"`
}

func (AttributePO) TableName() string { return "myerp_attribute" }

// AttributeOptionPO 对应 myerp_attribute_value_option(轻量基,无 updater/deleted)。
type AttributeOptionPO struct {
	LightBase
	AttributeID int64  `gorm:"column:attribute_id"`
	Value       string `gorm:"column:value"`
	Sort        int    `gorm:"column:sort"`
	PriceExtra  string `gorm:"column:price_extra"` // DECIMAL → string
}

func (AttributeOptionPO) TableName() string { return "myerp_attribute_value_option" }

// LightBase 用于仅含 id/tenant_id/create_time 的表。
type LightBase struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   int64     `gorm:"column:tenant_id"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime"`
}

// ProductPO 对应 myerp_product。
type ProductPO struct {
	orm.TenantModel
	CategoryID    int64   `gorm:"column:category_id"`
	TemplateID    *int64  `gorm:"column:template_id"`
	BaseUomID     *int64  `gorm:"column:base_uom_id"`
	UomMode       int8    `gorm:"column:uom_mode"`
	AuxUomID      *int64  `gorm:"column:aux_uom_id"`
	NominalFactor *string `gorm:"column:nominal_factor"` // DECIMAL → *string(可空)
	TolerancePct  string  `gorm:"column:tolerance_pct"`
	BatchTracked  bool    `gorm:"column:batch_tracked"`
	Code          string  `gorm:"column:code"`
	Name          string  `gorm:"column:name"`
	BarCode       string  `gorm:"column:bar_code"`
	PicURL        string  `gorm:"column:pic_url"`
	Description   string  `gorm:"column:description"`
	PurchasePrice string  `gorm:"column:purchase_price"` // DECIMAL → string
	SalePrice     string  `gorm:"column:sale_price"`
	Stock         string  `gorm:"column:stock"`
	StockAux      string  `gorm:"column:stock_aux"`
	Status        int8    `gorm:"column:status"`
	OwnerUserID   *int64  `gorm:"column:owner_user_id"`
}

func (ProductPO) TableName() string { return "myerp_product" }

// ProductBatchPO 对应 myerp_product_batch。
type ProductBatchPO struct {
	orm.TenantModel
	ProductID    int64      `gorm:"column:product_id"`
	BatchNo      string     `gorm:"column:batch_no"`
	ActualFactor *string    `gorm:"column:actual_factor"`
	StockBase    string     `gorm:"column:stock_base"`
	StockAux     string     `gorm:"column:stock_aux"`
	ProduceDate  *time.Time `gorm:"column:produce_date"`
	ExpireDate   *time.Time `gorm:"column:expire_date"`
	Status       int8       `gorm:"column:status"`
	Remark       string     `gorm:"column:remark"`
}

func (ProductBatchPO) TableName() string { return "myerp_product_batch" }

// StockMovePO 对应 myerp_stock_move(账本,仅 id/tenant/create_time,无 updater/deleted)。
type StockMovePO struct {
	LightBase
	ProductID       int64   `gorm:"column:product_id"`
	BatchID         *int64  `gorm:"column:batch_id"`
	MoveType        int8    `gorm:"column:move_type"`
	QtyBase         string  `gorm:"column:qty_base"`
	QtyAux          string  `gorm:"column:qty_aux"`
	EffectiveFactor *string `gorm:"column:effective_factor"`
	BizType         string  `gorm:"column:biz_type"`
	BizID           *int64  `gorm:"column:biz_id"`
	Remark          string  `gorm:"column:remark"`
	Creator         string  `gorm:"column:creator"`
}

func (StockMovePO) TableName() string { return "myerp_stock_move" }

// ProductTemplatePO 对应 myerp_product_template。
type ProductTemplatePO struct {
	orm.TenantModel
	Name        string `gorm:"column:name"`
	Code        string `gorm:"column:code"`
	CategoryID  int64  `gorm:"column:category_id"`
	BaseUomID   *int64 `gorm:"column:base_uom_id"`
	BasePrice   string `gorm:"column:base_price"`
	Description string `gorm:"column:description"`
	Status      int8   `gorm:"column:status"`
}

func (ProductTemplatePO) TableName() string { return "myerp_product_template" }

// TemplateAttributeLinePO 对应 myerp_template_attribute_line(轻量,无 updater/deleted)。
type TemplateAttributeLinePO struct {
	LightBase
	TemplateID  int64 `gorm:"column:template_id"`
	AttributeID int64 `gorm:"column:attribute_id"`
	Sort        int   `gorm:"column:sort"`
}

func (TemplateAttributeLinePO) TableName() string { return "myerp_template_attribute_line" }

// UomPO 对应 myerp_uom。
type UomPO struct {
	orm.TenantModel
	Name        string `gorm:"column:name"`
	Code        string `gorm:"column:code"`
	Category    string `gorm:"column:category"`
	Sort        int    `gorm:"column:sort"`
	Status      int8   `gorm:"column:status"`
	Description string `gorm:"column:description"`
}

func (UomPO) TableName() string { return "myerp_uom" }

// ProductUomPO 对应 myerp_product_uom(产品多单位换算)。
// is_purchase/is_sale 在 SQL 是 TINYINT(非 BIT),用 bool 让 GORM 自动 ↔ 0/1。
type ProductUomPO struct {
	orm.TenantModel
	ProductID  int64  `gorm:"column:product_id"`
	UomID      int64  `gorm:"column:uom_id"`
	Factor     string `gorm:"column:factor"` // DECIMAL → string
	IsPurchase bool   `gorm:"column:is_purchase"`
	IsSale     bool   `gorm:"column:is_sale"`
	Sort       int    `gorm:"column:sort"`
}

func (ProductUomPO) TableName() string { return "myerp_product_uom" }

// ProductAttrValuePO 对应 myerp_product_attr_value(EAV 值)。
type ProductAttrValuePO struct {
	LightBase
	ProductID     int64      `gorm:"column:product_id"`
	AttributeID   int64      `gorm:"column:attribute_id"`
	AttributeCode string     `gorm:"column:attribute_code"`
	Value         string     `gorm:"column:value"`
	ValueDecimal  *string    `gorm:"column:value_decimal"`
	ValueDate     *time.Time `gorm:"column:value_date"`
	ValueBool     *orm.Bit   `gorm:"column:value_bool"`
}

func (ProductAttrValuePO) TableName() string { return "myerp_product_attr_value" }

// Package model 是 myerp 模块的领域模型。
//
// EAV(Entity-Attribute-Value)5 元组:
//   - Category(分类树) + Attribute(属性定义) + AttributeOption(枚举选项)
//     + Product(产品主表) + ProductAttrValue(EAV 值)
//
// 设计要点(沿用源项目通过 10 项安全审查的约束):
//   - 所有聚合带 TenantID(行级多租户)
//   - 校验规则在 Attribute 上(input_type/min/max/regex 等)
//   - Product 通过 CategoryID 关联属性集合,实际值落在 ProductAttrValue
package model

import "time"

// InputType 是 Attribute 支持的输入类型(同步源项目 9 种)。
type InputType string

const (
	InputText        InputType = "text"
	InputNumber      InputType = "number"
	InputSelect      InputType = "select"
	InputMultiSelect InputType = "multi_select"
	InputBool        InputType = "bool"
	InputDate        InputType = "date"
	InputDateTime    InputType = "datetime"
	InputURL         InputType = "url"
	InputColor       InputType = "color"
)

// ValidInputTypes 白名单(防止从 API 注入未知 input_type)。
var ValidInputTypes = map[InputType]bool{
	InputText: true, InputNumber: true, InputSelect: true, InputMultiSelect: true,
	InputBool: true, InputDate: true, InputDateTime: true, InputURL: true, InputColor: true,
}

// Category 分类(树形)。
type Category struct {
	ID                 int64
	TenantID           int64
	Name               string
	ParentID           int64
	Code               string
	Sort               int
	Status             int8 // 0=启用 1=停用
	InheritParentAttrs bool
	Description        string
	Creator, Updater   string
	CreateTime         time.Time
	UpdateTime         time.Time
}

// Attribute 属性定义。
type Attribute struct {
	ID           int64
	TenantID     int64
	CategoryID   int64
	Code         string
	Name         string
	InputType    InputType
	Unit         string
	Required     bool
	Searchable   bool
	ShowInList   bool
	IsVariant    bool   // true=区分属性(可驱动变体生成)false=描述属性
	MinValue     *string // 用 string 避免 NULL ↔ 0 歧义
	MaxValue     *string
	MinLength    *int
	MaxLength    int // 默认 1024
	Regex        string
	DefaultValue string
	Sort         int
	Status       int8
	Description  string
	Creator      string
	Updater      string
	CreateTime   time.Time
	UpdateTime   time.Time
}

// AttributeOption 属性枚举选项(select/multi_select 用)。
type AttributeOption struct {
	ID          int64
	TenantID    int64
	AttributeID int64
	Value       string
	Sort        int
	PriceExtra  string // 选中此项的加价(变体售价 = template.base_price + Σ price_extra)
	CreateTime  time.Time
}

// Product 产品主表(SKU 层)。
type Product struct {
	ID            int64
	TenantID      int64
	CategoryID    int64
	TemplateID    *int64 // 所属模板;null=独立 SKU,非 null=模板变体
	BaseUomID     *int64 // 主/基本计量单位(库存单位)id → Uom
	UomMode       int8   // 换算方式 0=固定 1=浮动双计量
	AuxUomID      *int64 // 辅计量单位(异重单位,如克)→ Uom,仅浮动模式
	NominalFactor *string // 名义换算率 1主≈N辅(默认值+校验基准)
	TolerancePct  string  // 允许偏差% 0=不校验
	BatchTracked  bool    // 是否按批次管理库存(浮动产品有效;false=batch-less 随机重量)
	Code          string
	Name          string
	BarCode       string
	PicURL        string
	Description   string
	PurchasePrice string // DECIMAL → 用 string 避免精度丢失,序列化兼容
	SalePrice     string
	Stock         string // 主计量库存合计
	StockAux      string // 辅计量库存合计(浮动模式)
	Status        int8
	OwnerUserID   *int64
	Creator       string
	Updater       string
	CreateTime    time.Time
	UpdateTime    time.Time
}

// UomMode 换算方式枚举。
const (
	UomModeFixed = 0 // 固定:单数量 + product_uom.factor
	UomModeFloat = 1 // 浮动双计量:主+辅两列独立结存,经批次/流水记账
)

// StockMove 类型枚举。
const (
	MoveIn     int8 = 1 // 入库
	MoveOut    int8 = 2 // 出库
	MoveAdjust int8 = 3 // 盘点调整
)

// ProductBatch 产品批次(浮动双计量产品的库存载体)。
// 「因批而异」的实测换算率落在 ActualFactor(这批 10.0、下批 10.2)。
type ProductBatch struct {
	ID           int64
	TenantID     int64
	ProductID    int64
	BatchNo      string
	ActualFactor *string // 该批实测换算率
	StockBase    string  // 该批主计量结存
	StockAux     string  // 该批辅计量结存
	ProduceDate  *time.Time
	ExpireDate   *time.Time
	Status       int8 // 0=正常 1=冻结
	Remark       string
	Creator      string
	Updater      string
	CreateTime   time.Time
	UpdateTime   time.Time
}

// ProductTemplate 产品模板(SPU,借鉴 Odoo product.template,精简版)。
// 一个模板挂 N 个变体(Product),共享名称/分类/单位/基础售价等;
// 变体之间靠区分属性(Attribute 中 IsVariant=true)+ EAV 值不同来区分。
type ProductTemplate struct {
	ID          int64
	TenantID    int64
	Name        string
	Code        string
	CategoryID  int64
	BaseUomID   *int64
	BasePrice   string // 模板基础售价(变体最终 = base + Σ option.price_extra)
	Description string
	Status      int8
	Creator     string
	Updater     string
	CreateTime  time.Time
	UpdateTime  time.Time
}

// TemplateAttributeLine 模板用了哪些区分属性。
type TemplateAttributeLine struct {
	ID          int64
	TenantID    int64
	TemplateID  int64
	AttributeID int64
	Sort        int
	CreateTime  time.Time
}

// StockMove 出入库流水(账本,不可改不可删)。库存 = 流水之和。
type StockMove struct {
	ID              int64
	TenantID        int64
	ProductID       int64
	BatchID         *int64
	MoveType        int8
	QtyBase         string  // 主计量变动(入正出负)
	QtyAux          string  // 辅计量变动
	EffectiveFactor *string // 本次实际换算率 |qty_aux/qty_base|
	BizType         string
	BizID           *int64
	Remark          string
	Creator         string
	CreateTime      time.Time
}

// Uom 单位字典(颗/斤/箱/千克)。
type Uom struct {
	ID          int64
	TenantID    int64
	Name        string
	Code        string
	Category    string // count/weight/length…,仅分组展示,不强制同类换算
	Sort        int
	Status      int8
	Description string
	Creator     string
	Updater     string
	CreateTime  time.Time
	UpdateTime  time.Time
}

// ProductUom 产品多单位换算。换算公式:基本单位数量 = 辅助单位数量 × Factor。
// 螺丝:base=颗;辅助单位 斤 Factor=50 → 1 斤 = 50 颗。
type ProductUom struct {
	ID         int64
	TenantID   int64
	ProductID  int64
	UomID      int64
	Factor     string // DECIMAL → string,避免精度丢失
	IsPurchase bool   // 默认采购单位
	IsSale     bool   // 默认销售单位
	Sort       int
	CreateTime time.Time
}

// ProductAttrValue 产品-属性值(EAV 核心)。
// 一个 (Product, Attribute) 唯一一行。
type ProductAttrValue struct {
	ID            int64
	TenantID      int64
	ProductID     int64
	AttributeID   int64
	AttributeCode string  // 冗余便于不 join 取值
	Value         string  // 主存储(<=1024 chars)
	ValueDecimal  *string // number 类型冗余,范围筛选
	ValueDate     *time.Time
	ValueBool     *bool
	CreateTime    time.Time
}

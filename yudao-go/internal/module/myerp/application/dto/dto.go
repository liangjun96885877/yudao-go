// Package dto 是 myerp 应用层入参/出参类型。
package dto

// === Category ===

type CategoryCreateReq struct {
	Name               string `json:"name"`
	ParentID           int64  `json:"parentId"`
	Code               string `json:"code"`
	Sort               int    `json:"sort"`
	Status             int8   `json:"status"`
	InheritParentAttrs bool   `json:"inheritParentAttrs"`
	Description        string `json:"description"`
}

type CategoryUpdateReq struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	ParentID           int64  `json:"parentId"`
	Code               string `json:"code"`
	Sort               int    `json:"sort"`
	Status             int8   `json:"status"`
	InheritParentAttrs bool   `json:"inheritParentAttrs"`
	Description        string `json:"description"`
}

type CategoryDTO struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	ParentID           int64  `json:"parentId"`
	Code               string `json:"code"`
	Sort               int    `json:"sort"`
	Status             int8   `json:"status"`
	InheritParentAttrs bool   `json:"inheritParentAttrs"`
	Description        string `json:"description"`
	CreateTime         string `json:"createTime"`
}

// === Attribute ===

// AttributeOptionReq 枚举选项录入项(值 + 加价)。
type AttributeOptionReq struct {
	Value      string `json:"value"`
	PriceExtra string `json:"priceExtra"` // 选中此项加价(变体售价计算用)
}

// AttributeOptionDTO 枚举选项回显项。
type AttributeOptionDTO struct {
	Value      string `json:"value"`
	PriceExtra string `json:"priceExtra"`
}

type AttributeSaveReq struct {
	ID           int64                `json:"id"`
	CategoryID   int64                `json:"categoryId"`
	Code         string               `json:"code"`
	Name         string               `json:"name"`
	InputType    string               `json:"inputType"`
	Unit         string               `json:"unit"`
	Required     bool                 `json:"required"`
	Searchable   bool                 `json:"searchable"`
	ShowInList   bool                 `json:"showInList"`
	IsVariant    bool                 `json:"isVariant"` // true=区分属性(可驱动变体生成)
	MinValue     *string              `json:"minValue"`
	MaxValue     *string              `json:"maxValue"`
	MinLength    *int                 `json:"minLength"`
	MaxLength    int                  `json:"maxLength"`
	Regex        string               `json:"regex"`
	DefaultValue string               `json:"defaultValue"`
	Sort         int                  `json:"sort"`
	Status       int8                 `json:"status"`
	Description  string               `json:"description"`
	Options      []AttributeOptionReq `json:"options"` // select/multi_select 一并提交枚举(含 priceExtra)
}

type AttributeDTO struct {
	ID           int64                `json:"id"`
	CategoryID   int64                `json:"categoryId"`
	Code         string               `json:"code"`
	Name         string               `json:"name"`
	InputType    string               `json:"inputType"`
	Unit         string               `json:"unit"`
	Required     bool                 `json:"required"`
	Searchable   bool                 `json:"searchable"`
	ShowInList   bool                 `json:"showInList"`
	IsVariant    bool                 `json:"isVariant"`
	MinValue     *string              `json:"minValue"`
	MaxValue     *string              `json:"maxValue"`
	MinLength    *int                 `json:"minLength"`
	MaxLength    int                  `json:"maxLength"`
	Regex        string               `json:"regex"`
	DefaultValue string               `json:"defaultValue"`
	Sort         int                  `json:"sort"`
	Status       int8                 `json:"status"`
	Description  string               `json:"description"`
	Options      []AttributeOptionDTO `json:"options,omitempty"` // 只在 Get/ListByCategory 返回
	CreateTime   string               `json:"createTime"`
}

// === Product ===

// ProductUomReq 产品多单位换算项(创建/更新产品时一并提交)。
type ProductUomReq struct {
	UomID      int64  `json:"uomId"`
	Factor     string `json:"factor"` // 1 辅助单位 = factor 基本单位
	IsPurchase bool   `json:"isPurchase"`
	IsSale     bool   `json:"isSale"`
}

// ProductUomDTO 产品多单位换算回显(含单位名称)。
type ProductUomDTO struct {
	UomID      int64  `json:"uomId"`
	UomName    string `json:"uomName"`
	UomCode    string `json:"uomCode"`
	Factor     string `json:"factor"`
	IsPurchase bool   `json:"isPurchase"`
	IsSale     bool   `json:"isSale"`
}

type ProductSaveReq struct {
	ID            int64           `json:"id"`
	CategoryID    int64           `json:"categoryId"`
	TemplateID    *int64          `json:"templateId"` // 所属模板;null=独立 SKU
	BaseUomID     *int64          `json:"baseUomId"`
	UomMode       int8            `json:"uomMode"`       // 0=固定 1=浮动双计量
	AuxUomID      *int64          `json:"auxUomId"`      // 辅计量单位
	NominalFactor *string         `json:"nominalFactor"` // 名义换算率
	TolerancePct  string          `json:"tolerancePct"`  // 允许偏差%
	BatchTracked  bool            `json:"batchTracked"`  // 是否按批次管理(浮动)
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	BarCode       string          `json:"barCode"`
	PicURL        string          `json:"picUrl"`
	Description   string          `json:"description"`
	PurchasePrice string          `json:"purchasePrice"`
	SalePrice     string          `json:"salePrice"`
	Stock         string          `json:"stock"`
	Status        int8            `json:"status"`
	OwnerUserID   *int64          `json:"ownerUserId"`
	AttrValues    map[string]any  `json:"attrValues"` // key=attribute_code
	Uoms          []ProductUomReq `json:"uoms"`       // 多单位换算
}

type ProductDTO struct {
	ID            int64           `json:"id"`
	CategoryID    int64           `json:"categoryId"`
	TemplateID    *int64          `json:"templateId"`
	TemplateName  string          `json:"templateName"` // 回填便于列表展示
	BaseUomID     *int64          `json:"baseUomId"`
	BaseUomName   string          `json:"baseUomName"` // 基本/主计量单位名称(回填)
	UomMode       int8            `json:"uomMode"`
	AuxUomID      *int64          `json:"auxUomId"`
	AuxUomName    string          `json:"auxUomName"` // 辅计量单位名称(回填)
	NominalFactor *string         `json:"nominalFactor"`
	TolerancePct  string          `json:"tolerancePct"`
	BatchTracked  bool            `json:"batchTracked"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	BarCode       string          `json:"barCode"`
	PicURL        string          `json:"picUrl"`
	Description   string          `json:"description"`
	PurchasePrice string          `json:"purchasePrice"`
	SalePrice     string          `json:"salePrice"`
	Stock         string          `json:"stock"`     // 主计量库存合计
	StockAux      string          `json:"stockAux"`  // 辅计量库存合计
	Status        int8            `json:"status"`
	OwnerUserID   *int64          `json:"ownerUserId"`
	Attrs         map[string]any  `json:"attrs"`  // 回填 EAV 值
	Uoms          []ProductUomDTO `json:"uoms"`   // 多单位换算
	CreateTime    string          `json:"createTime"`
}

// === Uom ===

type UomSaveReq struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Category    string `json:"category"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Description string `json:"description"`
}

type UomDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Category    string `json:"category"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Description string `json:"description"`
	CreateTime  string `json:"createTime"`
}

// === ProductBatch(批次) ===

type BatchSaveReq struct {
	ID           int64   `json:"id"`
	ProductID    int64   `json:"productId"`
	BatchNo      string  `json:"batchNo"`
	ActualFactor *string `json:"actualFactor"`
	ProduceDate  string  `json:"produceDate"` // YYYY-MM-DD
	ExpireDate   string  `json:"expireDate"`
	Status       int8    `json:"status"`
	Remark       string  `json:"remark"`
}

type BatchDTO struct {
	ID           int64   `json:"id"`
	ProductID    int64   `json:"productId"`
	ProductName  string  `json:"productName"`
	BatchNo      string  `json:"batchNo"`
	ActualFactor *string `json:"actualFactor"`
	StockBase    string  `json:"stockBase"`
	StockAux     string  `json:"stockAux"`
	BaseUomName  string  `json:"baseUomName"`
	AuxUomName   string  `json:"auxUomName"`
	ProduceDate  string  `json:"produceDate"`
	ExpireDate   string  `json:"expireDate"`
	Status       int8    `json:"status"`
	Remark       string  `json:"remark"`
	CreateTime   string  `json:"createTime"`
}

// === StockMove(出入库流水) ===

// StockMoveReq 录一笔出入库。qtyBase/qtyAux 传正数,方向由 moveType 决定;
// moveType=3(调整)时 qtyBase 可为带符号的增量。
type StockMoveReq struct {
	ProductID int64  `json:"productId"`
	BatchID   *int64 `json:"batchId"`
	MoveType  int8   `json:"moveType"` // 1=入库 2=出库 3=调整
	QtyBase   string `json:"qtyBase"`
	QtyAux    string `json:"qtyAux"`
	Remark    string `json:"remark"`
}

type StockMoveDTO struct {
	ID              int64   `json:"id"`
	ProductID       int64   `json:"productId"`
	BatchID         *int64  `json:"batchId"`
	BatchNo         string  `json:"batchNo"`
	MoveType        int8    `json:"moveType"`
	QtyBase         string  `json:"qtyBase"`
	QtyAux          string  `json:"qtyAux"`
	EffectiveFactor *string `json:"effectiveFactor"`
	Remark          string  `json:"remark"`
	Creator         string  `json:"creator"`
	CreateTime      string  `json:"createTime"`
}

// === ProductTemplate(SPU 模板)===

// TemplateAttributeLineReq 模板区分属性配置(只需 attribute id 集合)。
type TemplateAttributeLineReq struct {
	AttributeID int64 `json:"attributeId"`
	Sort        int   `json:"sort"`
}

// TemplateAttributeLineDTO 模板区分属性回显(含属性名 + 该属性所有选项)。
type TemplateAttributeLineDTO struct {
	AttributeID   int64                `json:"attributeId"`
	AttributeName string               `json:"attributeName"`
	AttributeCode string               `json:"attributeCode"`
	Sort          int                  `json:"sort"`
	Options       []AttributeOptionDTO `json:"options"` // 该属性可选项(含 priceExtra)
}

type TemplateSaveReq struct {
	ID             int64                      `json:"id"`
	Name           string                     `json:"name"`
	Code           string                     `json:"code"`
	CategoryID     int64                      `json:"categoryId"`
	BaseUomID      *int64                     `json:"baseUomId"`
	BasePrice      string                     `json:"basePrice"`
	Description    string                     `json:"description"`
	Status         int8                       `json:"status"`
	AttributeLines []TemplateAttributeLineReq `json:"attributeLines"`
}

type TemplateDTO struct {
	ID             int64                      `json:"id"`
	Name           string                     `json:"name"`
	Code           string                     `json:"code"`
	CategoryID     int64                      `json:"categoryId"`
	CategoryName   string                     `json:"categoryName"`
	BaseUomID      *int64                     `json:"baseUomId"`
	BaseUomName    string                     `json:"baseUomName"`
	BasePrice      string                     `json:"basePrice"`
	Description    string                     `json:"description"`
	Status         int8                       `json:"status"`
	VariantCount   int64                      `json:"variantCount"`
	AttributeLines []TemplateAttributeLineDTO `json:"attributeLines"`
	CreateTime     string                     `json:"createTime"`
}

// GenerateVariantsReq 按属性组合生成 SKU。
// selections: map[attributeId][]optionValue —— 用户选了每个区分属性下哪些值参与组合。
type GenerateVariantsReq struct {
	TemplateID int64              `json:"templateId"`
	Selections map[string][]string `json:"selections"` // key=attributeId(json int 不便用,字符串化)
}

type GenerateVariantsResp struct {
	Created  int   `json:"created"`
	Skipped  int   `json:"skipped"` // 已存在的组合跳过
	VariantIDs []int64 `json:"variantIds"`
}

// === 通用分页 ===

type Page[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

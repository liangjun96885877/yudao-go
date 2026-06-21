// myerp 模块类型定义,与后端 dto 对齐。

export interface MyErpCategory {
  id: number
  name: string
  parentId: number
  code: string
  sort: number
  status: number // 0 启用 / 1 停用
  inheritParentAttrs: boolean
  description: string
  createTime: string
}

export type MyErpInputType =
  | 'text'
  | 'number'
  | 'select'
  | 'multi_select'
  | 'bool'
  | 'date'
  | 'datetime'
  | 'url'
  | 'color'

// 枚举选项(含 priceExtra)
export interface MyErpAttributeOption {
  value: string
  priceExtra: string // 加价(变体售价计算用)
}

export interface MyErpAttribute {
  id: number
  categoryId: number
  code: string
  name: string
  inputType: MyErpInputType
  unit: string
  required: boolean
  searchable: boolean
  showInList: boolean
  isVariant: boolean // true=区分属性(可驱动变体生成)
  minValue: string | null
  maxValue: string | null
  minLength: number | null
  maxLength: number
  regex: string
  defaultValue: string
  sort: number
  status: number
  description: string
  options?: MyErpAttributeOption[]
  createTime: string
}

// 单位字典
export interface MyErpUom {
  id: number
  name: string
  code: string
  category: string // count/weight/length…
  sort: number
  status: number
  description: string
  createTime: string
}

// 产品多单位换算项
export interface ProductUom {
  uomId: number
  uomName?: string
  uomCode?: string
  factor: string // 1 辅助单位 = factor 基本单位
  isPurchase: boolean
  isSale: boolean
}

export interface MyErpProduct {
  id: number
  categoryId: number
  templateId: number | null
  templateName?: string
  baseUomId: number | null
  baseUomName?: string
  uomMode: number // 0=固定 1=浮动双计量
  auxUomId: number | null
  auxUomName?: string
  nominalFactor: string | null
  tolerancePct: string
  batchTracked: boolean // 是否按批次管理(浮动产品有效)
  code: string
  name: string
  barCode: string
  picUrl: string
  description: string
  purchasePrice: string
  salePrice: string
  stock: string // 主计量库存合计
  stockAux: string // 辅计量库存合计
  status: number
  ownerUserId: number | null
  attrs: Record<string, any>
  uoms?: ProductUom[]
  createTime: string
}

// 产品批次(浮动双计量产品的库存载体)
export interface MyErpBatch {
  id: number
  productId: number
  productName?: string
  batchNo: string
  actualFactor: string | null // 该批实测换算率
  stockBase: string
  stockAux: string
  baseUomName?: string
  auxUomName?: string
  produceDate: string
  expireDate: string
  status: number // 0=正常 1=冻结
  remark: string
  createTime: string
}

// 产品模板(SPU)
export interface MyErpTemplate {
  id: number
  name: string
  code: string
  categoryId: number
  categoryName?: string
  baseUomId: number | null
  baseUomName?: string
  basePrice: string
  description: string
  status: number
  variantCount: number
  attributeLines: MyErpTemplateAttributeLine[]
  createTime: string
}

export interface MyErpTemplateAttributeLine {
  attributeId: number
  attributeName: string
  attributeCode: string
  sort: number
  options: MyErpAttributeOption[]
}

// 出入库流水
export interface MyErpStockMove {
  id: number
  productId: number
  batchId: number | null
  batchNo?: string
  moveType: number // 1=入库 2=出库 3=调整
  qtyBase: string
  qtyAux: string
  effectiveFactor: string | null
  remark: string
  creator: string
  createTime: string
}

export interface Page<T> {
  list: T[]
  total: number
}

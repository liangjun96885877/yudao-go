// AxelorGrid 列定义。
export type AxelorFilterType = 'text' | 'enum' | 'number'

export interface AxelorColumn {
  prop: string
  label: string
  width?: number | string
  minWidth?: number | string
  filter?: AxelorFilterType
  options?: Array<{ label: string; value: any }>
  /** 默认是否显示该列(用户偏好可覆盖) */
  visible?: boolean
  /** 是否对齐(适合金额/数字) */
  align?: 'left' | 'center' | 'right'
}

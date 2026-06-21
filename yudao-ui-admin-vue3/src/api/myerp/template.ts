import request from '@/config/axios'
import type { MyErpProduct, MyErpTemplate, Page } from '@/types/myerp'

export interface TemplatePageQuery {
  pageNo?: number
  pageSize?: number
  categoryId?: number
  name?: string
  code?: string
  status?: number
}

export interface GenerateVariantsReq {
  templateId: number
  /** key=attribute id 字符串,value=该属性下勾选的 value 数组 */
  selections: Record<string, string[]>
}

export interface GenerateVariantsResp {
  created: number
  skipped: number
  variantIds: number[]
}

export const myerpTemplateApi = {
  page: (q: TemplatePageQuery): Promise<Page<MyErpTemplate>> =>
    request.get({ url: '/myerp/template/page', params: q }),
  get: (id: number): Promise<MyErpTemplate> =>
    request.get({ url: '/myerp/template/get', params: { id } }),
  listVariants: (templateId: number): Promise<MyErpProduct[]> =>
    request.get({ url: '/myerp/template/list-variants', params: { templateId } }),
  create: (data: Partial<MyErpTemplate>): Promise<number> =>
    request.post({ url: '/myerp/template/create', data }),
  update: (data: Partial<MyErpTemplate>): Promise<unknown> =>
    request.put({ url: '/myerp/template/update', data }),
  delete: (id: number): Promise<unknown> =>
    request.delete({ url: '/myerp/template/delete', params: { id } }),
  generateVariants: (data: GenerateVariantsReq): Promise<GenerateVariantsResp> =>
    request.post({ url: '/myerp/template/generate-variants', data })
}

import request from '@/config/axios'
import type { MyErpAttribute, Page } from '@/types/myerp'

export interface AttributePageQuery {
  pageNo?: number
  pageSize?: number
  name?: string
  code?: string
  categoryId?: number
  status?: number
}

export const myerpAttributeApi = {
  page: (q: AttributePageQuery): Promise<Page<MyErpAttribute>> =>
    request.get({ url: '/myerp/attribute/page', params: q }),
  get: (id: number): Promise<MyErpAttribute> =>
    request.get({ url: '/myerp/attribute/get', params: { id } }),
  listByCategory: (categoryId: number): Promise<MyErpAttribute[]> =>
    request.get({ url: '/myerp/attribute/list-by-category', params: { categoryId } }),
  create: (data: Partial<MyErpAttribute> & { options?: string[] }): Promise<number> =>
    request.post({ url: '/myerp/attribute/create', data }),
  update: (data: Partial<MyErpAttribute> & { options?: string[] }): Promise<unknown> =>
    request.put({ url: '/myerp/attribute/update', data }),
  delete: (id: number): Promise<unknown> =>
    request.delete({ url: '/myerp/attribute/delete', params: { id } })
}

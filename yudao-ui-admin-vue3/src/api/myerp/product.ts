import request from '@/config/axios'
import type { MyErpProduct, Page } from '@/types/myerp'

export interface ProductPageQuery {
  pageNo?: number
  pageSize?: number
  name?: string
  code?: string
  barCode?: string
  categoryId?: number
  status?: number
}

export interface ProductSearchQuery extends ProductPageQuery {
  /** 动态属性筛选,key 形如 'attr_brand',值为期望的属性值 */
  [attrKey: string]: any
}

export const myerpProductApi = {
  page: (q: ProductPageQuery): Promise<Page<MyErpProduct>> =>
    request.get({ url: '/myerp/product/page', params: q }),
  get: (id: number): Promise<MyErpProduct> =>
    request.get({ url: '/myerp/product/get', params: { id } }),
  search: (q: ProductSearchQuery): Promise<Page<MyErpProduct>> =>
    request.get({ url: '/myerp/product/search', params: q }),
  create: (
    data: Partial<MyErpProduct> & { attrValues?: Record<string, any> }
  ): Promise<number> => request.post({ url: '/myerp/product/create', data }),
  update: (
    data: Partial<MyErpProduct> & { attrValues?: Record<string, any> }
  ): Promise<unknown> => request.put({ url: '/myerp/product/update', data }),
  delete: (id: number): Promise<unknown> =>
    request.delete({ url: '/myerp/product/delete', params: { id } })
}

import request from '@/config/axios'
import type { MyErpCategory, Page } from '@/types/myerp'

export interface CategoryPageQuery {
  pageNo?: number
  pageSize?: number
  name?: string
  code?: string
  status?: number
}

export const myerpCategoryApi = {
  page: (q: CategoryPageQuery): Promise<Page<MyErpCategory>> =>
    request.get({ url: '/myerp/category/page', params: q }),
  tree: (): Promise<MyErpCategory[]> => request.get({ url: '/myerp/category/tree' }),
  get: (id: number): Promise<MyErpCategory> =>
    request.get({ url: '/myerp/category/get', params: { id } }),
  create: (data: Partial<MyErpCategory>): Promise<number> =>
    request.post({ url: '/myerp/category/create', data }),
  update: (data: Partial<MyErpCategory>): Promise<unknown> =>
    request.put({ url: '/myerp/category/update', data }),
  delete: (id: number): Promise<unknown> =>
    request.delete({ url: '/myerp/category/delete', params: { id } })
}

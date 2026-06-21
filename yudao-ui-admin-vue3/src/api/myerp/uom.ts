import request from '@/config/axios'
import type { MyErpUom, Page } from '@/types/myerp'

export interface UomPageQuery {
  pageNo?: number
  pageSize?: number
  name?: string
  code?: string
  category?: string
  status?: number
}

export const myerpUomApi = {
  page: (q: UomPageQuery): Promise<Page<MyErpUom>> =>
    request.get({ url: '/myerp/uom/page', params: q }),
  listAll: (): Promise<MyErpUom[]> => request.get({ url: '/myerp/uom/list-all' }),
  get: (id: number): Promise<MyErpUom> =>
    request.get({ url: '/myerp/uom/get', params: { id } }),
  create: (data: Partial<MyErpUom>): Promise<number> =>
    request.post({ url: '/myerp/uom/create', data }),
  update: (data: Partial<MyErpUom>): Promise<unknown> =>
    request.put({ url: '/myerp/uom/update', data }),
  delete: (id: number): Promise<unknown> =>
    request.delete({ url: '/myerp/uom/delete', params: { id } })
}

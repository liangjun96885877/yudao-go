import request from '@/config/axios'
import type { MyErpBatch, MyErpStockMove, Page } from '@/types/myerp'

export interface BatchPageQuery {
  pageNo?: number
  pageSize?: number
  productId?: number
  batchNo?: string
  status?: number
}

export const myerpBatchApi = {
  page: (q: BatchPageQuery): Promise<Page<MyErpBatch>> =>
    request.get({ url: '/myerp/batch/page', params: q }),
  get: (id: number): Promise<MyErpBatch> =>
    request.get({ url: '/myerp/batch/get', params: { id } }),
  create: (data: Partial<MyErpBatch>): Promise<number> =>
    request.post({ url: '/myerp/batch/create', data }),
  update: (data: Partial<MyErpBatch>): Promise<unknown> =>
    request.put({ url: '/myerp/batch/update', data }),
  delete: (id: number): Promise<unknown> =>
    request.delete({ url: '/myerp/batch/delete', params: { id } })
}

export interface StockMovePageQuery {
  pageNo?: number
  pageSize?: number
  productId?: number
  batchId?: number
  moveType?: number
}

export interface StockMoveReq {
  productId: number
  batchId: number
  moveType: number // 1=入库 2=出库 3=调整
  qtyBase: string
  qtyAux: string
  remark?: string
}

export const myerpStockMoveApi = {
  page: (q: StockMovePageQuery): Promise<Page<MyErpStockMove>> =>
    request.get({ url: '/myerp/stock-move/page', params: q }),
  create: (data: StockMoveReq): Promise<number> =>
    request.post({ url: '/myerp/stock-move/create', data })
}

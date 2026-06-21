import request from '@/config/axios'

// 当前用户的可授出能力集合（用于前端隐藏不可分配的选项）
export interface MyCapabilitiesVO {
  superAdmin: boolean
  roleIds: number[]
  menuIds: number[]
  dataScope: number
  deptIds: number[]
}

export const getMyCapabilities = (): Promise<MyCapabilitiesVO> =>
  request.get({ url: '/system/permission/my-capabilities' })

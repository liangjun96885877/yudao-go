import request from '@/config/axios'

// 角色字段权限 API
export const RoleFieldPermApi = {
  // 查询某角色对各敏感字段的脱敏动作
  listByRole: (roleId: number) =>
    request.get({ url: '/system/role-field-perm/list?roleId=' + roleId }),
  // 保存某角色的字段权限
  save: (data: { roleId: number; items: any[] }) =>
    request.post({ url: '/system/role-field-perm/save', data })
}

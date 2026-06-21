<template>
  <Dialog v-model="dialogVisible" title="字段权限" width="560">
    <el-alert
      class="mb-2"
      type="info"
      :closable="false"
      title="对敏感字段配置该角色可见的形式：明文 / 打码 / 占位符（***）"
    />
    <el-table v-loading="loading" :data="list" border>
      <el-table-column label="字段" prop="label" min-width="140" />
      <el-table-column label="脱敏动作" width="280">
        <template #default="{ row }">
          <el-radio-group v-model="row.action">
            <el-radio v-if="canPlain(row)" value="plain">明文</el-radio>
            <el-radio value="mask">打码</el-radio>
            <el-radio value="hide">占位符</el-radio>
          </el-radio-group>
        </template>
      </el-table-column>
    </el-table>
    <template #footer>
      <el-button type="primary" :disabled="loading" @click="submitForm">确 定</el-button>
      <el-button @click="dialogVisible = false">取 消</el-button>
    </template>
  </Dialog>
</template>
<script lang="ts" setup>
import { RoleFieldPermApi } from '@/api/system/role/fieldPerm'
import { getMyCapabilities } from '@/api/system/permission/capabilities'

defineOptions({ name: 'SystemRoleFieldPermForm' })

const message = useMessage()

const dialogVisible = ref(false)
const loading = ref(false)
const roleId = ref<number>()
const list = ref<any[]>([])

// 防提权：自己有 plain 权限的字段才允许授「明文」给别人
const isSuperAdmin = ref(false)
const myFieldActions = ref<Record<string, string>>({})
const canPlain = (row: any) =>
  isSuperAdmin.value || myFieldActions.value[row.bizType + ':' + row.field] === 'plain'

/** 打开弹窗 */
const open = async (row: any) => {
  dialogVisible.value = true
  roleId.value = row.id
  loading.value = true
  try {
    // 取自己的能力 + 该角色当前配置
    const [cap, items] = await Promise.all([
      getMyCapabilities(),
      RoleFieldPermApi.listByRole(row.id)
    ])
    isSuperAdmin.value = cap.superAdmin
    myFieldActions.value = (cap as any).fieldActions || {}
    list.value = items
  } finally {
    loading.value = false
  }
}
defineExpose({ open })

/** 提交 */
const emit = defineEmits(['success'])
const submitForm = async () => {
  loading.value = true
  try {
    await RoleFieldPermApi.save({ roleId: roleId.value!, items: list.value })
    message.success('保存成功')
    dialogVisible.value = false
    emit('success')
  } finally {
    loading.value = false
  }
}
</script>

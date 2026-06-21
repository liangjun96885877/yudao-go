<script setup lang="ts">
// myerp 单位详情/新建页 —— Axelor 风格 List-Detail 样板。
//   路由:
//     /myerp/uom/create  → mode=create
//     /myerp/uom/:id     → mode=edit
//   单位无 chatter BizType,故 show-chatter=false。
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { AxelorDetail } from '@/components/AxelorStyle'
import { myerpUomApi } from '@/api/myerp/uom'
import { useRecordNav } from '@/composables/useRecordNav'
import type { MyErpUom } from '@/types/myerp'

defineOptions({ name: 'MyErpUomDetail' })

const route = useRoute()
const router = useRouter()

const isCreate = computed(() => route.path.endsWith('/create'))
const id = computed(() => Number(route.params.id))

// 记录导航:从列表存的 id 序列求相邻记录
const nav = useRecordNav('myerp:uom')
const neighbors = computed(() => nav.neighbors(id.value))
const prevTo = computed(() =>
  neighbors.value.prev != null ? `/myerp/uom/${neighbors.value.prev}` : null
)
const nextTo = computed(() =>
  neighbors.value.next != null ? `/myerp/uom/${neighbors.value.next}` : null
)

const CATEGORY_OPTIONS = [
  { label: '数量', value: 'count' },
  { label: '重量', value: 'weight' },
  { label: '长度', value: 'length' },
  { label: '体积', value: 'volume' },
  { label: '其它', value: '' }
]

const form = reactive<Partial<MyErpUom>>({
  name: '',
  code: '',
  category: 'count',
  sort: 0,
  status: 0,
  description: ''
})
const formRef = ref<FormInstance>()
const saving = ref(false)
const loading = ref(false)

const rules: FormRules = {
  name: [{ required: true, message: '请输入单位名称', trigger: 'blur' }],
  code: [
    {
      pattern: /^[a-zA-Z0-9_-]*$/,
      message: '只允许字母、数字、下划线、连字符',
      trigger: 'blur'
    }
  ]
}

const title = computed(() =>
  isCreate.value ? '新建单位' : `单位 #${id.value} ${form.name ? `· ${form.name}` : ''}`
)

async function loadDetail() {
  if (isCreate.value) return
  loading.value = true
  try {
    const u = await myerpUomApi.get(id.value)
    Object.assign(form, u)
  } catch (e: any) {
    ElMessage.error(e?.message || '加载单位失败')
  } finally {
    loading.value = false
  }
}

async function onSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    if (isCreate.value) {
      const newId = await myerpUomApi.create(form)
      ElMessage.success('创建成功')
      router.replace(`/myerp/uom/${newId}`)
    } else {
      await myerpUomApi.update({ ...form, id: id.value })
      ElMessage.success('已保存')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function onDelete() {
  try {
    await myerpUomApi.delete(id.value)
    ElMessage.success('已删除')
    router.replace('/myerp/uom')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

// 复制为新:带当前表单数据进新建页(code 清空避免唯一冲突)
function onCopy() {
  const draft = { ...form, code: '' }
  sessionStorage.setItem('myerp:uom:copy', JSON.stringify(draft))
  router.push('/myerp/uom/create')
}
function onRefresh() {
  loadDetail()
}

onMounted(() => {
  if (isCreate.value) {
    const raw = sessionStorage.getItem('myerp:uom:copy')
    if (raw) {
      try {
        Object.assign(form, JSON.parse(raw))
      } catch {
        /* ignore */
      }
      sessionStorage.removeItem('myerp:uom:copy')
    }
  } else {
    loadDetail()
  }
})
watch(
  () => route.params.id,
  () => !isCreate.value && loadDetail()
)
</script>

<template>
  <AxelorDetail
    :title="title"
    :loading="saving"
    :show-chatter="false"
    :show-delete="!isCreate"
    :show-copy="!isCreate"
    :show-refresh="!isCreate"
    :prev-to="isCreate ? undefined : prevTo"
    :next-to="isCreate ? undefined : nextTo"
    :nav-index="isCreate ? -1 : neighbors.index"
    :nav-total="neighbors.total"
    create-route="/myerp/uom/create"
    back-to="/myerp/uom"
    @save="onSave"
    @delete="onDelete"
    @copy="onCopy"
    @refresh="onRefresh"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="100px"
      v-loading="loading"
    >
      <el-tabs>
        <el-tab-pane label="基本信息">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="名称" prop="name">
                <el-input v-model="form.name" placeholder="如:颗、斤、箱" maxlength="32" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="编码" prop="code">
                <el-input v-model="form.code" placeholder="如:pcs、jin、box" maxlength="32" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="类别">
                <el-select v-model="form.category" placeholder="选择计量类别">
                  <el-option
                    v-for="o in CATEGORY_OPTIONS"
                    :key="o.value"
                    :value="o.value"
                    :label="o.label"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="排序">
                <el-input-number v-model="form.sort" :min="0" :max="9999" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="状态">
                <el-radio-group v-model="form.status">
                  <el-radio :value="0">启用</el-radio>
                  <el-radio :value="1">停用</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="说明">
                <el-input
                  v-model="form.description"
                  type="textarea"
                  :rows="3"
                  maxlength="255"
                  show-word-limit
                />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>
      </el-tabs>
    </el-form>
  </AxelorDetail>
</template>

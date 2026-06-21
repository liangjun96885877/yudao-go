<script setup lang="ts">
// myerp 分类详情/新建页 —— Axelor 风格 List-Detail 样板。
//   路由:
//     /myerp/category/create  → mode=create,无 chatter
//     /myerp/category/:id     → mode=edit,右侧挂 Chatter(biz_type=myerp_category)
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { AxelorDetail } from '@/components/AxelorStyle'
import Chatter from '@/components/chatter/Chatter.vue'
import { myerpCategoryApi } from '@/api/myerp/category'
import { useRecordNav } from '@/composables/useRecordNav'
import type { MyErpCategory } from '@/types/myerp'

defineOptions({ name: 'MyErpCategoryDetail' })

const route = useRoute()
const router = useRouter()

const isCreate = computed(() => route.path.endsWith('/create'))
const id = computed(() => Number(route.params.id))

// 记录导航:从列表存的 id 序列求相邻记录
const nav = useRecordNav('myerp:category')
const neighbors = computed(() => nav.neighbors(id.value))
const prevTo = computed(() =>
  neighbors.value.prev != null ? `/myerp/category/${neighbors.value.prev}` : null
)
const nextTo = computed(() =>
  neighbors.value.next != null ? `/myerp/category/${neighbors.value.next}` : null
)

const form = reactive<Partial<MyErpCategory>>({
  name: '',
  parentId: 0,
  code: '',
  sort: 0,
  status: 0,
  inheritParentAttrs: true,
  description: ''
})
const formRef = ref<FormInstance>()
const saving = ref(false)
const loading = ref(false)
const parentOptions = ref<MyErpCategory[]>([])

const rules: FormRules = {
  name: [{ required: true, message: '请输入分类名称', trigger: 'blur' }],
  code: [
    {
      pattern: /^[a-zA-Z0-9_-]*$/,
      message: '只允许字母、数字、下划线、连字符',
      trigger: 'blur'
    }
  ]
}

const title = computed(() =>
  isCreate.value ? '新建分类' : `分类 #${id.value} ${form.name ? `· ${form.name}` : ''}`
)

async function loadParents() {
  try {
    parentOptions.value = await myerpCategoryApi.tree()
  } catch {
    /* ignore */
  }
}

async function loadDetail() {
  if (isCreate.value) return
  loading.value = true
  try {
    const c = await myerpCategoryApi.get(id.value)
    Object.assign(form, c)
  } catch (e: any) {
    ElMessage.error(e?.message || '加载分类失败')
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
      const newId = await myerpCategoryApi.create(form)
      ElMessage.success('创建成功')
      router.replace(`/myerp/category/${newId}`)
    } else {
      await myerpCategoryApi.update({ ...form, id: id.value })
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
    await myerpCategoryApi.delete(id.value)
    ElMessage.success('已删除')
    router.replace('/myerp/category')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

// 复制为新:带当前表单数据进新建页(code 清空避免唯一冲突)
function onCopy() {
  const draft = { ...form, code: '' }
  sessionStorage.setItem('myerp:category:copy', JSON.stringify(draft))
  router.push('/myerp/category/create')
}
function onRefresh() {
  loadDetail()
}

onMounted(() => {
  loadParents()
  if (isCreate.value) {
    // 复制为新:消费草稿
    const raw = sessionStorage.getItem('myerp:category:copy')
    if (raw) {
      try {
        Object.assign(form, JSON.parse(raw))
      } catch {
        /* ignore */
      }
      sessionStorage.removeItem('myerp:category:copy')
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
    :show-chatter="!isCreate"
    :show-delete="!isCreate"
    :show-copy="!isCreate"
    :show-refresh="!isCreate"
    :prev-to="isCreate ? undefined : prevTo"
    :next-to="isCreate ? undefined : nextTo"
    :nav-index="isCreate ? -1 : neighbors.index"
    :nav-total="neighbors.total"
    create-route="/myerp/category/create"
    back-to="/myerp/category"
    @save="onSave"
    @delete="onDelete"
    @copy="onCopy"
    @refresh="onRefresh"
  >
    <!-- 左侧 form -->
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
                <el-input v-model="form.name" placeholder="如:电子产品" maxlength="64" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="编码" prop="code">
                <el-input v-model="form.code" placeholder="如:electronic" maxlength="32" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="父分类">
                <el-select
                  v-model="form.parentId"
                  placeholder="无(顶层分类)"
                  clearable
                  filterable
                >
                  <el-option :value="0" label="无(顶层)" />
                  <el-option
                    v-for="o in parentOptions.filter((p) => p.id !== id)"
                    :key="o.id"
                    :value="o.id"
                    :label="`${o.name} (#${o.id})`"
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
            <el-col :span="12">
              <el-form-item label="继承父属性">
                <el-switch v-model="form.inheritParentAttrs" />
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

    <!-- 右侧 chatter:仅编辑模式显示 -->
    <template #chatter>
      <Chatter v-if="!isCreate && id" biz-type="myerp_category" :biz-id="id" />
    </template>
  </AxelorDetail>
</template>

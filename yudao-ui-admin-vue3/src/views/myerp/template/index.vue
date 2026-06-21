<script setup lang="ts">
// myerp 产品模板(SPU)列表 —— Axelor 风格。
//   模板下挂 N 个变体(SKU);列表显示模板共享字段 + 变体数。
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { AxelorGrid } from '@/components/AxelorStyle'
import type { AxelorColumn } from '@/components/AxelorStyle'
import { myerpTemplateApi } from '@/api/myerp/template'
import { myerpCategoryApi } from '@/api/myerp/category'
import type { MyErpCategory, MyErpTemplate } from '@/types/myerp'

defineOptions({ name: 'MyErpTemplate' })

const router = useRouter()
const rows = ref<MyErpTemplate[]>([])
const total = ref(0)
const loading = ref(false)
const gridRef = ref<InstanceType<typeof AxelorGrid> | null>(null)

const categories = ref<MyErpCategory[]>([])
const categoryMap = ref<Record<number, string>>({})

const columns: AxelorColumn[] = [
  { prop: 'id', label: '编号', width: 80 },
  { prop: 'name', label: '模板名称', filter: 'text', minWidth: 180 },
  { prop: 'code', label: '编码', filter: 'text', minWidth: 140 },
  { prop: 'categoryName', label: '分类', minWidth: 120 },
  { prop: 'baseUomName', label: '基本单位', width: 100 },
  { prop: 'basePrice', label: '基础售价', width: 110, align: 'right' },
  { prop: 'variantCount', label: '变体数', width: 90, align: 'right' },
  {
    prop: 'status',
    label: '状态',
    width: 90,
    filter: 'enum',
    options: [
      { label: '启用', value: 0 },
      { label: '停用', value: 1 }
    ]
  },
  { prop: 'createTime', label: '创建时间', width: 170, visible: false }
]

async function loadCategories() {
  categories.value = await myerpCategoryApi.tree()
  const m: Record<number, string> = {}
  for (const c of categories.value) m[c.id] = c.name
  categoryMap.value = m
}

async function onQuery(filters: Record<string, any>, page: { pageNo: number; pageSize: number }) {
  loading.value = true
  try {
    const data = await myerpTemplateApi.page({
      ...filters,
      pageNo: page.pageNo,
      pageSize: page.pageSize
    })
    rows.value = data.list
    total.value = data.total
  } catch (e: any) {
    ElMessage.error(e?.message || '加载模板列表失败')
  } finally {
    loading.value = false
  }
}

function onCreate() {
  router.push('/myerp/template/create')
}

async function onDeleteBatch(ids: number[]) {
  let ok = 0
  for (const id of ids) {
    try {
      await myerpTemplateApi.delete(id)
      ok++
    } catch (e: any) {
      ElMessage.error(`#${id}: ${e?.message || '删除失败'}`)
    }
  }
  if (ok > 0) {
    ElMessage.success(`已删除 ${ok} 条`)
    gridRef.value?.refresh()
  }
}

onMounted(loadCategories)
</script>

<template>
  <AxelorGrid
    ref="gridRef"
    :columns="columns"
    :rows="rows"
    :total="total"
    :loading="loading"
    prefs-key="myerp:template:cols"
    nav-key="myerp:template"
    :detail-route="(row: MyErpTemplate) => `/myerp/template/${row.id}`"
    bulk-actions
    @query="onQuery"
    @create="onCreate"
    @delete-batch="onDeleteBatch"
  >
    <template #col-basePrice="{ row }">¥{{ row.basePrice }}</template>
    <template #col-variantCount="{ row }">
      <el-tag size="small" type="info">{{ row.variantCount }} 个</el-tag>
    </template>
    <template #col-status="{ row }">
      <el-tag :type="row.status === 0 ? 'success' : 'info'" size="small">
        {{ row.status === 0 ? '启用' : '停用' }}
      </el-tag>
    </template>
  </AxelorGrid>
</template>

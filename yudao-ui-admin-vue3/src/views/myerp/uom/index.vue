<script setup lang="ts">
// myerp 单位管理列表(扁平,无树形)。
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { AxelorGrid } from '@/components/AxelorStyle'
import type { AxelorColumn } from '@/components/AxelorStyle'
import { myerpUomApi } from '@/api/myerp/uom'
import type { MyErpUom } from '@/types/myerp'

defineOptions({ name: 'MyErpUom' })

const router = useRouter()
const rows = ref<MyErpUom[]>([])
const total = ref(0)
const loading = ref(false)
const gridRef = ref<InstanceType<typeof AxelorGrid> | null>(null)

const CATEGORY_OPTIONS = [
  { label: '数量', value: 'count' },
  { label: '重量', value: 'weight' },
  { label: '长度', value: 'length' },
  { label: '体积', value: 'volume' },
  { label: '其它', value: '' }
]
const categoryLabel = (v: string) =>
  CATEGORY_OPTIONS.find((o) => o.value === v)?.label || v || '—'

const columns: AxelorColumn[] = [
  { prop: 'id', label: '编号', width: 80 },
  { prop: 'name', label: '名称', filter: 'text', minWidth: 140 },
  { prop: 'code', label: '编码', filter: 'text', minWidth: 140 },
  { prop: 'category', label: '类别', width: 120, filter: 'enum', options: CATEGORY_OPTIONS },
  { prop: 'sort', label: '排序', width: 80 },
  {
    prop: 'status',
    label: '状态',
    width: 100,
    filter: 'enum',
    options: [
      { label: '启用', value: 0 },
      { label: '停用', value: 1 }
    ]
  },
  { prop: 'createTime', label: '创建时间', width: 170, visible: false }
]

async function onQuery(filters: Record<string, any>, page: { pageNo: number; pageSize: number }) {
  loading.value = true
  try {
    const data = await myerpUomApi.page({ ...filters, pageNo: page.pageNo, pageSize: page.pageSize })
    rows.value = data.list
    total.value = data.total
  } catch (e: any) {
    ElMessage.error(e?.message || '加载单位列表失败')
  } finally {
    loading.value = false
  }
}

function onCreate() {
  router.push('/myerp/uom/create')
}

async function onDeleteBatch(ids: number[]) {
  let ok = 0
  for (const id of ids) {
    try {
      await myerpUomApi.delete(id)
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
</script>

<template>
  <AxelorGrid
    ref="gridRef"
    :columns="columns"
    :rows="rows"
    :total="total"
    :loading="loading"
    prefs-key="myerp:uom:cols"
    nav-key="myerp:uom"
    :detail-route="(row: MyErpUom) => `/myerp/uom/${row.id}`"
    bulk-actions
    @query="onQuery"
    @create="onCreate"
    @delete-batch="onDeleteBatch"
  >
    <template #col-category="{ row }">{{ categoryLabel(row.category) }}</template>
    <template #col-status="{ row }">
      <el-tag :type="row.status === 0 ? 'success' : 'info'" size="small">
        {{ row.status === 0 ? '启用' : '停用' }}
      </el-tag>
    </template>
  </AxelorGrid>
</template>

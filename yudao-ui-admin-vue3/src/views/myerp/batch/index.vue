<script setup lang="ts">
// myerp 批次管理列表(浮动双计量产品的库存载体)。
//   - 工具栏「按产品」筛选(仅浮动双计量产品有批次)
//   - 主/辅结存带单位显示;实测换算率展示「因批而异」
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { AxelorGrid } from '@/components/AxelorStyle'
import type { AxelorColumn } from '@/components/AxelorStyle'
import { myerpBatchApi } from '@/api/myerp/batch'
import { myerpProductApi } from '@/api/myerp/product'
import type { MyErpBatch, MyErpProduct } from '@/types/myerp'

defineOptions({ name: 'MyErpBatch' })

const route = useRoute()
const router = useRouter()
const rows = ref<MyErpBatch[]>([])
const total = ref(0)
const loading = ref(false)
const gridRef = ref<InstanceType<typeof AxelorGrid> | null>(null)

// 浮动双计量产品(批次只对这些产品存在)
const floatProducts = ref<MyErpProduct[]>([])
// 从「去批次管理」链接带入的产品筛选(setup 同步取,确保首查即带上)
const initProductId = Number(route.query.productId) || undefined
const selectedProductId = ref<number | undefined>(initProductId)

const columns: AxelorColumn[] = [
  { prop: 'id', label: '编号', width: 80 },
  { prop: 'batchNo', label: '批次号', filter: 'text', minWidth: 160 },
  { prop: 'productName', label: '产品', minWidth: 150 },
  { prop: 'stockBase', label: '主计量结存', minWidth: 130, align: 'right' },
  { prop: 'stockAux', label: '辅计量结存', minWidth: 130, align: 'right' },
  { prop: 'actualFactor', label: '实测换算率', width: 120, align: 'right' },
  {
    prop: 'status',
    label: '状态',
    width: 90,
    filter: 'enum',
    options: [
      { label: '正常', value: 0 },
      { label: '冻结', value: 1 }
    ]
  },
  { prop: 'produceDate', label: '生产日期', width: 120, visible: false },
  { prop: 'createTime', label: '创建时间', width: 170, visible: false }
]

async function loadFloatProducts() {
  try {
    const data = await myerpProductApi.page({ pageNo: 1, pageSize: 200 })
    floatProducts.value = data.list.filter((p) => p.uomMode === 1)
  } catch {
    /* ignore */
  }
}

async function onQuery(filters: Record<string, any>, page: { pageNo: number; pageSize: number }) {
  loading.value = true
  try {
    const data = await myerpBatchApi.page({
      ...filters,
      productId: selectedProductId.value,
      pageNo: page.pageNo,
      pageSize: page.pageSize
    })
    rows.value = data.list
    total.value = data.total
  } catch (e: any) {
    ElMessage.error(e?.message || '加载批次列表失败')
  } finally {
    loading.value = false
  }
}

function onProductFilterChange(productId?: number) {
  selectedProductId.value = productId
  gridRef.value?.reset()
}

function onCreate() {
  // 新建批次需带产品:优先用当前筛选的产品
  const q = selectedProductId.value ? `?productId=${selectedProductId.value}` : ''
  router.push(`/myerp/batch/create${q}`)
}

async function onDeleteBatch(ids: number[]) {
  let ok = 0
  for (const id of ids) {
    try {
      await myerpBatchApi.delete(id)
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

onMounted(loadFloatProducts)
</script>

<template>
  <AxelorGrid
    ref="gridRef"
    :columns="columns"
    :rows="rows"
    :total="total"
    :loading="loading"
    prefs-key="myerp:batch:cols"
    nav-key="myerp:batch"
    :detail-route="(row: MyErpBatch) => `/myerp/batch/${row.id}`"
    bulk-actions
    @query="onQuery"
    @create="onCreate"
    @delete-batch="onDeleteBatch"
  >
    <template #toolbar-extra>
      <el-select
        :model-value="selectedProductId"
        placeholder="按产品筛选(仅浮动双计量)"
        clearable
        filterable
        size="small"
        style="width: 240px; margin-left: 8px"
        @update:model-value="onProductFilterChange"
      >
        <el-option
          v-for="p in floatProducts"
          :key="p.id"
          :value="p.id"
          :label="`${p.name} (#${p.id})`"
        />
      </el-select>
    </template>

    <template #col-stockBase="{ row }">
      {{ row.stockBase }}
      <span v-if="row.baseUomName" class="batch-unit">{{ row.baseUomName }}</span>
    </template>
    <template #col-stockAux="{ row }">
      {{ row.stockAux }}
      <span v-if="row.auxUomName" class="batch-unit">{{ row.auxUomName }}</span>
    </template>
    <template #col-actualFactor="{ row }">{{ row.actualFactor ?? '—' }}</template>
    <template #col-status="{ row }">
      <el-tag :type="row.status === 0 ? 'success' : 'warning'" size="small">
        {{ row.status === 0 ? '正常' : '冻结' }}
      </el-tag>
    </template>
  </AxelorGrid>
</template>

<style scoped>
.batch-unit {
  margin-left: 3px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>

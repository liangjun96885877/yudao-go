<script setup lang="ts">
// myerp 产品管理列表 ——
//   - 基础列:编号 / 名称 / 编码 / 分类 / 价格 / 库存 / 状态
//   - 工具栏分类筛选:选分类后,按该分类 show_in_list=true 的属性追加动态列
//   - 选分类后列搜索走 product/search(attr_xxx 维度);未选分类走 product/page
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { AxelorGrid } from '@/components/AxelorStyle'
import type { AxelorColumn } from '@/components/AxelorStyle'
import { myerpProductApi } from '@/api/myerp/product'
import { myerpCategoryApi } from '@/api/myerp/category'
import { myerpAttributeApi } from '@/api/myerp/attribute'
import type { MyErpAttribute, MyErpCategory, MyErpProduct } from '@/types/myerp'

defineOptions({ name: 'MyErpProduct' })

const router = useRouter()
const rows = ref<MyErpProduct[]>([])
const total = ref(0)
const loading = ref(false)
const gridRef = ref<InstanceType<typeof AxelorGrid> | null>(null)

const categories = ref<MyErpCategory[]>([])
const categoryNameMap = ref<Record<number, string>>({})
// 工具栏选中的分类(用于动态列 + 动态筛选)
const selectedCategoryId = ref<number | undefined>(undefined)
// 动态属性列(随选中分类变化)
const dynamicAttrs = ref<MyErpAttribute[]>([])

// 基础列 + 动态属性列拼接
const columns = ref<AxelorColumn[]>([])
function rebuildColumns() {
  const base: AxelorColumn[] = [
    { prop: 'id', label: '编号', width: 80 },
    { prop: 'name', label: '名称', filter: 'text', minWidth: 160 },
    { prop: 'code', label: '编码', filter: 'text', minWidth: 120 },
    { prop: 'categoryId', label: '分类', width: 120 },
    { prop: 'salePrice', label: '销售价', width: 110, align: 'right' },
    { prop: 'templateName', label: '所属模板', width: 140, visible: false },
    { prop: 'stock', label: '库存', width: 120, align: 'right' },
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
  // 动态属性列:prop 用 attr:<code>,在 col- 插槽里从 row.attrs 取值
  const dyn: AxelorColumn[] = dynamicAttrs.value.map((a) => ({
    prop: `attr:${a.code}`,
    label: a.name,
    minWidth: 120,
    // 仅 searchable 的属性允许列内联搜索(走 product/search)
    filter: a.searchable ? 'text' : undefined
  }))
  columns.value = [...base, ...dyn]
}

async function loadCategories() {
  categories.value = await myerpCategoryApi.tree()
  const m: Record<number, string> = {}
  for (const c of categories.value) m[c.id] = c.name
  categoryNameMap.value = m
}

// 工具栏切换分类:重拉该分类的 show_in_list 属性作为动态列
async function onCategoryFilterChange(categoryId?: number) {
  selectedCategoryId.value = categoryId
  if (categoryId) {
    try {
      const all = await myerpAttributeApi.listByCategory(categoryId)
      dynamicAttrs.value = all.filter((a) => a.showInList)
    } catch {
      dynamicAttrs.value = []
    }
  } else {
    dynamicAttrs.value = []
  }
  rebuildColumns()
  gridRef.value?.reset()
}

async function onQuery(filters: Record<string, any>, page: { pageNo: number; pageSize: number }) {
  loading.value = true
  try {
    // 拆出 attr:xxx 维度的筛选 → 走 search;否则走 page
    const attrFilters: Record<string, string> = {}
    const baseFilters: Record<string, any> = {}
    for (const [k, v] of Object.entries(filters)) {
      if (k.startsWith('attr:')) {
        attrFilters[`attr_${k.slice(5)}`] = v
      } else {
        baseFilters[k] = v
      }
    }
    let data
    if (selectedCategoryId.value && Object.keys(attrFilters).length > 0) {
      // 动态属性筛选走 search 端点
      data = await myerpProductApi.search({
        ...baseFilters,
        ...attrFilters,
        categoryId: selectedCategoryId.value,
        pageNo: page.pageNo,
        pageSize: page.pageSize
      })
    } else {
      data = await myerpProductApi.page({
        ...baseFilters,
        categoryId: selectedCategoryId.value,
        pageNo: page.pageNo,
        pageSize: page.pageSize
      })
    }
    rows.value = data.list
    total.value = data.total
  } catch (e: any) {
    ElMessage.error(e?.message || '加载产品列表失败')
  } finally {
    loading.value = false
  }
}

function onCreate() {
  router.push('/myerp/product/create')
}

async function onDeleteBatch(ids: number[]) {
  let ok = 0
  for (const id of ids) {
    try {
      await myerpProductApi.delete(id)
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

// setup 阶段同步初始化基础列,确保 AxelorGrid mounted(早于父 onMounted)时 columns 已就位。
rebuildColumns()

onMounted(async () => {
  await loadCategories()
})
</script>

<template>
  <AxelorGrid
    ref="gridRef"
    :columns="columns"
    :rows="rows"
    :total="total"
    :loading="loading"
    prefs-key="myerp:product:cols"
    nav-key="myerp:product"
    :detail-route="(row: MyErpProduct) => `/myerp/product/${row.id}`"
    bulk-actions
    @query="onQuery"
    @create="onCreate"
    @delete-batch="onDeleteBatch"
  >
    <!-- 工具栏追加:分类筛选(决定动态列) -->
    <template #toolbar-extra>
      <el-select
        :model-value="selectedCategoryId"
        placeholder="按分类(启用动态属性列)"
        clearable
        filterable
        size="small"
        style="width: 220px; margin-left: 8px"
        @update:model-value="onCategoryFilterChange"
      >
        <el-option
          v-for="c in categories"
          :key="c.id"
          :value="c.id"
          :label="`${c.name} (#${c.id})`"
        />
      </el-select>
    </template>

    <!-- 分类列翻译 -->
    <template #col-categoryId="{ row }">
      {{ categoryNameMap[row.categoryId] || `#${row.categoryId}` }}
    </template>
    <!-- 价格 -->
    <template #col-salePrice="{ row }">¥{{ row.salePrice }}</template>
    <!-- 所属模板 -->
    <template #col-templateName="{ row }">
      <el-link
        v-if="row.templateId"
        type="primary"
        :underline="false"
        @click.stop="$router.push(`/myerp/template/${row.templateId}`)"
      >
        {{ row.templateName || `#${row.templateId}` }}
      </el-link>
      <span v-else class="prod-stock-unit">—</span>
    </template>
    <!-- 库存:带基本单位后缀(如 10000 颗) -->
    <template #col-stock="{ row }">
      {{ row.stock }}
      <span v-if="row.baseUomName" class="prod-stock-unit">{{ row.baseUomName }}</span>
    </template>
    <!-- 状态 -->
    <template #col-status="{ row }">
      <el-tag :type="row.status === 0 ? 'success' : 'info'" size="small">
        {{ row.status === 0 ? '启用' : '停用' }}
      </el-tag>
    </template>
    <!-- 动态属性列:prop=attr:<code>,从 row.attrs 取值 -->
    <template
      v-for="a in dynamicAttrs"
      :key="a.code"
      #[`col-attr:${a.code}`]="{ row }"
    >
      <span>{{ (row.attrs || {})[a.code] ?? '—' }}</span>
    </template>
  </AxelorGrid>
</template>

<style scoped>
.prod-stock-unit {
  margin-left: 3px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>

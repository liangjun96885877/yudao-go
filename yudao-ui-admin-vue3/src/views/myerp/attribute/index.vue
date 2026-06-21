<script setup lang="ts">
// myerp 属性管理列表 ——
//   - 用 <AxelorGrid> 紧凑工具栏 + 列内联搜索 + 单图标进详情
//   - 顶部分类过滤(categoryId 维度,select 形式)
//   - 列内联搜索覆盖 name / code,enum 列状态/类型
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Check } from '@element-plus/icons-vue'

import { AxelorGrid } from '@/components/AxelorStyle'
import type { AxelorColumn } from '@/components/AxelorStyle'
import { myerpAttributeApi } from '@/api/myerp/attribute'
import { myerpCategoryApi } from '@/api/myerp/category'
import type { MyErpAttribute, MyErpCategory } from '@/types/myerp'

defineOptions({ name: 'MyErpAttribute' })

const router = useRouter()
const rows = ref<MyErpAttribute[]>([])
const total = ref(0)
const loading = ref(false)
const gridRef = ref<InstanceType<typeof AxelorGrid> | null>(null)

// 分类下拉:加载一次缓存在内存
const categories = ref<MyErpCategory[]>([])

const INPUT_TYPE_OPTIONS = [
  { label: '文本 text', value: 'text' },
  { label: '数字 number', value: 'number' },
  { label: '单选 select', value: 'select' },
  { label: '多选 multi_select', value: 'multi_select' },
  { label: '布尔 bool', value: 'bool' },
  { label: '日期 date', value: 'date' },
  { label: '日期时间 datetime', value: 'datetime' },
  { label: '链接 url', value: 'url' },
  { label: '颜色 color', value: 'color' }
]

const columns: AxelorColumn[] = [
  { prop: 'id', label: '编号', width: 80 },
  { prop: 'name', label: '名称', filter: 'text', minWidth: 140 },
  { prop: 'code', label: '编码', filter: 'text', minWidth: 140 },
  { prop: 'categoryId', label: '所属分类', width: 140 },
  {
    prop: 'inputType',
    label: '类型',
    width: 130,
    filter: 'enum',
    options: INPUT_TYPE_OPTIONS
  },
  { prop: 'unit', label: '单位', width: 80 },
  { prop: 'required', label: '必填', width: 80, align: 'center' },
  { prop: 'showInList', label: '列表显示', width: 100, align: 'center' },
  { prop: 'sort', label: '排序', width: 80, visible: false },
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
  { prop: 'createTime', label: '创建时间', width: 170 }
]

async function loadCategories() {
  try {
    categories.value = await myerpCategoryApi.tree()
  } catch {
    /* ignore */
  }
}

const categoryNameMap = ref<Record<number, string>>({})
function rebuildCategoryNameMap() {
  const m: Record<number, string> = {}
  for (const c of categories.value) m[c.id] = c.name
  categoryNameMap.value = m
}

async function onQuery(filters: Record<string, any>, page: { pageNo: number; pageSize: number }) {
  loading.value = true
  try {
    const data = await myerpAttributeApi.page({
      ...filters,
      pageNo: page.pageNo,
      pageSize: page.pageSize
    })
    rows.value = data.list
    total.value = data.total
  } catch (e: any) {
    ElMessage.error(e?.message || '加载属性列表失败')
  } finally {
    loading.value = false
  }
}

function onCreate() {
  router.push('/myerp/attribute/create')
}

async function onDeleteBatch(ids: number[]) {
  let ok = 0
  for (const id of ids) {
    try {
      await myerpAttributeApi.delete(id)
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

onMounted(async () => {
  await loadCategories()
  rebuildCategoryNameMap()
})
</script>

<template>
  <AxelorGrid
    ref="gridRef"
    :columns="columns"
    :rows="rows"
    :total="total"
    :loading="loading"
    prefs-key="myerp:attribute:cols"
    nav-key="myerp:attribute"
    :detail-route="(row: MyErpAttribute) => `/myerp/attribute/${row.id}`"
    bulk-actions
    @query="onQuery"
    @create="onCreate"
    @delete-batch="onDeleteBatch"
  >
    <!-- 分类列:把 id 翻译为分类名,失败回退 #id -->
    <template #col-categoryId="{ row }">
      {{ categoryNameMap[row.categoryId] || `#${row.categoryId}` }}
    </template>
    <!-- 类型列:用 tag 高亮 -->
    <template #col-inputType="{ row }">
      <el-tag size="small" type="info">{{ row.inputType }}</el-tag>
    </template>
    <!-- bool 字段:✓/× -->
    <template #col-required="{ row }">
      <el-icon v-if="row.required" color="#67c23a"><Check /></el-icon>
      <span v-else class="ax-muted">—</span>
    </template>
    <template #col-showInList="{ row }">
      <el-icon v-if="row.showInList" color="#67c23a"><Check /></el-icon>
      <span v-else class="ax-muted">—</span>
    </template>
    <!-- 状态列 -->
    <template #col-status="{ row }">
      <el-tag :type="row.status === 0 ? 'success' : 'info'" size="small">
        {{ row.status === 0 ? '启用' : '停用' }}
      </el-tag>
    </template>
  </AxelorGrid>
</template>

<style scoped>
.ax-muted {
  color: var(--el-text-color-placeholder);
}
</style>

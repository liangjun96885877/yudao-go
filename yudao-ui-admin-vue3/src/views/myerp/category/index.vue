<script setup lang="ts">
// myerp 分类管理列表 —— Axelor 风格首跑通样板。
//   - 列表用 <AxelorGrid> 紧凑工具栏 + 列内联搜索
//   - 点击行 / ✏ 进 /myerp/category/:id 详情
//   - 顶部 + 按钮进 /myerp/category/create
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Folder, Document } from '@element-plus/icons-vue'

import { AxelorGrid } from '@/components/AxelorStyle'
import type { AxelorColumn } from '@/components/AxelorStyle'
import { myerpCategoryApi } from '@/api/myerp/category'
import type { MyErpCategory } from '@/types/myerp'

defineOptions({ name: 'MyErpCategory' })

const router = useRouter()
const rows = ref<MyErpCategory[]>([])
const total = ref(0)
const loading = ref(false)
const gridRef = ref<InstanceType<typeof AxelorGrid> | null>(null)

// 是否树形展示(无筛选条件时树形,有筛选时平铺过滤)
const isTree = ref(true)

const columns: AxelorColumn[] = [
  // 名称放第一列:树形展开箭头 + 缩进挂在第一数据列,体现父子层级
  { prop: 'name', label: '名称', filter: 'text', minWidth: 220 },
  { prop: 'code', label: '编码', filter: 'text', minWidth: 140 },
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
  { prop: 'description', label: '说明', minWidth: 180 },
  { prop: 'createTime', label: '创建时间', width: 170, visible: false }
]

// 平铺分类列表 → 树(按 parentId 组装 children;空 children 删除避免无效展开箭头)
function buildTree(list: MyErpCategory[]): MyErpCategory[] {
  const map = new Map<number, any>()
  list.forEach((c) => map.set(c.id, { ...c, children: [] }))
  const roots: any[] = []
  map.forEach((node) => {
    const parent = node.parentId && map.get(node.parentId)
    if (parent) parent.children.push(node)
    else roots.push(node)
  })
  map.forEach((n) => {
    if (n.children.length === 0) delete n.children
  })
  return roots
}

async function onQuery(filters: Record<string, any>, page: { pageNo: number; pageSize: number }) {
  loading.value = true
  try {
    const hasFilter = Object.keys(filters).length > 0
    if (hasFilter) {
      // 有筛选:平铺过滤(树形下无法精准筛选,退化为普通列表)
      isTree.value = false
      const data = await myerpCategoryApi.page({
        ...filters,
        pageNo: page.pageNo,
        pageSize: page.pageSize
      })
      rows.value = data.list
      total.value = data.total
    } else {
      // 无筛选:树形展示全部分类
      isTree.value = true
      const all = await myerpCategoryApi.tree()
      rows.value = buildTree(all)
      total.value = all.length
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '加载分类列表失败')
  } finally {
    loading.value = false
  }
}

function onCreate() {
  router.push('/myerp/category/create')
}

async function onDeleteBatch(ids: number[]) {
  let ok = 0
  for (const id of ids) {
    try {
      await myerpCategoryApi.delete(id)
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
    prefs-key="myerp:category:cols"
    nav-key="myerp:category"
    :tree="isTree"
    default-expand-all
    :detail-route="(row: MyErpCategory) => `/myerp/category/${row.id}`"
    bulk-actions
    @query="onQuery"
    @create="onCreate"
    @delete-batch="onDeleteBatch"
  >
    <!-- 名称列:父分类(有子)用文件夹图标 + 加粗,子分类用文档图标,强化层级辨识 -->
    <template #col-name="{ row }">
      <span class="cat-name" :class="{ 'cat-name--parent': row.children?.length }">
        <el-icon class="cat-name__icon">
          <Folder v-if="row.children?.length" />
          <Document v-else />
        </el-icon>
        {{ row.name }}
      </span>
    </template>
    <!-- 状态列自定义渲染:启用绿 / 停用灰 -->
    <template #col-status="{ row }">
      <el-tag :type="row.status === 0 ? 'success' : 'info'" size="small">
        {{ row.status === 0 ? '启用' : '停用' }}
      </el-tag>
    </template>
  </AxelorGrid>
</template>

<style scoped>
.cat-name {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.cat-name__icon {
  color: var(--el-text-color-secondary);
  font-size: 15px;
}
.cat-name--parent {
  font-weight: 600;
}
.cat-name--parent .cat-name__icon {
  color: #e6a23c; /* 文件夹琥珀色,父分类更醒目 */
}
</style>

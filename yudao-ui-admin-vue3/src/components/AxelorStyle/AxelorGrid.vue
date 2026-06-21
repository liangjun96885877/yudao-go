<script setup lang="ts" generic="T extends { id: number }">
// Axelor 风格 Grid 组件 ——
//   ① 紧凑工具栏(+ ✏ 🗑 🔄 + 状态条)
//   ② 表头下列内联搜索行(每列一个 input,debounce 触发查询)
//   ③ 行最左单图标 ✏ 点击进详情
//   ④ 右上 ⋮ 列管理(localStorage 持久化用户偏好)
//   ⑤ 与 vue-element-plus-admin 共存,在 myerp 模块内使用
//
// API:
//   :columns="[{prop, label, width?, filter?: 'text'|'enum'|'number', options?: []}]"
//   :rows="[]"  :total=0  :loading=false
//   :detail-route="(row)=>`/myerp/category/${row.id}`"
//   @query="(filters, page)=>void"  // 列搜索/翻页/刷新都触发
//   @create / @delete-batch / @row-action(row)
//
// slot:
//   #col-<prop>="{row,value}"  - 自定义某列单元格
//   #row-extra="{row}"         - 行操作图标列后追加自定义图标
//   #toolbar-extra              - 工具栏末尾追加按钮
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Refresh, Setting } from '@element-plus/icons-vue'
import type { AxelorColumn } from './types'
import { useRecordNav } from '@/composables/useRecordNav'

const props = defineProps<{
  columns: AxelorColumn[]
  rows: T[]
  total: number
  loading?: boolean
  /** 列偏好的本地存储 key,例如 'myerp:category:cols' */
  prefsKey?: string
  /** 记录导航 key(如 'myerp:category'):存当前 id 序列,供详情页上下条导航 */
  navKey?: string
  /** 行点击或 ✏ 图标点击:返回该行详情路由 */
  detailRoute?: (row: T) => string
  /** 是否启用批量选择 + 工具栏批删 */
  bulkActions?: boolean
  /** 树形模式:rows 带 children 字段,el-table 可展开父子(分类等树形数据用) */
  tree?: boolean
  /** 行唯一键(树形必需),默认 'id' */
  rowKey?: string
  /** 树形默认全部展开 */
  defaultExpandAll?: boolean
  /** 每层缩进像素(默认 24,比 el-table 默认 16 更明显) */
  treeIndent?: number
}>()

const emit = defineEmits<{
  (e: 'query', filters: Record<string, any>, page: { pageNo: number; pageSize: number }): void
  (e: 'create'): void
  (e: 'delete-batch', ids: number[]): void
  (e: 'row-action', row: T): void
}>()

const router = useRouter()
const rowKey = computed(() => props.rowKey || 'id')

// 树形 rows 深度优先收集所有 id(含子节点),平铺 rows 直接取 id。
function collectIds(rows: T[]): number[] {
  const ids: number[] = []
  const walk = (arr: any[]) => {
    for (const r of arr) {
      ids.push(r.id)
      if (Array.isArray(r.children) && r.children.length) walk(r.children)
    }
  }
  walk(rows)
  return ids
}

// 记录导航:每次 rows 变化把当前 id 序列(树形按 DFS 展平)存入 sessionStorage,供 ◀▶ 翻记录。
watch(
  () => props.rows,
  (rows) => {
    if (props.navKey) {
      useRecordNav(props.navKey).remember(props.tree ? collectIds(rows) : rows.map((r) => r.id))
    }
  },
  { immediate: true, deep: false }
)

// === 列偏好(localStorage 持久化) ===
// 区分两类列:
//   - 静态列:走 localStorage 偏好,用户可隐藏
//   - 动态列(prop 以 'attr:' 前缀,EAV 属性列):始终显示,不持久化
//     (因为随选中分类变化,每次不同,持久化无意义)
const visibleCols = ref<string[]>([])
const isDynamic = (prop: string) => prop.startsWith('attr:')

// reconcileCols 依据当前 columns 重算 visibleCols。
// columns 异步变化(产品页 onMounted 才填充 / 切分类追加动态列)时需重新对账。
function reconcileCols() {
  const staticCols = props.columns.filter((c) => !isDynamic(c.prop))
  const dynamicProps = props.columns.filter((c) => isDynamic(c.prop)).map((c) => c.prop)

  let staticVisible: string[]
  let stored: string[] | null = null
  if (props.prefsKey) {
    try {
      const raw = localStorage.getItem(props.prefsKey)
      if (raw) stored = JSON.parse(raw) as string[]
    } catch {
      /* ignore */
    }
  }
  if (stored && stored.length) {
    const stillExist = staticCols.filter((c) => stored!.includes(c.prop)).map((c) => c.prop)
    staticVisible = stillExist.length
      ? stillExist
      : staticCols.filter((c) => c.visible !== false).map((c) => c.prop)
  } else {
    staticVisible = staticCols.filter((c) => c.visible !== false).map((c) => c.prop)
  }
  // 动态列始终全显示
  visibleCols.value = [...staticVisible, ...dynamicProps]
}

function saveColPrefs() {
  if (props.prefsKey) {
    try {
      // 仅持久化静态列偏好,动态列不存
      const staticVisible = visibleCols.value.filter((p) => !isDynamic(p))
      localStorage.setItem(props.prefsKey, JSON.stringify(staticVisible))
    } catch {
      /* ignore */
    }
  }
}
watch(visibleCols, saveColPrefs, { deep: true })

// 列集合(prop 序列)变化时重新对账,解决:产品页 columns 异步填充 / 切分类追加动态列。
watch(
  () => props.columns.map((c) => c.prop).join('|'),
  () => reconcileCols()
)

const renderedColumns = computed(() =>
  props.columns.filter((c) => visibleCols.value.includes(c.prop))
)

// === 列内联搜索(debounce 触发 query) ===
const filters = ref<Record<string, any>>({})
let debounceTimer: number | undefined
function onFilterChange() {
  if (debounceTimer) window.clearTimeout(debounceTimer)
  debounceTimer = window.setTimeout(() => {
    pageNo.value = 1 // 改条件回首页
    triggerQuery()
  }, 300)
}
function clearFilters() {
  filters.value = {}
  pageNo.value = 1
  triggerQuery()
}

// === 分页 ===
const pageNo = ref(1)
const pageSize = ref(20)
function triggerQuery() {
  const clean: Record<string, any> = {}
  for (const [k, v] of Object.entries(filters.value)) {
    if (v !== '' && v !== null && v !== undefined) clean[k] = v
  }
  emit('query', clean, { pageNo: pageNo.value, pageSize: pageSize.value })
}
function onSizeChange(size: number) {
  pageSize.value = size
  pageNo.value = 1
  triggerQuery()
}
function onPageChange(p: number) {
  pageNo.value = p
  triggerQuery()
}

// === 行操作 ===
function goDetail(row: T) {
  if (props.detailRoute) {
    router.push(props.detailRoute(row))
  } else {
    emit('row-action', row)
  }
}
function onCreate() {
  emit('create')
}

// === 批量选择 ===
const selectedIds = ref<number[]>([])
function onSelectionChange(rows: T[]) {
  selectedIds.value = rows.map((r) => r.id)
}
async function onDeleteBatch() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${selectedIds.value.length} 条记录?`,
      '删除确认',
      { type: 'warning' }
    )
    emit('delete-batch', [...selectedIds.value])
  } catch {
    /* cancel */
  }
}

// === 显示页码统计 ===
const stats = computed(() => {
  if (props.total === 0) return '0'
  const from = (pageNo.value - 1) * pageSize.value + 1
  const to = Math.min(pageNo.value * pageSize.value, props.total)
  return `${from} - ${to} of ${props.total}`
})

onMounted(() => {
  reconcileCols()
  triggerQuery()
})

defineExpose({
  /** 父组件可调用以重新加载当前页(保留筛选条件) */
  refresh: () => triggerQuery(),
  /** 重置到首页并触发查询 */
  reset: () => {
    pageNo.value = 1
    triggerQuery()
  }
})
</script>

<template>
  <div class="axelor-grid">
    <!-- 顶部工具栏:紧凑图标按钮 + 状态条 + 列管理 -->
    <div class="axelor-grid__toolbar">
      <div class="axelor-grid__toolbar-left">
        <el-tooltip content="新建" placement="top">
          <el-button :icon="Plus" circle size="small" type="primary" @click="onCreate" />
        </el-tooltip>
        <el-tooltip v-if="bulkActions" content="批量删除" placement="top">
          <el-button
            :icon="Delete"
            circle
            size="small"
            :disabled="selectedIds.length === 0"
            @click="onDeleteBatch"
          />
        </el-tooltip>
        <el-tooltip content="刷新" placement="top">
          <el-button :icon="Refresh" circle size="small" @click="triggerQuery" />
        </el-tooltip>
        <el-tooltip content="清空筛选" placement="top">
          <el-button size="small" link @click="clearFilters">清空</el-button>
        </el-tooltip>
        <slot name="toolbar-extra"></slot>
      </div>
      <div class="axelor-grid__toolbar-right">
        <span class="axelor-grid__stats">{{ stats }}</span>
        <el-pagination
          v-model:current-page="pageNo"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="prev, pager, next, sizes"
          small
          background
          @size-change="onSizeChange"
          @current-change="onPageChange"
        />
        <!-- 列管理 -->
        <el-popover trigger="click" placement="bottom-end" :width="220">
          <template #reference>
            <el-button :icon="Setting" circle size="small" />
          </template>
          <div class="axelor-grid__cols-pop">
            <div class="axelor-grid__cols-title">显示列</div>
            <el-checkbox-group v-model="visibleCols">
              <div v-for="c in columns" :key="c.prop" class="axelor-grid__col-item">
                <el-checkbox :value="c.prop" :label="c.label" />
              </div>
            </el-checkbox-group>
          </div>
        </el-popover>
      </div>
    </div>

    <!-- 表格 + 列内联搜索 + 单图标行操作 -->
    <el-table
      v-loading="loading"
      :data="rows"
      :row-key="rowKey"
      :tree-props="tree ? { children: 'children' } : undefined"
      :default-expand-all="tree && defaultExpandAll"
      :indent="treeIndent ?? 24"
      stripe
      border
      size="small"
      class="axelor-grid__table"
      @selection-change="onSelectionChange"
      @row-click="goDetail"
    >
      <el-table-column v-if="bulkActions" type="selection" width="38" />

      <!-- 行操作单图标:✏ 进详情。
           树形模式下隐藏此列,让「名称」成为承载展开箭头+缩进的第一普通列(否则层级缩进落在此窄列里看不出)。 -->
      <el-table-column v-if="!tree" width="48" align="center" :resizable="false">
        <template #default="{ row }">
          <el-icon class="axelor-grid__row-icon" @click.stop="goDetail(row)">
            <Edit />
          </el-icon>
          <slot name="row-extra" :row="row"></slot>
        </template>
      </el-table-column>

      <!-- 数据列 -->
      <el-table-column
        v-for="col in renderedColumns"
        :key="col.prop"
        :prop="col.prop"
        :label="col.label"
        :width="col.width"
        :min-width="col.minWidth"
        :align="col.align ?? 'left'"
        show-overflow-tooltip
      >
        <!-- 表头插槽:列名 + 下方内联搜索框 -->
        <template #header>
          <div class="axelor-grid__col-header">
            <div class="axelor-grid__col-label">{{ col.label }}</div>
            <template v-if="col.filter === 'text' || col.filter === 'number'">
              <el-input
                v-model="filters[col.prop]"
                placeholder="搜索…"
                clearable
                size="small"
                @input="onFilterChange"
                @click.stop
              />
            </template>
            <template v-else-if="col.filter === 'enum'">
              <el-select
                v-model="filters[col.prop]"
                placeholder="全部"
                clearable
                size="small"
                @change="onFilterChange"
                @click.stop
              >
                <el-option
                  v-for="o in col.options || []"
                  :key="o.value"
                  :label="o.label"
                  :value="o.value"
                />
              </el-select>
            </template>
            <template v-else>
              <div class="axelor-grid__col-blank">&nbsp;</div>
            </template>
          </div>
        </template>

        <template #default="{ row }">
          <slot :name="`col-${col.prop}`" :row="row" :value="(row as any)[col.prop]">
            {{ (row as any)[col.prop] }}
          </slot>
        </template>
      </el-table-column>

      <template #empty>
        <el-empty :image-size="60" description="暂无数据" />
      </template>
    </el-table>
  </div>
</template>

<style scoped>
.axelor-grid {
  --axelor-primary: #6259e8;
  background: #fff;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 480px;
}

.axelor-grid__toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.axelor-grid__toolbar-left {
  display: flex;
  align-items: center;
  gap: 6px;
}
.axelor-grid__toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.axelor-grid__stats {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-right: 4px;
}

/* 表头 + 列搜索行:把搜索框塞到列名下方 */
.axelor-grid__col-header {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.axelor-grid__col-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.axelor-grid__col-blank {
  height: 24px;
}

/* 单图标行操作 */
.axelor-grid__row-icon {
  color: var(--axelor-primary);
  cursor: pointer;
  font-size: 14px;
  transition: opacity 0.15s;
}
.axelor-grid__row-icon:hover {
  opacity: 0.7;
}

/* 列管理弹出 */
.axelor-grid__cols-pop {
  max-height: 320px;
  overflow-y: auto;
}
.axelor-grid__cols-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 6px;
}
.axelor-grid__col-item {
  padding: 2px 0;
}

/* 表格压缩 */
.axelor-grid__table {
  flex: 1;
}
.axelor-grid__table :deep(.el-table__row) {
  cursor: pointer;
}
.axelor-grid__table :deep(th.el-table__cell) {
  background-color: #fafafa;
  padding: 6px 0;
}
.axelor-grid__table :deep(td.el-table__cell) {
  padding: 6px 0;
}
</style>

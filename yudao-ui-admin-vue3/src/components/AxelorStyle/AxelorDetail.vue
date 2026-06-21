<script setup lang="ts">
// Axelor 风格详情容器(工具栏对齐 Axelor edit 页):
//   ← 返回  |  {title}  ◀ {idx}/{total} ▶        [+ 新建] [更多▾] [保存]
//   ┌─────────────────────────┬──────────┐
//   │  左 70% form slot         │ 右 30%    │
//   └─────────────────────────┴──────────┘
//   更多▾:复制为新 / 刷新 / 删除  (次要操作收纳,工具栏更清爽)
//
// Props:
//   title / loading / showChatter / showDelete / backTo
//   prevTo / nextTo        上一条/下一条记录的路由(null 则禁用),配合 useRecordNav
//   navIndex / navTotal    当前记录序号 / 总数(显示 "3 / 12")
//   createRoute            新建路由(传则显示「+ 新建」)
//   showCopy / showRefresh 是否在「更多▾」显示复制/刷新
//
// Emits: save / delete / copy / refresh
// Slots: default(form) / chatter / actions(保存左侧追加) / more(更多▾ 追加项)
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import {
  ArrowLeft,
  ArrowRight,
  CopyDocument,
  Delete,
  MoreFilled,
  Plus,
  Refresh
} from '@element-plus/icons-vue'

const props = defineProps<{
  title: string
  loading?: boolean
  showChatter?: boolean
  showDelete?: boolean
  backTo?: string
  prevTo?: string | null
  nextTo?: string | null
  navIndex?: number // 0-based;< 0 表示无导航信息
  navTotal?: number
  createRoute?: string
  showCopy?: boolean
  showRefresh?: boolean
}>()

const emit = defineEmits<{
  (e: 'save'): void
  (e: 'delete'): void
  (e: 'copy'): void
  (e: 'refresh'): void
}>()

const router = useRouter()

function onBack() {
  if (props.backTo) router.push(props.backTo)
  else router.go(-1)
}

function goPrev() {
  if (props.prevTo) router.push(props.prevTo)
}
function goNext() {
  if (props.nextTo) router.push(props.nextTo)
}
function onCreate() {
  if (props.createRoute) router.push(props.createRoute)
}

async function onDelete() {
  try {
    await ElMessageBox.confirm('确认删除该记录?', '删除确认', { type: 'warning' })
    emit('delete')
  } catch {
    /* cancel */
  }
}

function onMoreCommand(cmd: string) {
  if (cmd === 'delete') onDelete()
  else if (cmd === 'copy') emit('copy')
  else if (cmd === 'refresh') emit('refresh')
}

// 记录序号展示:有有效 index 时显示 "3 / 12"
const navLabel = computed(() => {
  if (props.navIndex != null && props.navIndex >= 0 && props.navTotal) {
    return `${props.navIndex + 1} / ${props.navTotal}`
  }
  return ''
})
// 是否显示记录导航区(有相邻记录或有序号)
const showNav = computed(
  () => props.prevTo !== undefined || props.nextTo !== undefined || navLabel.value !== ''
)
// 「更多▾」是否有内容
const hasMore = computed(() => props.showDelete || props.showCopy || props.showRefresh)
</script>

<template>
  <div class="axelor-detail">
    <!-- 顶栏 -->
    <div class="axelor-detail__header">
      <div class="axelor-detail__header-left">
        <el-button :icon="ArrowLeft" link size="default" @click="onBack">返回</el-button>
        <!-- 记录导航 ◀ idx/total ▶ -->
        <div v-if="showNav" class="axelor-detail__nav">
          <el-tooltip content="上一条" placement="top">
            <el-button
              :icon="ArrowLeft"
              circle
              size="small"
              :disabled="!prevTo"
              @click="goPrev"
            />
          </el-tooltip>
          <span v-if="navLabel" class="axelor-detail__nav-label">{{ navLabel }}</span>
          <el-tooltip content="下一条" placement="top">
            <el-button
              :icon="ArrowRight"
              circle
              size="small"
              :disabled="!nextTo"
              @click="goNext"
            />
          </el-tooltip>
        </div>
        <span class="axelor-detail__title">{{ title }}</span>
      </div>

      <div class="axelor-detail__header-right">
        <el-button v-if="createRoute" :icon="Plus" @click="onCreate">新建</el-button>
        <slot name="actions"></slot>
        <!-- 更多▾ 收纳次要操作 -->
        <el-dropdown v-if="hasMore" trigger="click" @command="onMoreCommand">
          <el-button :icon="MoreFilled">更多</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-if="showCopy" command="copy" :icon="CopyDocument">
                复制为新
              </el-dropdown-item>
              <el-dropdown-item v-if="showRefresh" command="refresh" :icon="Refresh">
                刷新
              </el-dropdown-item>
              <slot name="more"></slot>
              <el-dropdown-item
                v-if="showDelete"
                command="delete"
                :icon="Delete"
                divided
              >
                <span class="axelor-detail__danger">删除</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="primary" :loading="loading" @click="emit('save')">保存</el-button>
      </div>
    </div>

    <!-- 主体:左 form / 右 chatter -->
    <div class="axelor-detail__body" :class="{ 'no-chatter': !showChatter }">
      <div class="axelor-detail__form-area">
        <slot></slot>
        <slot name="form"></slot>
      </div>
      <div v-if="showChatter" class="axelor-detail__chatter-area">
        <slot name="chatter"></slot>
      </div>
    </div>
  </div>
</template>

<style scoped>
.axelor-detail {
  --axelor-primary: #6259e8;
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - 100px);
  background: var(--el-bg-color-page);
}

.axelor-detail__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  background: #fff;
  border-bottom: 1px solid var(--el-border-color-lighter);
  position: sticky;
  top: 0;
  z-index: 10;
}
.axelor-detail__header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.axelor-detail__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.axelor-detail__nav {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: 4px;
}
.axelor-detail__nav-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  min-width: 42px;
  text-align: center;
}
.axelor-detail__header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.axelor-detail__danger {
  color: var(--el-color-danger);
}

.axelor-detail__body {
  flex: 1;
  display: grid;
  grid-template-columns: 7fr 3fr;
  gap: 12px;
  padding: 12px;
}
.axelor-detail__body.no-chatter {
  grid-template-columns: 1fr;
}

.axelor-detail__form-area {
  background: #fff;
  border-radius: 6px;
  padding: 16px;
  min-height: 480px;
}
/* 修复:框架某处把 .el-tabs 设成 display:flex,导致 __content 被压扁 + overflow:hidden 裁切
   表单内容(属性 tab 控件底部被切)。这里强制恢复 Element Plus 默认布局。 */
.axelor-detail__form-area :deep(.el-tabs) {
  display: block;
}
.axelor-detail__form-area :deep(.el-tabs__content) {
  overflow: visible;
}

.axelor-detail__chatter-area {
  background: #fff;
  border-radius: 6px;
  padding: 12px;
  min-height: 480px;
  overflow-y: auto;
}

@media (max-width: 1100px) {
  .axelor-detail__body {
    grid-template-columns: 1fr;
  }
}
</style>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { CaretBottom, StarFilled } from '@element-plus/icons-vue'
import { chatterApi } from '@/api/chatter'
import type { TimelineItem } from '@/types/chatter'
import AuditDiff from './AuditDiff.vue'

// 显式声明组件名,使模板能自递归 <TimelineItem /> 渲染回复树。
defineOptions({ name: 'TimelineItem' })

const props = defineProps<{ item: TimelineItem }>()
const emit = defineEmits<{
  (e: 'reply', payload: { parentId: number; replyTo: string }): void
}>()

// Axelor 风格回复展开:点击"回复 (N)"按钮 → 调接口拉取直接子评论 → 缩进渲染。
// 子级也是 TimelineItem,递归本组件即可,二级回复自带 replyCount,可继续展开。
const replies = ref<TimelineItem[]>([])
const repliesLoading = ref(false)
const repliesLoaded = ref(false)
async function toggleReplies(): Promise<void> {
  if (repliesLoaded.value) {
    replies.value = []
    repliesLoaded.value = false
    return
  }
  repliesLoading.value = true
  try {
    replies.value = await chatterApi.commentReplies(props.item.refId)
    repliesLoaded.value = true
  } catch (e) {
    ElMessage.error((e as Error).message || '加载回复失败')
  } finally {
    repliesLoading.value = false
  }
}

// 事件类型 → 卡片标题
const TYPE_LABEL: Record<string, string> = {
  create: '记录创建',
  update: '记录更新',
  comment: '评论',
  approval: '审批动态',
  status_change: '状态变更',
  attachment: '附件',
  follow: '关注',
  system: '系统',
  ai_summary: 'AI 摘要'
}

const typeLabel = computed(() => TYPE_LABEL[props.item.eventType] || '动态')
const isComment = computed(() => props.item.eventType === 'comment')
const isUpdate = computed(() => props.item.eventType === 'update')
const hasChanges = computed(() => isUpdate.value && (props.item.changes?.length ?? 0) > 0)

// 头像：用户取昵称首字，系统/AI 用图标色块
const avatarText = computed(() => {
  if (props.item.actorType === 2) return ''
  return (props.item.actorName || '?').charAt(0).toUpperCase()
})
const avatarClass = computed(() => {
  if (props.item.actorType === 2) return 'is-system'
  if (props.item.actorType === 3) return 'is-ai'
  return 'is-user'
})

// 相对时间
const relativeTime = computed(() => {
  const s = props.item.createTime
  if (!s) return ''
  const t = new Date(s.replace(' ', 'T')).getTime()
  if (Number.isNaN(t)) return s
  const diff = Date.now() - t
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  if (diff < 2_592_000_000) return `${Math.floor(diff / 86_400_000)} 天前`
  return s.slice(0, 10)
})

// 卡片菜单：标记已读 / 标记重要
async function onCommand(cmd: 'read' | 'important'): Promise<void> {
  const read = cmd === 'read' ? !props.item.isRead : props.item.isRead
  const important = cmd === 'important' ? !props.item.isImportant : props.item.isImportant
  try {
    await chatterApi.setTimelineFlag(props.item.id, read, important)
    props.item.isRead = read
    props.item.isImportant = important
    ElMessage.success('已更新')
  } catch (e) {
    ElMessage.error((e as Error).message || '操作失败')
  }
}
</script>

<template>
  <div class="ax-msg">
    <span class="ax-msg__avatar" :class="avatarClass">{{ avatarText }}</span>
    <div class="ax-msg__card" :class="{ 'is-read': item.isRead, 'is-important': item.isImportant }">
      <span class="ax-msg__arrow"></span>
      <!-- 标题 + 右上角菜单 -->
      <div class="ax-msg__header">
        <el-icon v-if="item.isImportant" class="ax-msg__star"><StarFilled /></el-icon>
        <span class="ax-msg__title">{{ typeLabel }}</span>
        <el-dropdown class="ax-msg__menu" trigger="click" @command="onCommand">
          <span class="ax-msg__caret"><el-icon><CaretBottom /></el-icon></span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="read">
                {{ item.isRead ? '标记未读' : '标记已读' }}
              </el-dropdown-item>
              <el-dropdown-item command="important">
                {{ item.isImportant ? '取消重要' : '标记重要' }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
      <!-- 正文 -->
      <div class="ax-msg__body">
        <div v-if="isComment" class="ax-msg__comment">{{ item.body }}</div>
        <AuditDiff v-else-if="hasChanges" :changes="item.changes!" />
        <div v-else class="ax-msg__text">{{ item.summary }}</div>
      </div>
      <!-- 页脚 -->
      <div class="ax-msg__footer">
        <a class="ax-msg__author">{{ item.actorName }}</a>
        <span class="ax-msg__dot">·</span>
        <span class="ax-msg__time">{{ relativeTime }}</span>
        <a
          v-if="isComment"
          class="ax-msg__reply"
          @click="emit('reply', { parentId: item.refId, replyTo: item.actorName })"
        >
          回复
        </a>
        <a
          v-if="isComment && (item.replyCount ?? 0) > 0"
          class="ax-msg__expand"
          @click="toggleReplies"
        >
          {{ repliesLoaded ? `收起回复` : `展开回复 (${item.replyCount})` }}
          <span v-if="repliesLoading" class="ax-msg__expand-loading">加载中…</span>
        </a>
      </div>
    </div>
  </div>
  <!-- 子回复:递归本组件,缩进渲染。Vue3 SFC 用文件名自动递归。 -->
  <div v-if="repliesLoaded && replies.length" class="ax-msg__children">
    <TimelineItem
      v-for="r in replies"
      :key="r.id"
      :item="r"
      @reply="(p) => emit('reply', p)"
    />
  </div>
</template>

<style scoped>
.ax-msg {
  position: relative;
  display: flex;
  margin-left: -21px;
  margin-bottom: 14px;
}
/* 头像 */
.ax-msg__avatar {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 600;
  color: #fff;
  margin: 0 12px;
  z-index: 1;
  background: #fff;
}
.ax-msg__avatar.is-user {
  background: #3e9be6;
}
.ax-msg__avatar.is-ai {
  background: #8a63e8;
}
.ax-msg__avatar.is-system {
  background: #dcdfe6;
}
/* 气泡卡片 */
.ax-msg__card {
  position: relative;
  flex: 1;
  min-width: 0;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  background: #fff;
}
.ax-msg__card.is-important {
  border-color: #e6a23c;
}
.ax-msg__card.is-read {
  opacity: 0.72;
}
.ax-msg__arrow {
  position: absolute;
  left: -6px;
  top: 14px;
  width: 10px;
  height: 10px;
  background: #fff;
  border-left: 1px solid var(--el-border-color);
  border-bottom: 1px solid var(--el-border-color);
  transform: rotate(45deg);
}
.ax-msg__header {
  position: relative;
  display: flex;
  align-items: center;
  padding: 9px 14px;
  font-size: 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.ax-msg__star {
  color: #e6a23c;
  margin-right: 5px;
}
.ax-msg__title {
  flex: 1;
}
/* 右上角菜单 */
.ax-msg__menu {
  cursor: pointer;
}
.ax-msg__caret {
  display: inline-flex;
  color: var(--chatter-accent, #6259e8);
  font-size: 16px;
}
.ax-msg__body {
  padding: 10px 14px;
}
.ax-msg__comment {
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--el-text-color-primary);
}
.ax-msg__text {
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.ax-msg__footer {
  padding: 4px 14px 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.ax-msg__author {
  color: var(--chatter-accent, #6259e8);
  cursor: pointer;
}
.ax-msg__dot {
  margin: 0 4px;
}
.ax-msg__time {
  color: var(--chatter-accent, #6259e8);
}
.ax-msg__reply {
  margin-left: 12px;
  color: var(--chatter-accent, #6259e8);
  cursor: pointer;
}
.ax-msg__reply:hover {
  text-decoration: underline;
}
.ax-msg__expand {
  margin-left: 12px;
  color: var(--chatter-accent, #6259e8);
  cursor: pointer;
}
.ax-msg__expand:hover {
  text-decoration: underline;
}
.ax-msg__expand-loading {
  margin-left: 6px;
  color: var(--el-text-color-secondary);
}
/* 缩进展示子回复:左侧细线 + 内边距 */
.ax-msg__children {
  margin-left: 36px;
  margin-top: 4px;
  margin-bottom: 14px;
  padding-left: 14px;
  border-left: 2px solid var(--el-border-color-lighter);
}
</style>

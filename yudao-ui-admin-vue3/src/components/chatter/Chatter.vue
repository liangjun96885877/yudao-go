<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { chatterApi } from '@/api/chatter'
import { useChatterSocket } from '@/composables/useChatterSocket'
import type { TimelineItem } from '@/types/chatter'
import FollowerList from './FollowerList.vue'
import CommentBox from './CommentBox.vue'
import TimelineFeed from './TimelineFeed.vue'

const props = defineProps<{ bizType: string; bizId: number }>()

const items = ref<TimelineItem[]>([])
const nextCursor = ref(0)
const loading = ref(false)
const hasMore = ref(true)

// 回复某条评论：commentId 作为新评论的 parentId
const replyingTo = ref<{ parentId: number; replyTo: string } | null>(null)
function onReply(p: { parentId: number; replyTo: string }) {
  replyingTo.value = p
}
function cancelReply() {
  replyingTo.value = null
}

// 筛选：全部 / 评论 / 通知（参考 Axelor 的 All / Comments / Notifications）
type Filter = 'all' | 'comment' | 'notification'
const filter = ref<Filter>('all')
const FILTERS: { key: Filter; label: string }[] = [
  { key: 'all', label: '全部' },
  { key: 'comment', label: '评论' },
  { key: 'notification', label: '通知' }
]
const filteredItems = computed(() => {
  if (filter.value === 'comment') return items.value.filter((i) => i.eventType === 'comment')
  if (filter.value === 'notification') return items.value.filter((i) => i.eventType !== 'comment')
  return items.value
})

const socket = useChatterSocket()
let offTimeline: (() => void) | undefined

async function loadMore(): Promise<void> {
  if (loading.value || !hasMore.value) return
  loading.value = true
  try {
    const page = await chatterApi.timeline(props.bizType, props.bizId, nextCursor.value)
    items.value.push(...page.list)
    nextCursor.value = page.nextCursor
    hasMore.value = page.nextCursor !== 0
  } catch (e) {
    ElMessage.error((e as Error).message || '加载时间线失败')
  } finally {
    loading.value = false
  }
}

function reload(): void {
  items.value = []
  nextCursor.value = 0
  hasMore.value = true
  loadMore()
}

onMounted(() => {
  loadMore()
  socket.subscribe(props.bizType, props.bizId)
  // 实时收到新时间线条目：去重后插入到最前。
  offTimeline = socket.onTimeline((item, bizType, bizId) => {
    if (bizType !== props.bizType || bizId !== props.bizId) return
    if (!items.value.some((i) => i.id === item.id)) {
      items.value.unshift(item)
    }
  })
})

onBeforeUnmount(() => {
  socket.unsubscribe(props.bizType, props.bizId)
  offTimeline?.()
})
</script>

<template>
  <div class="ax-chatter">
    <!-- 关注者 -->
    <div class="ax-chatter__followers">
      <FollowerList :biz-type="bizType" :biz-id="bizId" />
    </div>

    <!-- 筛选胶囊 -->
    <div class="ax-chatter__filters">
      <button
        v-for="f in FILTERS"
        :key="f.key"
        class="ax-pill"
        :class="{ 'ax-pill--active': filter === f.key }"
        @click="filter = f.key"
      >
        {{ f.label }}
      </button>
    </div>

    <!-- 评论输入（通知筛选下隐藏，与 Axelor 一致） -->
    <CommentBox
      v-show="filter !== 'notification'"
      :biz-type="bizType"
      :biz-id="bizId"
      :parent-id="replyingTo?.parentId"
      :reply-to="replyingTo?.replyTo"
      @submitted="reload"
      @cancel-reply="cancelReply"
    />

    <!-- 时间线 -->
    <TimelineFeed
      :items="filteredItems"
      :loading="loading"
      :has-more="hasMore"
      @load-more="loadMore"
      @reply="onReply"
    />
  </div>
</template>

<style scoped>
.ax-chatter {
  --chatter-accent: #6259e8;
  background: #fff;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 16px;
}
.ax-chatter__followers {
  margin-bottom: 12px;
}
.ax-chatter__filters {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
/* Axelor 风格筛选胶囊 */
.ax-pill {
  padding: 6px 16px;
  font-size: 13px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid var(--chatter-accent);
  background: #fff;
  color: var(--chatter-accent);
  transition: opacity 0.15s;
}
.ax-pill:hover {
  opacity: 0.85;
}
.ax-pill--active {
  background: var(--chatter-accent);
  color: #fff;
}
</style>

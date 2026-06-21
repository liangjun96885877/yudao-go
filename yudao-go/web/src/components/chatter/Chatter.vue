<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { chatterApi } from '@/api/chatter'
import { useChatterSocket } from '@/composables/useChatterSocket'
import type { TimelineItem } from '@/types/chatter'
import FollowerList from './FollowerList.vue'
import CommentBox from './CommentBox.vue'
import TimelineFeed from './TimelineFeed.vue'
import AttachmentList from './AttachmentList.vue'

const props = defineProps<{ bizType: string; bizId: number }>()

const items = ref<TimelineItem[]>([])
const nextCursor = ref(0)
const loading = ref(false)
const hasMore = ref(true)
const tab = ref<'timeline' | 'attachment'>('timeline')

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
  <el-card class="chatter" shadow="never">
    <template #header>
      <div class="chatter__header">
        <span class="chatter__title">业务动态</span>
        <FollowerList :biz-type="bizType" :biz-id="bizId" />
      </div>
    </template>

    <CommentBox :biz-type="bizType" :biz-id="bizId" @submitted="reload" />

    <el-tabs v-model="tab">
      <el-tab-pane label="时间线" name="timeline">
        <TimelineFeed
          :items="items"
          :loading="loading"
          :has-more="hasMore"
          @load-more="loadMore"
        />
      </el-tab-pane>
      <el-tab-pane label="附件" name="attachment">
        <AttachmentList :biz-type="bizType" :biz-id="bizId" />
      </el-tab-pane>
    </el-tabs>
  </el-card>
</template>

<style scoped>
.chatter {
  width: 100%;
}
.chatter__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.chatter__title {
  font-weight: 600;
}
</style>

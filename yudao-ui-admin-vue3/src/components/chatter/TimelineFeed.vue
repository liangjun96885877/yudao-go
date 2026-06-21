<script setup lang="ts">
import type { TimelineItem } from '@/types/chatter'
import TimelineItemView from './TimelineItem.vue'

defineProps<{
  items: TimelineItem[]
  loading: boolean
  hasMore: boolean
}>()
const emit = defineEmits<{
  (e: 'load-more'): void
  (e: 'reply', payload: { parentId: number; replyTo: string }): void
}>()
</script>

<template>
  <div class="ax-feed">
    <el-empty v-if="!loading && items.length === 0" description="暂无动态" :image-size="70" />
    <!-- 左侧贯穿时间线 -->
    <div v-else class="ax-feed__thread">
      <TimelineItemView
        v-for="it in items"
        :key="it.id"
        :item="it"
        @reply="(p) => emit('reply', p)"
      />
    </div>
    <div class="ax-feed__more">
      <el-button v-if="hasMore" text type="primary" :loading="loading" @click="$emit('load-more')">
        加载更多
      </el-button>
      <span v-else-if="items.length" class="ax-feed__end">没有更多了</span>
    </div>
  </div>
</template>

<style scoped>
.ax-feed__thread {
  margin-left: 20px;
  padding-top: 4px;
  border-left: 2px solid var(--el-border-color-lighter);
}
.ax-feed__more {
  text-align: center;
  padding: 6px 0;
}
.ax-feed__end {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>

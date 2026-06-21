<script setup lang="ts">
import type { TimelineItem } from '@/types/chatter'
import TimelineItemView from './TimelineItem.vue'

defineProps<{
  items: TimelineItem[]
  loading: boolean
  hasMore: boolean
}>()
defineEmits<{ (e: 'load-more'): void }>()
</script>

<template>
  <div class="feed">
    <el-empty v-if="!loading && items.length === 0" description="暂无动态" :image-size="80" />
    <el-timeline v-else>
      <TimelineItemView v-for="it in items" :key="it.id" :item="it" />
    </el-timeline>
    <div class="feed__more">
      <el-button
        v-if="hasMore"
        text
        type="primary"
        :loading="loading"
        @click="$emit('load-more')"
      >
        加载更多
      </el-button>
      <span v-else-if="items.length" class="feed__end">没有更多了</span>
    </div>
  </div>
</template>

<style scoped>
.feed__more {
  text-align: center;
  padding: 8px 0;
}
.feed__end {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>

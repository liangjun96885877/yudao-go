<script setup lang="ts">
import { computed } from 'vue'
import type { TimelineItem } from '@/types/chatter'
import AuditDiff from './AuditDiff.vue'

const props = defineProps<{ item: TimelineItem }>()

// 事件类型 → 时间线节点颜色。
const nodeColor = computed(() => {
  const map: Record<string, string> = {
    create: '#67C23A',
    update: '#E6A23C',
    comment: '#409EFF',
    approval: '#909399',
    status_change: '#E6A23C',
    attachment: '#409EFF',
    follow: '#C0C4CC',
    system: '#909399',
    ai_summary: '#9254DE',
  }
  return map[props.item.eventType] || '#909399'
})

const isComment = computed(() => props.item.eventType === 'comment')
const isUpdate = computed(() => props.item.eventType === 'update')
</script>

<template>
  <el-timeline-item :color="nodeColor" :timestamp="item.createTime" placement="top">
    <div class="ti">
      <div class="ti__head">
        <el-avatar :size="22">{{ item.actorName.charAt(0) || '?' }}</el-avatar>
        <span class="ti__summary">{{ item.summary }}</span>
      </div>
      <div v-if="isComment && item.body" class="ti__comment">{{ item.body }}</div>
      <AuditDiff
        v-else-if="isUpdate && item.changes && item.changes.length"
        :changes="item.changes"
      />
    </div>
  </el-timeline-item>
</template>

<style scoped>
.ti__head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ti__summary {
  font-size: 13px;
}
.ti__comment {
  margin: 6px 0 0 30px;
  padding: 8px 10px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  white-space: pre-wrap;
  font-size: 13px;
}
</style>

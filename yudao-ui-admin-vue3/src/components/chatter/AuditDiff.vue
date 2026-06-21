<script setup lang="ts">
import type { FieldChange } from '@/types/chatter'

defineProps<{ changes: FieldChange[] }>()

// 优先展示解析后的展示值（枚举/外键），其次原始值。
function display(c: FieldChange, which: 'old' | 'new'): string {
  if (which === 'old') return c.oldDisplay || c.oldValue || ''
  return c.newDisplay || c.newValue || ''
}
</script>

<template>
  <ul class="ax-track">
    <li v-for="c in changes" :key="c.field" class="ax-track__item">
      <span class="ax-track__label">{{ c.label || c.field }} :</span>
      <span class="ax-track__val">
        <template v-if="display(c, 'old')">{{ display(c, 'old') }}</template>
        <span v-else class="ax-track__empty">无</span>
      </span>
      <span class="ax-track__arrow">→</span>
      <span class="ax-track__val">
        <template v-if="display(c, 'new')">{{ display(c, 'new') }}</template>
        <span v-else class="ax-track__empty">无</span>
      </span>
    </li>
  </ul>
</template>

<style scoped>
.ax-track {
  margin: 0;
  padding: 0 0 0 18px;
  list-style: disc;
}
.ax-track__item {
  font-size: 13px;
  line-height: 1.9;
  color: var(--el-text-color-secondary);
}
.ax-track__label {
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-right: 4px;
}
.ax-track__val {
  color: var(--el-text-color-secondary);
}
.ax-track__arrow {
  margin: 0 4px;
  color: var(--el-text-color-secondary);
}
.ax-track__empty {
  font-style: italic;
  opacity: 0.6;
}
</style>

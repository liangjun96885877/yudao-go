<script setup lang="ts">
import { ref } from 'vue'
import { ArrowDown, ArrowRight } from '@element-plus/icons-vue'
import type { FieldChange } from '@/types/chatter'

defineProps<{ changes: FieldChange[] }>()

const expanded = ref(false)

// 优先展示解析后的展示值（枚举/外键），否则原始值。
function display(c: FieldChange, which: 'old' | 'new'): string {
  if (which === 'old') return c.oldDisplay || c.oldValue || '空'
  return c.newDisplay || c.newValue || '空'
}
</script>

<template>
  <div class="diff">
    <el-link type="primary" underline="never" @click="expanded = !expanded">
      <el-icon><ArrowDown v-if="expanded" /><ArrowRight v-else /></el-icon>
      修改了 {{ changes.length }} 个字段
    </el-link>
    <table v-show="expanded" class="diff__table">
      <tbody>
        <tr v-for="c in changes" :key="c.field">
          <td class="diff__label">{{ c.label || c.field }}</td>
          <td class="diff__old">{{ display(c, 'old') }}</td>
          <td class="diff__arrow">→</td>
          <td class="diff__new">{{ display(c, 'new') }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.diff {
  margin: 4px 0 0 30px;
  font-size: 13px;
}
.diff__table {
  margin-top: 6px;
  border-collapse: collapse;
}
.diff__table td {
  padding: 2px 8px;
}
.diff__label {
  color: var(--el-text-color-secondary);
}
.diff__old {
  color: var(--el-color-danger);
  text-decoration: line-through;
}
.diff__new {
  color: var(--el-color-success);
}
.diff__arrow {
  color: var(--el-text-color-secondary);
}
</style>

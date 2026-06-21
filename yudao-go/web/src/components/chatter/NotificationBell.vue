<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Bell } from '@element-plus/icons-vue'
import { chatterApi } from '@/api/chatter'
import { useChatterSocket } from '@/composables/useChatterSocket'
import type { NotificationItem } from '@/types/chatter'

const count = ref(0)
const list = ref<NotificationItem[]>([])
const socket = useChatterSocket()
let off: (() => void) | undefined

async function refreshCount(): Promise<void> {
  try {
    const r = await chatterApi.unreadCount()
    count.value = r.count
  } catch {
    /* ignore */
  }
}

async function openPanel(visible: boolean): Promise<void> {
  if (!visible) return
  try {
    const page = await chatterApi.notifications(0, 20)
    list.value = page.list
  } catch {
    /* ignore */
  }
}

async function readAll(): Promise<void> {
  try {
    await chatterApi.markAllRead()
    count.value = 0
    list.value.forEach((n) => (n.isRead = true))
  } catch {
    /* ignore */
  }
}

onMounted(() => {
  refreshCount()
  // 实时收到新通知时未读数 +1
  off = socket.onNotification(() => {
    count.value += 1
  })
})
onBeforeUnmount(() => off?.())
</script>

<template>
  <el-dropdown trigger="click" @visible-change="openPanel">
    <el-badge :value="count" :hidden="count === 0" :max="99">
      <el-icon :size="20"><Bell /></el-icon>
    </el-badge>
    <template #dropdown>
      <div class="nbell">
        <div class="nbell__head">
          <span>通知</span>
          <el-link type="primary" underline="never" @click="readAll">全部已读</el-link>
        </div>
        <el-empty v-if="list.length === 0" description="暂无通知" :image-size="60" />
        <div
          v-for="n in list"
          :key="n.id"
          class="nbell__item"
          :class="{ 'nbell__item--unread': !n.isRead }"
        >
          <div class="nbell__title">{{ n.title }}</div>
          <div class="nbell__content">{{ n.content }}</div>
          <div class="nbell__time">{{ n.createTime }}</div>
        </div>
      </div>
    </template>
  </el-dropdown>
</template>

<style scoped>
.nbell {
  width: 300px;
  max-height: 400px;
  overflow-y: auto;
}
.nbell__head {
  display: flex;
  justify-content: space-between;
  padding: 8px;
  font-weight: 600;
}
.nbell__item {
  padding: 8px;
  border-top: 1px solid var(--el-border-color-lighter);
}
.nbell__item--unread {
  background: var(--el-color-primary-light-9);
}
.nbell__title {
  font-size: 13px;
  font-weight: 600;
}
.nbell__content {
  font-size: 12px;
  color: var(--el-text-color-regular);
}
.nbell__time {
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
</style>

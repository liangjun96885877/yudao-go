<script setup lang="ts">
import { ref } from 'vue'
import Chatter from './Chatter.vue'

// ChatterDrawer 把 Chatter 面板包进抽屉，供各业务列表/详情页复用。
// 用法：<ChatterDrawer ref="chatterRef" />，再调用 chatterRef.value.open(bizType, bizId, name)。

const visible = ref(false)
const bizType = ref('')
const bizId = ref(0)
const title = ref('业务动态')

function open(t: string, id: number, name?: string) {
  bizType.value = t
  bizId.value = id
  title.value = name ? `业务动态 · ${name}` : '业务动态'
  visible.value = true
}

defineExpose({ open })
</script>

<template>
  <el-drawer v-model="visible" :title="title" size="620px" destroy-on-close>
    <!-- v-if 确保每次打开时 Chatter 重新挂载：重新订阅 WebSocket、重新加载时间线 -->
    <Chatter v-if="visible" :biz-type="bizType" :biz-id="bizId" />
  </el-drawer>
</template>

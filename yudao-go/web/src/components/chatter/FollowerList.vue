<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { chatterApi } from '@/api/chatter'
import type { Follower } from '@/types/chatter'

const props = defineProps<{ bizType: string; bizId: number }>()

// 演示用当前用户 ID；实际接入时从登录态读取。
const currentUserId = 1

const followers = ref<Follower[]>([])
const following = ref(false)
const busy = ref(false)

async function load(): Promise<void> {
  try {
    followers.value = await chatterApi.listFollowers(props.bizType, props.bizId)
    following.value = followers.value.some((f) => f.userId === currentUserId)
  } catch {
    /* ignore */
  }
}

async function toggle(): Promise<void> {
  busy.value = true
  try {
    if (following.value) {
      await chatterApi.unfollow(props.bizType, props.bizId)
    } else {
      await chatterApi.follow(props.bizType, props.bizId)
    }
    await load()
  } catch (e) {
    ElMessage.error((e as Error).message || '操作失败')
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="followers">
    <el-tooltip v-for="f in followers" :key="f.id" :content="f.userName" placement="top">
      <el-avatar :size="24">{{ f.userName.charAt(0) || '?' }}</el-avatar>
    </el-tooltip>
    <el-button
      size="small"
      :type="following ? 'default' : 'primary'"
      :loading="busy"
      @click="toggle"
    >
      {{ following ? '已关注' : '关注' }}
    </el-button>
  </div>
</template>

<style scoped>
.followers {
  display: flex;
  align-items: center;
  gap: 4px;
}
</style>

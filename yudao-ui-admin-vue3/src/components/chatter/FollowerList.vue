<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Setting } from '@element-plus/icons-vue'
import { chatterApi } from '@/api/chatter'
import { useUserStore } from '@/store/modules/user'
import type { Follower } from '@/types/chatter'

const props = defineProps<{ bizType: string; bizId: number }>()

// 当前登录用户的 ID（关注是按用户维度的，必须读登录态而非写死）。
const currentUserId = useUserStore().getUser.id

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

// 订阅设置弹窗
const settingsVisible = ref(false)
const muted = ref(false)
const subscribeTypes = ref<string[]>([])
const SUBSCRIBE_TYPES = [
  { value: 'create', label: '记录创建' },
  { value: 'update', label: '字段修改' },
  { value: 'comment', label: '评论' },
  { value: 'approval', label: '审批动态' },
  { value: 'status_change', label: '状态变更' },
  { value: 'attachment', label: '附件' },
  { value: 'system', label: '系统通知' }
]
function openSettings(): void {
  const me = followers.value.find((f) => f.userId === currentUserId)
  muted.value = me?.muted ?? false
  // 后端 subscribe_types 为 JSON 数组,空数组 = 订阅全部
  subscribeTypes.value = me?.subscribeTypes ? [...me.subscribeTypes] : []
  settingsVisible.value = true
}
async function saveSettings(): Promise<void> {
  busy.value = true
  try {
    await chatterApi.updateFollowerSettings({
      bizType: props.bizType,
      bizId: props.bizId,
      subscribeTypes: subscribeTypes.value,
      muted: muted.value
    })
    settingsVisible.value = false
    ElMessage.success('已更新订阅设置')
    await load()
  } catch (e) {
    ElMessage.error((e as Error).message || '保存失败')
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
    <!-- 已关注才显示订阅设置齿轮 -->
    <el-tooltip v-if="following" content="订阅设置" placement="top">
      <el-button size="small" :icon="Setting" circle @click="openSettings" />
    </el-tooltip>

    <el-dialog v-model="settingsVisible" title="订阅设置" width="460">
      <el-form label-width="100px">
        <el-form-item label="静音">
          <el-switch v-model="muted" />
          <span class="ax-hint">静音时仅记录到时间线,不再触发通知</span>
        </el-form-item>
        <el-form-item label="订阅事件">
          <el-checkbox-group v-model="subscribeTypes" :disabled="muted">
            <el-checkbox
              v-for="t in SUBSCRIBE_TYPES"
              :key="t.value"
              :value="t.value"
            >
              {{ t.label }}
            </el-checkbox>
          </el-checkbox-group>
          <div class="ax-hint">不勾选任何项 = 订阅全部</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="settingsVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="saveSettings">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.followers {
  display: flex;
  align-items: center;
  gap: 4px;
}
.ax-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  display: block;
}
</style>

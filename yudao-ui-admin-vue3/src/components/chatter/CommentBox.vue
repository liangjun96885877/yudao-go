<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Paperclip, Close } from '@element-plus/icons-vue'
import { chatterApi } from '@/api/chatter'
import { getSimpleUserList } from '@/api/system/user'

const props = defineProps<{
  bizType: string
  bizId: number
  parentId?: number
  replyTo?: string // 显示「正在回复 xxx」用
}>()
const emit = defineEmits<{
  (e: 'submitted'): void
  (e: 'cancel-reply'): void
}>()

const content = ref('')
const submitting = ref(false)
const fileInput = ref<HTMLInputElement>()

// @ 提及候选
const allUsers = ref<{ id: number; nickname: string }[]>([])
const mentionOptions = ref<{ value: string; label: string }[]>([])
// 已选中过的 @用户：nickname → id（提交时按文本里是否仍含 @nickname 过滤）
const mentionedIds = ref(new Map<string, number>())

async function loadUsers() {
  if (allUsers.value.length === 0) {
    try {
      const list: any = await getSimpleUserList()
      allUsers.value = (list || []).map((u: any) => ({
        id: u.id,
        nickname: u.nickname || u.name || ('user' + u.id)
      }))
    } catch {
      allUsers.value = []
    }
  }
}
function onMentionSearch(pattern: string) {
  loadUsers().then(() => {
    const p = (pattern || '').toLowerCase()
    mentionOptions.value = allUsers.value
      .filter((u) => u.nickname.toLowerCase().includes(p))
      .slice(0, 10)
      .map((u) => ({ value: u.nickname, label: u.nickname }))
  })
}
function onMentionSelect(option: any) {
  const u = allUsers.value.find((x) => x.nickname === option.value)
  if (u) mentionedIds.value.set(u.nickname, u.id)
}
function collectMentionUserIDs(): number[] {
  const ids: number[] = []
  for (const [name, id] of mentionedIds.value) {
    if (content.value.includes('@' + name)) ids.push(id)
  }
  return ids
}

async function submit(): Promise<void> {
  const text = content.value.trim()
  if (!text) return
  submitting.value = true
  try {
    await chatterApi.addComment({
      bizType: props.bizType,
      bizId: props.bizId,
      content: text,
      parentId: props.parentId,
      mentionUserIds: collectMentionUserIDs()
    })
    content.value = ''
    mentionedIds.value.clear()
    ElMessage.success(props.parentId ? '已发表回复' : '已发表评论')
    emit('submitted')
    if (props.parentId) emit('cancel-reply')
  } catch (e) {
    ElMessage.error((e as Error).message || '评论失败')
  } finally {
    submitting.value = false
  }
}

// 附件：演示用直接以文件元数据登记；实际接入应先上传到文件服务取得 fileId。
async function onFilePicked(e: Event): Promise<void> {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    await chatterApi.linkAttachment({
      bizType: props.bizType,
      bizId: props.bizId,
      files: [
        {
          fileId: Date.now(),
          fileName: file.name,
          fileUrl: '',
          fileSize: file.size,
          contentType: file.type
        }
      ]
    })
    ElMessage.success('附件已添加')
    emit('submitted')
  } catch (err) {
    ElMessage.error((err as Error).message || '附件添加失败')
  } finally {
    if (fileInput.value) fileInput.value.value = ''
  }
}
</script>

<template>
  <div class="ax-input">
    <div v-if="parentId" class="ax-input__reply-banner">
      <span>正在回复 <b>{{ replyTo || '评论' }}</b></span>
      <el-icon class="ax-input__reply-close" @click="emit('cancel-reply')"><Close /></el-icon>
    </div>
    <el-mention
      v-model="content"
      type="textarea"
      :rows="3"
      :options="mentionOptions"
      :prefix="['@']"
      :placeholder="parentId ? '回复…（@ 触发用户提及，Ctrl + Enter 发送）' : '在此输入评论…（@ 触发用户提及）'"
      @search="onMentionSearch"
      @select="onMentionSelect"
      @keydown.ctrl.enter="submit"
    />
    <div class="ax-input__bar">
      <button class="ax-btn ax-btn--primary" :disabled="submitting" @click="submit">
        {{ parentId ? '回复' : '发表' }}
      </button>
      <button v-if="!parentId" class="ax-btn ax-btn--icon" title="添加附件" @click="fileInput?.click()">
        <el-icon><Paperclip /></el-icon>
      </button>
      <input ref="fileInput" type="file" hidden @change="onFilePicked" />
      <span class="ax-input__hint">Ctrl + Enter 发送</span>
    </div>
  </div>
</template>

<style scoped>
.ax-input {
  margin-bottom: 4px;
}
.ax-input__reply-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  margin-bottom: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  border-radius: 6px;
}
.ax-input__reply-close {
  cursor: pointer;
}
.ax-input__bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.ax-input__hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.ax-btn {
  height: 32px;
  padding: 0 14px;
  font-size: 13px;
  border-radius: 6px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--chatter-accent, #6259e8);
  background: #fff;
  color: var(--chatter-accent, #6259e8);
  transition: opacity 0.15s;
}
.ax-btn:hover {
  opacity: 0.85;
}
.ax-btn--primary {
  background: var(--chatter-accent, #6259e8);
  color: #fff;
}
.ax-btn--primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.ax-btn--icon {
  width: 32px;
  padding: 0;
}
</style>

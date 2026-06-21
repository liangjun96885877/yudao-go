<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { chatterApi } from '@/api/chatter'

const props = defineProps<{ bizType: string; bizId: number }>()
const emit = defineEmits<{ (e: 'submitted'): void }>()

const content = ref('')
const submitting = ref(false)

async function submit(): Promise<void> {
  const text = content.value.trim()
  if (!text) return
  submitting.value = true
  try {
    await chatterApi.addComment({ bizType: props.bizType, bizId: props.bizId, content: text })
    content.value = ''
    ElMessage.success('已发表评论')
    emit('submitted')
  } catch (e) {
    ElMessage.error((e as Error).message || '评论失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="cbox">
    <el-input
      v-model="content"
      type="textarea"
      :rows="2"
      :maxlength="2000"
      show-word-limit
      resize="none"
      placeholder="写评论…（支持 @用户）"
      @keydown.ctrl.enter="submit"
    />
    <div class="cbox__bar">
      <span class="cbox__hint">Ctrl + Enter 发送</span>
      <el-button type="primary" size="small" :loading="submitting" @click="submit">
        发表
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.cbox {
  margin-bottom: 12px;
}
.cbox__bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 6px;
}
.cbox__hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>

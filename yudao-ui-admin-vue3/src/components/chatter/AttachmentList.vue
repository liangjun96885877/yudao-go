<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { chatterApi } from '@/api/chatter'
import type { Attachment } from '@/types/chatter'

const props = defineProps<{ bizType: string; bizId: number }>()
const list = ref<Attachment[]>([])

async function load(): Promise<void> {
  try {
    list.value = await chatterApi.listAttachments(props.bizType, props.bizId)
  } catch {
    /* ignore */
  }
}

// 演示：直接以文件元数据登记附件。
// 实际接入时应先上传到 infra 文件服务取得 fileId / fileUrl，再调用 linkAttachment。
async function onChange(file: UploadFile): Promise<void> {
  if (!file.raw) return
  try {
    await chatterApi.linkAttachment({
      bizType: props.bizType,
      bizId: props.bizId,
      files: [
        {
          fileId: Date.now(),
          fileName: file.name,
          fileUrl: '',
          fileSize: file.size ?? 0,
          contentType: file.raw.type || '',
        },
      ],
    })
    ElMessage.success('附件已关联')
    await load()
  } catch (e) {
    ElMessage.error((e as Error).message || '附件关联失败')
  }
}

function sizeText(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

onMounted(load)
</script>

<template>
  <div class="attach">
    <el-upload :auto-upload="false" :show-file-list="false" :on-change="onChange">
      <el-button size="small" type="primary">上传附件</el-button>
    </el-upload>
    <el-empty v-if="list.length === 0" description="暂无附件" :image-size="60" />
    <ul v-else class="attach__list">
      <li v-for="a in list" :key="a.id">
        <span class="attach__name">{{ a.fileName }}</span>
        <span class="attach__meta">
          {{ sizeText(a.fileSize) }} · {{ a.uploaderName }} · {{ a.createTime }}
        </span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.attach__list {
  list-style: none;
  padding: 0;
  margin: 8px 0 0;
}
.attach__list li {
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.attach__name {
  font-size: 13px;
}
.attach__meta {
  display: block;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
</style>

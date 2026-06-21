<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Chatter from '@/components/chatter/Chatter.vue'

defineOptions({ name: 'Chatter' })

// 演示:选择业务对象,右侧 Chatter 面板展示其动态。
// 顶栏铃铛点击某条通知时,会带 query.bizType / query.bizId 跳进来,自动定位到该记录的动态。
const route = useRoute()
const bizType = ref<string>((route.query.bizType as string) || 'crm_customer')
const bizId = ref<number>(Number(route.query.bizId) || 2002)

watch(
  () => [route.query.bizType, route.query.bizId],
  ([bt, bid]) => {
    if (bt) bizType.value = String(bt)
    if (bid) bizId.value = Number(bid)
  }
)
</script>

<template>
  <div class="chatter-page">
    <el-card shadow="never" class="chatter-page__intro">
      <template #header>
        <span style="font-weight: 600">业务时间线（Chatter）</span>
      </template>
      <p class="chatter-page__hint">
        为任意业务对象提供操作历史、字段变更审计、评论、@、附件、关注与实时通知。
        修改下方业务类型 / ID 可切换 Chatter 挂载的记录。
      </p>
      <el-form :inline="true">
        <el-form-item label="业务类型">
          <el-input v-model="bizType" style="width: 220px" />
        </el-form-item>
        <el-form-item label="业务 ID">
          <el-input v-model.number="bizId" style="width: 160px" />
        </el-form-item>
      </el-form>
    </el-card>

    <Chatter :key="`${bizType}-${bizId}`" :biz-type="bizType" :biz-id="bizId" />
  </div>
</template>

<style scoped>
.chatter-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.chatter-page__hint {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin: 0 0 8px;
}
</style>

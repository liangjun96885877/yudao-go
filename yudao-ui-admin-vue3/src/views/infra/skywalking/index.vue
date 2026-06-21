<template>
  <doc-alert title="链路追踪" url="https://opentelemetry.io/" />

  <ContentWrap :bodyStyle="{ padding: '0px' }" class="!mb-0">
    <IFrame v-if="!loading" v-loading="loading" :src="src" />
  </ContentWrap>
</template>
<script lang="ts" setup>
import * as ConfigApi from '@/api/infra/config'

defineOptions({ name: 'InfraSkyWalking' })

const loading = ref(true) // 是否加载中
// 链路追踪改用 Jaeger（OpenTelemetry 采集 Go 后端 + BPM Java 服务）。
// 可通过配置项 url.skywalking 覆盖为其它地址。
const src = ref('http://localhost:16686')

/** 初始化 */
onMounted(async () => {
  try {
    const data = await ConfigApi.getConfigKey('url.skywalking')
    if (data && data.length > 0) {
      src.value = data
    }
  } finally {
    loading.value = false
  }
})
</script>

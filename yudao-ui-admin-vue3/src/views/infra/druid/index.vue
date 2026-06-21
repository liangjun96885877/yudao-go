<template>
  <doc-alert title="数据库 MyBatis" url="https://doc.iocoder.cn/mybatis/" />

  <el-row :gutter="12">
    <!-- 连接池 -->
    <el-col :span="24" class="mb-3">
      <el-card shadow="hover">
        <template #header><span class="font-bold">数据库连接池</span></template>
        <el-descriptions :column="4" border>
          <el-descriptions-item label="最大连接数">{{ pool.maxOpenConnections }}</el-descriptions-item>
          <el-descriptions-item label="当前连接数">{{ pool.openConnections }}</el-descriptions-item>
          <el-descriptions-item label="使用中">{{ pool.inUse }}</el-descriptions-item>
          <el-descriptions-item label="空闲">{{ pool.idle }}</el-descriptions-item>
          <el-descriptions-item label="等待次数">{{ pool.waitCount }}</el-descriptions-item>
          <el-descriptions-item label="等待总时长">{{ pool.waitDurationMs }} ms</el-descriptions-item>
          <el-descriptions-item label="超空闲关闭">{{ pool.maxIdleClosed }}</el-descriptions-item>
          <el-descriptions-item label="超寿命关闭">{{ pool.maxLifetimeClosed }}</el-descriptions-item>
        </el-descriptions>
      </el-card>
    </el-col>
    <!-- MySQL 服务器 -->
    <el-col :span="24">
      <el-card shadow="hover">
        <template #header><span class="font-bold">MySQL 服务器</span></template>
        <el-descriptions :column="4" border>
          <el-descriptions-item label="版本">{{ server.version }}</el-descriptions-item>
          <el-descriptions-item label="最大连接数">{{ server.max_connections }}</el-descriptions-item>
          <el-descriptions-item label="运行时长">{{ uptimeText }}</el-descriptions-item>
          <el-descriptions-item label="历史连接数">{{ server.Connections }}</el-descriptions-item>
          <el-descriptions-item label="当前连接">{{ server.Threads_connected }}</el-descriptions-item>
          <el-descriptions-item label="活跃线程">{{ server.Threads_running }}</el-descriptions-item>
          <el-descriptions-item label="峰值连接">{{ server.Max_used_connections }}</el-descriptions-item>
          <el-descriptions-item label="失败连接">{{ server.Aborted_connects }}</el-descriptions-item>
          <el-descriptions-item label="查询总数">{{ server.Queries }}</el-descriptions-item>
          <el-descriptions-item label="慢查询">{{ server.Slow_queries }}</el-descriptions-item>
          <el-descriptions-item label="表锁等待">{{ server.Table_locks_waited }}</el-descriptions-item>
          <el-descriptions-item label="行读取数">{{ server.Innodb_rows_read }}</el-descriptions-item>
          <el-descriptions-item label="SELECT">{{ server.Com_select }}</el-descriptions-item>
          <el-descriptions-item label="INSERT">{{ server.Com_insert }}</el-descriptions-item>
          <el-descriptions-item label="UPDATE">{{ server.Com_update }}</el-descriptions-item>
          <el-descriptions-item label="DELETE">{{ server.Com_delete }}</el-descriptions-item>
          <el-descriptions-item label="接收流量">{{ bytesText(server.Bytes_received) }}</el-descriptions-item>
          <el-descriptions-item label="发送流量">{{ bytesText(server.Bytes_sent) }}</el-descriptions-item>
        </el-descriptions>
      </el-card>
    </el-col>
  </el-row>
</template>
<script lang="ts" setup>
import request from '@/config/axios'

defineOptions({ name: 'InfraDruid' })

const pool = ref<Record<string, any>>({})
const server = ref<Record<string, any>>({})

/** 运行时长（秒）转可读文本 */
const uptimeText = computed(() => {
  const s = Number(server.value.Uptime || 0)
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  return `${d} 天 ${h} 时 ${m} 分`
})

/** 字节数转可读单位 */
const bytesText = (v: any) => {
  let n = Number(v || 0)
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(2)} ${units[i]}`
}

/** 初始化 */
onMounted(async () => {
  const data = await request.get({ url: '/infra/db/get-monitor-info' })
  pool.value = data.pool || {}
  server.value = data.server || {}
})
</script>

<script setup lang="ts">
import { ref } from 'vue'
import Chatter from './components/chatter/Chatter.vue'
import NotificationBell from './components/chatter/NotificationBell.vue'

// 演示：模拟一个业务记录详情页，右侧挂载 Chatter 面板。
const bizType = ref('crm_customer')
const bizId = ref(2002)
</script>

<template>
  <div class="app">
    <header class="app__header">
      <span class="app__brand">yudao-go · 业务时间线演示</span>
      <NotificationBell />
    </header>

    <main class="app__main">
      <section class="app__record">
        <h3>客户档案 #{{ bizId }}</h3>
        <p class="app__hint">
          这里是业务记录主体（演示占位）。修改下方业务类型 / ID 可切换 Chatter 挂载的记录。
        </p>
        <el-form label-width="90px">
          <el-form-item label="业务类型">
            <el-input v-model="bizType" />
          </el-form-item>
          <el-form-item label="业务 ID">
            <el-input v-model.number="bizId" />
          </el-form-item>
        </el-form>
      </section>

      <aside class="app__chatter">
        <Chatter :key="`${bizType}-${bizId}`" :biz-type="bizType" :biz-id="bizId" />
      </aside>
    </main>
  </div>
</template>

<style>
body {
  margin: 0;
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  background: #f5f7fa;
}
.app__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
}
.app__brand {
  font-weight: 600;
}
.app__main {
  display: flex;
  gap: 24px;
  padding: 24px;
  align-items: flex-start;
}
.app__record {
  flex: 1;
  background: #fff;
  padding: 20px;
  border-radius: 8px;
}
.app__hint {
  color: #909399;
  font-size: 13px;
}
.app__chatter {
  width: 560px;
}
</style>

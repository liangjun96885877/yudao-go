import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发时把后端接口与 WebSocket 代理到 yudao-go 服务（端口 48090）。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': '/src' },
  },
  server: {
    port: 5173,
    proxy: {
      '/admin-api': 'http://127.0.0.1:48090',
      '/infra/ws': { target: 'ws://127.0.0.1:48090', ws: true },
    },
  },
})

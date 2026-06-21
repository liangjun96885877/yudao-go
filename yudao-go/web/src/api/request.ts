import axios from 'axios'

// 后端统一前缀；开发时由 vite 代理到 yudao-go 服务。
const request = axios.create({
  baseURL: '/admin-api',
  timeout: 15000,
})

// 请求拦截：附加令牌。演示用 devtoken，实际接入时从登录态读取。
request.interceptors.request.use((config) => {
  const token = localStorage.getItem('token') || 'devtoken'
  config.headers.Authorization = `Bearer ${token}`
  return config
})

// 响应拦截：拆解 CommonResult，业务码非 0 视为错误。
request.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && typeof body.code !== 'undefined') {
      if (body.code !== 0) {
        return Promise.reject(new Error(body.msg || '请求失败'))
      }
      return body.data
    }
    return body
  },
  (err) => Promise.reject(err),
)

// http 是已拆解 CommonResult 的便捷封装：Promise 直接解析为业务数据。
export const http = {
  get: (url: string, params?: Record<string, unknown>): Promise<any> =>
    request.get(url, { params }),
  post: (url: string, data?: unknown): Promise<any> => request.post(url, data),
  put: (url: string, data?: unknown): Promise<any> => request.put(url, data),
  del: (url: string, data?: unknown): Promise<any> => request.delete(url, { data }),
}

export default request

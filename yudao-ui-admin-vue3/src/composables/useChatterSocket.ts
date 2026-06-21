import { ref } from 'vue'
import { getAccessToken } from '@/utils/auth'
import type { NotificationItem, TimelineItem } from '@/types/chatter'

// chatter WebSocket：全模块共享一条连接，断线自动重连并恢复订阅。
// 直连 yudao-go 后端（VITE_BASE_URL），与原版 infra WebSocket 分别独立。

type TimelineHandler = (item: TimelineItem, bizType: string, bizId: number) => void
type NotificationHandler = (item: NotificationItem) => void

let ws: WebSocket | null = null
let reconnectDelay = 1000
let reconnectTimer: number | undefined

const subscriptions = new Set<string>() // "bizType|bizId"
const timelineHandlers = new Set<TimelineHandler>()
const notificationHandlers = new Set<NotificationHandler>()
const connected = ref(false)

function wsURL(): string {
  // VITE_BASE_URL 形如 http://localhost:48090，转为 ws/wss。
  const base = String(import.meta.env.VITE_BASE_URL || '').replace(/^http/, 'ws')
  const token = getAccessToken() || ''
  return `${base}/infra/ws?token=${encodeURIComponent(token)}`
}

function sendRaw(obj: unknown): void {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(obj))
  }
}

function scheduleReconnect(): void {
  if (reconnectTimer !== undefined) return
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = undefined
    reconnectDelay = Math.min(reconnectDelay * 2, 30000) // 指数退避
    connect()
  }, reconnectDelay)
}

function connect(): void {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return
  }
  ws = new WebSocket(wsURL())
  ws.onopen = () => {
    connected.value = true
    reconnectDelay = 1000
    subscriptions.forEach((key) => {
      const [bizType, bizId] = key.split('|')
      sendRaw({ type: 'subscribe', bizType, bizId: Number(bizId) })
    })
  }
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data as string)
      if (msg.type === 'timeline.new') {
        timelineHandlers.forEach((h) => h(msg.item, msg.bizType, msg.bizId))
      } else if (msg.type === 'notification.new') {
        notificationHandlers.forEach((h) => h(msg.item))
      }
    } catch {
      /* 忽略非法消息 */
    }
  }
  ws.onclose = () => {
    connected.value = false
    ws = null
    scheduleReconnect()
  }
  ws.onerror = () => ws?.close()
}

export function useChatterSocket() {
  function subscribe(bizType: string, bizId: number): void {
    subscriptions.add(`${bizType}|${bizId}`)
    connect()
    sendRaw({ type: 'subscribe', bizType, bizId })
  }
  function unsubscribe(bizType: string, bizId: number): void {
    subscriptions.delete(`${bizType}|${bizId}`)
    sendRaw({ type: 'unsubscribe', bizType, bizId })
  }
  function onTimeline(h: TimelineHandler): () => void {
    timelineHandlers.add(h)
    return () => timelineHandlers.delete(h)
  }
  function onNotification(h: NotificationHandler): () => void {
    notificationHandlers.add(h)
    return () => notificationHandlers.delete(h)
  }
  return { connected, subscribe, unsubscribe, onTimeline, onNotification }
}

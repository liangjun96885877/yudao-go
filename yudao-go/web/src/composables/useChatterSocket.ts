import { ref } from 'vue'
import type { NotificationItem, TimelineItem } from '@/types/chatter'

// chatter WebSocket：全模块共享一条连接，断线自动重连并恢复订阅。

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
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const token = localStorage.getItem('token') || 'devtoken'
  return `${proto}://${location.host}/infra/ws?token=${encodeURIComponent(token)}`
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
    // 重连后恢复全部订阅
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
  // on* 返回取消注册函数。
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

// chatter 模块前端类型定义，与后端 DTO 对齐。

export interface FieldChange {
  field: string
  label: string
  oldValue: string
  newValue: string
  oldDisplay: string
  newDisplay: string
  valueType: string
}

export interface TimelineItem {
  id: number
  eventType: string
  eventSubtype: string
  summary: string
  body: string
  actorType: number
  actorId: number
  actorName: string
  refType: string
  refId: number
  createTime: string
  changes?: FieldChange[]
}

export interface Follower {
  id: number
  userId: number
  userName: string
  reason: number
  muted: boolean
  createTime: string
}

export interface Attachment {
  id: number
  fileId: number
  fileName: string
  fileUrl: string
  fileSize: number
  contentType: string
  uploaderId: number
  uploaderName: string
  createTime: string
}

export interface NotificationItem {
  id: number
  bizType: string
  bizId: number
  timelineId: number
  type: string
  title: string
  content: string
  isRead: boolean
  createTime: string
}

export interface CursorPage<T> {
  list: T[]
  nextCursor: number
}

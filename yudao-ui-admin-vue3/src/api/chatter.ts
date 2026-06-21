import request from '@/config/axios'
import type {
  Attachment,
  CursorPage,
  Follower,
  NotificationItem,
  TimelineItem
} from '@/types/chatter'

// chatter 业务时间线接口封装（对接 yudao-go 后端）。
export const chatterApi = {
  // 时间线
  timeline: (bizType: string, bizId: number, cursor = 0, limit = 20): Promise<CursorPage<TimelineItem>> =>
    request.get({ url: '/chatter/timeline', params: { bizType, bizId, cursor, limit } }),

  // 时间线条目标记（已读 / 重要）
  setTimelineFlag: (id: number, read: boolean, important: boolean): Promise<unknown> =>
    request.put({ url: `/chatter/timeline/${id}/flag`, data: { read, important } }),

  // 评论
  addComment: (data: {
    bizType: string
    bizId: number
    content: string
    parentId?: number
    mentionUserIds?: number[]
  }): Promise<unknown> => request.post({ url: '/chatter/comment', data }),
  updateComment: (id: number, content: string, version: number): Promise<unknown> =>
    request.put({ url: `/chatter/comment/${id}`, data: { content, version } }),
  deleteComment: (id: number): Promise<unknown> =>
    request.delete({ url: `/chatter/comment/${id}` }),
  commentReplies: (id: number): Promise<TimelineItem[]> =>
    request.get({ url: `/chatter/comment/${id}/replies` }),

  // 关注者
  listFollowers: (bizType: string, bizId: number): Promise<Follower[]> =>
    request.get({ url: '/chatter/follower', params: { bizType, bizId } }),
  follow: (bizType: string, bizId: number): Promise<unknown> =>
    request.post({ url: '/chatter/follower', data: { bizType, bizId } }),
  unfollow: (bizType: string, bizId: number): Promise<unknown> =>
    request.delete({ url: '/chatter/follower', data: { bizType, bizId } }),
  updateFollowerSettings: (data: {
    bizType: string
    bizId: number
    subscribeTypes: string[]
    muted: boolean
  }): Promise<unknown> => request.put({ url: '/chatter/follower/settings', data }),

  // 附件
  listAttachments: (bizType: string, bizId: number): Promise<Attachment[]> =>
    request.get({ url: '/chatter/attachment', params: { bizType, bizId } }),
  linkAttachment: (data: {
    bizType: string
    bizId: number
    files: Array<{
      fileId: number
      fileName: string
      fileUrl: string
      fileSize: number
      contentType: string
    }>
  }): Promise<Attachment[]> => request.post({ url: '/chatter/attachment', data }),

  // 通知
  notifications: (cursor = 0, limit = 20, unread = false): Promise<CursorPage<NotificationItem>> =>
    request.get({ url: '/chatter/notification', params: { cursor, limit, unread } }),
  unreadCount: (): Promise<{ count: number }> =>
    request.get({ url: '/chatter/notification/unread-count' }),
  markRead: (id: number): Promise<unknown> =>
    request.put({ url: `/chatter/notification/${id}/read` }),
  markAllRead: (): Promise<unknown> => request.put({ url: '/chatter/notification/read-all' })
}

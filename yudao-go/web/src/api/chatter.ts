import { http } from './request'
import type {
  Attachment,
  CursorPage,
  Follower,
  NotificationItem,
  TimelineItem,
} from '@/types/chatter'

// chatter 模块接口封装。
export const chatterApi = {
  // 时间线
  timeline: (bizType: string, bizId: number, cursor = 0, limit = 20):
    Promise<CursorPage<TimelineItem>> =>
    http.get('/chatter/timeline', { bizType, bizId, cursor, limit }),

  // 评论
  addComment: (data: {
    bizType: string
    bizId: number
    content: string
    parentId?: number
    mentionUserIds?: number[]
  }): Promise<unknown> => http.post('/chatter/comment', data),
  updateComment: (id: number, content: string, version: number): Promise<unknown> =>
    http.put(`/chatter/comment/${id}`, { content, version }),
  deleteComment: (id: number): Promise<unknown> => http.del(`/chatter/comment/${id}`),

  // 关注者
  listFollowers: (bizType: string, bizId: number): Promise<Follower[]> =>
    http.get('/chatter/follower', { bizType, bizId }),
  follow: (bizType: string, bizId: number): Promise<unknown> =>
    http.post('/chatter/follower', { bizType, bizId }),
  unfollow: (bizType: string, bizId: number): Promise<unknown> =>
    http.del('/chatter/follower', { bizType, bizId }),

  // 附件
  listAttachments: (bizType: string, bizId: number): Promise<Attachment[]> =>
    http.get('/chatter/attachment', { bizType, bizId }),
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
  }): Promise<Attachment[]> => http.post('/chatter/attachment', data),

  // 通知
  notifications: (cursor = 0, limit = 20, unread = false):
    Promise<CursorPage<NotificationItem>> =>
    http.get('/chatter/notification', { cursor, limit, unread }),
  unreadCount: (): Promise<{ count: number }> =>
    http.get('/chatter/notification/unread-count'),
  markRead: (id: number): Promise<unknown> =>
    http.put(`/chatter/notification/${id}/read`),
  markAllRead: (): Promise<unknown> => http.put('/chatter/notification/read-all'),
}

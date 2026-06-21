// Package repository 定义 chatter 领域仓储接口。实现位于 infrastructure/persistence。
package repository

import (
	"context"

	"yudao-go/internal/module/chatter/domain/model"
)

// TimelineRepository 时间线仓储。
type TimelineRepository interface {
	Save(ctx context.Context, t *model.Timeline) error
	SaveAuditLogs(ctx context.Context, logs []*model.AuditLog) error
	// PageByBiz 按业务记录游标分页查询，cursor 为上一页最小 id（首页传 0）。
	PageByBiz(ctx context.Context, ref model.BizRef, cursor int64, limit int) ([]*model.Timeline, error)
	// ListAuditLogs 按时间线 ID 批量查询字段变更明细。
	ListAuditLogs(ctx context.Context, timelineIDs []int64) ([]*model.AuditLog, error)
	ExistsByEventID(ctx context.Context, eventID string) (bool, error)
	// ListFlags 查询某用户对一批时间线的标记。
	ListFlags(ctx context.Context, userID int64, timelineIDs []int64) ([]*model.TimelineFlag, error)
	// UpsertFlag 写入/更新某用户对某条时间线的标记。
	UpsertFlag(ctx context.Context, timelineID, userID int64, read, important bool) error
	// ListByRefs 按 ref_type + ref_id 集合查询时间线条目(用于展开评论的回复链)。
	ListByRefs(ctx context.Context, refType string, refIDs []int64) ([]*model.Timeline, error)
}

// CommentRepository 评论仓储。
type CommentRepository interface {
	Create(ctx context.Context, c *model.Comment) error
	// Update 基于乐观锁更新（WHERE version=?）；版本不匹配应返回 errcode.Conflict。
	Update(ctx context.Context, c *model.Comment) error
	GetByID(ctx context.Context, id int64) (*model.Comment, error)
	DeleteByID(ctx context.Context, id int64) error
	// CountChildrenByParents 批量统计每个父评论的直接子评论数(parent_id IN ?)。
	CountChildrenByParents(ctx context.Context, parentIDs []int64) (map[int64]int, error)
	// ListIDsByParent 列出指定父评论的所有直接子评论 ID,按创建时间升序。
	ListIDsByParent(ctx context.Context, parentID int64) ([]int64, error)
}

// FollowerRepository 关注者仓储。
type FollowerRepository interface {
	// Add 新增或更新关注关系（按 biz+user 幂等）。
	Add(ctx context.Context, f *model.Follower) error
	Remove(ctx context.Context, ref model.BizRef, userID int64) error
	ListByBiz(ctx context.Context, ref model.BizRef) ([]*model.Follower, error)
	Exists(ctx context.Context, ref model.BizRef, userID int64) (bool, error)
	// UpdateSettings 更新订阅事件类型与静音标记。
	UpdateSettings(ctx context.Context, ref model.BizRef, userID int64, subscribeTypes []string, muted bool) error
}

// AttachmentRepository 附件仓储。
type AttachmentRepository interface {
	CreateBatch(ctx context.Context, items []*model.Attachment) error
	ListByBiz(ctx context.Context, ref model.BizRef) ([]*model.Attachment, error)
}

// NotificationRepository 通知仓储。
type NotificationRepository interface {
	CreateBatch(ctx context.Context, items []*model.Notification) error
	PageByRecipient(ctx context.Context, tenantID, recipientID, cursor int64, limit int, unreadOnly bool) ([]*model.Notification, error)
	MarkRead(ctx context.Context, tenantID, recipientID, id int64) error
	MarkAllRead(ctx context.Context, tenantID, recipientID int64) error
	UnreadCount(ctx context.Context, tenantID, recipientID int64) (int64, error)
}

package persistence

import (
	"context"
	"time"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
)

// NotificationRepo 是 NotificationRepository 的 GORM 实现。
type NotificationRepo struct {
	tx *orm.TxManager
}

func NewNotificationRepo(tx *orm.TxManager) *NotificationRepo { return &NotificationRepo{tx: tx} }

func (r *NotificationRepo) CreateBatch(ctx context.Context, items []*model.Notification) error {
	if len(items) == 0 {
		return nil
	}
	pos := make([]*NotificationPO, 0, len(items))
	for _, n := range items {
		pos = append(pos, toNotificationPO(n))
	}
	return r.tx.DB(ctx).Create(pos).Error
}

func (r *NotificationRepo) PageByRecipient(
	ctx context.Context, tenantID, recipientID, cursor int64, limit int, unreadOnly bool,
) ([]*model.Notification, error) {
	q := r.tx.DB(ctx).Model(&NotificationPO{}).Where("recipient_id = ?", recipientID)
	if unreadOnly {
		q = q.Where("is_read = ?", false)
	}
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var pos []*NotificationPO
	if err := q.Order("id DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Notification, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromNotificationPO(po))
	}
	return out, nil
}

func (r *NotificationRepo) MarkRead(ctx context.Context, tenantID, recipientID, id int64) error {
	return r.tx.DB(ctx).Model(&NotificationPO{}).
		Where("id = ? AND recipient_id = ?", id, recipientID).
		Updates(map[string]any{"is_read": true, "read_at": time.Now()}).Error
}

func (r *NotificationRepo) MarkAllRead(ctx context.Context, tenantID, recipientID int64) error {
	return r.tx.DB(ctx).Model(&NotificationPO{}).
		Where("recipient_id = ? AND is_read = ?", recipientID, false).
		Updates(map[string]any{"is_read": true, "read_at": time.Now()}).Error
}

func (r *NotificationRepo) UnreadCount(ctx context.Context, tenantID, recipientID int64) (int64, error) {
	var count int64
	err := r.tx.DB(ctx).Model(&NotificationPO{}).
		Where("recipient_id = ? AND is_read = ?", recipientID, false).
		Count(&count).Error
	return count, err
}

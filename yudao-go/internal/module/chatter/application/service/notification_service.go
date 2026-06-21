package service

import (
	"context"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/application/assembler"
	"yudao-go/internal/module/chatter/application/dto"
	"yudao-go/internal/module/chatter/domain/repository"
	"yudao-go/internal/pkg/errcode"
)

// NotificationService 通知（用户收件箱）应用服务。
type NotificationService struct {
	notifications repository.NotificationRepository
	tx            *orm.TxManager
}

func NewNotificationService(
	notifications repository.NotificationRepository, tx *orm.TxManager,
) *NotificationService {
	return &NotificationService{notifications: notifications, tx: tx}
}

// recipient 返回当前登录用户的租户与用户 ID，未登录返回错误。
func (s *NotificationService) recipient(ctx context.Context) (tenantID, userID int64, err error) {
	userID = contextx.UserID(ctx)
	if userID == 0 {
		return 0, 0, errcode.Unauthorized
	}
	return contextx.TenantID(ctx), userID, nil
}

// Inbox 游标分页查询当前用户的通知。
func (s *NotificationService) Inbox(
	ctx context.Context, cursor int64, limit int, unreadOnly bool,
) (*dto.CursorPage[*dto.NotificationDTO], error) {
	tenantID, userID, err := s.recipient(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxFeedLimit {
		limit = defaultFeedLimit
	}
	list, err := s.notifications.PageByRecipient(ctx, tenantID, userID, cursor, limit, unreadOnly)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.NotificationDTO, 0, len(list))
	var nextCursor int64
	for _, n := range list {
		items = append(items, assembler.ToNotificationDTO(n))
		nextCursor = n.ID
	}
	if len(list) < limit {
		nextCursor = 0
	}
	return &dto.CursorPage[*dto.NotificationDTO]{List: items, NextCursor: nextCursor}, nil
}

// MarkRead 标记一条通知为已读。
func (s *NotificationService) MarkRead(ctx context.Context, id int64) error {
	tenantID, userID, err := s.recipient(ctx)
	if err != nil {
		return err
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.notifications.MarkRead(ctx, tenantID, userID, id)
	})
}

// MarkAllRead 标记当前用户全部通知为已读。
func (s *NotificationService) MarkAllRead(ctx context.Context) error {
	tenantID, userID, err := s.recipient(ctx)
	if err != nil {
		return err
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.notifications.MarkAllRead(ctx, tenantID, userID)
	})
}

// UnreadCount 返回当前用户的未读通知数。
func (s *NotificationService) UnreadCount(ctx context.Context) (int64, error) {
	tenantID, userID, err := s.recipient(ctx)
	if err != nil {
		return 0, err
	}
	return s.notifications.UnreadCount(ctx, tenantID, userID)
}

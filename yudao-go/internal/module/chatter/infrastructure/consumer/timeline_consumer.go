// Package consumer 消费领域事件，构建时间线与通知，并触发实时推送。
package consumer

import (
	"context"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/domain/repository"
	"yudao-go/internal/module/chatter/domain/service"
	"yudao-go/internal/module/chatter/infrastructure/outbox"
)

// consumerGroup 是时间线消费者的幂等分组标识。
const consumerGroup = "chatter-timeline"

// WSNotifier 把领域变更实时推送给前端。
type WSNotifier interface {
	PushTimeline(ctx context.Context, tl *model.Timeline)
	PushNotifications(ctx context.Context, ns []*model.Notification)
}

// NopNotifier 不做任何推送（用于测试或禁用实时推送）。
type NopNotifier struct{}

func (NopNotifier) PushTimeline(context.Context, *model.Timeline)            {}
func (NopNotifier) PushNotifications(context.Context, []*model.Notification) {}

// TimelineConsumer 订阅 chatter 领域事件，生成时间线条目与通知。
type TimelineConsumer struct {
	timelines     repository.TimelineRepository
	comments      repository.CommentRepository
	followers     repository.FollowerRepository
	notifications repository.NotificationRepository
	dedup         *outbox.Deduplicator
	notifier      WSNotifier
	factory       service.TimelineFactory
	tx            *orm.TxManager
}

func NewTimelineConsumer(
	timelines repository.TimelineRepository,
	comments repository.CommentRepository,
	followers repository.FollowerRepository,
	notifications repository.NotificationRepository,
	dedup *outbox.Deduplicator,
	notifier WSNotifier,
	tx *orm.TxManager,
) *TimelineConsumer {
	return &TimelineConsumer{
		timelines: timelines, comments: comments, followers: followers,
		notifications: notifications, dedup: dedup, notifier: notifier, tx: tx,
	}
}

// Register 将各事件主题订阅到对应处理器。
func (c *TimelineConsumer) Register(bus eventbus.Bus) error {
	subs := []struct {
		topic   string
		handler eventbus.Handler
	}{
		{event.TopicRecordCreated, c.onRecordCreated},
		{event.TopicRecordUpdated, c.onRecordUpdated},
		{event.TopicStatusChanged, c.onStatusChanged},
		{event.TopicApproval, c.onApproval},
		{event.TopicCommentAdded, c.onCommentAdded},
		{event.TopicAttachmentAdded, c.onAttachmentAdded},
		{event.TopicFollowerAdded, c.onFollowerAdded},
	}
	for _, s := range subs {
		if err := bus.Subscribe(s.topic, s.handler); err != nil {
			return err
		}
	}
	return nil
}

// notifySpec 描述一次通知扇出；为 nil 表示不产生通知。
type notifySpec struct {
	notifyType model.NotificationType
	title      string
	extra      []int64 // 额外接收人（如被 @ 的用户）
}

// process 落库时间线 + 审计明细 + 通知（一个事务、幂等），提交后推送 WebSocket。
func (c *TimelineConsumer) process(
	ctx context.Context,
	tl *model.Timeline,
	logsFn func(timelineID int64) []*model.AuditLog,
	spec *notifySpec,
) error {
	// 事件经异步总线传递，ctx 不含租户；从事件的业务引用恢复租户上下文。
	ctx = contextx.WithTenantID(ctx, tl.Ref.TenantID)

	var notifications []*model.Notification
	err := c.tx.Do(ctx, func(ctx context.Context) error {
		// 幂等：事务内登记消费记录，重复事件直接跳过。
		first, err := c.dedup.MarkConsumed(ctx, consumerGroup, tl.EventID)
		if err != nil {
			return err
		}
		if !first {
			return nil
		}
		if err := c.timelines.Save(ctx, tl); err != nil {
			return err
		}
		if logsFn != nil {
			if err := c.timelines.SaveAuditLogs(ctx, logsFn(tl.ID)); err != nil {
				return err
			}
		}
		if spec != nil {
			ns, err := c.fanout(ctx, tl, spec)
			if err != nil {
				return err
			}
			notifications = ns
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务提交后推送实时消息。tl.ID==0 表示重复事件被跳过，不推送。
	if tl.ID != 0 {
		c.notifier.PushTimeline(ctx, tl)
		if len(notifications) > 0 {
			c.notifier.PushNotifications(ctx, notifications)
		}
	}
	return nil
}

// fanout 为关注者及额外接收人生成通知，排除操作者本人，返回已创建的通知。
func (c *TimelineConsumer) fanout(
	ctx context.Context, tl *model.Timeline, spec *notifySpec,
) ([]*model.Notification, error) {
	followers, err := c.followers.ListByBiz(ctx, tl.Ref)
	if err != nil {
		return nil, err
	}
	recipients := make(map[int64]struct{})
	for _, f := range followers {
		if f.UserID == tl.Actor.ID || !f.Subscribed(tl.EventType) {
			continue
		}
		recipients[f.UserID] = struct{}{}
	}
	for _, uid := range spec.extra {
		if uid != tl.Actor.ID {
			recipients[uid] = struct{}{}
		}
	}
	if len(recipients) == 0 {
		return nil, nil
	}
	items := make([]*model.Notification, 0, len(recipients))
	for uid := range recipients {
		items = append(items, &model.Notification{
			TenantID:    tl.Ref.TenantID,
			RecipientID: uid,
			Ref:         tl.Ref,
			TimelineID:  tl.ID,
			Type:        spec.notifyType,
			Title:       spec.title,
			Content:     tl.Summary,
		})
	}
	if err := c.notifications.CreateBatch(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

// --- 各事件处理器 ---

func (c *TimelineConsumer) onRecordCreated(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.RecordCreated)
	if !ok {
		return nil
	}
	return c.process(ctx, c.factory.FromRecordCreated(evt), nil,
		&notifySpec{notifyType: model.NotifyCreate, title: "新建记录"})
}

func (c *TimelineConsumer) onRecordUpdated(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.RecordUpdated)
	if !ok {
		return nil
	}
	return c.process(ctx, c.factory.FromRecordUpdated(evt),
		func(timelineID int64) []*model.AuditLog { return c.factory.AuditLogsFor(evt, timelineID) },
		&notifySpec{notifyType: model.NotifyUpdate, title: "记录更新"})
}

func (c *TimelineConsumer) onStatusChanged(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.StatusChanged)
	if !ok {
		return nil
	}
	return c.process(ctx, c.factory.FromStatusChanged(evt), nil,
		&notifySpec{notifyType: model.NotifyStatus, title: "状态变更"})
}

func (c *TimelineConsumer) onApproval(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.ApprovalProcessed)
	if !ok {
		return nil
	}
	return c.process(ctx, c.factory.FromApproval(evt), nil,
		&notifySpec{notifyType: model.NotifyApproval, title: "审批动态"})
}

func (c *TimelineConsumer) onCommentAdded(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.CommentAdded)
	if !ok {
		return nil
	}
	comment, err := c.comments.GetByID(ctx, evt.CommentID)
	if err != nil {
		return err
	}
	return c.process(ctx, c.factory.FromCommentAdded(evt, comment), nil,
		&notifySpec{notifyType: model.NotifyComment, title: "新评论", extra: evt.Mentions})
}

func (c *TimelineConsumer) onAttachmentAdded(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.AttachmentAdded)
	if !ok {
		return nil
	}
	return c.process(ctx, c.factory.FromAttachmentAdded(evt), nil,
		&notifySpec{notifyType: model.NotifyAttachment, title: "新附件"})
}

// onFollowerAdded 生成关注类时间线（内部可见），不产生通知。
func (c *TimelineConsumer) onFollowerAdded(ctx context.Context, e eventbus.DomainEvent) error {
	evt, ok := e.(event.FollowerAdded)
	if !ok {
		return nil
	}
	if err := c.process(ctx, c.factory.FromFollowerAdded(evt), nil, nil); err != nil {
		logger.WithContext(ctx).Error("chatter: handle follower added failed", "error", err)
		return err
	}
	return nil
}

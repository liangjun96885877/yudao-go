package service

import (
	"fmt"

	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
)

// TimelineFactory 将领域事件转换为时间线条目（领域服务）。
type TimelineFactory struct{}

const bodyMaxLen = 1000 // 时间线 body 截断长度

// FromRecordCreated 创建类时间线。
func (TimelineFactory) FromRecordCreated(e event.RecordCreated) *model.Timeline {
	return &model.Timeline{
		Ref:        e.Ref,
		EventType:  model.EventCreate,
		Summary:    fmt.Sprintf("%s 创建了「%s」", e.Actor.Name, e.Title),
		Actor:      e.Actor,
		Visibility: model.VisibilityPublic,
		EventID:    e.EventID(),
	}
}

// FromRecordUpdated 字段修改类时间线（审计明细另由 AuditLogsFor 生成）。
func (TimelineFactory) FromRecordUpdated(e event.RecordUpdated) *model.Timeline {
	return &model.Timeline{
		Ref:        e.Ref,
		EventType:  model.EventUpdate,
		Summary:    fmt.Sprintf("%s 修改了 %d 个字段", e.Actor.Name, len(e.Changes)),
		Actor:      e.Actor,
		RefType:    "audit_batch",
		Visibility: model.VisibilityPublic,
		EventID:    e.EventID(),
	}
}

// AuditLogsFor 为字段修改事件生成审计明细，需在时间线落库取得 ID 后调用。
func (TimelineFactory) AuditLogsFor(e event.RecordUpdated, timelineID int64) []*model.AuditLog {
	logs := make([]*model.AuditLog, 0, len(e.Changes))
	for _, ch := range e.Changes {
		logs = append(logs, &model.AuditLog{
			Ref:        e.Ref,
			TimelineID: timelineID,
			Change:     ch,
		})
	}
	return logs
}

// FromStatusChanged 状态变更类时间线。
func (TimelineFactory) FromStatusChanged(e event.StatusChanged) *model.Timeline {
	return &model.Timeline{
		Ref:        e.Ref,
		EventType:  model.EventStatusChange,
		Summary:    fmt.Sprintf("状态变更：%s → %s", e.FromState, e.ToState),
		Actor:      e.Actor,
		Visibility: model.VisibilityPublic,
		EventID:    e.EventID(),
	}
}

// FromApproval 审批动态类时间线。
func (TimelineFactory) FromApproval(e event.ApprovalProcessed) *model.Timeline {
	subtype, summary := "rejected", fmt.Sprintf("%s 审批拒绝", e.Actor.Name)
	if e.Approved {
		subtype, summary = "approved", fmt.Sprintf("%s 审批通过", e.Actor.Name)
	}
	if e.Reason != "" {
		summary += "：" + e.Reason
	}
	return &model.Timeline{
		Ref:          e.Ref,
		EventType:    model.EventApproval,
		EventSubtype: subtype,
		Summary:      summary,
		Actor:        e.Actor,
		Visibility:   model.VisibilityPublic,
		EventID:      e.EventID(),
	}
}

// FromCommentAdded 评论类时间线。
func (TimelineFactory) FromCommentAdded(e event.CommentAdded, c *model.Comment) *model.Timeline {
	return &model.Timeline{
		Ref:        e.Ref,
		EventType:  model.EventComment,
		Summary:    fmt.Sprintf("%s 发表了评论", e.Actor.Name),
		Body:       truncate(c.Content, bodyMaxLen),
		Actor:      e.Actor,
		RefType:    "comment",
		RefID:      c.ID,
		Visibility: model.VisibilityPublic,
		EventID:    e.EventID(),
	}
}

// FromAttachmentAdded 附件类时间线。
func (TimelineFactory) FromAttachmentAdded(e event.AttachmentAdded) *model.Timeline {
	return &model.Timeline{
		Ref:        e.Ref,
		EventType:  model.EventAttachment,
		Summary:    fmt.Sprintf("%s 上传了附件「%s」", e.Actor.Name, e.FileName),
		Actor:      e.Actor,
		RefType:    "attachment",
		RefID:      e.AttachmentID,
		Visibility: model.VisibilityPublic,
		EventID:    e.EventID(),
	}
}

// FromFollowerAdded 关注类时间线（内部可见）。
func (TimelineFactory) FromFollowerAdded(e event.FollowerAdded) *model.Timeline {
	return &model.Timeline{
		Ref:        e.Ref,
		EventType:  model.EventFollow,
		Summary:    fmt.Sprintf("%s 关注了此记录", e.Actor.Name),
		Actor:      e.Actor,
		Visibility: model.VisibilityInternal,
		EventID:    e.EventID(),
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Package assembler 负责 chatter 领域模型与 DTO 之间的转换。
package assembler

import (
	"time"

	"yudao-go/internal/module/chatter/application/dto"
	"yudao-go/internal/module/chatter/domain/model"
)

const timeLayout = "2006-01-02 15:04:05"

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(timeLayout)
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return fmtTime(*t)
}

// ToCommentDTO 转换评论。
func ToCommentDTO(c *model.Comment) *dto.CommentDTO {
	return &dto.CommentDTO{
		ID:             c.ID,
		BizType:        c.Ref.BizType,
		BizID:          c.Ref.BizID,
		ParentID:       c.ParentID,
		RootID:         c.RootID,
		Content:        c.Content,
		ContentHTML:    c.ContentHTML,
		AuthorID:       c.Author.ID,
		AuthorName:     c.Author.Name,
		MentionUserIDs: c.MentionUserIDs,
		Version:        c.Version,
		EditedAt:       fmtTimePtr(c.EditedAt),
		CreateTime:     fmtTime(c.CreateTime),
	}
}

// ToFollowerDTO 转换关注者。
func ToFollowerDTO(f *model.Follower) *dto.FollowerDTO {
	subs := make([]string, 0, len(f.SubscribeTypes))
	for _, t := range f.SubscribeTypes {
		subs = append(subs, string(t))
	}
	return &dto.FollowerDTO{
		ID:             f.ID,
		UserID:         f.UserID,
		UserName:       f.UserName,
		Reason:         int8(f.Reason),
		SubscribeTypes: subs,
		Muted:          f.Muted,
		CreateTime:     fmtTime(f.CreateTime),
	}
}

// ToAttachmentDTO 转换附件。
func ToAttachmentDTO(a *model.Attachment) *dto.AttachmentDTO {
	return &dto.AttachmentDTO{
		ID:           a.ID,
		BizType:      a.Ref.BizType,
		BizID:        a.Ref.BizID,
		FileID:       a.FileID,
		FileName:     a.FileName,
		FileURL:      a.FileURL,
		FileSize:     a.FileSize,
		ContentType:  a.ContentType,
		UploaderID:   a.Uploader.ID,
		UploaderName: a.Uploader.Name,
		CreateTime:   fmtTime(a.CreateTime),
	}
}

// ToNotificationDTO 转换通知。
func ToNotificationDTO(n *model.Notification) *dto.NotificationDTO {
	return &dto.NotificationDTO{
		ID:         n.ID,
		BizType:    n.Ref.BizType,
		BizID:      n.Ref.BizID,
		TimelineID: n.TimelineID,
		Type:       string(n.Type),
		Title:      n.Title,
		Content:    n.Content,
		IsRead:     n.IsRead,
		CreateTime: fmtTime(n.CreateTime),
	}
}

// ToTimelineItemDTO 转换时间线条目，changes 为该条目（update 类型）的字段变更。
func ToTimelineItemDTO(t *model.Timeline, changes []*model.AuditLog) *dto.TimelineItemDTO {
	item := &dto.TimelineItemDTO{
		ID:           t.ID,
		EventType:    string(t.EventType),
		EventSubtype: t.EventSubtype,
		Summary:      t.Summary,
		Body:         t.Body,
		ActorType:    int8(t.Actor.Type),
		ActorID:      t.Actor.ID,
		ActorName:    t.Actor.Name,
		RefType:      t.RefType,
		RefID:        t.RefID,
		CreateTime:   fmtTime(t.CreateTime),
	}
	for _, a := range changes {
		item.Changes = append(item.Changes, dto.FieldChangeDTO{
			Field:      a.Change.Field,
			Label:      a.Change.Label,
			OldValue:   a.Change.OldValue,
			NewValue:   a.Change.NewValue,
			OldDisplay: a.Change.OldDisplay,
			NewDisplay: a.Change.NewDisplay,
			ValueType:  string(a.Change.ValueType),
		})
	}
	return item
}

package persistence

import "yudao-go/internal/module/chatter/domain/model"

// --- Timeline ---

func toTimelinePO(t *model.Timeline) *TimelinePO {
	po := &TimelinePO{
		BizType:      t.Ref.BizType,
		BizID:        t.Ref.BizID,
		EventType:    string(t.EventType),
		EventSubtype: t.EventSubtype,
		Summary:      t.Summary,
		Body:         t.Body,
		ActorType:    int8(t.Actor.Type),
		ActorID:      t.Actor.ID,
		ActorName:    t.Actor.Name,
		RefType:      t.RefType,
		RefID:        t.RefID,
		Visibility:   int8(t.Visibility),
		EventID:      t.EventID,
	}
	po.ID = t.ID
	po.TenantID = t.Ref.TenantID
	return po
}

func fromTimelinePO(po *TimelinePO) *model.Timeline {
	return &model.Timeline{
		ID: po.ID,
		Ref: model.BizRef{
			TenantID: po.TenantID, BizType: po.BizType, BizID: po.BizID,
		},
		EventType:    model.EventType(po.EventType),
		EventSubtype: po.EventSubtype,
		Summary:      po.Summary,
		Body:         po.Body,
		Actor:        model.Actor{Type: model.ActorType(po.ActorType), ID: po.ActorID, Name: po.ActorName},
		RefType:      po.RefType,
		RefID:        po.RefID,
		Visibility:   model.Visibility(po.Visibility),
		EventID:      po.EventID,
		CreateTime:   po.CreateTime,
	}
}

// --- AuditLog ---

func toAuditLogPO(a *model.AuditLog) *AuditLogPO {
	po := &AuditLogPO{
		BizType:    a.Ref.BizType,
		BizID:      a.Ref.BizID,
		TimelineID: a.TimelineID,
		FieldName:  a.Change.Field,
		FieldLabel: a.Change.Label,
		OldValue:   a.Change.OldValue,
		NewValue:   a.Change.NewValue,
		OldDisplay: a.Change.OldDisplay,
		NewDisplay: a.Change.NewDisplay,
		ValueType:  string(a.Change.ValueType),
	}
	po.TenantID = a.Ref.TenantID
	return po
}

func fromAuditLogPO(po *AuditLogPO) *model.AuditLog {
	return &model.AuditLog{
		ID:         po.ID,
		Ref:        model.BizRef{TenantID: po.TenantID, BizType: po.BizType, BizID: po.BizID},
		TimelineID: po.TimelineID,
		Change: model.FieldChange{
			Field: po.FieldName, Label: po.FieldLabel,
			OldValue: po.OldValue, NewValue: po.NewValue,
			OldDisplay: po.OldDisplay, NewDisplay: po.NewDisplay,
			ValueType: model.ValueType(po.ValueType),
		},
		CreateTime: po.CreateTime,
	}
}

// --- Comment ---

func toCommentPO(c *model.Comment) *CommentPO {
	po := &CommentPO{
		BizType:         c.Ref.BizType,
		BizID:           c.Ref.BizID,
		TimelineID:      c.TimelineID,
		ParentID:        c.ParentID,
		RootID:          c.RootID,
		Content:         c.Content,
		ContentHTML:     c.ContentHTML,
		AuthorID:        c.Author.ID,
		AuthorName:      c.Author.Name,
		MentionUserIDs:  c.MentionUserIDs,
		AttachmentCount: c.AttachmentCount,
		Version:         c.Version,
		EditedAt:        c.EditedAt,
	}
	po.ID = c.ID
	po.TenantID = c.Ref.TenantID
	return po
}

func fromCommentPO(po *CommentPO) *model.Comment {
	return &model.Comment{
		ID:              po.ID,
		Ref:             model.BizRef{TenantID: po.TenantID, BizType: po.BizType, BizID: po.BizID},
		TimelineID:      po.TimelineID,
		ParentID:        po.ParentID,
		RootID:          po.RootID,
		Content:         po.Content,
		ContentHTML:     po.ContentHTML,
		Author:          model.Actor{Type: model.ActorUser, ID: po.AuthorID, Name: po.AuthorName},
		MentionUserIDs:  po.MentionUserIDs,
		AttachmentCount: po.AttachmentCount,
		Version:         po.Version,
		EditedAt:        po.EditedAt,
		CreateTime:      po.CreateTime,
	}
}

// --- Follower ---

func toFollowerPO(f *model.Follower) *FollowerPO {
	types := make([]string, 0, len(f.SubscribeTypes))
	for _, t := range f.SubscribeTypes {
		types = append(types, string(t))
	}
	po := &FollowerPO{
		BizType:        f.Ref.BizType,
		BizID:          f.Ref.BizID,
		UserID:         f.UserID,
		UserName:       f.UserName,
		Reason:         int8(f.Reason),
		SubscribeTypes: types,
		Muted:          f.Muted,
	}
	po.ID = f.ID
	po.TenantID = f.Ref.TenantID
	return po
}

func fromFollowerPO(po *FollowerPO) *model.Follower {
	types := make([]model.EventType, 0, len(po.SubscribeTypes))
	for _, t := range po.SubscribeTypes {
		types = append(types, model.EventType(t))
	}
	return &model.Follower{
		ID:             po.ID,
		Ref:            model.BizRef{TenantID: po.TenantID, BizType: po.BizType, BizID: po.BizID},
		UserID:         po.UserID,
		UserName:       po.UserName,
		Reason:         model.FollowReason(po.Reason),
		SubscribeTypes: types,
		Muted:          po.Muted,
		CreateTime:     po.CreateTime,
	}
}

// --- Attachment ---

func toAttachmentPO(a *model.Attachment) *AttachmentPO {
	po := &AttachmentPO{
		BizType:      a.Ref.BizType,
		BizID:        a.Ref.BizID,
		TimelineID:   a.TimelineID,
		CommentID:    a.CommentID,
		FileID:       a.FileID,
		FileName:     a.FileName,
		FileURL:      a.FileURL,
		FileSize:     a.FileSize,
		ContentType:  a.ContentType,
		UploaderID:   a.Uploader.ID,
		UploaderName: a.Uploader.Name,
	}
	po.ID = a.ID
	po.TenantID = a.Ref.TenantID
	return po
}

func fromAttachmentPO(po *AttachmentPO) *model.Attachment {
	return &model.Attachment{
		ID:          po.ID,
		Ref:         model.BizRef{TenantID: po.TenantID, BizType: po.BizType, BizID: po.BizID},
		TimelineID:  po.TimelineID,
		CommentID:   po.CommentID,
		FileID:      po.FileID,
		FileName:    po.FileName,
		FileURL:     po.FileURL,
		FileSize:    po.FileSize,
		ContentType: po.ContentType,
		Uploader:    model.Actor{Type: model.ActorUser, ID: po.UploaderID, Name: po.UploaderName},
		CreateTime:  po.CreateTime,
	}
}

// --- Notification ---

func toNotificationPO(n *model.Notification) *NotificationPO {
	po := &NotificationPO{
		RecipientID: n.RecipientID,
		BizType:     n.Ref.BizType,
		BizID:       n.Ref.BizID,
		TimelineID:  n.TimelineID,
		Type:        string(n.Type),
		Title:       n.Title,
		Content:     n.Content,
		IsRead:      n.IsRead,
		ReadAt:      n.ReadAt,
	}
	po.ID = n.ID
	po.TenantID = n.TenantID
	return po
}

func fromNotificationPO(po *NotificationPO) *model.Notification {
	return &model.Notification{
		ID:          po.ID,
		TenantID:    po.TenantID,
		RecipientID: po.RecipientID,
		Ref:         model.BizRef{TenantID: po.TenantID, BizType: po.BizType, BizID: po.BizID},
		TimelineID:  po.TimelineID,
		Type:        model.NotificationType(po.Type),
		Title:       po.Title,
		Content:     po.Content,
		IsRead:      po.IsRead,
		ReadAt:      po.ReadAt,
		CreateTime:  po.CreateTime,
	}
}

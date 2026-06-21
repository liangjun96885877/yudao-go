// Package event 定义 chatter 领域事件。
// 事件方法集结构化满足 framework/eventbus.DomainEvent，故 domain 层无需 import framework。
package event

import (
	"time"

	"yudao-go/internal/module/chatter/domain/model"
)

// 事件主题常量。
const (
	TopicRecordCreated   = "chatter.record.created"
	TopicRecordUpdated   = "chatter.record.updated"
	TopicStatusChanged   = "chatter.record.status_changed"
	TopicApproval        = "chatter.approval.processed"
	TopicCommentAdded    = "chatter.comment.added"
	TopicAttachmentAdded = "chatter.attachment.added"
	TopicFollowerAdded   = "chatter.follower.added"
)

// Base 提供领域事件公共字段与方法。字段由应用层在构造事件时填充。
type Base struct {
	EvtID    string    // 全局唯一事件 ID，用于幂等
	EvtTopic string    // 事件主题
	AggType  string    // 聚合类型
	AggID    int64     // 聚合根 ID
	EvtTime  time.Time // 发生时间
}

func (b Base) EventID() string       { return b.EvtID }
func (b Base) Topic() string         { return b.EvtTopic }
func (b Base) AggregateType() string { return b.AggType }
func (b Base) AggregateID() int64    { return b.AggID }
func (b Base) OccurredAt() time.Time { return b.EvtTime }

// NewBase 构造事件公共字段。eventID 由调用方（应用层）用 idgen.UUID() 生成。
func NewBase(eventID, topic, aggType string, aggID int64) Base {
	return Base{EvtID: eventID, EvtTopic: topic, AggType: aggType, AggID: aggID, EvtTime: time.Now()}
}

// RecordCreated 业务记录被创建。
type RecordCreated struct {
	Base
	Ref   model.BizRef
	Actor model.Actor
	Title string // 记录标题，用于生成摘要
}

// RecordUpdated 业务记录字段被修改。
type RecordUpdated struct {
	Base
	Ref     model.BizRef
	Actor   model.Actor
	Changes []model.FieldChange
}

// StatusChanged 业务记录状态变更。
type StatusChanged struct {
	Base
	Ref       model.BizRef
	Actor     model.Actor
	FromState string
	ToState   string
}

// ApprovalProcessed 审批被处理。
type ApprovalProcessed struct {
	Base
	Ref      model.BizRef
	Actor    model.Actor
	Approved bool
	Reason   string
}

// CommentAdded 新增评论。
type CommentAdded struct {
	Base
	Ref       model.BizRef
	Actor     model.Actor
	CommentID int64
	Mentions  []int64
}

// AttachmentAdded 新增附件。
type AttachmentAdded struct {
	Base
	Ref          model.BizRef
	Actor        model.Actor
	AttachmentID int64
	FileName     string
}

// FollowerAdded 新增关注者。
type FollowerAdded struct {
	Base
	Ref    model.BizRef
	Actor  model.Actor
	UserID int64
}

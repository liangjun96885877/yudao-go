// Package persistence 是 chatter 的持久化层：PO 定义、PO↔领域模型转换、仓储实现。
package persistence

import (
	"time"

	"yudao-go/internal/framework/orm"
)

// LightBase 用于仅含 id/tenant_id/create_time 的表（无 creator/updater/deleted）。
type LightBase struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   int64     `gorm:"column:tenant_id"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime"`
}

// TimelinePO 对应 chatter_timeline。
type TimelinePO struct {
	orm.TenantModel
	BizType      string `gorm:"column:biz_type"`
	BizID        int64  `gorm:"column:biz_id"`
	EventType    string `gorm:"column:event_type"`
	EventSubtype string `gorm:"column:event_subtype"`
	Summary      string `gorm:"column:summary"`
	Body         string `gorm:"column:body"`
	ActorType    int8   `gorm:"column:actor_type"`
	ActorID      int64  `gorm:"column:actor_id"`
	ActorName    string `gorm:"column:actor_name"`
	RefType      string `gorm:"column:ref_type"`
	RefID        int64  `gorm:"column:ref_id"`
	Visibility   int8   `gorm:"column:visibility"`
	EventID      string `gorm:"column:event_id"`
}

func (TimelinePO) TableName() string { return "chatter_timeline" }

// AuditLogPO 对应 chatter_audit_log。
type AuditLogPO struct {
	LightBase
	BizType    string `gorm:"column:biz_type"`
	BizID      int64  `gorm:"column:biz_id"`
	TimelineID int64  `gorm:"column:timeline_id"`
	FieldName  string `gorm:"column:field_name"`
	FieldLabel string `gorm:"column:field_label"`
	OldValue   string `gorm:"column:old_value"`
	NewValue   string `gorm:"column:new_value"`
	OldDisplay string `gorm:"column:old_display"`
	NewDisplay string `gorm:"column:new_display"`
	ValueType  string `gorm:"column:value_type"`
}

func (AuditLogPO) TableName() string { return "chatter_audit_log" }

// CommentPO 对应 chatter_comment。
type CommentPO struct {
	orm.TenantModel
	BizType         string     `gorm:"column:biz_type"`
	BizID           int64      `gorm:"column:biz_id"`
	TimelineID      int64      `gorm:"column:timeline_id"`
	ParentID        int64      `gorm:"column:parent_id"`
	RootID          int64      `gorm:"column:root_id"`
	Content         string     `gorm:"column:content"`
	ContentHTML     string     `gorm:"column:content_html"`
	AuthorID        int64      `gorm:"column:author_id"`
	AuthorName      string     `gorm:"column:author_name"`
	MentionUserIDs  []int64    `gorm:"column:mention_user_ids;serializer:json"`
	AttachmentCount int        `gorm:"column:attachment_count"`
	Version         int        `gorm:"column:version"`
	EditedAt        *time.Time `gorm:"column:edited_at"`
}

func (CommentPO) TableName() string { return "chatter_comment" }

// FollowerPO 对应 chatter_follower。
type FollowerPO struct {
	LightBase
	BizType        string   `gorm:"column:biz_type"`
	BizID          int64    `gorm:"column:biz_id"`
	UserID         int64    `gorm:"column:user_id"`
	UserName       string   `gorm:"column:user_name"`
	Reason         int8     `gorm:"column:reason"`
	SubscribeTypes []string `gorm:"column:subscribe_types;serializer:json"`
	Muted          bool     `gorm:"column:muted"`
}

func (FollowerPO) TableName() string { return "chatter_follower" }

// AttachmentPO 对应 chatter_attachment。
type AttachmentPO struct {
	LightBase
	BizType      string `gorm:"column:biz_type"`
	BizID        int64  `gorm:"column:biz_id"`
	TimelineID   int64  `gorm:"column:timeline_id"`
	CommentID    int64  `gorm:"column:comment_id"`
	FileID       int64  `gorm:"column:file_id"`
	FileName     string `gorm:"column:file_name"`
	FileURL      string `gorm:"column:file_url"`
	FileSize     int64  `gorm:"column:file_size"`
	ContentType  string `gorm:"column:content_type"`
	UploaderID   int64  `gorm:"column:uploader_id"`
	UploaderName string `gorm:"column:uploader_name"`
}

func (AttachmentPO) TableName() string { return "chatter_attachment" }

// NotificationPO 对应 chatter_notification。
type NotificationPO struct {
	LightBase
	RecipientID int64      `gorm:"column:recipient_id"`
	BizType     string     `gorm:"column:biz_type"`
	BizID       int64      `gorm:"column:biz_id"`
	TimelineID  int64      `gorm:"column:timeline_id"`
	Type        string     `gorm:"column:type"`
	Title       string     `gorm:"column:title"`
	Content     string     `gorm:"column:content"`
	IsRead      bool       `gorm:"column:is_read"`
	ReadAt      *time.Time `gorm:"column:read_at"`
}

func (NotificationPO) TableName() string { return "chatter_notification" }

// TimelineFlagPO 对应 chatter_timeline_flag。
type TimelineFlagPO struct {
	TimelineID  int64     `gorm:"column:timeline_id;primaryKey"`
	UserID      int64     `gorm:"column:user_id;primaryKey"`
	IsRead      bool      `gorm:"column:is_read"`
	IsImportant bool      `gorm:"column:is_important"`
	UpdateTime  time.Time `gorm:"column:update_time;autoUpdateTime"`
}

func (TimelineFlagPO) TableName() string { return "chatter_timeline_flag" }

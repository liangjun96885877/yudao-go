// Package dto 定义 chatter 应用层的请求/响应数据传输对象。
package dto

// --- 请求 ---

// AddCommentReq 新增评论请求。
type AddCommentReq struct {
	BizType        string  `json:"bizType" binding:"required"`
	BizID          int64   `json:"bizId" binding:"required"`
	Content        string  `json:"content" binding:"required"`
	ParentID       int64   `json:"parentId"`
	MentionUserIDs []int64 `json:"mentionUserIds"`
}

// UpdateCommentReq 编辑评论请求（含乐观锁版本）。
type UpdateCommentReq struct {
	Content string `json:"content" binding:"required"`
	Version int    `json:"version"`
}

// FollowReq 关注/取消关注请求。
type FollowReq struct {
	BizType string `json:"bizType" binding:"required"`
	BizID   int64  `json:"bizId" binding:"required"`
}

// FollowerSettingsReq 更新关注者订阅设置:订阅事件类型 + 是否静音。
type FollowerSettingsReq struct {
	BizType        string   `json:"bizType" binding:"required"`
	BizID          int64    `json:"bizId" binding:"required"`
	SubscribeTypes []string `json:"subscribeTypes"` // 空 = 订阅全部
	Muted          bool     `json:"muted"`
}

// AttachmentItemReq 单个附件信息（fileId 来自 infra 文件服务）。
type AttachmentItemReq struct {
	FileID      int64  `json:"fileId" binding:"required"`
	FileName    string `json:"fileName"`
	FileURL     string `json:"fileUrl"`
	FileSize    int64  `json:"fileSize"`
	ContentType string `json:"contentType"`
}

// LinkAttachmentReq 关联附件到业务记录请求。
type LinkAttachmentReq struct {
	BizType string              `json:"bizType" binding:"required"`
	BizID   int64               `json:"bizId" binding:"required"`
	Files   []AttachmentItemReq `json:"files" binding:"required"`
}

// --- 响应 ---

// FieldChangeDTO 字段变更。
type FieldChangeDTO struct {
	Field      string `json:"field"`
	Label      string `json:"label"`
	OldValue   string `json:"oldValue"`
	NewValue   string `json:"newValue"`
	OldDisplay string `json:"oldDisplay"`
	NewDisplay string `json:"newDisplay"`
	ValueType  string `json:"valueType"`
}

// TimelineItemDTO 时间线条目。
type TimelineItemDTO struct {
	ID           int64            `json:"id"`
	EventType    string           `json:"eventType"`
	EventSubtype string           `json:"eventSubtype"`
	Summary      string           `json:"summary"`
	Body         string           `json:"body"`
	ActorType    int8             `json:"actorType"`
	ActorID      int64            `json:"actorId"`
	ActorName    string           `json:"actorName"`
	RefType      string           `json:"refType"`
	RefID        int64            `json:"refId"`
	CreateTime   string           `json:"createTime"`
	Changes      []FieldChangeDTO `json:"changes,omitempty"` // 仅 update 类型
	IsRead       bool             `json:"isRead"`            // 当前用户是否已读
	IsImportant  bool             `json:"isImportant"`       // 当前用户是否标记重要
	ReplyCount   int              `json:"replyCount"`        // comment 类型:直接子评论数,Axelor 风格的"回复 (N)"按钮用
}

// CommentDTO 评论。
type CommentDTO struct {
	ID             int64   `json:"id"`
	BizType        string  `json:"bizType"`
	BizID          int64   `json:"bizId"`
	ParentID       int64   `json:"parentId"`
	RootID         int64   `json:"rootId"`
	Content        string  `json:"content"`
	ContentHTML    string  `json:"contentHtml"`
	AuthorID       int64   `json:"authorId"`
	AuthorName     string  `json:"authorName"`
	MentionUserIDs []int64 `json:"mentionUserIds"`
	Version        int     `json:"version"`
	EditedAt       string  `json:"editedAt"`
	CreateTime     string  `json:"createTime"`
}

// FollowerDTO 关注者。
type FollowerDTO struct {
	ID             int64    `json:"id"`
	UserID         int64    `json:"userId"`
	UserName       string   `json:"userName"`
	Reason         int8     `json:"reason"`
	SubscribeTypes []string `json:"subscribeTypes"` // 空 = 订阅全部
	Muted          bool     `json:"muted"`
	CreateTime     string   `json:"createTime"`
}

// AttachmentDTO 附件。
type AttachmentDTO struct {
	ID           int64  `json:"id"`
	BizType      string `json:"bizType"`
	BizID        int64  `json:"bizId"`
	FileID       int64  `json:"fileId"`
	FileName     string `json:"fileName"`
	FileURL      string `json:"fileUrl"`
	FileSize     int64  `json:"fileSize"`
	ContentType  string `json:"contentType"`
	UploaderID   int64  `json:"uploaderId"`
	UploaderName string `json:"uploaderName"`
	CreateTime   string `json:"createTime"`
}

// NotificationDTO 通知。
type NotificationDTO struct {
	ID         int64  `json:"id"`
	BizType    string `json:"bizType"`
	BizID      int64  `json:"bizId"`
	TimelineID int64  `json:"timelineId"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	IsRead     bool   `json:"isRead"`
	CreateTime string `json:"createTime"`
}

// CursorPage 游标分页结果。
type CursorPage[T any] struct {
	List       []T   `json:"list"`
	NextCursor int64 `json:"nextCursor"` // 0 表示无更多数据
}

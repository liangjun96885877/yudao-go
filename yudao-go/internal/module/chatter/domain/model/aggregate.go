package model

import "time"

// Timeline 时间线条目（活动流主体）。
type Timeline struct {
	ID           int64
	Ref          BizRef
	EventType    EventType
	EventSubtype string
	Summary      string
	Body         string
	Actor        Actor
	RefType      string // 关联类型：comment / audit_batch / approval
	RefID        int64  // 关联 ID
	Visibility   Visibility
	EventID      string // 幂等 ID，来源于领域事件
	CreateTime   time.Time
}

// AuditLog 字段变更审计记录，按 TimelineID 聚合为一次保存。
type AuditLog struct {
	ID         int64
	Ref        BizRef
	TimelineID int64
	Change     FieldChange
	CreateTime time.Time
}

// Comment 评论。
type Comment struct {
	ID              int64
	Ref             BizRef
	TimelineID      int64
	ParentID        int64 // 回复的父评论，0 表示根评论
	RootID          int64 // 所属顶层评论，便于整条线程查询
	Content         string
	ContentHTML     string
	Author          Actor
	MentionUserIDs  []int64 // @ 的用户
	AttachmentCount int
	Version         int // 乐观锁版本
	EditedAt        *time.Time
	CreateTime      time.Time
}

// MarkEdited 标记评论已被编辑（更新内容时调用），版本号自增。
func (c *Comment) MarkEdited(content, html string) {
	c.Content = content
	c.ContentHTML = html
	c.EditedAt = nowPtr()
	c.Version++
}

// FollowReason 关注来源。
type FollowReason int8

const (
	FollowManual    FollowReason = 1 // 手动关注
	FollowCreator   FollowReason = 2 // 创建人自动关注
	FollowMentioned FollowReason = 3 // 被 @ 自动关注
	FollowAssignee  FollowReason = 4 // 负责人自动关注
	FollowAuto      FollowReason = 5 // 其它规则自动关注
)

// Follower 关注者。
type Follower struct {
	ID             int64
	Ref            BizRef
	UserID         int64
	UserName       string
	Reason         FollowReason
	SubscribeTypes []EventType // 订阅的事件类型；为空表示订阅全部
	Muted          bool
	CreateTime     time.Time
}

// Subscribed 判断该关注者是否应收到某事件类型的通知。
func (f Follower) Subscribed(t EventType) bool {
	if f.Muted {
		return false
	}
	if len(f.SubscribeTypes) == 0 {
		return true // 未指定 = 全部订阅
	}
	for _, st := range f.SubscribeTypes {
		if st == t {
			return true
		}
	}
	return false
}

// Attachment 附件关联（文件本体存于 infra 文件服务）。
type Attachment struct {
	ID          int64
	Ref         BizRef
	TimelineID  int64 // 与 CommentID 二选一
	CommentID   int64
	FileID      int64
	FileName    string
	FileURL     string
	FileSize    int64
	ContentType string
	Uploader    Actor
	CreateTime  time.Time
}

// NotificationType 通知类型。
type NotificationType string

const (
	NotifyComment    NotificationType = "comment"
	NotifyMention    NotificationType = "mention"
	NotifyApproval   NotificationType = "approval"
	NotifyFollow     NotificationType = "follow"
	NotifyStatus     NotificationType = "status"
	NotifyCreate     NotificationType = "create"
	NotifyUpdate     NotificationType = "update"
	NotifyAttachment NotificationType = "attachment"
)

// Notification 系统通知（用户收件箱条目）。
type Notification struct {
	ID          int64
	TenantID    int64
	RecipientID int64
	Ref         BizRef
	TimelineID  int64
	Type        NotificationType
	Title       string
	Content     string
	IsRead      bool
	ReadAt      *time.Time
	CreateTime  time.Time
}

// MarkRead 标记通知为已读。
func (n *Notification) MarkRead() {
	if n.IsRead {
		return
	}
	n.IsRead = true
	n.ReadAt = nowPtr()
}

// TimelineFlag 是某用户对某条时间线的标记（已读 / 重要）。
type TimelineFlag struct {
	TimelineID  int64
	UserID      int64
	IsRead      bool
	IsImportant bool
}

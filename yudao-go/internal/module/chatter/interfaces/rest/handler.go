// Package rest 是 chatter 模块的 HTTP 接入层。
package rest

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/chatter/application/dto"
	"yudao-go/internal/module/chatter/application/service"
	"yudao-go/internal/pkg/errcode"
)

// Handler 聚合 chatter 各应用服务，提供 HTTP 处理函数。
type Handler struct {
	timeline     *service.TimelineService
	comment      *service.CommentService
	follower     *service.FollowerService
	notification *service.NotificationService
	attachment   *service.AttachmentService
}

func NewHandler(
	timeline *service.TimelineService,
	comment *service.CommentService,
	follower *service.FollowerService,
	notification *service.NotificationService,
	attachment *service.AttachmentService,
) *Handler {
	return &Handler{
		timeline: timeline, comment: comment, follower: follower,
		notification: notification, attachment: attachment,
	}
}

// Register 在给定路由分组下注册 chatter 的全部接口。
func (h *Handler) Register(group *gin.RouterGroup) {
	g := group.Group("/chatter")
	g.GET("/timeline", h.getTimeline)
	g.PUT("/timeline/:id/flag", h.setTimelineFlag)
	g.POST("/comment", h.addComment)
	g.PUT("/comment/:id", h.updateComment)
	g.DELETE("/comment/:id", h.deleteComment)
	g.GET("/comment/:id/replies", h.commentReplies)
	g.GET("/follower", h.listFollowers)
	g.POST("/follower", h.follow)
	g.DELETE("/follower", h.unfollow)
	g.PUT("/follower/settings", h.updateFollowerSettings)
	g.POST("/attachment", h.linkAttachment)
	g.GET("/attachment", h.listAttachments)
	g.GET("/notification", h.getNotifications)
	g.GET("/notification/unread-count", h.unreadCount)
	g.PUT("/notification/read-all", h.markAllRead)
	g.PUT("/notification/:id/read", h.markRead)
}

// --- 时间线 ---

func (h *Handler) getTimeline(c *gin.Context) {
	page, err := h.timeline.Feed(c.Request.Context(),
		c.Query("bizType"), queryInt64(c, "bizId"),
		queryInt64(c, "cursor"), queryInt(c, "limit"))
	respond(c, page, err)
}

// setTimelineFlag 设置当前用户对某条时间线的标记（已读 / 重要）。
func (h *Handler) setTimelineFlag(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req struct {
		Read      bool `json:"read"`
		Important bool `json:"important"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.timeline.SetFlag(c.Request.Context(), id, req.Read, req.Important); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// --- 评论 ---

func (h *Handler) addComment(c *gin.Context) {
	var req dto.AddCommentReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.comment.Add(c.Request.Context(), &req)
	respond(c, res, err)
}

func (h *Handler) updateComment(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateCommentReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.comment.Update(c.Request.Context(), id, &req)
	respond(c, res, err)
}

func (h *Handler) deleteComment(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if err := h.comment.Delete(c.Request.Context(), id); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// commentReplies 返回某评论的直接子评论(timeline 条目),用于 Axelor 风格的展开。
func (h *Handler) commentReplies(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	res, err := h.timeline.CommentReplies(c.Request.Context(), id)
	respond(c, res, err)
}

// --- 关注者 ---

func (h *Handler) listFollowers(c *gin.Context) {
	res, err := h.follower.List(c.Request.Context(), c.Query("bizType"), queryInt64(c, "bizId"))
	respond(c, res, err)
}

func (h *Handler) follow(c *gin.Context) {
	var req dto.FollowReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.follower.Follow(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) unfollow(c *gin.Context) {
	var req dto.FollowReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.follower.Unfollow(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) updateFollowerSettings(c *gin.Context) {
	var req dto.FollowerSettingsReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.follower.UpdateSettings(c.Request.Context(), &req); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// --- 附件 ---

func (h *Handler) linkAttachment(c *gin.Context) {
	var req dto.LinkAttachmentReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.attachment.Link(c.Request.Context(), &req)
	respond(c, res, err)
}

func (h *Handler) listAttachments(c *gin.Context) {
	res, err := h.attachment.List(c.Request.Context(), c.Query("bizType"), queryInt64(c, "bizId"))
	respond(c, res, err)
}

// --- 通知 ---

func (h *Handler) getNotifications(c *gin.Context) {
	res, err := h.notification.Inbox(c.Request.Context(),
		queryInt64(c, "cursor"), queryInt(c, "limit"), c.Query("unread") == "true")
	respond(c, res, err)
}

func (h *Handler) unreadCount(c *gin.Context) {
	count, err := h.notification.UnreadCount(c.Request.Context())
	respond(c, gin.H{"count": count}, err)
}

func (h *Handler) markRead(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	if err := h.notification.MarkRead(c.Request.Context(), id); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

func (h *Handler) markAllRead(c *gin.Context) {
	if err := h.notification.MarkAllRead(c.Request.Context()); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Ok(c)
}

// --- 辅助函数 ---

func respond[T any](c *gin.Context, data T, err error) {
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, data)
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		web.Fail(c, errcode.BadRequest.WithMsg("请求参数不正确: "+err.Error()))
		return false
	}
	return true
}

func queryInt64(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return v
}

func queryInt(c *gin.Context, key string) int {
	v, _ := strconv.Atoi(c.Query(key))
	return v
}

func pathInt64(c *gin.Context, key string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		web.Fail(c, errcode.BadRequest)
		return 0, false
	}
	return v, true
}

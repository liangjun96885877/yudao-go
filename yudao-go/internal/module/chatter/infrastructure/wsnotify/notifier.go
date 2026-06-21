// Package wsnotify 把 chatter 领域变更经 WebSocket Hub 实时推送给前端。
package wsnotify

import (
	"context"
	"encoding/json"

	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/websocket"
	"yudao-go/internal/module/chatter/application/assembler"
	"yudao-go/internal/module/chatter/domain/model"
)

// wsMessage 是发送给前端的实时消息。
type wsMessage struct {
	Type    string `json:"type"`
	BizType string `json:"bizType,omitempty"`
	BizID   int64  `json:"bizId,omitempty"`
	Item    any    `json:"item"`
}

// Notifier 经 WebSocket Hub 推送时间线与通知。
type Notifier struct {
	hub *websocket.Hub
}

func NewNotifier(hub *websocket.Hub) *Notifier { return &Notifier{hub: hub} }

// PushTimeline 向查看该业务记录的客户端推送新时间线条目。
func (n *Notifier) PushTimeline(ctx context.Context, tl *model.Timeline) {
	n.send(ctx,
		websocket.RecordChannel(tl.Ref.TenantID, tl.Ref.BizType, tl.Ref.BizID),
		wsMessage{
			Type:    "timeline.new",
			BizType: tl.Ref.BizType,
			BizID:   tl.Ref.BizID,
			Item:    assembler.ToTimelineItemDTO(tl, nil),
		})
}

// PushNotifications 向各接收者的个人频道推送新通知。
func (n *Notifier) PushNotifications(ctx context.Context, ns []*model.Notification) {
	for _, nt := range ns {
		n.send(ctx,
			websocket.UserChannel(nt.TenantID, nt.RecipientID),
			wsMessage{Type: "notification.new", Item: assembler.ToNotificationDTO(nt)})
	}
}

func (n *Notifier) send(ctx context.Context, channel string, msg wsMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.WithContext(ctx).Error("wsnotify: marshal failed", "error", err)
		return
	}
	if err := n.hub.Broadcast(ctx, []string{channel}, data); err != nil {
		logger.WithContext(ctx).Error("wsnotify: broadcast failed", "error", err)
	}
}

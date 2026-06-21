// Package eventcodec 把 chatter 领域事件的解码器注册到事件编解码器。
package eventcodec

import (
	"encoding/json"

	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/module/chatter/domain/event"
)

// register 为某主题注册「JSON → 具体事件类型」的解码器。
func register[T eventbus.DomainEvent](codec *eventbus.Codec, topic string) {
	codec.Register(topic, func(payload json.RawMessage) (eventbus.DomainEvent, error) {
		var e T
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return e, nil
	})
}

// Register 注册 chatter 全部领域事件的解码器。
func Register(codec *eventbus.Codec) {
	register[event.RecordCreated](codec, event.TopicRecordCreated)
	register[event.RecordUpdated](codec, event.TopicRecordUpdated)
	register[event.StatusChanged](codec, event.TopicStatusChanged)
	register[event.ApprovalProcessed](codec, event.TopicApproval)
	register[event.CommentAdded](codec, event.TopicCommentAdded)
	register[event.AttachmentAdded](codec, event.TopicAttachmentAdded)
	register[event.FollowerAdded](codec, event.TopicFollowerAdded)
}

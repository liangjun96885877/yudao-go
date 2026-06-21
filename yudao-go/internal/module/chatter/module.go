// Package chatter 是业务时间线模块的组合根：装配各层并对外暴露注册入口。
package chatter

import (
	"context"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/websocket"
	"yudao-go/internal/module/chatter/application/service"
	"yudao-go/internal/module/chatter/infrastructure/consumer"
	"yudao-go/internal/module/chatter/infrastructure/eventcodec"
	"yudao-go/internal/module/chatter/infrastructure/outbox"
	"yudao-go/internal/module/chatter/infrastructure/persistence"
	"yudao-go/internal/module/chatter/infrastructure/wsnotify"
	"yudao-go/internal/module/chatter/interfaces/rest"
	"yudao-go/internal/module/chatter/registry"
)

// Module 持有 chatter 的对外注册入口与生命周期组件。
type Module struct {
	Registry *registry.Registry   // 业务类型注册表，供业务模块接入
	Audit    *service.AuditService // 字段审计入口，供业务模块更新时调用

	handler  *rest.Handler
	consumer *consumer.TimelineConsumer
	relay    *outbox.Relay
}

// New 装配 chatter 模块的全部依赖。
func New(tx *orm.TxManager, bus eventbus.Bus, codec *eventbus.Codec, hub *websocket.Hub) *Module {
	// 注册领域事件解码器（发件箱与跨进程传输都依赖）。
	eventcodec.Register(codec)

	reg := registry.New()

	// 仓储
	timelineRepo := persistence.NewTimelineRepo(tx)
	commentRepo := persistence.NewCommentRepo(tx)
	followerRepo := persistence.NewFollowerRepo(tx)
	attachmentRepo := persistence.NewAttachmentRepo(tx)
	notificationRepo := persistence.NewNotificationRepo(tx)

	// 事务发件箱、消费幂等、投递中继
	sink := outbox.NewOutbox(tx, codec)
	dedup := outbox.NewDeduplicator(tx)
	relay := outbox.NewRelay(tx, bus, codec)

	// 权限：P2/P3 用放行实现，待 system 模块接入真实校验。
	var perm service.Permission = service.AllowAll{}

	// 应用服务（事件出口统一为发件箱 sink）
	timelineSvc := service.NewTimelineService(timelineRepo, commentRepo, tx, perm)
	commentSvc := service.NewCommentService(commentRepo, sink, tx, perm)
	followerSvc := service.NewFollowerService(followerRepo, sink, tx, perm)
	notificationSvc := service.NewNotificationService(notificationRepo, tx)
	attachmentSvc := service.NewAttachmentService(attachmentRepo, sink, tx, perm)
	auditSvc := service.NewAuditService(reg, sink)

	return &Module{
		Registry: reg,
		Audit:    auditSvc,
		handler:  rest.NewHandler(timelineSvc, commentSvc, followerSvc, notificationSvc, attachmentSvc),
		consumer: consumer.NewTimelineConsumer(timelineRepo, commentRepo, followerRepo, notificationRepo,
			dedup, wsnotify.NewNotifier(hub), tx),
		relay:    relay,
	}
}

// RegisterConsumers 订阅领域事件，须在事件总线 Start 之前调用。
func (m *Module) RegisterConsumers(bus eventbus.Bus) error {
	return m.consumer.Register(bus)
}

// RegisterRoutes 在给定 API 分组下挂载 chatter 的 HTTP 接口。
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.Register(group)
}

// StartRelay 启动发件箱投递中继，须在事件总线 Start 之后调用。
func (m *Module) StartRelay() { m.relay.Start() }

// StopRelay 停止发件箱投递中继，须在事件总线 Stop 之前调用。
func (m *Module) StopRelay(ctx context.Context) error { return m.relay.Stop(ctx) }

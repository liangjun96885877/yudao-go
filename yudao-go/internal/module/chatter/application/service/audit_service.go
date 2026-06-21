package service

import (
	"context"

	"yudao-go/internal/module/chatter/domain/event"
	domainsvc "yudao-go/internal/module/chatter/domain/service"
	"yudao-go/internal/module/chatter/registry"
	"yudao-go/internal/pkg/idgen"
)

// AuditService 为业务模块提供显式的字段变更审计入口。
// 移植标准（横切能力 #4）：无注解魔法，业务更新服务显式调用 TrackUpdate。
type AuditService struct {
	registry *registry.Registry
	sink     EventSink
	differ   domainsvc.AuditDiffer
}

func NewAuditService(reg *registry.Registry, sink EventSink) *AuditService {
	return &AuditService{registry: reg, sink: sink}
}

// TrackUpdate 比对业务记录修改前后的字段值，若有变更则向发件箱写入 RecordUpdated 事件。
// 须在业务更新所在事务内调用，使审计事件与业务更新原子提交。
// bizType 未注册或无字段变更时静默返回 nil。
func (s *AuditService) TrackUpdate(
	ctx context.Context, bizType string, bizID int64, oldVals, newVals map[string]any,
) error {
	bt, ok := s.registry.Lookup(bizType)
	if !ok {
		return nil // 该业务类型未接入 chatter 审计
	}
	changes := s.differ.Diff(bt.AuditFields, oldVals, newVals)
	if len(changes) == 0 {
		return nil
	}
	return s.sink.Append(ctx, event.RecordUpdated{
		Base:    event.NewBase(idgen.UUID(), event.TopicRecordUpdated, bizType, bizID),
		Ref:     bizRef(ctx, bizType, bizID),
		Actor:   actorFromContext(ctx),
		Changes: changes,
	})
}

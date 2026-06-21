package service

import (
	"testing"

	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/registry"
)

func TestAuditDiffer_Diff(t *testing.T) {
	fields := []registry.AuditField{
		{Column: "name", Field: "Name", Label: "名称", Type: "string"},
		{Column: "status", Field: "Status", Label: "状态", Type: "int"},
		{Column: "remark", Field: "Remark", Label: "备注", Type: "string"},
	}
	oldVals := map[string]any{"name": "草稿", "status": 0, "remark": "x"}
	newVals := map[string]any{"name": "已审批", "status": 0} // status 未变；remark 未更新

	changes := AuditDiffer{}.Diff(fields, oldVals, newVals)

	if len(changes) != 1 {
		t.Fatalf("期望 1 处变更，实际 %d", len(changes))
	}
	c := changes[0]
	if c.Field != "Name" || c.OldValue != "草稿" || c.NewValue != "已审批" {
		t.Fatalf("变更内容不符: %+v", c)
	}
}

func TestAuditDiffer_NoChange(t *testing.T) {
	fields := []registry.AuditField{{Column: "name", Field: "Name", Label: "名称", Type: "string"}}
	changes := AuditDiffer{}.Diff(fields,
		map[string]any{"name": "同"}, map[string]any{"name": "同"})
	if len(changes) != 0 {
		t.Fatalf("期望 0 处变更，实际 %d", len(changes))
	}
}

func TestTimelineFactory_RecordUpdated(t *testing.T) {
	evt := event.RecordUpdated{
		Base:  event.NewBase("e1", event.TopicRecordUpdated, "erp_order", 1),
		Ref:   model.BizRef{TenantID: 1, BizType: "erp_order", BizID: 1},
		Actor: model.Actor{Type: model.ActorUser, ID: 9, Name: "张三"},
		Changes: []model.FieldChange{
			{Field: "Name", Label: "名称", OldValue: "a", NewValue: "b"},
			{Field: "Status", Label: "状态", OldValue: "0", NewValue: "1"},
		},
	}
	f := TimelineFactory{}

	tl := f.FromRecordUpdated(evt)
	if tl.EventType != model.EventUpdate {
		t.Fatalf("事件类型应为 update，实际 %s", tl.EventType)
	}
	if tl.Summary != "张三 修改了 2 个字段" {
		t.Fatalf("摘要不符: %q", tl.Summary)
	}
	if tl.EventID != "e1" {
		t.Fatalf("事件 ID 应透传，实际 %q", tl.EventID)
	}

	logs := f.AuditLogsFor(evt, 100)
	if len(logs) != 2 {
		t.Fatalf("期望 2 条审计明细，实际 %d", len(logs))
	}
	for _, l := range logs {
		if l.TimelineID != 100 {
			t.Fatalf("审计明细应关联时间线 100，实际 %d", l.TimelineID)
		}
	}
}

func TestTimelineFactory_Approval(t *testing.T) {
	evt := event.ApprovalProcessed{
		Base:     event.NewBase("e2", event.TopicApproval, "erp_order", 1),
		Ref:      model.BizRef{TenantID: 1, BizType: "erp_order", BizID: 1},
		Actor:    model.Actor{Type: model.ActorUser, ID: 9, Name: "李四"},
		Approved: false,
		Reason:   "金额超限",
	}
	tl := TimelineFactory{}.FromApproval(evt)
	if tl.EventSubtype != "rejected" {
		t.Fatalf("子类型应为 rejected，实际 %q", tl.EventSubtype)
	}
	if tl.Summary != "李四 审批拒绝：金额超限" {
		t.Fatalf("摘要不符: %q", tl.Summary)
	}
}

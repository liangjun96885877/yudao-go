// Package service 字段审计接口 ——
// myerp 与 chatter 解耦,通过 SetAuditor 由组合根注入。
// nil auditor 时所有 audit 调用静默返回,跑测试方便。
package service

import "context"

// Auditor 字段变更审计器(chatter.AuditService 的最小子集)。
// 与 system 模块的 rest.Auditor 同形,可直接传 chatterModule.Audit 进来。
type Auditor interface {
	TrackUpdate(ctx context.Context, bizType string, bizID int64, oldVals, newVals map[string]any) error
}

// nopAuditor 占位实现,调用方未注入 auditor 时使用,避免 nil 检查散落各处。
type nopAuditor struct{}

func (nopAuditor) TrackUpdate(context.Context, string, int64, map[string]any, map[string]any) error {
	return nil
}

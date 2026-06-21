// Package service 是 chatter 应用服务层：用例编排、事务边界、事件发布。
package service

import (
	"context"
	"html"
	"strings"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/security"
	"yudao-go/internal/module/chatter/domain/model"
)

// Permission 校验当前用户能否访问某业务记录的 chatter。
// 移植标准：权限校验委派给业务模块，chatter 不重复实现各业务的权限规则。
type Permission interface {
	CanRead(ctx context.Context, ref model.BizRef) error
}

// AllowAll 放行所有访问（P2 占位实现，待 system 模块接入真实权限）。
type AllowAll struct{}

func (AllowAll) CanRead(context.Context, model.BizRef) error { return nil }

// actorFromContext 从登录态构造操作者；无登录态时回退为系统。
func actorFromContext(ctx context.Context) model.Actor {
	if u := security.CurrentUser(ctx); u != nil {
		return model.Actor{Type: model.ActorUser, ID: u.ID, Name: u.Username}
	}
	return model.SystemActor()
}

// bizRef 由 context 租户与请求参数构造业务引用。
func bizRef(ctx context.Context, bizType string, bizID int64) model.BizRef {
	return model.BizRef{TenantID: contextx.TenantID(ctx), BizType: bizType, BizID: bizID}
}

// renderHTML 将纯文本评论渲染为安全 HTML（P2 简单实现：转义 + 换行）。
// TODO 后续阶段接入 bluemonday 以支持富文本。
func renderHTML(text string) string {
	return strings.ReplaceAll(html.EscapeString(text), "\n", "<br>")
}

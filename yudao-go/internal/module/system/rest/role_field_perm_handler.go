package rest

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/service"
)

// fieldDef 描述一个可脱敏字段。
type fieldDef struct {
	BizType string `json:"bizType"`
	Field   string `json:"field"`
	Label   string `json:"label"`
}

// maskableFields 是支持字段权限配置的字段注册表，须与模型上的 mask 标签保持一致。
var maskableFields = []fieldDef{
	{"system_user", "mobile", "用户 - 手机号"},
	{"system_user", "email", "用户 - 邮箱"},
}

// RoleFieldPermHandler 提供角色字段权限的查询与配置接口。
type RoleFieldPermHandler struct {
	svc     *service.FieldPermService
	guard   *service.PrivilegeGuard
	auditor Auditor
}

func NewRoleFieldPermHandler(tx *orm.TxManager) *RoleFieldPermHandler {
	return &RoleFieldPermHandler{
		svc:   service.NewFieldPermService(tx),
		guard: service.NewPrivilegeGuard(tx),
	}
}

// SetAuditor 注入 chatter 审计器，启用字段权限变更的时间线记录。
func (h *RoleFieldPermHandler) SetAuditor(a Auditor) { h.auditor = a }

// fieldPermSummary 把字段权限 map 压成稳定字符串，便于审计 diff。
func fieldPermSummary(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		action := m[k]
		if action == "" {
			action = "mask"
		}
		parts = append(parts, k+"="+action)
	}
	return strings.Join(parts, ";")
}

func (h *RoleFieldPermHandler) Register(g *gin.RouterGroup) {
	g.GET("/system/role-field-perm/fields", h.fields)
	g.GET("/system/role-field-perm/list", h.listByRole)
	g.POST("/system/role-field-perm/save", h.save)
}

// fields 返回全部可脱敏字段。
func (h *RoleFieldPermHandler) fields(c *gin.Context) {
	web.Success(c, maskableFields)
}

// roleFieldVO 是角色字段权限的展示项（字段定义 + 当前动作）。
type roleFieldVO struct {
	fieldDef
	Action string `json:"action"`
}

// listByRole 返回某角色对每个可脱敏字段的动作（未配置默认 mask）。
func (h *RoleFieldPermHandler) listByRole(c *gin.Context) {
	roleID, _ := strconv.ParseInt(c.Query("roleId"), 10, 64)
	configured, err := h.svc.ListByRole(c.Request.Context(), roleID)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	out := make([]roleFieldVO, 0, len(maskableFields))
	for _, f := range maskableFields {
		action := configured[f.BizType+":"+f.Field]
		if action == "" {
			action = "mask"
		}
		out = append(out, roleFieldVO{fieldDef: f, Action: action})
	}
	web.Success(c, out)
}

// save 保存某角色的字段权限配置。
func (h *RoleFieldPermHandler) save(c *gin.Context) {
	var req struct {
		RoleID int64 `json:"roleId"`
		Items  []struct {
			BizType string `json:"bizType"`
			Field   string `json:"field"`
			Action  string `json:"action"`
		} `json:"items"`
	}
	if !bind(c, &req) {
		return
	}
	// 防提权：不能改超管角色;非超管不能授「明文」给自己也看不到明文的字段。
	guardItems := make([]struct{ BizType, Field, Action string }, 0, len(req.Items))
	for _, it := range req.Items {
		guardItems = append(guardItems, struct{ BizType, Field, Action string }{it.BizType, it.Field, it.Action})
	}
	if err := h.guard.EnsureCanSaveRoleFieldPerm(c.Request.Context(), req.RoleID, guardItems); err != nil {
		web.FailErr(c, err)
		return
	}
	// 审计:旧字段权限
	oldMap, _ := h.svc.ListByRole(c.Request.Context(), req.RoleID)
	items := make([]model.RoleFieldPerm, 0, len(req.Items))
	newMap := make(map[string]string, len(req.Items))
	for _, it := range req.Items {
		items = append(items, model.RoleFieldPerm{
			BizType: it.BizType, Field: it.Field, Action: it.Action,
		})
		newMap[it.BizType+":"+it.Field] = it.Action
	}
	err := h.svc.Save(c.Request.Context(), req.RoleID, items)
	if err == nil && h.auditor != nil {
		_ = h.auditor.TrackUpdate(c.Request.Context(), "system_role", req.RoleID,
			map[string]any{"field_perm": fieldPermSummary(oldMap)},
			map[string]any{"field_perm": fieldPermSummary(newMap)})
	}
	respondOK(c, err)
}

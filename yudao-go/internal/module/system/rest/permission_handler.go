package rest

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/module/system/service"
)

// PermissionHandler 提供 RBAC 权限分配接口：角色↔菜单、角色数据范围、用户↔角色。
type PermissionHandler struct {
	roles   *repo.CRUD[model.Role]
	tx      *orm.TxManager
	guard   *service.PrivilegeGuard
	auditor Auditor
}

func NewPermissionHandler(tx *orm.TxManager) *PermissionHandler {
	return &PermissionHandler{
		roles: repo.NewCRUD[model.Role](tx), tx: tx,
		guard: service.NewPrivilegeGuard(tx),
	}
}

// SetAuditor 注入 chatter 审计器，启用角色权限分配变更的时间线记录。
func (h *PermissionHandler) SetAuditor(a Auditor) { h.auditor = a }

func (h *PermissionHandler) Register(g *gin.RouterGroup) {
	g.GET("/system/permission/list-role-menus", h.listRoleMenus)
	g.POST("/system/permission/assign-role-menu", h.assignRoleMenu)
	g.POST("/system/permission/assign-role-data-scope", h.assignRoleDataScope)
	g.GET("/system/permission/list-user-roles", h.listUserRoles)
	g.POST("/system/permission/assign-user-role", h.assignUserRole)
	g.GET("/system/permission/my-capabilities", h.myCapabilities)
}

// myCapabilities 返回当前用户可授出的能力集合，供前端隐藏不可选项。
func (h *PermissionHandler) myCapabilities(c *gin.Context) {
	ctx := c.Request.Context()
	scope, deptIDs := h.guard.MyDataScope(ctx)
	// 自己对每个敏感字段的有效动作（供「字段权限」表单决定能否授「明文」）
	fieldActions := map[string]string{}
	if fp := contextx.FieldPermOf(ctx); fp != nil {
		for k, v := range fp.Actions {
			fieldActions[k] = v
		}
	}
	web.Success(c, gin.H{
		"superAdmin":   h.guard.IsSuperAdmin(ctx),
		"roleIds":      h.guard.MyRoleIDs(ctx),
		"menuIds":      h.guard.MyMenuIDs(ctx),
		"dataScope":    scope,
		"deptIds":      deptIDs,
		"fieldActions": fieldActions,
	})
}

// listRoleMenus 返回角色已分配的菜单编号集合。
func (h *PermissionHandler) listRoleMenus(c *gin.Context) {
	roleID, _ := strconv.ParseInt(c.Query("roleId"), 10, 64)
	ids := make([]int64, 0)
	err := h.tx.DB(c.Request.Context()).Model(&model.RoleMenu{}).
		Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, ids)
}

// assignRoleMenu 重新分配角色的菜单权限（全量覆盖）。
func (h *PermissionHandler) assignRoleMenu(c *gin.Context) {
	var req struct {
		RoleID  int64   `json:"roleId"`
		MenuIDs []int64 `json:"menuIds"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.guard.EnsureCanModifyRole(c.Request.Context(), req.RoleID); err != nil {
		web.FailErr(c, err)
		return
	}
	if err := h.guard.EnsureCanGrantMenus(c.Request.Context(), req.MenuIDs); err != nil {
		web.FailErr(c, err)
		return
	}
	// 审计:旧菜单数量
	var oldCount int64
	h.tx.DB(c.Request.Context()).Model(&model.RoleMenu{}).
		Where("role_id = ?", req.RoleID).Count(&oldCount)
	err := h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		db := h.tx.DB(ctx)
		// 物理删除旧分配，再写入新集合。
		if err := db.Unscoped().Where("role_id = ?", req.RoleID).
			Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(req.MenuIDs) == 0 {
			return nil
		}
		rows := make([]model.RoleMenu, 0, len(req.MenuIDs))
		for _, mid := range req.MenuIDs {
			var rm model.RoleMenu
			rm.RoleID, rm.MenuID = req.RoleID, mid
			rows = append(rows, rm)
		}
		return db.Create(&rows).Error
	})
	if err == nil && h.auditor != nil {
		_ = h.auditor.TrackUpdate(c.Request.Context(), "system_role", req.RoleID,
			map[string]any{"menus": int(oldCount)},
			map[string]any{"menus": len(req.MenuIDs)})
	}
	respondOK(c, err)
}

// assignRoleDataScope 设置角色的数据权限范围。
func (h *PermissionHandler) assignRoleDataScope(c *gin.Context) {
	var req struct {
		RoleID           int64   `json:"roleId"`
		DataScope        int8    `json:"dataScope"`
		DataScopeDeptIds []int64 `json:"dataScopeDeptIds"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.guard.EnsureCanModifyRole(c.Request.Context(), req.RoleID); err != nil {
		web.FailErr(c, err)
		return
	}
	if err := h.guard.EnsureCanGrantDataScope(c.Request.Context(), req.DataScope, req.DataScopeDeptIds); err != nil {
		web.FailErr(c, err)
		return
	}
	old, _ := h.roles.Get(c.Request.Context(), req.RoleID)
	deptJSON, _ := json.Marshal(req.DataScopeDeptIds)
	err := h.roles.UpdateFields(c.Request.Context(), req.RoleID, map[string]any{
		"data_scope":          req.DataScope,
		"data_scope_dept_ids": string(deptJSON),
	})
	if err == nil && h.auditor != nil && old != nil {
		oldDeptJSON, _ := json.Marshal([]int64(old.DataScopeDeptIDs))
		_ = h.auditor.TrackUpdate(c.Request.Context(), "system_role", req.RoleID,
			map[string]any{"data_scope": old.DataScope, "data_scope_dept_ids": string(oldDeptJSON)},
			map[string]any{"data_scope": req.DataScope, "data_scope_dept_ids": string(deptJSON)})
	}
	respondOK(c, err)
}

// listUserRoles 返回用户已分配的角色编号集合。
func (h *PermissionHandler) listUserRoles(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Query("userId"), 10, 64)
	ids := make([]int64, 0)
	err := h.tx.DB(c.Request.Context()).Model(&model.UserRole{}).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, ids)
}

// assignUserRole 重新分配用户的角色（全量覆盖）。
func (h *PermissionHandler) assignUserRole(c *gin.Context) {
	var req struct {
		UserID  int64   `json:"userId"`
		RoleIDs []int64 `json:"roleIds"`
	}
	if !bind(c, &req) {
		return
	}
	ctx := c.Request.Context()
	if err := h.guard.EnsureCanAssignToUser(ctx, req.UserID, req.RoleIDs); err != nil {
		web.FailErr(c, err)
		return
	}
	// 审计:旧角色名集合（查询失败不阻塞主流程，只是动态里显示为空）。
	oldRoleIDs := make([]int64, 0)
	h.tx.DB(ctx).Model(&model.UserRole{}).
		Where("user_id = ?", req.UserID).Pluck("role_id", &oldRoleIDs)
	oldNames := h.roleNamesByIDs(ctx, oldRoleIDs)
	err := h.tx.Do(ctx, func(ctx context.Context) error {
		db := h.tx.DB(ctx)
		if err := db.Unscoped().Where("user_id = ?", req.UserID).
			Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if len(req.RoleIDs) == 0 {
			return nil
		}
		rows := make([]model.UserRole, 0, len(req.RoleIDs))
		for _, rid := range req.RoleIDs {
			var ur model.UserRole
			ur.UserID, ur.RoleID = req.UserID, rid
			rows = append(rows, ur)
		}
		return db.Create(&rows).Error
	})
	if err == nil && h.auditor != nil {
		newNames := h.roleNamesByIDs(ctx, req.RoleIDs)
		_ = h.auditor.TrackUpdate(ctx, "system_user", req.UserID,
			map[string]any{"roles": oldNames},
			map[string]any{"roles": newNames})
	}
	respondOK(c, err)
}

// roleNamesByIDs 按角色 ID 集合查询角色名,返回逗号拼接的字符串(用于审计 diff 的稳定比较)。
func (h *PermissionHandler) roleNamesByIDs(ctx context.Context, ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	names := make([]string, 0, len(ids))
	// 按 id 升序查,保证字符串稳定,避免顺序差异被误判为变更。
	_ = h.tx.DB(ctx).Model(&model.Role{}).
		Where("id IN ?", ids).Order("id ASC").Pluck("name", &names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ","
		}
		out += n
	}
	return out
}

package rest

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/module/system/service"
	"yudao-go/internal/pkg/errcode"
)

// SysHandler 提供系统管理（部门/岗位/角色/菜单）的 CRUD 接口。
type SysHandler struct {
	dept    *repo.CRUD[model.Dept]
	post    *repo.CRUD[model.Post]
	role    *repo.CRUD[model.Role]
	menu    *repo.CRUD[model.Menu]
	guard   *service.PrivilegeGuard
	auditor Auditor
}

// SetAuditor 注入 chatter 字段变更审计器，启用角色变更的时间线记录。
func (h *SysHandler) SetAuditor(a Auditor) { h.auditor = a }

func NewSysHandler(tx *orm.TxManager) *SysHandler {
	return &SysHandler{
		dept:  repo.NewCRUD[model.Dept](tx),
		post:  repo.NewCRUD[model.Post](tx),
		role:  repo.NewCRUD[model.Role](tx),
		menu:  repo.NewCRUD[model.Menu](tx),
		guard: service.NewPrivilegeGuard(tx),
	}
}

// Register 注册系统管理路由（需认证）。
func (h *SysHandler) Register(g *gin.RouterGroup) {
	// 部门
	g.GET("/system/dept/list", h.deptList)
	g.GET("/system/dept/simple-list", h.deptList)
	g.GET("/system/dept/get", h.deptGet)
	g.POST("/system/dept/create", h.deptCreate)
	g.PUT("/system/dept/update", h.deptUpdate)
	g.DELETE("/system/dept/delete", h.deptDelete)
	// 岗位
	g.GET("/system/post/page", h.postPage)
	g.GET("/system/post/simple-list", h.postSimpleList)
	g.GET("/system/post/get", h.postGet)
	g.POST("/system/post/create", h.postCreate)
	g.PUT("/system/post/update", h.postUpdate)
	g.DELETE("/system/post/delete", h.postDelete)
	g.DELETE("/system/post/delete-list", h.postDeleteList)
	// 角色
	g.GET("/system/role/page", h.rolePage)
	g.GET("/system/role/simple-list", h.roleSimpleList)
	g.GET("/system/role/get", h.roleGet)
	g.POST("/system/role/create", h.roleCreate)
	g.PUT("/system/role/update", h.roleUpdate)
	g.DELETE("/system/role/delete", h.roleDelete)
	g.DELETE("/system/role/delete-list", h.roleDeleteList)
	// 菜单
	g.GET("/system/menu/list", h.menuList)
	g.GET("/system/menu/simple-list", h.menuList)
	g.GET("/system/menu/get", h.menuGet)
	g.POST("/system/menu/create", h.menuCreate)
	g.PUT("/system/menu/update", h.menuUpdate)
	g.DELETE("/system/menu/delete", h.menuDelete)
}

// ===== 部门 =====

func (h *SysHandler) deptList(c *gin.Context) {
	name, status := c.Query("name"), c.Query("status")
	list, err := h.dept.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		q = likeIf(q, "name", name)
		q = eqIf(q, "status", status)
		return q.Order("sort ASC")
	})
	respond(c, list, err)
}

func (h *SysHandler) deptGet(c *gin.Context) {
	m, err := h.dept.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *SysHandler) deptCreate(c *gin.Context) {
	var m model.Dept
	if !bind(c, &m) {
		return
	}
	if err := h.dept.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *SysHandler) deptUpdate(c *gin.Context) {
	var m model.Dept
	if !bind(c, &m) {
		return
	}
	err := h.dept.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "parent_id": m.ParentID, "sort": m.Sort,
		"leader_user_id": m.LeaderUserID, "phone": m.Phone, "email": m.Email, "status": m.Status,
	})
	respondOK(c, err)
}

func (h *SysHandler) deptDelete(c *gin.Context) {
	respondOK(c, h.dept.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

// ===== 岗位 =====

func (h *SysHandler) postPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	code, name, status := c.Query("code"), c.Query("name"), c.Query("status")
	list, total, err := h.post.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "code", code)
			q = likeIf(q, "name", name)
			q = eqIf(q, "status", status)
			return q.Order("sort ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *SysHandler) postSimpleList(c *gin.Context) {
	list, err := h.post.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0").Order("sort ASC")
	})
	respond(c, list, err)
}

func (h *SysHandler) postGet(c *gin.Context) {
	m, err := h.post.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *SysHandler) postCreate(c *gin.Context) {
	var m model.Post
	if !bind(c, &m) {
		return
	}
	if err := h.post.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *SysHandler) postUpdate(c *gin.Context) {
	var m model.Post
	if !bind(c, &m) {
		return
	}
	err := h.post.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"code": m.Code, "name": m.Name, "sort": m.Sort, "status": m.Status, "remark": m.Remark,
	})
	respondOK(c, err)
}

func (h *SysHandler) postDelete(c *gin.Context) {
	respondOK(c, h.post.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *SysHandler) postDeleteList(c *gin.Context) {
	respondOK(c, h.post.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 角色 =====

func (h *SysHandler) rolePage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, code, status := c.Query("name"), c.Query("code"), c.Query("status")
	list, total, err := h.role.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = likeIf(q, "code", code)
			q = eqIf(q, "status", status)
			return q.Order("sort ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *SysHandler) roleSimpleList(c *gin.Context) {
	list, err := h.role.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0").Order("sort ASC")
	})
	respond(c, list, err)
}

func (h *SysHandler) roleGet(c *gin.Context) {
	m, err := h.role.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *SysHandler) roleCreate(c *gin.Context) {
	var m model.Role
	if !bind(c, &m) {
		return
	}
	if m.Type == 0 {
		m.Type = 2 // 自定义角色
	}
	if err := h.role.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *SysHandler) roleUpdate(c *gin.Context) {
	var m model.Role
	if !bind(c, &m) {
		return
	}
	ctx := c.Request.Context()
	if err := h.guard.EnsureCanModifyRole(ctx, m.ID); err != nil {
		web.FailErr(c, err)
		return
	}
	old, _ := h.role.Get(ctx, m.ID)
	err := h.role.UpdateFields(ctx, m.ID, map[string]any{
		"name": m.Name, "code": m.Code, "sort": m.Sort,
		"status": m.Status, "remark": m.Remark,
	})
	if err == nil && h.auditor != nil && old != nil {
		_ = h.auditor.TrackUpdate(ctx, "system_role", m.ID,
			map[string]any{"name": old.Name, "code": old.Code, "sort": old.Sort, "status": old.Status, "remark": old.Remark},
			map[string]any{"name": m.Name, "code": m.Code, "sort": m.Sort, "status": m.Status, "remark": m.Remark})
	}
	respondOK(c, err)
}

func (h *SysHandler) roleDelete(c *gin.Context) {
	id := qID(c)
	if err := h.guard.EnsureCanModifyRole(c.Request.Context(), id); err != nil {
		web.FailErr(c, err)
		return
	}
	respondOK(c, h.role.SoftDelete(c.Request.Context(), []int64{id}))
}

func (h *SysHandler) roleDeleteList(c *gin.Context) {
	ids := qIDs(c)
	for _, id := range ids {
		if err := h.guard.EnsureCanModifyRole(c.Request.Context(), id); err != nil {
			web.FailErr(c, err)
			return
		}
	}
	respondOK(c, h.role.SoftDelete(c.Request.Context(), ids))
}

// ===== 菜单 =====

func (h *SysHandler) menuList(c *gin.Context) {
	name, status := c.Query("name"), c.Query("status")
	list, err := h.menu.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		q = likeIf(q, "name", name)
		q = eqIf(q, "status", status)
		return q.Order("sort ASC")
	})
	respond(c, list, err)
}

func (h *SysHandler) menuGet(c *gin.Context) {
	m, err := h.menu.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *SysHandler) menuCreate(c *gin.Context) {
	if err := h.guard.EnsureCanManageMenu(c.Request.Context()); err != nil {
		web.FailErr(c, err)
		return
	}
	var m model.Menu
	if !bind(c, &m) {
		return
	}
	if err := h.menu.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *SysHandler) menuUpdate(c *gin.Context) {
	if err := h.guard.EnsureCanManageMenu(c.Request.Context()); err != nil {
		web.FailErr(c, err)
		return
	}
	var m model.Menu
	if !bind(c, &m) {
		return
	}
	err := h.menu.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "permission": m.Permission, "type": m.Type, "sort": m.Sort,
		"parent_id": m.ParentID, "path": m.Path, "icon": m.Icon, "component": m.Component,
		"component_name": m.ComponentName, "status": m.Status,
		"visible": m.Visible, "keep_alive": m.KeepAlive, "always_show": m.AlwaysShow,
	})
	respondOK(c, err)
}

func (h *SysHandler) menuDelete(c *gin.Context) {
	if err := h.guard.EnsureCanManageMenu(c.Request.Context()); err != nil {
		web.FailErr(c, err)
		return
	}
	respondOK(c, h.menu.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

// ===== 辅助函数 =====

func bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		web.Fail(c, errcode.BadRequest.WithMsg("请求参数不正确"))
		return false
	}
	return true
}

func respond[T any](c *gin.Context, data T, err error) {
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, data)
}

func respondOne[T any](c *gin.Context, data *T, err error) {
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if data == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	web.Success(c, data)
}

func respondOK(c *gin.Context, err error) {
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, true)
}

func qID(c *gin.Context) int64 {
	id, _ := strconv.ParseInt(c.Query("id"), 10, 64)
	return id
}

func qIDs(c *gin.Context) []int64 {
	var ids []int64
	for _, s := range strings.Split(c.Query("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func likeIf(q *gorm.DB, col, val string) *gorm.DB {
	if val != "" {
		return q.Where(col+" LIKE ?", "%"+val+"%")
	}
	return q
}

func eqIf(q *gorm.DB, col, val string) *gorm.DB {
	if val != "" {
		return q.Where(col+" = ?", val)
	}
	return q
}

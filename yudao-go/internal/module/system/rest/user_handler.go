package rest

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/module/system/service"
	"yudao-go/internal/pkg/errcode"
)

// Auditor 记录业务记录的字段变更，由 chatter 模块实现。
type Auditor interface {
	TrackUpdate(ctx context.Context, bizType string, bizID int64, oldVals, newVals map[string]any) error
}

// UserHandler 提供用户管理 CRUD 接口。
type UserHandler struct {
	users   *repo.CRUD[model.User]
	depts   *repo.CRUD[model.Dept]
	posts   *repo.CRUD[model.Post]
	dicts   *repo.CRUD[model.DictData]
	tx      *orm.TxManager
	auditor Auditor
	guard   *service.PrivilegeGuard
}

func NewUserHandler(tx *orm.TxManager) *UserHandler {
	return &UserHandler{
		users: repo.NewCRUD[model.User](tx),
		depts: repo.NewCRUD[model.Dept](tx),
		posts: repo.NewCRUD[model.Post](tx),
		dicts: repo.NewCRUD[model.DictData](tx),
		tx:    tx,
		guard: service.NewPrivilegeGuard(tx),
	}
}

// sexLabel 把性别值解析为字典标签（system_user_sex），解析不到则返回原值。
func (h *UserHandler) sexLabel(ctx context.Context, sex int8) string {
	v := strconv.Itoa(int(sex))
	list, _ := h.dicts.List(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("dict_type = ? AND value = ?", "system_user_sex", v).Limit(1)
	})
	if len(list) > 0 {
		return list[0].Label
	}
	return v
}

// deptName 把部门 ID 解析为部门名称，解析不到则返回 ID 字符串。
func (h *UserHandler) deptName(ctx context.Context, id int64) string {
	if id == 0 {
		return ""
	}
	if d, _ := h.depts.Get(ctx, id); d != nil {
		return d.Name
	}
	return strconv.FormatInt(id, 10)
}

// postNames 把岗位 ID 数组解析为「名称, 名称」。先排序以保证比较稳定（忽略顺序差异）。
func (h *UserHandler) postNames(ctx context.Context, ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	sorted := append([]int64(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	posts, _ := h.posts.List(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("id IN ?", sorted)
	})
	nameByID := make(map[int64]string, len(posts))
	for _, p := range posts {
		nameByID[p.ID] = p.Name
	}
	names := make([]string, 0, len(sorted))
	for _, id := range sorted {
		if n := nameByID[id]; n != "" {
			names = append(names, n)
		} else {
			names = append(names, strconv.FormatInt(id, 10))
		}
	}
	return strings.Join(names, ", ")
}

// SetAuditor 注入字段变更审计器（chatter 模块），启用用户修改的时间线记录。
func (h *UserHandler) SetAuditor(a Auditor) { h.auditor = a }

func (h *UserHandler) Register(g *gin.RouterGroup) {
	g.GET("/system/user/page", h.page)
	g.GET("/system/user/list", h.list)
	g.GET("/system/user/simple-list", h.simpleList)
	g.GET("/system/user/get", h.get)
	g.POST("/system/user/create", h.create)
	g.PUT("/system/user/update", h.update)
	g.PUT("/system/user/update-status", h.updateStatus)
	g.PUT("/system/user/update-password", h.updatePassword)
	g.DELETE("/system/user/delete", h.delete)
	g.DELETE("/system/user/delete-list", h.deleteList)
}

// userVO 是用户响应对象：用户字段 + 部门名 + 岗位 ID 数组。
type userVO struct {
	*model.User
	DeptName   string  `json:"deptName"`
	PostIDList []int64 `json:"postIds"`
}

// userSaveReq 是用户创建/修改请求。
type userSaveReq struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Nickname string  `json:"nickname"`
	Password string  `json:"password"`
	DeptID   int64   `json:"deptId"`
	PostIDs  []int64 `json:"postIds"`
	Email    string  `json:"email"`
	Mobile   string  `json:"mobile"`
	Sex      int8    `json:"sex"`
	Remark   string  `json:"remark"`
	Status   int8    `json:"status"`
}

// toVOs 把用户列表转为响应对象，并批量填充部门名与岗位数组。
func (h *UserHandler) toVOs(ctx context.Context, users []*model.User) []*userVO {
	deptIDs := make([]int64, 0, len(users))
	for _, u := range users {
		deptIDs = append(deptIDs, u.DeptID)
	}
	deptName := map[int64]string{}
	if len(deptIDs) > 0 {
		depts, _ := h.depts.List(ctx, func(q *gorm.DB) *gorm.DB {
			return q.Where("id IN ?", deptIDs)
		})
		for _, d := range depts {
			deptName[d.ID] = d.Name
		}
	}
	out := make([]*userVO, 0, len(users))
	for _, u := range users {
		out = append(out, &userVO{
			User: u, DeptName: deptName[u.DeptID], PostIDList: parsePostIDs(u.PostIDs),
		})
	}
	return out
}

func (h *UserHandler) page(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	username, mobile, status, deptID := c.Query("username"), c.Query("mobile"),
		c.Query("status"), c.Query("deptId")
	list, total, err := h.users.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "username", username)
			q = likeIf(q, "mobile", mobile)
			q = eqIf(q, "status", status)
			q = eqIf(q, "dept_id", deptID)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(h.toVOs(c.Request.Context(), list), total))
}

func (h *UserHandler) list(c *gin.Context) {
	ids := qIDs(c)
	list, err := h.users.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		if len(ids) > 0 {
			q = q.Where("id IN ?", ids)
		}
		return q.Order("id ASC")
	})
	respond(c, h.toVOs(c.Request.Context(), list), err)
}

func (h *UserHandler) simpleList(c *gin.Context) {
	list, err := h.users.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0").Order("id ASC")
	})
	respond(c, h.toVOs(c.Request.Context(), list), err)
}

func (h *UserHandler) get(c *gin.Context) {
	u, err := h.users.Get(c.Request.Context(), qID(c))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if u == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	web.Success(c, h.toVOs(c.Request.Context(), []*model.User{u})[0])
}

func (h *UserHandler) create(c *gin.Context) {
	var req userSaveReq
	if !bind(c, &req) {
		return
	}
	// 用户名唯一校验
	exist, _ := h.users.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("username = ?", req.Username).Limit(1)
	})
	if len(exist) > 0 {
		web.Fail(c, errcode.New(1002000001, "用户账号已存在"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	u := &model.User{
		Username: req.Username, Nickname: req.Nickname, Password: string(hash),
		DeptID: req.DeptID, PostIDs: marshalPostIDs(req.PostIDs),
		Email: req.Email, Mobile: req.Mobile, Sex: req.Sex,
		Remark: req.Remark, Status: req.Status,
	}
	if err := h.users.Create(c.Request.Context(), u); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, u.ID)
}

func (h *UserHandler) update(c *gin.Context) {
	var req userSaveReq
	if !bind(c, &req) {
		return
	}
	// 更新与字段审计在同一事务内完成。
	err := h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		old, err := h.users.Get(ctx, req.ID)
		if err != nil {
			return err
		}
		if old == nil {
			return errcode.NotFound
		}
		if err := h.users.UpdateFields(ctx, req.ID, map[string]any{
			"nickname": req.Nickname, "dept_id": req.DeptID,
			"post_ids": marshalPostIDs(req.PostIDs), "email": req.Email,
			"mobile": req.Mobile, "sex": req.Sex, "remark": req.Remark,
		}); err != nil {
			return err
		}
		// 记录字段变更 → chatter 时间线。
		// 枚举（性别）/外键（部门）在此解析为显示值再传入——与 Odoo/Axelor 一致。
		if h.auditor != nil {
			oldVals := map[string]any{
				"nickname": old.Nickname, "dept_id": h.deptName(ctx, old.DeptID),
				"post_ids": h.postNames(ctx, parsePostIDs(old.PostIDs)),
				"email":    old.Email, "mobile": old.Mobile,
				"sex": h.sexLabel(ctx, old.Sex), "remark": old.Remark,
			}
			newVals := map[string]any{
				"nickname": req.Nickname, "dept_id": h.deptName(ctx, req.DeptID),
				"post_ids": h.postNames(ctx, req.PostIDs),
				"email":    req.Email, "mobile": req.Mobile,
				"sex": h.sexLabel(ctx, req.Sex), "remark": req.Remark,
			}
			return h.auditor.TrackUpdate(ctx, "system_user", req.ID, oldVals, newVals)
		}
		return nil
	})
	respondOK(c, err)
}

func (h *UserHandler) updateStatus(c *gin.Context) {
	var req struct {
		ID     int64 `json:"id"`
		Status int8  `json:"status"`
	}
	if !bind(c, &req) {
		return
	}
	respondOK(c, h.users.UpdateFields(c.Request.Context(), req.ID,
		map[string]any{"status": req.Status}))
}

func (h *UserHandler) updatePassword(c *gin.Context) {
	var req struct {
		ID       int64  `json:"id"`
		Password string `json:"password"`
	}
	if !bind(c, &req) {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	respondOK(c, h.users.UpdateFields(c.Request.Context(), req.ID,
		map[string]any{"password": string(hash)}))
}

func (h *UserHandler) delete(c *gin.Context) {
	id := qID(c)
	if err := h.guard.EnsureNotSelf(c.Request.Context(), id, "删除"); err != nil {
		web.FailErr(c, err)
		return
	}
	respondOK(c, h.users.SoftDelete(c.Request.Context(), []int64{id}))
}

func (h *UserHandler) deleteList(c *gin.Context) {
	ids := qIDs(c)
	for _, id := range ids {
		if err := h.guard.EnsureNotSelf(c.Request.Context(), id, "删除"); err != nil {
			web.FailErr(c, err)
			return
		}
	}
	respondOK(c, h.users.SoftDelete(c.Request.Context(), ids))
}

// parsePostIDs 解析 post_ids JSON 字符串为 ID 数组。
func parsePostIDs(s string) []int64 {
	ids := make([]int64, 0)
	if s != "" {
		_ = json.Unmarshal([]byte(s), &ids)
	}
	return ids
}

// marshalPostIDs 把岗位 ID 数组序列化为 JSON 字符串。
func marshalPostIDs(ids []int64) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

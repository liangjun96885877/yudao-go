package rest

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// NotifyHandler 提供消息中心「站内信模板」「站内信」接口。
type NotifyHandler struct {
	tpl *repo.CRUD[model.NotifyTemplate]
	msg *repo.CRUD[model.NotifyMessage]
	tx  *orm.TxManager
}

func NewNotifyHandler(tx *orm.TxManager) *NotifyHandler {
	return &NotifyHandler{
		tpl: repo.NewCRUD[model.NotifyTemplate](tx),
		msg: repo.NewCRUD[model.NotifyMessage](tx),
		tx:  tx,
	}
}

func (h *NotifyHandler) Register(g *gin.RouterGroup) {
	g.GET("/system/notify-template/page", h.tplPage)
	g.GET("/system/notify-template/get", h.tplGet)
	g.POST("/system/notify-template/create", h.tplCreate)
	g.PUT("/system/notify-template/update", h.tplUpdate)
	g.DELETE("/system/notify-template/delete", h.tplDelete)
	g.DELETE("/system/notify-template/delete-list", h.tplDeleteList)
	g.POST("/system/notify-template/send-notify", h.sendNotify)
	g.GET("/system/notify-message/page", h.msgPage)
	g.GET("/system/notify-message/my-page", h.msgMyPage)
	g.PUT("/system/notify-message/update-read", h.msgUpdateRead)
	g.PUT("/system/notify-message/update-all-read", h.msgUpdateAllRead)
	g.GET("/system/notify-message/get-unread-list", h.msgUnreadList)
	g.GET("/system/notify-message/get-unread-count", h.msgUnreadCount)
}

// ===== 站内信模板 =====

var paramPattern = regexp.MustCompile(`\{(\w+)\}`)

// extractParams 从模板内容提取 {xxx} 占位符名，返回 JSON 数组字符串。
func extractParams(content string) string {
	seen := map[string]bool{}
	var names []string
	for _, m := range paramPattern.FindAllStringSubmatch(content, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	if names == nil {
		names = []string{}
	}
	b, _ := json.Marshal(names)
	return string(b)
}

// notifyTplVO 在模板字段外把 params 以字符串数组返回。
type notifyTplVO struct {
	*model.NotifyTemplate
	Params []string `json:"params"`
}

// notifyTplReq 是模板创建 / 修改请求（params 由 content 自动解析）。
type notifyTplReq struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
	Type     int8   `json:"type"`
	Status   int8   `json:"status"`
	Remark   string `json:"remark"`
}

func (h *NotifyHandler) tplPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, code, status := c.Query("name"), c.Query("code"), c.Query("status")
	list, total, err := h.tpl.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = likeIf(q, "code", code)
			q = eqIf(q, "status", status)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *NotifyHandler) tplGet(c *gin.Context) {
	m, err := h.tpl.Get(c.Request.Context(), qID(c))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if m == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	var params []string
	_ = json.Unmarshal([]byte(m.Params), &params)
	web.Success(c, &notifyTplVO{NotifyTemplate: m, Params: params})
}

func (h *NotifyHandler) tplCreate(c *gin.Context) {
	var req notifyTplReq
	if !bind(c, &req) {
		return
	}
	m := &model.NotifyTemplate{
		Name: req.Name, Code: req.Code, Nickname: req.Nickname, Content: req.Content,
		Type: req.Type, Status: req.Status, Remark: req.Remark,
		Params: extractParams(req.Content),
	}
	if err := h.tpl.Create(c.Request.Context(), m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *NotifyHandler) tplUpdate(c *gin.Context) {
	var req notifyTplReq
	if !bind(c, &req) {
		return
	}
	respondOK(c, h.tpl.UpdateFields(c.Request.Context(), req.ID, map[string]any{
		"name": req.Name, "code": req.Code, "nickname": req.Nickname,
		"content": req.Content, "type": req.Type, "status": req.Status,
		"remark": req.Remark, "params": extractParams(req.Content),
	}))
}

func (h *NotifyHandler) tplDelete(c *gin.Context) {
	respondOK(c, h.tpl.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *NotifyHandler) tplDeleteList(c *gin.Context) {
	respondOK(c, h.tpl.SoftDelete(c.Request.Context(), qIDs(c)))
}

// sendNotify 按模板编码向指定用户发送一条站内信。
func (h *NotifyHandler) sendNotify(c *gin.Context) {
	var req struct {
		UserID         int64          `json:"userId"`
		UserType       int8           `json:"userType"`
		TemplateCode   string         `json:"templateCode"`
		TemplateParams map[string]any `json:"templateParams"`
	}
	if !bind(c, &req) {
		return
	}
	tpls, err := h.tpl.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("code = ?", req.TemplateCode).Limit(1)
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if len(tpls) == 0 {
		web.Fail(c, errcode.NotFound.WithMsg("站内信模板不存在"))
		return
	}
	tpl := tpls[0]
	content := tpl.Content
	for k, v := range req.TemplateParams {
		content = strings.ReplaceAll(content, "{"+k+"}", fmt.Sprintf("%v", v))
	}
	paramsJSON, _ := json.Marshal(req.TemplateParams)
	m := &model.NotifyMessage{
		UserID: req.UserID, UserType: req.UserType,
		TemplateID: tpl.ID, TemplateCode: tpl.Code, TemplateNickname: tpl.Nickname,
		TemplateContent: content, TemplateType: int(tpl.Type),
		TemplateParams: string(paramsJSON),
	}
	if err := h.msg.Create(c.Request.Context(), m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

// ===== 站内信 =====

func (h *NotifyHandler) msgPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	userID, code := c.Query("userId"), c.Query("templateCode")
	list, total, err := h.msg.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = eqIf(q, "user_id", userID)
			q = likeIf(q, "template_code", code)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

// msgMyPage 返回当前登录用户的站内信。
func (h *NotifyHandler) msgMyPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	uid := contextx.UserID(c.Request.Context())
	readStatus := c.Query("readStatus")
	list, total, err := h.msg.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = q.Where("user_id = ?", uid)
			q = eqIf(q, "read_status", readStatus)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

// msgUpdateRead 把指定站内信标记为已读。
func (h *NotifyHandler) msgUpdateRead(c *gin.Context) {
	ids := qIDs(c)
	now := time.Now()
	for _, id := range ids {
		_ = h.msg.UpdateFields(c.Request.Context(), id, map[string]any{
			"read_status": orm.Bit(true), "read_time": now,
		})
	}
	web.Success(c, true)
}

// msgUpdateAllRead 把当前用户的全部未读站内信标记为已读。
func (h *NotifyHandler) msgUpdateAllRead(c *gin.Context) {
	uid := contextx.UserID(c.Request.Context())
	err := h.tx.DB(c.Request.Context()).Model(&model.NotifyMessage{}).
		Where("user_id = ? AND read_status = 0 AND deleted = 0", uid).
		Updates(map[string]any{"read_status": orm.Bit(true), "read_time": time.Now()}).Error
	respondOK(c, err)
}

// msgUnreadList 返回当前用户最近的未读站内信。
func (h *NotifyHandler) msgUnreadList(c *gin.Context) {
	uid := contextx.UserID(c.Request.Context())
	list, err := h.msg.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("user_id = ? AND read_status = 0", uid).Order("id DESC").Limit(10)
	})
	respond(c, list, err)
}

// msgUnreadCount 返回当前用户未读站内信数量。
func (h *NotifyHandler) msgUnreadCount(c *gin.Context) {
	uid := contextx.UserID(c.Request.Context())
	var count int64
	err := h.tx.DB(c.Request.Context()).Model(&model.NotifyMessage{}).
		Where("user_id = ? AND read_status = 0 AND deleted = 0", uid).Count(&count).Error
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, count)
}

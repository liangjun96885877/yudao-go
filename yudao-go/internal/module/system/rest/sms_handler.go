package rest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// SmsHandler 提供消息中心「短信渠道」「短信模板」「短信记录」接口。
type SmsHandler struct {
	channel  *repo.CRUD[model.SmsChannel]
	template *repo.CRUD[model.SmsTemplate]
	log      *repo.CRUD[model.SmsLog]
}

func NewSmsHandler(tx *orm.TxManager) *SmsHandler {
	return &SmsHandler{
		channel:  repo.NewCRUD[model.SmsChannel](tx),
		template: repo.NewCRUD[model.SmsTemplate](tx),
		log:      repo.NewCRUD[model.SmsLog](tx),
	}
}

func (h *SmsHandler) Register(g *gin.RouterGroup) {
	g.GET("/system/sms-channel/page", h.channelPage)
	g.GET("/system/sms-channel/get", h.channelGet)
	g.GET("/system/sms-channel/simple-list", h.channelSimpleList)
	g.POST("/system/sms-channel/create", h.channelCreate)
	g.PUT("/system/sms-channel/update", h.channelUpdate)
	g.DELETE("/system/sms-channel/delete", h.channelDelete)
	g.DELETE("/system/sms-channel/delete-list", h.channelDeleteList)
	g.GET("/system/sms-template/page", h.templatePage)
	g.GET("/system/sms-template/get", h.templateGet)
	g.POST("/system/sms-template/create", h.templateCreate)
	g.PUT("/system/sms-template/update", h.templateUpdate)
	g.DELETE("/system/sms-template/delete", h.templateDelete)
	g.DELETE("/system/sms-template/delete-list", h.templateDeleteList)
	g.POST("/system/sms-template/send-sms", h.sendSms)
	g.GET("/system/sms-log/page", h.logPage)
}

// ===== 短信渠道 =====

func (h *SmsHandler) channelPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	signature, status := c.Query("signature"), c.Query("status")
	list, total, err := h.channel.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "signature", signature)
			q = eqIf(q, "status", status)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *SmsHandler) channelGet(c *gin.Context) {
	m, err := h.channel.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

// channelSimpleList 仅返回启用状态的渠道。
func (h *SmsHandler) channelSimpleList(c *gin.Context) {
	list, err := h.channel.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0").Order("id DESC")
	})
	respond(c, list, err)
}

func (h *SmsHandler) channelCreate(c *gin.Context) {
	var m model.SmsChannel
	if !bind(c, &m) {
		return
	}
	if err := h.channel.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *SmsHandler) channelUpdate(c *gin.Context) {
	var m model.SmsChannel
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.channel.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"signature": m.Signature, "code": m.Code, "status": m.Status,
		"remark": m.Remark, "api_key": m.APIKey, "api_secret": m.APISecret,
		"callback_url": m.CallbackURL,
	}))
}

func (h *SmsHandler) channelDelete(c *gin.Context) {
	respondOK(c, h.channel.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *SmsHandler) channelDeleteList(c *gin.Context) {
	respondOK(c, h.channel.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 短信模板 =====

func (h *SmsHandler) templatePage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, code, status := c.Query("name"), c.Query("code"), c.Query("status")
	list, total, err := h.template.Page(c.Request.Context(), p.Offset(), p.Limit(),
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

func (h *SmsHandler) templateGet(c *gin.Context) {
	m, err := h.template.Get(c.Request.Context(), qID(c))
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
	web.Success(c, gin.H{
		"id": m.ID, "type": m.Type, "status": m.Status, "code": m.Code,
		"name": m.Name, "content": m.Content, "remark": m.Remark,
		"apiTemplateId": m.APITemplateID, "channelId": m.ChannelID,
		"channelCode": m.ChannelCode, "params": params, "createTime": m.CreateTime,
	})
}

// resolveChannelCode 据 channelId 查出渠道编码。
func (h *SmsHandler) resolveChannelCode(c *gin.Context, channelID int64) string {
	ch, err := h.channel.Get(c.Request.Context(), channelID)
	if err != nil || ch == nil {
		return ""
	}
	return ch.Code
}

func (h *SmsHandler) templateCreate(c *gin.Context) {
	var m model.SmsTemplate
	if !bind(c, &m) {
		return
	}
	m.Params = extractParams(m.Content)
	m.ChannelCode = h.resolveChannelCode(c, m.ChannelID)
	if err := h.template.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *SmsHandler) templateUpdate(c *gin.Context) {
	var m model.SmsTemplate
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.template.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"type": m.Type, "status": m.Status, "code": m.Code, "name": m.Name,
		"content": m.Content, "remark": m.Remark, "api_template_id": m.APITemplateID,
		"channel_id": m.ChannelID, "channel_code": h.resolveChannelCode(c, m.ChannelID),
		"params": extractParams(m.Content),
	}))
}

func (h *SmsHandler) templateDelete(c *gin.Context) {
	respondOK(c, h.template.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *SmsHandler) templateDeleteList(c *gin.Context) {
	respondOK(c, h.template.SoftDelete(c.Request.Context(), qIDs(c)))
}

// sendSms 按模板编码渲染并“发送”短信，结果记入 system_sms_log。
// 注：真实短信网关（阿里云/腾讯云等）需厂商 SDK 与凭证，此处为模拟投递。
func (h *SmsHandler) sendSms(c *gin.Context) {
	var req struct {
		Mobile         string         `json:"mobile"`
		TemplateCode   string         `json:"templateCode"`
		TemplateParams map[string]any `json:"templateParams"`
	}
	if !bind(c, &req) {
		return
	}
	ctx := c.Request.Context()
	tpls, err := h.template.List(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("code = ?", req.TemplateCode).Limit(1)
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if len(tpls) == 0 {
		web.Fail(c, errcode.NotFound.WithMsg("短信模板不存在"))
		return
	}
	tpl := tpls[0]
	content := tpl.Content
	for k, v := range req.TemplateParams {
		content = strings.ReplaceAll(content, "{"+k+"}", fmt.Sprintf("%v", v))
	}
	paramsJSON, _ := json.Marshal(req.TemplateParams)
	now := time.Now()
	logEntry := &model.SmsLog{
		ChannelID: tpl.ChannelID, ChannelCode: tpl.ChannelCode,
		TemplateID: tpl.ID, TemplateCode: tpl.Code, TemplateType: tpl.Type,
		TemplateContent: content, TemplateParams: string(paramsJSON),
		APITemplateID: tpl.APITemplateID, Mobile: req.Mobile, UserType: 2,
		SendStatus: 10, SendTime: &now, // 模拟发送成功
		APISendCode: "MOCK", APISendMsg: "模拟发送成功（短信网关未接入）",
	}
	if err := h.log.Create(ctx, logEntry); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, logEntry.ID)
}

// ===== 短信记录 =====

func (h *SmsHandler) logPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	mobile, code := c.Query("mobile"), c.Query("templateCode")
	list, total, err := h.log.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "mobile", mobile)
			q = likeIf(q, "template_code", code)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

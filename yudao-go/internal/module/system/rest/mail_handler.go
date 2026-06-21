package rest

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
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

// MailHandler 提供消息中心「邮箱账号」「邮件模板」「邮件记录」接口。
type MailHandler struct {
	account  *repo.CRUD[model.MailAccount]
	template *repo.CRUD[model.MailTemplate]
	log      *repo.CRUD[model.MailLog]
}

func NewMailHandler(tx *orm.TxManager) *MailHandler {
	return &MailHandler{
		account:  repo.NewCRUD[model.MailAccount](tx),
		template: repo.NewCRUD[model.MailTemplate](tx),
		log:      repo.NewCRUD[model.MailLog](tx),
	}
}

func (h *MailHandler) Register(g *gin.RouterGroup) {
	g.GET("/system/mail-account/page", h.accountPage)
	g.GET("/system/mail-account/get", h.accountGet)
	g.GET("/system/mail-account/simple-list", h.accountSimpleList)
	g.POST("/system/mail-account/create", h.accountCreate)
	g.PUT("/system/mail-account/update", h.accountUpdate)
	g.DELETE("/system/mail-account/delete", h.accountDelete)
	g.DELETE("/system/mail-account/delete-list", h.accountDeleteList)
	g.GET("/system/mail-template/page", h.templatePage)
	g.GET("/system/mail-template/get", h.templateGet)
	g.POST("/system/mail-template/create", h.templateCreate)
	g.PUT("/system/mail-template/update", h.templateUpdate)
	g.DELETE("/system/mail-template/delete", h.templateDelete)
	g.DELETE("/system/mail-template/delete-list", h.templateDeleteList)
	g.POST("/system/mail-template/send-mail", h.sendMail)
	g.GET("/system/mail-log/page", h.logPage)
	g.GET("/system/mail-log/get", h.logGet)
}

// ===== 邮箱账号 =====

func (h *MailHandler) accountPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	mail, username := c.Query("mail"), c.Query("username")
	list, total, err := h.account.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "mail", mail)
			q = likeIf(q, "username", username)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *MailHandler) accountGet(c *gin.Context) {
	m, err := h.account.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

func (h *MailHandler) accountSimpleList(c *gin.Context) {
	list, err := h.account.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Order("id DESC")
	})
	respond(c, list, err)
}

func (h *MailHandler) accountCreate(c *gin.Context) {
	var m model.MailAccount
	if !bind(c, &m) {
		return
	}
	if err := h.account.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *MailHandler) accountUpdate(c *gin.Context) {
	var m model.MailAccount
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.account.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"mail": m.Mail, "username": m.Username, "password": m.Password,
		"host": m.Host, "port": m.Port,
		"ssl_enable": m.SSLEnable, "starttls_enable": m.StarttlsEnable,
	}))
}

func (h *MailHandler) accountDelete(c *gin.Context) {
	respondOK(c, h.account.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *MailHandler) accountDeleteList(c *gin.Context) {
	respondOK(c, h.account.SoftDelete(c.Request.Context(), qIDs(c)))
}

// ===== 邮件模板 =====

func (h *MailHandler) templatePage(c *gin.Context) {
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

func (h *MailHandler) templateGet(c *gin.Context) {
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
		"id": m.ID, "name": m.Name, "code": m.Code, "accountId": m.AccountID,
		"nickname": m.Nickname, "title": m.Title, "content": m.Content,
		"status": m.Status, "remark": m.Remark, "params": params,
		"createTime": m.CreateTime,
	})
}

func (h *MailHandler) templateCreate(c *gin.Context) {
	var m model.MailTemplate
	if !bind(c, &m) {
		return
	}
	m.Params = extractParams(m.Title + " " + m.Content)
	if err := h.template.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, m.ID)
}

func (h *MailHandler) templateUpdate(c *gin.Context) {
	var m model.MailTemplate
	if !bind(c, &m) {
		return
	}
	respondOK(c, h.template.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "code": m.Code, "account_id": m.AccountID,
		"nickname": m.Nickname, "title": m.Title, "content": m.Content,
		"status": m.Status, "remark": m.Remark,
		"params": extractParams(m.Title + " " + m.Content),
	}))
}

func (h *MailHandler) templateDelete(c *gin.Context) {
	respondOK(c, h.template.SoftDelete(c.Request.Context(), []int64{qID(c)}))
}

func (h *MailHandler) templateDeleteList(c *gin.Context) {
	respondOK(c, h.template.SoftDelete(c.Request.Context(), qIDs(c)))
}

// sendMail 按模板编码渲染并发送邮件，结果记入 system_mail_log。
func (h *MailHandler) sendMail(c *gin.Context) {
	var req struct {
		Mail           string         `json:"mail"`
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
		web.Fail(c, errcode.NotFound.WithMsg("邮件模板不存在"))
		return
	}
	tpl := tpls[0]
	acc, err := h.account.Get(ctx, tpl.AccountID)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if acc == nil {
		web.Fail(c, errcode.NotFound.WithMsg("邮箱账号不存在"))
		return
	}
	title, content := tpl.Title, tpl.Content
	for k, v := range req.TemplateParams {
		ph := "{" + k + "}"
		title = strings.ReplaceAll(title, ph, fmt.Sprintf("%v", v))
		content = strings.ReplaceAll(content, ph, fmt.Sprintf("%v", v))
	}
	paramsJSON, _ := json.Marshal(req.TemplateParams)

	// 尝试 SMTP 投递，结果落库。
	sendErr := smtpSend(acc, req.Mail, title, content)
	now := time.Now()
	logEntry := &model.MailLog{
		UserType: 2, ToMails: req.Mail, AccountID: acc.ID, FromMail: acc.Mail,
		TemplateID: tpl.ID, TemplateCode: tpl.Code, TemplateNickname: tpl.Nickname,
		TemplateTitle: title, TemplateContent: content, TemplateParams: string(paramsJSON),
		SendTime: &now,
	}
	if sendErr != nil {
		logEntry.SendStatus = 20 // 发送失败
		logEntry.SendException = truncate(sendErr.Error(), 4000)
	} else {
		logEntry.SendStatus = 10 // 发送成功
	}
	if err := h.log.Create(ctx, logEntry); err != nil {
		web.FailErr(c, err)
		return
	}
	if sendErr != nil {
		web.Fail(c, errcode.InternalError.WithMsg("邮件发送失败："+sendErr.Error()))
		return
	}
	web.Success(c, logEntry.ID)
}

// ===== 邮件记录 =====

func (h *MailHandler) logPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	accountID, code := c.Query("accountId"), c.Query("templateCode")
	list, total, err := h.log.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = eqIf(q, "account_id", accountID)
			q = likeIf(q, "template_code", code)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *MailHandler) logGet(c *gin.Context) {
	m, err := h.log.Get(c.Request.Context(), qID(c))
	respondOne(c, m, err)
}

// smtpSend 经 SMTP 发送一封 HTML 邮件，连接带 10s 超时。
func smtpSend(acc *model.MailAccount, to, subject, htmlBody string) error {
	addr := net.JoinHostPort(acc.Host, strconv.Itoa(acc.Port))
	tlsCfg := &tls.Config{ServerName: acc.Host}
	var conn net.Conn
	var err error
	if bool(acc.SSLEnable) {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}
	if err != nil {
		return err
	}
	cli, err := smtp.NewClient(conn, acc.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer cli.Close()
	if !bool(acc.SSLEnable) {
		if ok, _ := cli.Extension("STARTTLS"); ok {
			if err := cli.StartTLS(tlsCfg); err != nil {
				return err
			}
		}
	}
	if ok, _ := cli.Extension("AUTH"); ok {
		if err := cli.Auth(smtp.PlainAuth("", acc.Username, acc.Password, acc.Host)); err != nil {
			return err
		}
	}
	if err := cli.Mail(acc.Mail); err != nil {
		return err
	}
	if err := cli.Rcpt(to); err != nil {
		return err
	}
	w, err := cli.Data()
	if err != nil {
		return err
	}
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n\r\n",
		acc.Mail, to, mime.QEncoding.Encode("utf-8", subject))
	if _, err := w.Write([]byte(header + htmlBody)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return cli.Quit()
}

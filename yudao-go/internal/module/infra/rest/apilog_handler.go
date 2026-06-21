package rest

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
)

// ApiLogHandler 提供「API 访问日志」「API 错误日志」查询接口。
type ApiLogHandler struct {
	access *sysrepo.CRUD[model.ApiAccessLog]
	errLog *sysrepo.CRUD[model.ApiErrorLog]
}

func NewApiLogHandler(tx *orm.TxManager) *ApiLogHandler {
	return &ApiLogHandler{
		access: sysrepo.NewCRUD[model.ApiAccessLog](tx),
		errLog: sysrepo.NewCRUD[model.ApiErrorLog](tx),
	}
}

func (h *ApiLogHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/api-access-log/page", h.accessPage)
	g.GET("/infra/api-error-log/page", h.errorPage)
	g.PUT("/infra/api-error-log/update-status", h.errorUpdateStatus)
	g.GET("/infra/api-error-log/__test-panic", h.testPanic) // 临时：验证 panic 捕获
}

// testPanic 故意触发 panic，用于验证错误日志中间件（验证后删除）。
func (h *ApiLogHandler) testPanic(c *gin.Context) {
	var p *int
	_ = *p // nil 指针解引用
}

func (h *ApiLogHandler) accessPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	url, userID := c.Query("requestUrl"), c.Query("userId")
	list, total, err := h.access.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "request_url", url)
			q = eqIf(q, "user_id", userID)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *ApiLogHandler) errorPage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	url, status := c.Query("requestUrl"), c.Query("processStatus")
	list, total, err := h.errLog.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "request_url", url)
			q = eqIf(q, "process_status", status)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

// errorUpdateStatus 标记错误日志的处理状态（已处理 / 已忽略）。
func (h *ApiLogHandler) errorUpdateStatus(c *gin.Context) {
	id := qID(c)
	status := c.Query("processStatus")
	now := time.Now()
	respondOK(c, h.errLog.UpdateFields(c.Request.Context(), id, map[string]any{
		"process_status":  status,
		"process_time":    now,
		"process_user_id": contextx.UserID(c.Request.Context()),
	}))
}

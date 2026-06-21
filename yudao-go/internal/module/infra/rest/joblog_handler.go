package rest

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// JobLogHandler 提供「定时任务执行日志」查询接口。
type JobLogHandler struct {
	logs *sysrepo.CRUD[model.JobLog]
}

func NewJobLogHandler(tx *orm.TxManager) *JobLogHandler {
	return &JobLogHandler{logs: sysrepo.NewCRUD[model.JobLog](tx)}
}

func (h *JobLogHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/job-log/page", h.page)
	g.GET("/infra/job-log/get", h.get)
}

func (h *JobLogHandler) page(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	jobID, handler, status := c.Query("jobId"), c.Query("handlerName"), c.Query("status")
	list, total, err := h.logs.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = eqIf(q, "job_id", jobID)
			q = likeIf(q, "handler_name", handler)
			q = eqIf(q, "status", status)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *JobLogHandler) get(c *gin.Context) {
	m, err := h.logs.Get(c.Request.Context(), qID(c))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if m == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	web.Success(c, m)
}

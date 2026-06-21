package rest

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	jobsched "yudao-go/internal/framework/job"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// JobHandler 提供基础设施「定时任务」CRUD 接口，并对接 cron 调度器。
type JobHandler struct {
	job   *sysrepo.CRUD[model.Job]
	sched *jobsched.Scheduler
}

func NewJobHandler(tx *orm.TxManager, sched *jobsched.Scheduler) *JobHandler {
	return &JobHandler{job: sysrepo.NewCRUD[model.Job](tx), sched: sched}
}

func (h *JobHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/job/page", h.page)
	g.GET("/infra/job/get", h.get)
	g.GET("/infra/job/get_next_times", h.nextTimes)
	g.POST("/infra/job/create", h.create)
	g.PUT("/infra/job/update", h.update)
	g.PUT("/infra/job/update-status", h.updateStatus)
	g.PUT("/infra/job/trigger", h.trigger)
	g.DELETE("/infra/job/delete", h.del)
	g.DELETE("/infra/job/delete-list", h.delList)
}

func (h *JobHandler) page(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	name, status, handler := c.Query("name"), c.Query("status"), c.Query("handlerName")
	list, total, err := h.job.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "name", name)
			q = eqIf(q, "status", status)
			q = likeIf(q, "handler_name", handler)
			return q.Order("id ASC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

func (h *JobHandler) get(c *gin.Context) {
	m, err := h.job.Get(c.Request.Context(), qID(c))
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

// nextTimes 返回任务后续执行时间：暂返回空列表。
func (h *JobHandler) nextTimes(c *gin.Context) {
	web.Success(c, []string{})
}

func (h *JobHandler) create(c *gin.Context) {
	var m model.Job
	if !bind(c, &m) {
		return
	}
	if err := h.job.Create(c.Request.Context(), &m); err != nil {
		web.FailErr(c, err)
		return
	}
	if m.Status == 0 { // 新建即启用则加入调度
		_ = h.sched.Schedule(m.ID, m.HandlerName, m.HandlerParam, m.CronExpression)
	}
	web.Success(c, m.ID)
}

func (h *JobHandler) update(c *gin.Context) {
	var m model.Job
	if !bind(c, &m) {
		return
	}
	if err := h.job.UpdateFields(c.Request.Context(), m.ID, map[string]any{
		"name": m.Name, "handler_name": m.HandlerName, "handler_param": m.HandlerParam,
		"cron_expression": m.CronExpression, "retry_count": m.RetryCount,
		"retry_interval": m.RetryInterval, "monitor_timeout": m.MonitorTimeout,
	}); err != nil {
		web.FailErr(c, err)
		return
	}
	h.reschedule(c, m.ID)
	web.Success(c, true)
}

func (h *JobHandler) updateStatus(c *gin.Context) {
	var req struct {
		ID     int64 `json:"id" form:"id"`
		Status int8  `json:"status" form:"status"`
	}
	_ = c.ShouldBindQuery(&req)
	_ = c.ShouldBindJSON(&req)
	if err := h.job.UpdateFields(c.Request.Context(), req.ID,
		map[string]any{"status": req.Status}); err != nil {
		web.FailErr(c, err)
		return
	}
	h.reschedule(c, req.ID)
	web.Success(c, true)
}

// reschedule 依据任务最新状态重新挂载 / 移除调度。
func (h *JobHandler) reschedule(c *gin.Context, jobID int64) {
	h.sched.Unschedule(jobID)
	if j, _ := h.job.Get(c.Request.Context(), jobID); j != nil && j.Status == 0 {
		_ = h.sched.Schedule(j.ID, j.HandlerName, j.HandlerParam, j.CronExpression)
	}
}

// trigger 立即执行一次任务。
func (h *JobHandler) trigger(c *gin.Context) {
	h.sched.RunNow(qID(c))
	web.Success(c, true)
}

func (h *JobHandler) del(c *gin.Context) {
	id := qID(c)
	h.sched.Unschedule(id)
	respondOK(c, h.job.SoftDelete(c.Request.Context(), []int64{id}))
}

func (h *JobHandler) delList(c *gin.Context) {
	respondOK(c, h.job.SoftDelete(c.Request.Context(), qIDs(c)))
}

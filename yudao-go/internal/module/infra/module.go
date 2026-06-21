// Package infra 是基础设施模块的组合根。
package infra

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	jobsched "yudao-go/internal/framework/job"
	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/redisx"
	"yudao-go/internal/module/infra/model"
	"yudao-go/internal/module/infra/rest"
	sysrepo "yudao-go/internal/module/system/repo"
)

// Module 持有 infra 模块的对外注册入口与定时任务调度器。
type Module struct {
	config     *rest.ConfigHandler
	job        *rest.JobHandler
	jobLog     *rest.JobLogHandler
	dataSource *rest.DataSourceHandler
	file       *rest.FileHandler
	fileConfig *rest.FileConfigHandler
	apiLog     *rest.ApiLogHandler
	monitor    *rest.MonitorHandler
	codegen    *rest.CodegenHandler
	apiAccLog  gin.HandlerFunc
	scheduler  *jobsched.Scheduler
	jobRepo    *sysrepo.CRUD[model.Job]
}

// jobLogSink 把任务执行结果写入 infra_job_log。
type jobLogSink struct {
	logs *sysrepo.CRUD[model.JobLog]
}

func (s *jobLogSink) Record(ctx context.Context, rec jobsched.ExecRecord) {
	status := int8(2) // 成功
	if !rec.Success {
		status = 3 // 失败
	}
	_ = s.logs.Create(ctx, &model.JobLog{
		JobID: rec.JobID, HandlerName: rec.HandlerName, HandlerParam: rec.HandlerParam,
		ExecuteIndex: 1, BeginTime: rec.Begin, EndTime: rec.End,
		Duration: int(rec.End.Sub(rec.Begin).Milliseconds()),
		Status:   status, Result: rec.Result,
	})
}

// New 装配 infra 模块。locker 供定时任务调度器在集群下加分布式锁，
// 单实例可传 nil。
func New(tx *orm.TxManager, locker *redisx.Client) *Module {
	sched := jobsched.NewScheduler(&jobLogSink{logs: sysrepo.NewCRUD[model.JobLog](tx)}, locker)
	registerJobHandlers(sched)
	m := &Module{
		config:     rest.NewConfigHandler(tx),
		job:        rest.NewJobHandler(tx, sched),
		jobLog:     rest.NewJobLogHandler(tx),
		dataSource: rest.NewDataSourceHandler(tx),
		file:       rest.NewFileHandler(tx),
		fileConfig: rest.NewFileConfigHandler(tx),
		apiLog:     rest.NewApiLogHandler(tx),
		codegen:    rest.NewCodegenHandler(tx),
		apiAccLog:  rest.NewAPIAccessLogMiddleware(tx),
		scheduler:  sched,
		jobRepo:    sysrepo.NewCRUD[model.Job](tx),
	}
	// Redis 监控依赖 Redis 客户端；locker 为 nil 时不提供监控接口。
	if locker != nil {
		m.monitor = rest.NewMonitorHandler(locker.Raw(), tx)
	}
	return m
}

// APIAccessLogMiddleware 返回 API 访问日志中间件，挂在需认证的 API 分组上。
func (m *Module) APIAccessLogMiddleware() gin.HandlerFunc { return m.apiAccLog }

// registerJobHandlers 注册内置定时任务处理器。
// 业务任务（payNotifyJob 等）随对应模块迁移时再注册。
func registerJobHandlers(s *jobsched.Scheduler) {
	s.Register("demoJob", func(_ context.Context, param string) error {
		logger.L().Info("demoJob 执行", "param", param)
		return nil
	})
}

// RegisterAuthed 注册需认证路由。
func (m *Module) RegisterAuthed(g *gin.RouterGroup) {
	m.config.Register(g)
	m.job.Register(g)
	m.jobLog.Register(g)
	m.dataSource.Register(g)
	m.file.Register(g)
	m.fileConfig.Register(g)
	m.apiLog.Register(g)
	m.codegen.Register(g)
	if m.monitor != nil {
		m.monitor.Register(g)
	}
}

// RegisterPublic 注册免认证路由（文件内容服务）。
func (m *Module) RegisterPublic(g *gin.RouterGroup) {
	m.file.RegisterPublic(g)
}

// Start 加载启用的定时任务并启动调度器。
func (m *Module) Start() error {
	jobs, err := m.jobRepo.List(context.Background(), func(q *gorm.DB) *gorm.DB {
		return q.Where("status = 0")
	})
	if err != nil {
		return err
	}
	for _, j := range jobs {
		_ = m.scheduler.Schedule(j.ID, j.HandlerName, j.HandlerParam, j.CronExpression)
	}
	m.scheduler.Start()
	logger.L().Info("infra job scheduler started", "scheduled", len(jobs))
	return nil
}

// Stop 停止定时任务调度器。
func (m *Module) Stop() { m.scheduler.Stop() }

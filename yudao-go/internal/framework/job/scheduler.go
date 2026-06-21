// Package job 提供基于 cron 的定时任务调度器。
package job

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/redisx"
)

// jobLockTTL 是定时任务分布式锁的存活时间。锁键按「计划触发时刻」唯一，
// 故 TTL 只需大于集群内的最大时钟偏差即可，到期自动清理。
const jobLockTTL = 2 * time.Minute

// JobFunc 是定时任务处理器。param 为任务配置的处理器参数。
type JobFunc func(ctx context.Context, param string) error

// ExecRecord 是一次任务执行的记录。
type ExecRecord struct {
	JobID        int64
	HandlerName  string
	HandlerParam string
	Begin        time.Time
	End          time.Time
	Success      bool
	Result       string
}

// LogSink 接收任务执行记录（由 infra 模块实现，写入 infra_job_log）。
type LogSink interface {
	Record(ctx context.Context, rec ExecRecord)
}

// Scheduler 是基于 robfig/cron 的定时任务调度器。并发安全。
type Scheduler struct {
	cron   *cron.Cron
	sink   LogSink
	locker *redisx.Client // 分布式锁；nil 表示单实例，不加锁

	mu       sync.RWMutex
	handlers map[string]JobFunc     // handlerName -> 处理器
	entries  map[int64]cron.EntryID // jobID -> cron 条目
	jobs     map[int64]jobMeta      // jobID -> 元信息（供立即执行）
}

type jobMeta struct {
	handler string
	param   string
}

// NewScheduler 创建调度器。locker 用于集群下避免任务重复执行，
// 单实例部署可传 nil。
func NewScheduler(sink LogSink, locker *redisx.Client) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		sink:     sink,
		locker:   locker,
		handlers: make(map[string]JobFunc),
		entries:  make(map[int64]cron.EntryID),
		jobs:     make(map[int64]jobMeta),
	}
}

// Register 注册一个任务处理器。须在 Schedule 之前完成。
func (s *Scheduler) Register(handlerName string, fn JobFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[handlerName] = fn
}

// translateCron 把 Quartz 风格 cron 转为 robfig/cron 6 段格式（秒 分 时 日 月 周）。
func translateCron(expr string) string {
	expr = strings.ReplaceAll(expr, "?", "*")
	if fields := strings.Fields(expr); len(fields) >= 6 {
		return strings.Join(fields[:6], " ") // 丢弃可选的「年」字段
	}
	return expr
}

// Schedule 把一个任务加入调度。处理器未注册则跳过（仅记录告警）。
func (s *Scheduler) Schedule(jobID int64, handlerName, param, cronExpr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn, ok := s.handlers[handlerName]
	if !ok {
		logger.L().Warn("job: handler not registered, skip scheduling",
			"handler", handlerName, "job_id", jobID)
		return nil
	}
	s.jobs[jobID] = jobMeta{handler: handlerName, param: param}
	entryID, err := s.cron.AddFunc(translateCron(cronExpr), func() {
		s.runScheduled(jobID, handlerName, param, fn)
	})
	if err != nil {
		return fmt.Errorf("job: bad cron %q: %w", cronExpr, err)
	}
	s.entries[jobID] = entryID
	return nil
}

// Unschedule 从调度移除一个任务。
func (s *Scheduler) Unschedule(jobID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[jobID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, jobID)
	}
}

// RunNow 立即异步执行一次任务（「执行一次」）。
func (s *Scheduler) RunNow(jobID int64) bool {
	s.mu.RLock()
	meta, ok := s.jobs[jobID]
	fn := s.handlers[meta.handler]
	s.mu.RUnlock()
	if !ok || fn == nil {
		return false
	}
	go s.run(jobID, meta.handler, meta.param, fn)
	return true
}

// runScheduled 是 cron 触发的执行入口：集群下先抢分布式锁，只有一个实例真正执行。
// 「立即执行」(RunNow) 是人工单实例触发，不走此路径、不加锁。
func (s *Scheduler) runScheduled(jobID int64, handler, param string, fn JobFunc) {
	if s.locker == nil {
		s.run(jobID, handler, param, fn)
		return
	}
	// 锁键用本次调度的「计划触发时刻」：由 cron 表达式算出，各实例取值一致，
	// 不受实例间 time.Now() 偏差影响，从而保证同一次触发只被锁定一次。
	fireAt := time.Now().Truncate(time.Second)
	s.mu.RLock()
	entryID, ok := s.entries[jobID]
	s.mu.RUnlock()
	if ok {
		if prev := s.cron.Entry(entryID).Prev; !prev.IsZero() {
			fireAt = prev
		}
	}
	key := fmt.Sprintf("job:cron:%d:%d", jobID, fireAt.Unix())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, acquired, err := s.locker.AcquireLock(ctx, key, jobLockTTL)
	cancel()
	switch {
	case err != nil:
		// 锁服务异常：宁可执行（可能重复）也不漏跑 —— 漏跑通常比重复更严重。
		logger.L().Error("job: 获取分布式锁失败，本实例仍执行", "job_id", jobID, "error", err)
	case !acquired:
		// 已被其它实例抢到，本实例跳过。锁不释放，靠 TTL 过期。
		logger.L().Debug("job: 本次触发已由其它实例执行，跳过", "job_id", jobID)
		return
	}
	s.run(jobID, handler, param, fn)
}

// run 执行任务并把结果交给 LogSink。
func (s *Scheduler) run(jobID int64, handler, param string, fn JobFunc) {
	begin := time.Now()
	ctx := context.Background()
	err := safeRun(ctx, fn, param)
	rec := ExecRecord{
		JobID: jobID, HandlerName: handler, HandlerParam: param,
		Begin: begin, End: time.Now(), Success: err == nil,
	}
	if err != nil {
		rec.Result = err.Error()
		logger.L().Error("job: execution failed", "handler", handler, "error", err)
	}
	if s.sink != nil {
		s.sink.Record(ctx, rec)
	}
}

// safeRun 隔离处理器 panic。
func safeRun(ctx context.Context, fn JobFunc, param string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panic: %v", r)
		}
	}()
	return fn(ctx, param)
}

func (s *Scheduler) Start() { s.cron.Start() }

// Stop 停止调度并等待运行中的任务结束。
func (s *Scheduler) Stop() { <-s.cron.Stop().Done() }

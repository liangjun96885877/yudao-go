package eventbus

import (
	"context"
	"errors"
	"sync"

	"yudao-go/internal/framework/logger"
)

// InProcBus 是进程内事件总线：缓冲队列 + worker 协程池。
// 并发安全：handlers 由 RWMutex 保护；事件经 channel 投递；handler panic 被隔离。
type InProcBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler

	queue   chan task
	workers int

	wg        sync.WaitGroup
	closeOnce sync.Once
	closed    chan struct{} // 关闭信号
}

type task struct {
	ctx context.Context
	e   DomainEvent
}

// NewInProcBus 创建进程内总线。workers 为处理协程数，buffer 为队列容量。
func NewInProcBus(workers, buffer int) *InProcBus {
	if workers <= 0 {
		workers = 4
	}
	if buffer <= 0 {
		buffer = 256
	}
	return &InProcBus{
		handlers: make(map[string][]Handler),
		queue:    make(chan task, buffer),
		workers:  workers,
		closed:   make(chan struct{}),
	}
}

func (b *InProcBus) Subscribe(topic string, h Handler) error {
	if h == nil {
		return errors.New("eventbus: nil handler")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], h)
	return nil
}

func (b *InProcBus) Publish(ctx context.Context, e DomainEvent) error {
	// 脱离请求 context 的取消链：响应返回后异步处理不应被取消，
	// 但保留其中的租户/用户/链路值。
	t := task{ctx: context.WithoutCancel(ctx), e: e}
	select {
	case <-b.closed:
		return errors.New("eventbus: closed")
	case b.queue <- t:
		return nil
	}
}

func (b *InProcBus) Start() error {
	for i := 0; i < b.workers; i++ {
		b.wg.Add(1)
		go b.worker()
	}
	return nil
}

func (b *InProcBus) worker() {
	defer b.wg.Done()
	for {
		select {
		case t := <-b.queue:
			b.dispatch(t)
		case <-b.closed:
			// 关闭信号到达后，排空剩余事件再退出，避免事件丢失。
			for {
				select {
				case t := <-b.queue:
					b.dispatch(t)
				default:
					return
				}
			}
		}
	}
}

func (b *InProcBus) dispatch(t task) {
	b.mu.RLock()
	hs := b.handlers[t.e.Topic()]
	b.mu.RUnlock()

	for _, h := range hs {
		b.safeInvoke(t.ctx, h, t.e)
	}
}

// safeInvoke 隔离单个 handler 的 panic 与 error，避免影响 worker 与其它 handler。
func (b *InProcBus) safeInvoke(ctx context.Context, h Handler, e DomainEvent) {
	defer func() {
		if r := recover(); r != nil {
			logger.WithContext(ctx).Error("eventbus handler panic",
				"topic", e.Topic(), "event_id", e.EventID(), "panic", r)
		}
	}()
	if err := h(ctx, e); err != nil {
		logger.WithContext(ctx).Error("eventbus handler error",
			"topic", e.Topic(), "event_id", e.EventID(), "error", err)
	}
}

func (b *InProcBus) Stop(ctx context.Context) error {
	b.closeOnce.Do(func() { close(b.closed) })

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

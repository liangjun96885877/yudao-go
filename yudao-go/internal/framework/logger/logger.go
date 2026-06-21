// Package logger 提供结构化日志。移植标准：日志统一带 trace_id/tenant_id/user_id。
package logger

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"

	"yudao-go/internal/framework/contextx"
)

// base 以原子指针持有全局 logger，保证 Init 与并发读取安全。
var base atomic.Pointer[slog.Logger]

func init() {
	base.Store(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

// Init 设置全局日志级别。debug=true 时输出 Debug 级别。
func Init(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	base.Store(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

// L 返回全局 logger（无上下文字段）。
func L() *slog.Logger { return base.Load() }

// WithContext 返回带有 trace_id/tenant_id/user_id 字段的 logger。
func WithContext(ctx context.Context) *slog.Logger {
	l := base.Load()
	if ctx == nil {
		return l
	}
	return l.With(
		"trace_id", contextx.TraceID(ctx),
		"tenant_id", contextx.TenantID(ctx),
		"user_id", contextx.UserID(ctx),
	)
}

package web

import (
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"
	oteltrace "go.opentelemetry.io/otel/trace"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/logger"
	"yudao-go/internal/pkg/errcode"
	"yudao-go/internal/pkg/idgen"
)

const traceHeader = "X-Trace-Id"

// CORS 跨域中间件。开发环境回显请求来源；生产环境应收紧为白名单。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
			// 回显预检请求声明的头部，避免自定义头（如 tenant-id）被拦截。
			if reqHeaders := c.GetHeader("Access-Control-Request-Headers"); reqHeaders != "" {
				c.Header("Access-Control-Allow-Headers", reqHeaders)
			} else {
				c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,tenant-id")
			}
			c.Header("Access-Control-Expose-Headers", "X-Trace-Id")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Recovery 捕获 panic，记录堆栈并返回系统异常。横切能力 #2。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithContext(c.Request.Context()).Error("panic recovered",
					"panic", r, "path", c.FullPath(), "stack", string(debug.Stack()))
				if !c.Writer.Written() {
					Fail(c, errcode.InternalError)
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Trace 生成 / 透传链路 ID，写入 context 与响应头。横切能力 #8。
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先用 OTel 的 TraceID，使日志的 trace_id 与 Jaeger 中的链路一致。
		traceID := ""
		if sc := oteltrace.SpanContextFromContext(c.Request.Context()); sc.HasTraceID() {
			traceID = sc.TraceID().String()
		}
		if traceID == "" {
			traceID = c.GetHeader(traceHeader)
		}
		if traceID == "" {
			traceID = idgen.UUID()
		}
		ctx := contextx.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(traceHeader, traceID)
		c.Next()
	}
}

// Tenant 从请求头 tenant-id 解析租户并写入 context。横切能力 #5（Web 侧）。
// 注意：认证中间件会用登录用户的租户覆盖此值，以登录态为准。
func Tenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		if raw := c.GetHeader("tenant-id"); raw != "" {
			if tid, err := strconv.ParseInt(raw, 10, 64); err == nil && tid > 0 {
				ctx := contextx.WithTenantID(c.Request.Context(), tid)
				c.Request = c.Request.WithContext(ctx)
			}
		}
		c.Next()
	}
}

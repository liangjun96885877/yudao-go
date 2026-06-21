package web

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"yudao-go/internal/framework/config"
)

// traceServiceName 是本服务在链路追踪中的名字。
const traceServiceName = "yudao-go"

// NewEngine 创建并配置 Gin 引擎，注册基础中间件链。
// 中间件顺序：CORS → Recovery → OTel(链路追踪) → Trace(TraceID) → Tenant（认证等由路由分组追加）。
func NewEngine(cfg config.Server) *gin.Engine {
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()
	r.Use(CORS(), Recovery(), otelgin.Middleware(traceServiceName), Trace(), Tenant())

	r.GET("/health", func(c *gin.Context) {
		Success(c, gin.H{"status": "up"})
	})
	return r
}

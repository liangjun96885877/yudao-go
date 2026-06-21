package web

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewReverseProxyHandler 返回把请求原样转发到 target 的 Gin handler。
// 用于方案 A：把 /admin-api/bpm/** 反向代理到独立运行的 BPM Java 服务。
func NewReverseProxyHandler(target string) (gin.HandlerFunc, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	// 链路追踪：出站请求生成 client span 并注入 W3C traceparent，
	// 下游 BPM Java 服务（挂 OTel agent）据此续接同一条 trace。
	proxy.Transport = otelhttp.NewTransport(http.DefaultTransport)
	// 剥掉上游服务的 CORS 响应头，避免与 Go 网关的全局 CORS 中间件重复
	// （浏览器拒绝出现多个 Access-Control-Allow-Origin）。
	proxy.ModifyResponse = func(resp *http.Response) error {
		for _, h := range []string{
			"Access-Control-Allow-Origin", "Access-Control-Allow-Methods",
			"Access-Control-Allow-Headers", "Access-Control-Allow-Credentials",
			"Access-Control-Expose-Headers", "Access-Control-Max-Age",
		} {
			resp.Header.Del(h)
		}
		return nil
	}
	// BPM 服务不可用时返回统一的 502 响应，而非裸错误。
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":502,"msg":"BPM 服务不可用，请确认 yudao-server-bpm 已启动"}`))
	}
	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}, nil
}

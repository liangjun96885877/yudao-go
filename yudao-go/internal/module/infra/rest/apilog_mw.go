package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// apiRespCapture 包装 gin.ResponseWriter 以捕获响应体。
type apiRespCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *apiRespCapture) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// NewAPIAccessLogMiddleware 返回 API 日志中间件：
//   - 横切能力 #3：记录每个请求到 infra_api_access_log（异步落库）。
//   - 横切能力 #2：捕获 handler panic，写 infra_api_error_log 并返回统一错误响应。
func NewAPIAccessLogMiddleware(tx *orm.TxManager) gin.HandlerFunc {
	accessLogs := sysrepo.NewCRUD[model.ApiAccessLog](tx)
	errorLogs := sysrepo.NewCRUD[model.ApiErrorLog](tx)
	return func(c *gin.Context) {
		begin := time.Now()
		// 缓存请求体以便记录，并恢复供后续 handler 读取（跳过文件上传）。
		var reqBody []byte
		if c.Request.Body != nil && !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(reqBody))
		}
		cap := &apiRespCapture{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = cap

		defer func() {
			end := time.Now()
			ctx := c.Request.Context()
			params := c.Request.URL.RawQuery
			if len(reqBody) > 0 {
				params += " " + string(reqBody)
			}

			// 捕获 panic：写错误日志 + 返回 500 统一响应。
			rec := recover()
			if rec != nil {
				stack := string(debug.Stack())
				fn, file, line := panicSite(stack)
				logger.L().Error("API 处理发生 panic",
					"path", c.Request.URL.Path, "panic", rec, "at", fmt.Sprintf("%s:%d", file, line))
				errEntry := &model.ApiErrorLog{
					TenantID:                  contextx.TenantID(ctx),
					TraceID:                   contextx.TraceID(ctx),
					UserID:                    contextx.UserID(ctx),
					UserType:                  2, // 管理后台用户
					ApplicationName:           "yudao-go",
					RequestMethod:             c.Request.Method,
					RequestURL:                truncate(c.Request.URL.Path, 255),
					RequestParams:             truncate(params, 4000),
					UserIP:                    c.ClientIP(),
					UserAgent:                 truncate(c.GetHeader("User-Agent"), 500),
					ExceptionTime:             end,
					ExceptionName:             truncate(fmt.Sprintf("%T", rec), 512),
					ExceptionMessage:          truncate(fmt.Sprint(rec), 2000),
					ExceptionRootCauseMessage: truncate(fmt.Sprint(rec), 2000),
					ExceptionStackTrace:       truncate(stack, 8000),
					ExceptionClassName:        truncate(fn, 512),
					ExceptionFileName:         truncate(file, 512),
					ExceptionMethodName:       truncate(fn, 512),
					ExceptionLineNumber:       line,
					ProcessStatus:             0, // 0=未处理
				}
				detachedErr := context.WithoutCancel(ctx)
				go func() { _ = errorLogs.Create(detachedErr, errEntry) }()
				if !c.Writer.Written() {
					web.Fail(c, errcode.InternalError)
				}
			}

			// 访问日志（正常与 panic 两种情况都记录）。
			code, msg := parseResult(cap.body.Bytes())
			if rec != nil {
				code, msg = errcode.InternalError.Code, fmt.Sprint(rec)
			}
			entry := &model.ApiAccessLog{
				TenantID:        contextx.TenantID(ctx),
				TraceID:         contextx.TraceID(ctx),
				UserID:          contextx.UserID(ctx),
				UserType:        2, // 管理后台用户
				ApplicationName: "yudao-go",
				RequestMethod:   c.Request.Method,
				RequestURL:      truncate(c.Request.URL.Path, 255),
				RequestParams:   truncate(params, 4000),
				ResponseBody:    truncate(cap.body.String(), 4000),
				UserIP:          c.ClientIP(),
				UserAgent:       truncate(c.GetHeader("User-Agent"), 500),
				OperateModule:   truncate(strings.TrimPrefix(c.Request.URL.Path, "/admin-api/"), 200),
				OperateType:     operateType(c.Request.Method),
				BeginTime:       begin,
				EndTime:         end,
				Duration:        int(end.Sub(begin).Milliseconds()),
				ResultCode:      code,
				ResultMsg:       truncate(msg, 512),
			}
			detached := context.WithoutCancel(ctx)
			go func() { _ = accessLogs.Create(detached, entry) }()
		}()

		c.Next()
	}
}

// parseResult 从 CommonResult 响应体解析 code 与 msg。
func parseResult(body []byte) (int, string) {
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &r) == nil {
		return r.Code, r.Msg
	}
	return 0, ""
}

// panicSite 从 debug.Stack() 输出中定位 panic 发生的函数、文件与行号。
// 栈格式：紧随 "panic(" 帧之后的两行即为触发 panic 的函数及其源码位置。
func panicSite(stack string) (fn, file string, line int) {
	lines := strings.Split(stack, "\n")
	for i := 0; i+3 < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "panic(") {
			continue
		}
		fn = strings.TrimSpace(lines[i+2])
		if idx := strings.IndexByte(fn, '('); idx >= 0 {
			fn = fn[:idx]
		}
		loc := strings.TrimSpace(lines[i+3])
		if sp := strings.IndexByte(loc, ' '); sp >= 0 {
			loc = loc[:sp]
		}
		if c := strings.LastIndexByte(loc, ':'); c >= 0 {
			file = loc[:c]
			line, _ = strconv.Atoi(loc[c+1:])
		}
		return
	}
	return
}

// operateType 把 HTTP 方法映射为 yudao 操作类型枚举。
func operateType(method string) int8 {
	switch method {
	case "GET":
		return 1 // 查询
	case "POST":
		return 2 // 新增
	case "PUT":
		return 3 // 修改
	case "DELETE":
		return 4 // 删除
	default:
		return 0
	}
}

// truncate 按字符（rune）截断，避免切坏多字节字符。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

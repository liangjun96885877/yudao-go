package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
)

// respCapture 包装 gin.ResponseWriter，捕获响应体以判定业务是否成功。
type respCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *respCapture) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// NewOperateLogMiddleware 返回操作日志中间件：把所有写操作（非 GET）记入 system_operate_log。
// 横切能力 #4：异步落库，不阻塞响应。
func NewOperateLogMiddleware(tx *orm.TxManager) gin.HandlerFunc {
	logs := repo.NewCRUD[model.OperateLog](tx)
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		cap := &respCapture{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = cap
		c.Next()

		typ, sub := deriveOp(c.Request.URL.Path)
		entry := &model.OperateLog{
			TraceID:       contextx.TraceID(c.Request.Context()),
			UserID:        contextx.UserID(c.Request.Context()),
			UserType:      2, // 管理后台用户
			Type:          truncate(typ, 50),
			SubType:       truncate(sub, 50),
			Action:        truncate(c.Request.Method+" "+c.Request.URL.Path, 500),
			Success:       orm.Bit(bizSuccess(cap.body.Bytes())),
			RequestMethod: c.Request.Method,
			RequestURL:    truncate(c.Request.URL.Path, 255),
			UserIP:        c.ClientIP(),
			UserAgent:     truncate(c.GetHeader("User-Agent"), 500),
		}
		entry.TenantID = contextx.TenantID(c.Request.Context())
		// 脱离请求生命周期异步写入。
		ctx := context.WithoutCancel(c.Request.Context())
		go func() { _ = logs.Create(ctx, entry) }()
	}
}

// bizSuccess 解析 CommonResult 的 code 判定业务是否成功。
func bizSuccess(body []byte) bool {
	var r struct {
		Code int `json:"code"`
	}
	return json.Unmarshal(body, &r) == nil && r.Code == 0
}

// deriveOp 从请求路径解析操作类型与动作。
// 例：/admin-api/system/user/create → ("system/user", "create")
func deriveOp(path string) (typ, sub string) {
	segs := strings.Split(strings.Trim(strings.TrimPrefix(path, "/admin-api/"), "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return path, ""
	}
	sub = segs[len(segs)-1]
	if len(segs) > 1 {
		return strings.Join(segs[:len(segs)-1], "/"), sub
	}
	return sub, sub
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

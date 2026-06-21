// Package web 封装 Gin：统一响应、分页、中间件、引擎。
// 移植标准：handler 一律返回 CommonResult，禁止裸写 c.JSON；HTTP 恒 200 + 业务码。
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/pkg/errcode"
	"yudao-go/internal/pkg/mask"
)

// CommonResult 是统一响应体，字段与前端 yudao-ui 契约保持一致，不可改名。
type CommonResult[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// Success 返回成功响应。按请求的字段权限对带 mask 标签的字段脱敏。
func Success[T any](c *gin.Context, data T) {
	out := mask.Process(any(data), contextx.FieldPermOf(c.Request.Context()))
	c.JSON(http.StatusOK, CommonResult[any]{Code: 0, Msg: "成功", Data: out})
}

// Ok 返回无数据的成功响应。
func Ok(c *gin.Context) {
	c.JSON(http.StatusOK, CommonResult[any]{Code: 0, Msg: "成功"})
}

// Fail 返回业务错误响应。
func Fail(c *gin.Context, err *errcode.Error) {
	c.JSON(http.StatusOK, CommonResult[any]{Code: err.Code, Msg: err.Msg})
}

// FailErr 将任意 error 转换为响应：*errcode.Error 原样返回，其余归为系统异常。
func FailErr(c *gin.Context, err error) {
	if e, ok := errcode.As(err); ok {
		Fail(c, e)
		return
	}
	Fail(c, errcode.InternalError)
}

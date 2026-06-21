// Package errcode 定义统一错误码体系。移植标准：Service 返回 *Error；错误码分段沿用 Java 约定。
package errcode

import (
	"errors"
	"fmt"
)

// Error 业务错误，携带错误码与提示信息。
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Msg) }

// New 创建错误码。
func New(code int, msg string) *Error { return &Error{Code: code, Msg: msg} }

// WithMsg 返回保留错误码、替换提示信息的副本（不修改原对象，并发安全）。
func (e *Error) WithMsg(msg string) *Error { return &Error{Code: e.Code, Msg: msg} }

// WithMsgf 同 WithMsg，支持格式化。
func (e *Error) WithMsgf(format string, args ...any) *Error {
	return &Error{Code: e.Code, Msg: fmt.Sprintf(format, args...)}
}

// As 从普通 error 中提取 *Error。
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// 全局错误码（与 yudao Java 端 GlobalErrorCodeConstants 对齐）。
var (
	Success         = New(0, "成功")
	BadRequest      = New(400, "请求参数不正确")
	Unauthorized    = New(401, "账号未登录")
	Forbidden       = New(403, "没有该操作权限")
	NotFound        = New(404, "请求未找到")
	MethodNotAllow  = New(405, "请求方法不正确")
	TooManyRequests = New(429, "请求过于频繁，请稍后重试")
	Conflict        = New(409, "数据已被修改，请刷新后重试") // 乐观锁冲突
	InternalError   = New(500, "系统异常")
	RepeatedRequest = New(900, "重复请求，请稍后重试")
	DemoDeny        = New(901, "演示模式，禁止写操作")
)

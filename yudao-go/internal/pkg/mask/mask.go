// Package mask 提供字段级数据脱敏（横切能力 #14）。
// 用法：结构体字段打 `mask:"bizType:field:kind"` 标签；web.Success 按请求的
// 字段权限（contextx.FieldPerm）决定该字段输出 明文 / 打码 / 占位符。
package mask

import (
	"reflect"
	"strings"
	"sync"

	"yudao-go/internal/framework/contextx"
)

// placeholder 是 hide 动作的占位符。
const placeholder = "***"

// By 按脱敏类型对单个值打码。
func By(kind, v string) string {
	if v == "" {
		return v
	}
	switch kind {
	case "mobile":
		return keep(v, 3, 4)
	case "email":
		at := strings.IndexByte(v, '@')
		if at <= 0 {
			return keep(v, 1, 0)
		}
		return keep(v[:at], 1, 0) + v[at:]
	case "idcard":
		return keep(v, 6, 4)
	case "bankcard":
		return keep(v, 0, 4)
	case "name":
		r := []rune(v)
		if len(r) <= 1 {
			return v
		}
		return string(r[:1]) + strings.Repeat("*", len(r)-1)
	case "secret":
		return "******"
	default:
		return keep(v, 1, 1)
	}
}

func keep(v string, head, tail int) string {
	r := []rune(v)
	if len(r) <= head+tail {
		if len(r) <= 1 {
			return v
		}
		return string(r[:1]) + strings.Repeat("*", len(r)-1)
	}
	return string(r[:head]) + strings.Repeat("*", len(r)-head-tail) + string(r[len(r)-tail:])
}

// Process 返回 v 的脱敏副本：按 fp 对带 mask 标签的字段执行 明文/打码/占位符。
// 无 mask 标签的类型原样返回；fp 为 nil 时按安全默认（打码）处理。
func Process(v any, fp *contextx.FieldPerm) any {
	if v == nil {
		return nil
	}
	if fp != nil && fp.All {
		return v // 超级管理员：全部明文，免反射
	}
	rv := reflect.ValueOf(v)
	if !maskable(rv.Type()) {
		return v
	}
	return processValue(rv, fp).Interface()
}

func processValue(rv reflect.Value, fp *contextx.FieldPerm) reflect.Value {
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return rv
		}
		out := reflect.New(rv.Type().Elem())
		out.Elem().Set(processValue(rv.Elem(), fp))
		return out
	case reflect.Slice:
		if rv.IsNil() {
			return rv
		}
		out := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out.Index(i).Set(processValue(rv.Index(i), fp))
		}
		return out
	case reflect.Map:
		if rv.IsNil() {
			return rv
		}
		out := reflect.MakeMap(rv.Type())
		it := rv.MapRange()
		for it.Next() {
			out.SetMapIndex(it.Key(), processValue(it.Value(), fp))
		}
		return out
	case reflect.Interface:
		if rv.IsNil() {
			return rv
		}
		out := reflect.New(rv.Type()).Elem()
		out.Set(processValue(rv.Elem(), fp))
		return out
	case reflect.Struct:
		out := reflect.New(rv.Type()).Elem()
		out.Set(rv)
		t := rv.Type()
		for i := 0; i < t.NumField(); i++ {
			fv := out.Field(i)
			if !fv.CanSet() {
				continue
			}
			if tag := t.Field(i).Tag.Get("mask"); tag != "" && fv.Kind() == reflect.String {
				fv.SetString(applyTag(tag, fv.String(), fp))
				continue
			}
			fv.Set(processValue(rv.Field(i), fp))
		}
		return out
	default:
		return rv
	}
}

// applyTag 解析 `bizType:field:kind` 标签并按字段权限处理值。
func applyTag(tag, value string, fp *contextx.FieldPerm) string {
	parts := strings.SplitN(tag, ":", 3)
	if len(parts) != 3 {
		return By(tag, value) // 兜底：整段当作 kind
	}
	bizType, field, kind := parts[0], parts[1], parts[2]
	switch fp.Action(bizType, field) {
	case "plain":
		return value
	case "hide":
		if value == "" {
			return value
		}
		return placeholder
	default: // mask
		return By(kind, value)
	}
}

var maskableCache sync.Map // reflect.Type -> bool

func maskable(t reflect.Type) bool {
	if v, ok := maskableCache.Load(t); ok {
		return v.(bool)
	}
	r := computeMaskable(t, map[reflect.Type]bool{})
	maskableCache.Store(t, r)
	return r
}

func computeMaskable(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return computeMaskable(t.Elem(), seen)
	case reflect.Interface:
		return true
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Tag.Get("mask") != "" {
				return true
			}
			if computeMaskable(f.Type, seen) {
				return true
			}
		}
	}
	return false
}

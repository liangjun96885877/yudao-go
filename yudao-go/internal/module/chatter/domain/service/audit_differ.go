// Package service 是 chatter 领域服务层：承载不属于单个聚合的纯业务逻辑。
package service

import (
	"fmt"
	"time"

	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/registry"
)

// AuditDiffer 比较业务记录修改前后的字段值，产出字段变更列表。
type AuditDiffer struct{}

// Diff 按注册的审计字段比对新旧值。
//   - fields：该业务类型登记的审计字段；
//   - oldVals/newVals：列名 → 值；newVals 中不存在的列视为本次未更新。
func (AuditDiffer) Diff(fields []registry.AuditField, oldVals, newVals map[string]any) []model.FieldChange {
	changes := make([]model.FieldChange, 0)
	for _, f := range fields {
		nv, updated := newVals[f.Column]
		if !updated {
			continue // 本次未更新该字段
		}
		oldStr := stringify(oldVals[f.Column])
		newStr := stringify(nv)
		if oldStr == newStr {
			continue // 值未变化
		}
		changes = append(changes, model.FieldChange{
			Field:     f.Field,
			Label:     f.Label,
			OldValue:  oldStr,
			NewValue:  newStr,
			ValueType: model.ValueType(f.Type),
		})
	}
	return changes
}

// stringify 将任意值转为可比较、可存储的字符串表示。
func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		return x.Format("2006-01-02 15:04:05")
	case *time.Time:
		if x == nil {
			return ""
		}
		return x.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

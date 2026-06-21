package orm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StrArray 映射「以 JSON 数组字符串存储的 varchar 列」（原版 yudao 用 TypeHandler 实现）。
// 读写时自动在 []string 与 JSON 字符串之间转换，JSON 序列化按数组输出。
type StrArray []string

func (a *StrArray) Scan(v any) error {
	if v == nil {
		*a = StrArray{}
		return nil
	}
	var b []byte
	switch t := v.(type) {
	case []byte:
		b = t
	case string:
		b = []byte(t)
	default:
		return fmt.Errorf("orm.StrArray: 不支持的类型 %T", v)
	}
	if len(b) == 0 {
		*a = StrArray{}
		return nil
	}
	return json.Unmarshal(b, (*[]string)(a))
}

func (a StrArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(a))
	return string(b), err
}

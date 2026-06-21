package orm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Int64Array 映射「以 JSON 数组字符串存储的 varchar 列」的 int64 数组。
// 读写时自动在 []int64 与 JSON 字符串间转换，JSON 序列化按数组输出。
type Int64Array []int64

func (a *Int64Array) Scan(v any) error {
	if v == nil {
		*a = Int64Array{}
		return nil
	}
	var b []byte
	switch t := v.(type) {
	case []byte:
		b = t
	case string:
		b = []byte(t)
	default:
		return fmt.Errorf("orm.Int64Array: 不支持的类型 %T", v)
	}
	if len(b) == 0 {
		*a = Int64Array{}
		return nil
	}
	return json.Unmarshal(b, (*[]int64)(a))
}

func (a Int64Array) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]int64(a))
	return string(b), err
}

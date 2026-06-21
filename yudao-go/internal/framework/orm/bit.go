package orm

import "database/sql/driver"

// Bit 映射 MySQL bit(1) 列（原版 yudao 用 bit(1) 表示布尔）。
type Bit bool

func (b *Bit) Scan(v any) error {
	switch x := v.(type) {
	case []byte:
		*b = Bit(len(x) > 0 && x[0] == 1)
	case int64:
		*b = Bit(x == 1)
	case bool:
		*b = Bit(x)
	default:
		*b = false
	}
	return nil
}

func (b Bit) Value() (driver.Value, error) {
	if b {
		return []byte{1}, nil
	}
	return []byte{0}, nil
}

func (b Bit) Bool() bool { return bool(b) }

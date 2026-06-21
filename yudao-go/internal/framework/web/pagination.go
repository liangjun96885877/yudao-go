package web

// PageSizeNone 表示不分页，查询全部数据。
const PageSizeNone = -1

const (
	defaultPageNo   = 1
	defaultPageSize = 10
	maxPageSize     = 200
)

// PageParam 是分页请求参数，沿用 yudao 的 pageNo/pageSize 命名。
type PageParam struct {
	PageNo   int `form:"pageNo" json:"pageNo"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

// Normalize 归一化分页参数：补默认值并约束上限。返回副本，不修改原值。
func (p PageParam) Normalize() PageParam {
	out := p
	if out.PageNo <= 0 {
		out.PageNo = defaultPageNo
	}
	if out.PageSize == PageSizeNone {
		return out
	}
	if out.PageSize <= 0 {
		out.PageSize = defaultPageSize
	}
	if out.PageSize > maxPageSize {
		out.PageSize = maxPageSize
	}
	return out
}

// Offset 返回 SQL OFFSET。
func (p PageParam) Offset() int {
	n := p.Normalize()
	return (n.PageNo - 1) * n.PageSize
}

// Limit 返回 SQL LIMIT。
func (p PageParam) Limit() int { return p.Normalize().PageSize }

// NoPaging 是否查询全部。
func (p PageParam) NoPaging() bool { return p.PageSize == PageSizeNone }

// PageResult 是分页结果。
type PageResult[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

// NewPageResult 构造分页结果，list 为 nil 时返回空切片避免前端拿到 null。
func NewPageResult[T any](list []T, total int64) PageResult[T] {
	if list == nil {
		list = []T{}
	}
	return PageResult[T]{List: list, Total: total}
}

// Package registry 提供业务类型注册表：让任意业务模块以零侵入方式接入 chatter。
// 业务模块在启动时注册自己的 BizType，审计拦截器与权限校验据此工作。
package registry

import "sync"

// AuditField 描述一个需要审计的字段。
type AuditField struct {
	Column string // 数据库列名
	Field  string // Go 结构体字段名
	Label  string // 展示名，如 "用户昵称"
	Type   string // 值类型：string/int/date/enum/ref
}

// BizType 描述一个接入 chatter 的业务类型。
type BizType struct {
	Type         string       // 唯一标识，如 system_user
	DisplayName  string       // 展示名，如 "用户"
	Table        string       // 数据库表名，审计拦截器据此匹配模型
	AuditFields  []AuditField // 需审计的字段列表
	PermResource string       // 权限资源前缀，如 system:user
}

// auditFieldByColumn 返回按列名索引的审计字段表（构建一次，只读使用）。
func (b *BizType) auditFieldByColumn() map[string]AuditField {
	m := make(map[string]AuditField, len(b.AuditFields))
	for _, f := range b.AuditFields {
		m[f.Column] = f
	}
	return m
}

// Registry 是并发安全的业务类型注册表。
// 注册通常发生在启动期，查询发生在运行期；用 RWMutex 保证安全。
type Registry struct {
	mu      sync.RWMutex
	byType  map[string]*BizType
	byTable map[string]*BizType
}

// New 创建空注册表。
func New() *Registry {
	return &Registry{
		byType:  make(map[string]*BizType),
		byTable: make(map[string]*BizType),
	}
}

// Register 注册一个业务类型。重复 Type 将覆盖。
func (r *Registry) Register(bt BizType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := bt // 拷贝，避免外部持有引用后修改
	r.byType[cp.Type] = &cp
	if cp.Table != "" {
		r.byTable[cp.Table] = &cp
	}
}

// Lookup 按业务类型查找。
func (r *Registry) Lookup(bizType string) (*BizType, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bt, ok := r.byType[bizType]
	return bt, ok
}

// LookupByTable 按数据库表名查找（供 GORM 审计拦截器使用）。
func (r *Registry) LookupByTable(table string) (*BizType, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bt, ok := r.byTable[table]
	return bt, ok
}

// AuditFieldOf 返回某业务类型某列的审计字段定义。
func (r *Registry) AuditFieldOf(bizType, column string) (AuditField, bool) {
	bt, ok := r.Lookup(bizType)
	if !ok {
		return AuditField{}, false
	}
	f, ok := bt.auditFieldByColumn()[column]
	return f, ok
}

// All 返回所有已注册业务类型的快照。
func (r *Registry) All() []*BizType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*BizType, 0, len(r.byType))
	for _, bt := range r.byType {
		out = append(out, bt)
	}
	return out
}

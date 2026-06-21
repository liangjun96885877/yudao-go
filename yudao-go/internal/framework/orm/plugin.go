package orm

import (
	"reflect"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"yudao-go/internal/framework/contextx"
)

// RegisterTenantPlugin 注册多租户回调：
//   - 写入（Create）：自动填充 tenant_id；
//   - 查询/更新/删除：自动追加 tenant_id = ? 过滤条件。
// 仅对存在 TenantID 字段的模型生效；context 标记 IgnoreTenant 或租户为 0 时跳过。
func RegisterTenantPlugin(db *gorm.DB) error {
	cb := db.Callback()
	if err := cb.Create().Before("gorm:create").Register("tenant:create", tenantOnCreate); err != nil {
		return err
	}
	if err := cb.Query().Before("gorm:query").Register("tenant:query", tenantOnQuery); err != nil {
		return err
	}
	if err := cb.Update().Before("gorm:update").Register("tenant:update", tenantOnQuery); err != nil {
		return err
	}
	if err := cb.Delete().Before("gorm:delete").Register("tenant:delete", tenantOnQuery); err != nil {
		return err
	}
	return nil
}

// tenantField 返回模型的 TenantID 字段；不存在或上下文忽略时返回 (nil, 0, false)。
func tenantField(db *gorm.DB) (*schema.Field, int64, bool) {
	if db.Statement.Schema == nil {
		return nil, 0, false
	}
	field := db.Statement.Schema.LookUpField("TenantID")
	if field == nil {
		return nil, 0, false
	}
	ctx := db.Statement.Context
	if contextx.IgnoreTenant(ctx) {
		return nil, 0, false
	}
	tid := contextx.TenantID(ctx)
	if tid == 0 { // 无租户上下文，跳过（避免误把全部数据归到 0 号租户）
		return nil, 0, false
	}
	return field, tid, true
}

func tenantOnCreate(db *gorm.DB) {
	field, tid, ok := tenantField(db)
	if !ok {
		return
	}
	rv := db.Statement.ReflectValue
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			_ = field.Set(db.Statement.Context, rv.Index(i), tid)
		}
	case reflect.Struct:
		_ = field.Set(db.Statement.Context, rv, tid)
	}
}

func tenantOnQuery(db *gorm.DB) {
	field, tid, ok := tenantField(db)
	if !ok {
		return
	}
	// 追加 WHERE tenant_id = ?，与已有条件以 AND 合并。
	db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
		clause.Eq{Column: clause.Column{Name: field.DBName}, Value: tid},
	}})
}

// RegisterAuditFillPlugin 注册审计字段自动填充：
//   - Create：填充 creator 与 updater；
//   - Update：填充 updater。
// 取值来源：context 中的用户名，缺省回退到用户 ID。
func RegisterAuditFillPlugin(db *gorm.DB) error {
	cb := db.Callback()
	if err := cb.Create().Before("gorm:create").Register("audit:create", auditOnCreate); err != nil {
		return err
	}
	return cb.Update().Before("gorm:update").Register("audit:update", auditOnUpdate)
}

func currentOperator(db *gorm.DB) string {
	ctx := db.Statement.Context
	if name := contextx.UserName(ctx); name != "" {
		return name
	}
	if id := contextx.UserID(ctx); id != 0 {
		return strconv.FormatInt(id, 10)
	}
	return ""
}

func auditOnCreate(db *gorm.DB) {
	if db.Statement.Schema == nil {
		return
	}
	operator := currentOperator(db)
	if operator == "" {
		return
	}
	for _, name := range []string{"Creator", "Updater"} {
		field := db.Statement.Schema.LookUpField(name)
		if field == nil {
			continue
		}
		rv := db.Statement.ReflectValue
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < rv.Len(); i++ {
				_ = field.Set(db.Statement.Context, rv.Index(i), operator)
			}
		case reflect.Struct:
			_ = field.Set(db.Statement.Context, rv, operator)
		}
	}
}

func auditOnUpdate(db *gorm.DB) {
	if db.Statement.Schema == nil || db.Statement.Schema.LookUpField("Updater") == nil {
		return
	}
	if operator := currentOperator(db); operator != "" {
		// SetColumn 对结构体更新与 map 更新均生效。
		db.Statement.SetColumn("updater", operator)
	}
}

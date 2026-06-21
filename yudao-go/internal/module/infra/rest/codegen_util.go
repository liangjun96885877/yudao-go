package rest

import "strings"

// camelCase 把 snake_case 转为 camelCase：user_name -> userName。
func camelCase(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			b.WriteString(strings.ToLower(p))
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
		}
	}
	return b.String()
}

// pascalCase 把 snake_case 转为 PascalCase：user_name -> UserName。
func pascalCase(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
	}
	return b.String()
}

// splitTableName 把表名拆为 模块名 / 业务名 / 类名。
// 例：biz_product -> ("biz", "product", "Product")；system_user_role -> ("system", "userRole", "UserRole")。
func splitTableName(tableName string) (module, business, class string) {
	idx := strings.IndexByte(tableName, '_')
	if idx < 0 {
		return "", camelCase(tableName), pascalCase(tableName)
	}
	module = tableName[:idx]
	rest := tableName[idx+1:]
	return module, camelCase(rest), pascalCase(rest)
}

// goTypeOf 把 MySQL 数据类型映射为 Go 类型。
func goTypeOf(dataType string) string {
	switch strings.ToLower(dataType) {
	case "bigint":
		return "int64"
	case "int", "integer", "mediumint", "smallint", "tinyint":
		return "int"
	case "bit":
		return "bool"
	case "decimal", "double", "float":
		return "float64"
	case "datetime", "timestamp", "date", "time":
		return "time.Time"
	default:
		return "string"
	}
}

// conditionOf 给出列的默认查询方式。
func conditionOf(dataType string) string {
	switch strings.ToLower(dataType) {
	case "varchar", "char", "text", "tinytext", "mediumtext", "longtext":
		return "LIKE"
	default:
		return "="
	}
}

// htmlTypeOf 给出列的默认表单控件类型。
func htmlTypeOf(dataType string) string {
	switch strings.ToLower(dataType) {
	case "datetime", "timestamp", "date", "time":
		return "datetime"
	case "text", "mediumtext", "longtext":
		return "textarea"
	default:
		return "input"
	}
}

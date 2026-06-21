package orm

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yudao-go/internal/framework/contextx"
)

// RegisterDataPermPlugin 注册数据权限回调（横切能力 #6）：
// 查询/更新/删除含 dept_id 列的表时，按当前用户的数据范围追加 WHERE。
// context 标记 IgnoreDataPerm、或数据权限为「全部」、或无数据权限上下文时跳过。
func RegisterDataPermPlugin(db *gorm.DB) error {
	cb := db.Callback()
	if err := cb.Query().Before("gorm:query").Register("dataperm:query", dataPermFilter); err != nil {
		return err
	}
	if err := cb.Update().Before("gorm:update").Register("dataperm:update", dataPermFilter); err != nil {
		return err
	}
	return cb.Delete().Before("gorm:delete").Register("dataperm:delete", dataPermFilter)
}

func dataPermFilter(db *gorm.DB) {
	if db.Statement.Schema == nil {
		return
	}
	ctx := db.Statement.Context
	if contextx.IgnoreDataPerm(ctx) {
		return
	}
	dp := contextx.DataPermOf(ctx)
	if dp == nil || dp.All {
		return // 无数据权限上下文 或 可见全部 → 不过滤
	}
	deptField := db.Statement.Schema.LookUpField("dept_id")
	if deptField == nil {
		return // 表无 dept_id 列 → 数据权限规则不适用
	}
	var conds []string
	var vars []any
	if len(dp.DeptIDs) > 0 {
		conds = append(conds, deptField.DBName+" IN ?")
		vars = append(vars, dp.DeptIDs)
	}
	if dp.SelfOnly && db.Statement.Schema.LookUpField("creator") != nil {
		conds = append(conds, "creator = ?")
		vars = append(vars, strconv.FormatInt(dp.UserID, 10))
	}
	var expr clause.Expression
	if len(conds) == 0 {
		expr = clause.Expr{SQL: "1 = 0"} // 无任何可见范围 → 查不到数据
	} else {
		expr = clause.Expr{SQL: "(" + strings.Join(conds, " OR ") + ")", Vars: vars}
	}
	db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{expr}})
}

package rest

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	sysrepo "yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// CodegenHandler 提供代码生成器接口：导入数据库表、配置字段、预览、下载。
type CodegenHandler struct {
	tables  *sysrepo.CRUD[model.CodegenTable]
	columns *sysrepo.CRUD[model.CodegenColumn]
	tx      *orm.TxManager
}

func NewCodegenHandler(tx *orm.TxManager) *CodegenHandler {
	return &CodegenHandler{
		tables:  sysrepo.NewCRUD[model.CodegenTable](tx),
		columns: sysrepo.NewCRUD[model.CodegenColumn](tx),
		tx:      tx,
	}
}

func (h *CodegenHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/codegen/db/table/list", h.dbTableList)
	g.GET("/infra/codegen/table/list", h.tableList)
	g.GET("/infra/codegen/table/page", h.tablePage)
	g.GET("/infra/codegen/detail", h.detail)
	g.POST("/infra/codegen/create-list", h.createList)
	g.PUT("/infra/codegen/update", h.update)
	g.GET("/infra/codegen/sync-from-db", h.syncFromDB)
	g.GET("/infra/codegen/preview", h.preview)
	g.GET("/infra/codegen/download", h.download)
	g.DELETE("/infra/codegen/delete", h.del)
	g.DELETE("/infra/codegen/delete-list", h.delList)
}

// baseColumns 是 yudao 基类列，代码生成时排除出业务字段（自动管理）。
var baseColumns = map[string]bool{
	"id": true, "creator": true, "create_time": true,
	"updater": true, "update_time": true, "deleted": true, "tenant_id": true,
}

// ===== 数据库表 =====

// schemaColumn 是 information_schema 读出的原始列信息。
type schemaColumn struct {
	ColumnName    string `gorm:"column:COLUMN_NAME"`
	DataType      string `gorm:"column:DATA_TYPE"`
	ColumnComment string `gorm:"column:COLUMN_COMMENT"`
	IsNullable    string `gorm:"column:IS_NULLABLE"`
	ColumnKey     string `gorm:"column:COLUMN_KEY"`
	OrdinalPos    int    `gorm:"column:ORDINAL_POSITION"`
}

// dbTableList 列出当前数据库中尚未导入的表。
func (h *CodegenHandler) dbTableList(c *gin.Context) {
	ctx := c.Request.Context()
	db := h.tx.DB(ctx)
	// 已导入的表名集合
	imported := map[string]bool{}
	var existed []model.CodegenTable
	db.Model(&model.CodegenTable{}).Find(&existed)
	for _, t := range existed {
		imported[t.Name] = true
	}
	var rows []struct {
		TableName    string `gorm:"column:TABLE_NAME"`
		TableComment string `gorm:"column:TABLE_COMMENT"`
	}
	db.Raw(`SELECT TABLE_NAME, TABLE_COMMENT FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`).Scan(&rows)
	out := make([]gin.H, 0)
	for _, r := range rows {
		if imported[r.TableName] || strings.HasPrefix(r.TableName, "ACT_") {
			continue
		}
		out = append(out, gin.H{"name": r.TableName, "comment": r.TableComment})
	}
	web.Success(c, out)
}

// ===== 代码生成表配置 =====

func (h *CodegenHandler) tableList(c *gin.Context) {
	list, err := h.tables.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Order("id ASC")
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, list)
}

func (h *CodegenHandler) tablePage(c *gin.Context) {
	var p web.PageParam
	_ = c.ShouldBindQuery(&p)
	tableName, tableComment := c.Query("tableName"), c.Query("tableComment")
	list, total, err := h.tables.Page(c.Request.Context(), p.Offset(), p.Limit(),
		func(q *gorm.DB) *gorm.DB {
			q = likeIf(q, "table_name", tableName)
			q = likeIf(q, "table_comment", tableComment)
			return q.Order("id DESC")
		})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, web.NewPageResult(list, total))
}

// codegenDetailVO 是表 + 字段配置的组合返回。
type codegenDetailVO struct {
	Table   *model.CodegenTable   `json:"table"`
	Columns []*model.CodegenColumn `json:"columns"`
}

func (h *CodegenHandler) detail(c *gin.Context) {
	tableID := qInt64(c, "tableId")
	t, err := h.tables.Get(c.Request.Context(), tableID)
	if err != nil {
		web.FailErr(c, err)
		return
	}
	if t == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	cols, err := h.columns.List(c.Request.Context(), func(q *gorm.DB) *gorm.DB {
		return q.Where("table_id = ?", tableID).Order("ordinal_position ASC")
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, &codegenDetailVO{Table: t, Columns: cols})
}

// createList 导入选中的数据库表：读取表结构，生成表与字段配置。
func (h *CodegenHandler) createList(c *gin.Context) {
	var req struct {
		DataSourceConfigID int64    `json:"dataSourceConfigId"`
		TableNames         []string `json:"tableNames"`
	}
	if !bind(c, &req) {
		return
	}
	ids := make([]int64, 0, len(req.TableNames))
	err := h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		for _, name := range req.TableNames {
			id, err := h.importTable(ctx, req.DataSourceConfigID, name)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, ids)
}

// importTable 读取一张数据库表的结构，落库为代码生成的表 + 字段配置。
func (h *CodegenHandler) importTable(ctx context.Context, dsID int64, tableName string) (int64, error) {
	db := h.tx.DB(ctx)
	var comment string
	db.Raw(`SELECT TABLE_COMMENT FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, tableName).Scan(&comment)

	module, business, class := splitTableName(tableName)
	t := &model.CodegenTable{
		DataSourceConfigID: dsID, Scene: 1, Name: tableName, TableComment: comment,
		ModuleName: module, BusinessName: business, ClassName: class,
		ClassComment: strings.TrimSuffix(comment, "表"), Author: "yudao",
		TemplateType: 1, FrontType: 30, // 1=单表，30=Vue3
	}
	if err := h.tables.Create(ctx, t); err != nil {
		return 0, err
	}
	var cols []schemaColumn
	db.Raw(`SELECT COLUMN_NAME, DATA_TYPE, COLUMN_COMMENT, IS_NULLABLE, COLUMN_KEY, ORDINAL_POSITION
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, tableName).Scan(&cols)
	for _, sc := range cols {
		pk := sc.ColumnKey == "PRI"
		isBase := baseColumns[sc.ColumnName]
		showInList := !isBase || sc.ColumnName == "id" || sc.ColumnName == "create_time"
		mc := &model.CodegenColumn{
			TableID: t.ID, ColumnName: sc.ColumnName, DataType: sc.DataType,
			ColumnComment: sc.ColumnComment, Nullable: orm.Bit(sc.IsNullable == "YES"),
			PrimaryKey: orm.Bit(pk), OrdinalPosition: sc.OrdinalPos,
			JavaType: goTypeOf(sc.DataType), JavaField: camelCase(sc.ColumnName),
			CreateOperation:        orm.Bit(!isBase && !pk),
			UpdateOperation:        orm.Bit(!isBase && !pk),
			ListOperation:          orm.Bit(false),
			ListOperationCondition: conditionOf(sc.DataType),
			ListOperationResult:    orm.Bit(showInList),
			HtmlType:               htmlTypeOf(sc.DataType),
		}
		if err := h.columns.Create(ctx, mc); err != nil {
			return 0, err
		}
	}
	return t.ID, nil
}

// update 保存表与字段的代码生成配置。
func (h *CodegenHandler) update(c *gin.Context) {
	var req codegenDetailVO
	if !bind(c, &req) {
		return
	}
	if req.Table == nil {
		web.Fail(c, errcode.BadRequest)
		return
	}
	err := h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		t := req.Table
		if err := h.tables.UpdateFields(ctx, t.ID, map[string]any{
			"scene": t.Scene, "table_comment": t.TableComment, "remark": t.Remark,
			"module_name": t.ModuleName, "business_name": t.BusinessName,
			"class_name": t.ClassName, "class_comment": t.ClassComment, "author": t.Author,
			"template_type": t.TemplateType, "front_type": t.FrontType,
			"parent_menu_id": t.ParentMenuID, "master_table_id": t.MasterTableID,
			"sub_join_column_id": t.SubJoinColumnID, "sub_join_many": t.SubJoinMany,
			"tree_parent_column_id": t.TreeParentColumnID, "tree_name_column_id": t.TreeNameColumnID,
		}); err != nil {
			return err
		}
		for _, col := range req.Columns {
			if err := h.columns.UpdateFields(ctx, col.ID, map[string]any{
				"column_comment": col.ColumnComment, "java_type": col.JavaType,
				"java_field": col.JavaField, "dict_type": col.DictType, "example": col.Example,
				"create_operation": col.CreateOperation, "update_operation": col.UpdateOperation,
				"list_operation": col.ListOperation, "list_operation_condition": col.ListOperationCondition,
				"list_operation_result": col.ListOperationResult, "html_type": col.HtmlType,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	respondOK(c, err)
}

// syncFromDB 按数据库表结构重新同步字段（保留已有配置无法自动合并，简化为重建）。
func (h *CodegenHandler) syncFromDB(c *gin.Context) {
	tableID := qInt64(c, "tableId")
	t, err := h.tables.Get(c.Request.Context(), tableID)
	if err != nil || t == nil {
		web.Fail(c, errcode.NotFound)
		return
	}
	err = h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		old, _ := h.columns.List(ctx, func(q *gorm.DB) *gorm.DB {
			return q.Where("table_id = ?", tableID)
		})
		ids := make([]int64, 0, len(old))
		for _, o := range old {
			ids = append(ids, o.ID)
		}
		if len(ids) > 0 {
			if err := h.columns.SoftDelete(ctx, ids); err != nil {
				return err
			}
		}
		db := h.tx.DB(ctx)
		var cols []schemaColumn
		db.Raw(`SELECT COLUMN_NAME, DATA_TYPE, COLUMN_COMMENT, IS_NULLABLE, COLUMN_KEY, ORDINAL_POSITION
			FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION`, t.Name).Scan(&cols)
		for _, sc := range cols {
			pk := sc.ColumnKey == "PRI"
			isBase := baseColumns[sc.ColumnName]
			if err := h.columns.Create(ctx, &model.CodegenColumn{
				TableID: tableID, ColumnName: sc.ColumnName, DataType: sc.DataType,
				ColumnComment: sc.ColumnComment, Nullable: orm.Bit(sc.IsNullable == "YES"),
				PrimaryKey: orm.Bit(pk), OrdinalPosition: sc.OrdinalPos,
				JavaType: goTypeOf(sc.DataType), JavaField: camelCase(sc.ColumnName),
				CreateOperation: orm.Bit(!isBase && !pk), UpdateOperation: orm.Bit(!isBase && !pk),
				ListOperationCondition: conditionOf(sc.DataType),
				ListOperationResult:    orm.Bit(!isBase || sc.ColumnName == "id"),
				HtmlType:               htmlTypeOf(sc.DataType),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	respondOK(c, err)
}

func (h *CodegenHandler) del(c *gin.Context) {
	tableID := qInt64(c, "tableId")
	respondOK(c, h.deleteTables(c, []int64{tableID}))
}

func (h *CodegenHandler) delList(c *gin.Context) {
	respondOK(c, h.deleteTables(c, qIDs(c)))
}

// deleteTables 删除表配置及其字段配置。
func (h *CodegenHandler) deleteTables(c *gin.Context, ids []int64) error {
	return h.tx.Do(c.Request.Context(), func(ctx context.Context) error {
		if err := h.tables.SoftDelete(ctx, ids); err != nil {
			return err
		}
		cols, _ := h.columns.List(ctx, func(q *gorm.DB) *gorm.DB {
			return q.Where("table_id IN ?", ids)
		})
		colIDs := make([]int64, 0, len(cols))
		for _, col := range cols {
			colIDs = append(colIDs, col.ID)
		}
		if len(colIDs) == 0 {
			return nil
		}
		return h.columns.SoftDelete(ctx, colIDs)
	})
}

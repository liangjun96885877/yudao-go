package rest

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"fmt"
	"go/format"
	"net/http"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yudao-go/internal/framework/web"
	"yudao-go/internal/module/infra/model"
	"yudao-go/internal/pkg/errcode"
)

//go:embed codegen_tpl
var codegenTpl embed.FS

// genFile 是一个生成出的文件。
type genFile struct {
	Path string `json:"filePath"`
	Code string `json:"code"`
}

// genData 是模板渲染上下文。
type genData struct {
	Table         *model.CodegenTable
	Columns       []*model.CodegenColumn
	CreateColumns []*model.CodegenColumn // 新增表单字段
	UpdateColumns []*model.CodegenColumn // 修改表单字段
	ListColumns   []*model.CodegenColumn // 列表展示字段
	QueryColumns  []*model.CodegenColumn // 查询条件字段
	PK            *model.CodegenColumn   // 主键列
	NeedTime      bool                   // 是否需要 import time
	TreeParentCol *model.CodegenColumn   // 树表：父节点列
	TreeNameCol   *model.CodegenColumn   // 树表：显示名列
}

// buildGenData 按字段配置预计算各子集。
func buildGenData(t *model.CodegenTable, cols []*model.CodegenColumn) *genData {
	d := &genData{Table: t, Columns: cols}
	for _, c := range cols {
		if c.CreateOperation {
			d.CreateColumns = append(d.CreateColumns, c)
		}
		if c.UpdateOperation {
			d.UpdateColumns = append(d.UpdateColumns, c)
		}
		if c.ListOperationResult {
			d.ListColumns = append(d.ListColumns, c)
		}
		if c.ListOperation {
			d.QueryColumns = append(d.QueryColumns, c)
		}
		if c.PrimaryKey && d.PK == nil {
			d.PK = c
		}
		if c.JavaType == "time.Time" {
			d.NeedTime = true
		}
	}
	// 树表：解析父节点列与名称列（未配置则按 parent_id / name 猜测）。
	for _, c := range cols {
		if c.ID == t.TreeParentColumnID || (d.TreeParentCol == nil && t.TreeParentColumnID == 0 && c.ColumnName == "parent_id") {
			d.TreeParentCol = c
		}
		if c.ID == t.TreeNameColumnID || (d.TreeNameCol == nil && t.TreeNameColumnID == 0 && c.ColumnName == "name") {
			d.TreeNameCol = c
		}
	}
	return d
}

// codegenFuncs 是模板可用的辅助函数。
var codegenFuncs = template.FuncMap{
	"export": func(s string) string {
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	},
	"lower":  strings.ToLower,
	"pascal": pascalCase,
}

// renderTpl 渲染单个模板。Vue 模板用 [[ ]] 定界符以避开 Vue 的 {{ }} 插值。
func renderTpl(name string, data *genData, leftDelim, rightDelim string) (string, error) {
	raw, err := codegenTpl.ReadFile(name)
	if err != nil {
		return "", err
	}
	t, err := template.New(name).Delims(leftDelim, rightDelim).Funcs(codegenFuncs).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// generate 按表配置渲染出全部代码文件。
func (h *CodegenHandler) generate(ctx context.Context, tableID int64) ([]genFile, error) {
	t, err := h.tables.Get(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errcode.NotFound
	}
	cols, err := h.columns.List(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("table_id = ?", tableID).Order("ordinal_position ASC")
	})
	if err != nil {
		return nil, err
	}
	d := buildGenData(t, cols)

	// 模板类型：1=单表 2=树表 3=主子表。model 与 menu 三类通用，handler/api/index 分目录。
	dir := "single"
	switch t.TemplateType {
	case 2:
		dir = "tree"
	case 3:
		// 主子表：主表按单表生成；子表由用户单独导入生成。
		dir = "single"
	}
	m, b := t.ModuleName, t.BusinessName
	specs := []struct{ tpl, out, ld, rd string }{
		{"single/model.go.tmpl", fmt.Sprintf("internal/module/%s/model/%s.go", m, b), "{{", "}}"},
		{dir + "/handler.go.tmpl", fmt.Sprintf("internal/module/%s/rest/%s_handler.go", m, b), "{{", "}}"},
		{dir + "/api.ts.tmpl", fmt.Sprintf("web/src/api/%s/%s/index.ts", m, b), "{{", "}}"},
		{dir + "/index.vue.tmpl", fmt.Sprintf("web/src/views/%s/%s/index.vue", m, b), "[[", "]]"},
		{"menu.sql.tmpl", fmt.Sprintf("sql/%s_%s_menu.sql", m, b), "{{", "}}"},
	}
	files := make([]genFile, 0, len(specs))
	for _, s := range specs {
		code, err := renderTpl("codegen_tpl/"+s.tpl, d, s.ld, s.rd)
		if err != nil {
			return nil, fmt.Errorf("渲染模板 %s 失败: %w", s.tpl, err)
		}
		// Go 文件用 gofmt 规整；格式化失败（模板产生非法 Go）则保留原文。
		if strings.HasSuffix(s.out, ".go") {
			if formatted, ferr := format.Source([]byte(code)); ferr == nil {
				code = string(formatted)
			}
		}
		files = append(files, genFile{Path: s.out, Code: code})
	}
	return files, nil
}

// preview 预览生成的代码。
func (h *CodegenHandler) preview(c *gin.Context) {
	files, err := h.generate(c.Request.Context(), qInt64(c, "tableId"))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	web.Success(c, files)
}

// download 下载生成的代码压缩包。
func (h *CodegenHandler) download(c *gin.Context) {
	files, err := h.generate(c.Request.Context(), qInt64(c, "tableId"))
	if err != nil {
		web.FailErr(c, err)
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.Path)
		if err != nil {
			web.FailErr(c, err)
			return
		}
		_, _ = w.Write([]byte(f.Code))
	}
	if err := zw.Close(); err != nil {
		web.FailErr(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=codegen.zip")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

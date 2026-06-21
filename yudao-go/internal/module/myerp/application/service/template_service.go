package service

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

// TemplateService 产品模板(SPU)应用服务。
// 借鉴 Odoo product.template 思想,精简到「共享字段集中 + 按属性组合生成 SKU」。
type TemplateService struct {
	templates    repository.TemplateRepository
	tplAttrLines repository.TemplateAttributeLineRepository
	attrs        repository.AttributeRepository
	options      repository.AttributeOptionRepository
	cats         repository.CategoryRepository
	uoms         repository.UomRepository
	products     repository.ProductRepository
	attrValues   repository.AttrValueRepository
	tx           *orm.TxManager
}

func NewTemplateService(
	t repository.TemplateRepository,
	tal repository.TemplateAttributeLineRepository,
	a repository.AttributeRepository,
	o repository.AttributeOptionRepository,
	c repository.CategoryRepository,
	u repository.UomRepository,
	p repository.ProductRepository,
	av repository.AttrValueRepository,
	tx *orm.TxManager,
) *TemplateService {
	return &TemplateService{
		templates: t, tplAttrLines: tal, attrs: a, options: o, cats: c, uoms: u,
		products: p, attrValues: av, tx: tx,
	}
}

func (s *TemplateService) Create(ctx context.Context, req *dto.TemplateSaveReq) (int64, error) {
	name, basePrice, desc, err := validateTemplate(req)
	if err != nil {
		return 0, err
	}
	cat, _ := s.cats.GetByID(ctx, req.CategoryID)
	if cat == nil {
		return 0, badReq("分类不存在")
	}
	if req.Code != "" {
		if exist, _ := s.templates.GetByCode(ctx, req.Code); exist != nil {
			return 0, badReq("模板 code 已存在")
		}
	}
	if err := s.validateAttributeLines(ctx, req.AttributeLines); err != nil {
		return 0, err
	}

	t := &model.ProductTemplate{
		TenantID:    contextx.TenantID(ctx),
		Name:        name,
		Code:        req.Code,
		CategoryID:  req.CategoryID,
		BaseUomID:   req.BaseUomID,
		BasePrice:   basePrice,
		Description: desc,
		Status:      req.Status,
	}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.templates.Create(ctx, t); err != nil {
			return wrapDuplicateError(err, "模板 code 已存在")
		}
		return s.tplAttrLines.UpsertBatch(ctx, t.ID, s.toAttrLines(t.ID, req.AttributeLines))
	})
	if err != nil {
		return 0, err
	}
	return t.ID, nil
}

func (s *TemplateService) Update(ctx context.Context, req *dto.TemplateSaveReq) error {
	if req.ID == 0 {
		return badReq("id 必填")
	}
	cur, err := s.templates.GetByID(ctx, req.ID)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("模板不存在")
		}
		return err
	}
	_ = cur
	name, basePrice, desc, err := validateTemplate(req)
	if err != nil {
		return err
	}
	if cat, _ := s.cats.GetByID(ctx, req.CategoryID); cat == nil {
		return badReq("分类不存在")
	}
	if err := s.validateAttributeLines(ctx, req.AttributeLines); err != nil {
		return err
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.templates.Update(ctx, req.ID, map[string]any{
			"name":        name,
			"code":        req.Code,
			"category_id": req.CategoryID,
			"base_uom_id": req.BaseUomID,
			"base_price":  basePrice,
			"description": desc,
			"status":      req.Status,
		}); err != nil {
			return wrapDuplicateError(err, "模板 code 已存在")
		}
		return s.tplAttrLines.UpsertBatch(ctx, req.ID, s.toAttrLines(req.ID, req.AttributeLines))
	})
}

func (s *TemplateService) Delete(ctx context.Context, id int64) error {
	cur, err := s.templates.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("模板不存在")
		}
		return err
	}
	_ = cur
	// 有变体禁删
	cnt, err := s.products.CountByTemplate(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return badReq("该模板下还有 %d 个 SKU,不能删除", cnt)
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.tplAttrLines.DeleteByTemplate(ctx, id); err != nil {
			return err
		}
		return s.templates.DeleteByID(ctx, id)
	})
}

func (s *TemplateService) Get(ctx context.Context, id int64) (*dto.TemplateDTO, error) {
	t, err := s.templates.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("模板不存在")
		}
		return nil, err
	}
	d := templateToDTO(t)
	// 名称回填
	if cat, _ := s.cats.GetByID(ctx, t.CategoryID); cat != nil {
		d.CategoryName = cat.Name
	}
	if t.BaseUomID != nil {
		if u, _ := s.uoms.GetByID(ctx, *t.BaseUomID); u != nil {
			d.BaseUomName = u.Name
		}
	}
	// 区分属性配置
	lines, _ := s.tplAttrLines.ListByTemplateIDs(ctx, []int64{id})
	if len(lines) > 0 {
		attrIDs := make([]int64, 0, len(lines))
		for _, l := range lines {
			attrIDs = append(attrIDs, l.AttributeID)
		}
		// 一次查 attribute 详情 + 选项
		attrs := s.attrsByIDs(ctx, attrIDs)
		opts, _ := s.options.ListByAttributeIDs(ctx, attrIDs)
		optsByAttr := make(map[int64][]dto.AttributeOptionDTO)
		for _, o := range opts {
			optsByAttr[o.AttributeID] = append(optsByAttr[o.AttributeID], dto.AttributeOptionDTO{
				Value: o.Value, PriceExtra: o.PriceExtra,
			})
		}
		for _, l := range lines {
			a := attrs[l.AttributeID]
			if a == nil {
				continue
			}
			d.AttributeLines = append(d.AttributeLines, dto.TemplateAttributeLineDTO{
				AttributeID:   a.ID,
				AttributeName: a.Name,
				AttributeCode: a.Code,
				Sort:          l.Sort,
				Options:       optsByAttr[a.ID],
			})
		}
	}
	// 变体数
	d.VariantCount, _ = s.products.CountByTemplate(ctx, id)
	return d, nil
}

func (s *TemplateService) Page(ctx context.Context, q repository.TemplateQuery) (*dto.Page[*dto.TemplateDTO], error) {
	list, total, err := s.templates.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.TemplateDTO, 0, len(list))
	if len(list) == 0 {
		return &dto.Page[*dto.TemplateDTO]{List: items, Total: total}, nil
	}
	// 批量回填:分类名、基本单位名、变体数
	catIDSet := make(map[int64]bool)
	uomIDSet := make(map[int64]bool)
	for _, t := range list {
		catIDSet[t.CategoryID] = true
		if t.BaseUomID != nil {
			uomIDSet[*t.BaseUomID] = true
		}
	}
	catNameMap := make(map[int64]string)
	allCats, _ := s.cats.ListAll(ctx)
	for _, c := range allCats {
		if catIDSet[c.ID] {
			catNameMap[c.ID] = c.Name
		}
	}
	uomIDs := make([]int64, 0, len(uomIDSet))
	for id := range uomIDSet {
		uomIDs = append(uomIDs, id)
	}
	uomList, _ := s.uoms.ListByIDs(ctx, uomIDs)
	uomNameMap := make(map[int64]string)
	for _, u := range uomList {
		uomNameMap[u.ID] = u.Name
	}
	for _, t := range list {
		d := templateToDTO(t)
		d.CategoryName = catNameMap[t.CategoryID]
		if t.BaseUomID != nil {
			d.BaseUomName = uomNameMap[*t.BaseUomID]
		}
		d.VariantCount, _ = s.products.CountByTemplate(ctx, t.ID)
		items = append(items, d)
	}
	return &dto.Page[*dto.TemplateDTO]{List: items, Total: total}, nil
}

// GenerateVariants 按属性组合笛卡尔积生成 SKU。
// 规则:
//   - 模板必须存在;只接受已在 template_attribute_line 中的 attribute
//   - selection 中的 value 必须在该属性的选项里
//   - 已存在的组合跳过(根据现有变体的 EAV 值签名比对)
//   - 每个组合的 SKU 售价 = template.base_price + Σ 所选 option.price_extra
func (s *TemplateService) GenerateVariants(ctx context.Context, req *dto.GenerateVariantsReq) (*dto.GenerateVariantsResp, error) {
	t, err := s.templates.GetByID(ctx, req.TemplateID)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("模板不存在")
		}
		return nil, err
	}
	if len(req.Selections) == 0 {
		return nil, badReq("请至少为一个区分属性选一些值")
	}
	// 检查模板配置了哪些 attribute_line
	lines, _ := s.tplAttrLines.ListByTemplateIDs(ctx, []int64{t.ID})
	validAttr := make(map[int64]bool, len(lines))
	for _, l := range lines {
		validAttr[l.AttributeID] = true
	}
	// 解析 selections,验证 attribute 与 value 合法
	var axes []variantAxis
	for kStr, values := range req.Selections {
		aid, err := strconv.ParseInt(kStr, 10, 64)
		if err != nil {
			return nil, badReq("非法属性 id %s", kStr)
		}
		if !validAttr[aid] {
			return nil, badReq("属性 %d 未配置在该模板的区分属性中", aid)
		}
		a, err := s.attrs.GetByID(ctx, aid)
		if err != nil || a == nil {
			return nil, badReq("属性 %d 不存在", aid)
		}
		if !a.IsVariant {
			return nil, badReq("属性 %s 不是区分属性", a.Name)
		}
		// 查该属性的所有选项,过滤出本次选中的值,带回 priceExtra
		allOpts, _ := s.options.ListByAttributeIDs(ctx, []int64{aid})
		optByValue := make(map[string]*model.AttributeOption, len(allOpts))
		for _, o := range allOpts {
			optByValue[o.Value] = o
		}
		picked := make([]dto.AttributeOptionDTO, 0, len(values))
		for _, v := range values {
			o, ok := optByValue[v]
			if !ok {
				return nil, badReq("属性 %s 不存在值 %s", a.Name, v)
			}
			picked = append(picked, dto.AttributeOptionDTO{Value: o.Value, PriceExtra: o.PriceExtra})
		}
		if len(picked) == 0 {
			continue
		}
		axes = append(axes, variantAxis{attributeID: aid, attributeCode: a.Code, values: picked})
	}
	if len(axes) == 0 {
		return nil, badReq("没有可用的属性选项")
	}
	// 按 attributeID 排序,保证签名稳定
	sort.Slice(axes, func(i, j int) bool { return axes[i].attributeID < axes[j].attributeID })

	// 取已有变体的签名集合(去重用)
	existing, _ := s.products.ListByTemplate(ctx, t.ID)
	existIDs := make([]int64, 0, len(existing))
	for _, p := range existing {
		existIDs = append(existIDs, p.ID)
	}
	existSig := make(map[string]bool, len(existing))
	if len(existIDs) > 0 {
		allAVs, _ := s.attrValues.ListByProductIDs(ctx, existIDs)
		byProduct := make(map[int64]map[int64]string, len(existIDs))
		for _, v := range allAVs {
			m := byProduct[v.ProductID]
			if m == nil {
				m = make(map[int64]string)
				byProduct[v.ProductID] = m
			}
			m[v.AttributeID] = v.Value
		}
		for _, pid := range existIDs {
			existSig[buildVariantSig(axes, byProduct[pid])] = true
		}
	}

	// 笛卡尔积
	combos := cartesian(axes)

	// 起始序号:已有变体数 + 1
	startSeq := int64(len(existing)) + 1
	tenantID := contextx.TenantID(ctx)
	out := &dto.GenerateVariantsResp{}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		for _, combo := range combos {
			sig := buildVariantSigFromCombo(combo)
			if existSig[sig] {
				out.Skipped++
				continue
			}
			// 算售价 = base + Σ price_extra
			salePrice := sumExtraPrices(t.BasePrice, combo)
			// 生成 code / name
			valueStrs := make([]string, 0, len(combo))
			for _, c := range combo {
				valueStrs = append(valueStrs, c.value.Value)
			}
			codeBase := t.Code
			if codeBase == "" {
				codeBase = fmt.Sprintf("TPL_%d", t.ID)
			}
			code := fmt.Sprintf("%s_%d", codeBase, startSeq)
			startSeq++
			name := t.Name + " · " + strings.Join(valueStrs, " / ")

			p := &model.Product{
				TenantID:      tenantID,
				CategoryID:    t.CategoryID,
				TemplateID:    &t.ID,
				BaseUomID:     t.BaseUomID,
				UomMode:       model.UomModeFixed, // 模板生成的 SKU 默认走固定换算
				TolerancePct:  "0",                 // DECIMAL NOT NULL,空串会报 1366
				Code:          code,
				Name:          name,
				SalePrice:     salePrice,
				PurchasePrice: "0",
				Stock:         "0",
				StockAux:      "0",
				BatchTracked:  true, // 默认值;非浮动模式时不生效
				Status:        0,
			}
			if err := s.products.Create(ctx, p); err != nil {
				return wrapDuplicateError(err, "生成 SKU 编码冲突,请调整模板 code 后重试")
			}
			// 写区分属性的 EAV 值
			avs := make([]*model.ProductAttrValue, 0, len(combo))
			for _, c := range combo {
				avs = append(avs, &model.ProductAttrValue{
					TenantID:      tenantID,
					ProductID:     p.ID,
					AttributeID:   c.attributeID,
					AttributeCode: c.attributeCode,
					Value:         html.EscapeString(c.value.Value),
				})
			}
			if err := s.attrValues.UpsertBatch(ctx, p.ID, avs); err != nil {
				return err
			}
			out.Created++
			out.VariantIDs = append(out.VariantIDs, p.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListVariants 列模板下的所有 SKU(简洁回显)。
func (s *TemplateService) ListVariants(ctx context.Context, templateID int64) ([]*dto.ProductDTO, error) {
	list, err := s.products.ListByTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return []*dto.ProductDTO{}, nil
	}
	// 回填 EAV(展示属性组合)
	ids := make([]int64, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	avs, _ := s.attrValues.ListByProductIDs(ctx, ids)
	byProduct := make(map[int64]map[string]any, len(list))
	for _, v := range avs {
		m := byProduct[v.ProductID]
		if m == nil {
			m = make(map[string]any)
			byProduct[v.ProductID] = m
		}
		m[v.AttributeCode] = v.Value
	}
	out := make([]*dto.ProductDTO, 0, len(list))
	for _, p := range list {
		d := productToDTO(p)
		if m, ok := byProduct[p.ID]; ok {
			d.Attrs = m
		} else {
			d.Attrs = map[string]any{}
		}
		out = append(out, d)
	}
	return out, nil
}

// === Helpers ===

// variantAxis 一个区分属性轴(用户为此属性选了 K 个值参与组合)。
type variantAxis struct {
	attributeID   int64
	attributeCode string
	values        []dto.AttributeOptionDTO
}

type combinedAxis struct {
	attributeID   int64
	attributeCode string
	value         dto.AttributeOptionDTO
}

// cartesian 笛卡尔积:输入 N 轴(每轴 K_i 个值),输出 ΠK_i 组合。
func cartesian(axes []variantAxis) [][]combinedAxis {
	if len(axes) == 0 {
		return nil
	}
	result := [][]combinedAxis{{}}
	for _, ax := range axes {
		var next [][]combinedAxis
		for _, prev := range result {
			for _, v := range ax.values {
				row := append([]combinedAxis{}, prev...)
				row = append(row, combinedAxis{
					attributeID: ax.attributeID, attributeCode: ax.attributeCode, value: v,
				})
				next = append(next, row)
			}
		}
		result = next
	}
	return result
}

// buildVariantSig 计算变体在「本次轴集」上的签名(用于 dedup 已有变体)。
// 只看 axes 里出现的 attribute(不在轴里的 EAV 值不影响签名)。
func buildVariantSig(axes []variantAxis, eavByAttrID map[int64]string) string {
	parts := make([]string, 0, len(axes))
	for _, ax := range axes {
		v := eavByAttrID[ax.attributeID]
		parts = append(parts, fmt.Sprintf("%d=%s", ax.attributeID, v))
	}
	return strings.Join(parts, "|")
}

func buildVariantSigFromCombo(combo []combinedAxis) string {
	parts := make([]string, 0, len(combo))
	for _, c := range combo {
		parts = append(parts, fmt.Sprintf("%d=%s", c.attributeID, c.value.Value))
	}
	return strings.Join(parts, "|")
}

// sumExtraPrices base + Σ extra (decimal 字符串相加,float 估算即可,精度由 base/extra 字符串保证)。
func sumExtraPrices(base string, combo []combinedAxis) string {
	b, _ := strconv.ParseFloat(strings.TrimSpace(base), 64)
	sum := b
	for _, c := range combo {
		e, _ := strconv.ParseFloat(strings.TrimSpace(c.value.PriceExtra), 64)
		sum += e
	}
	return strconv.FormatFloat(sum, 'f', 2, 64)
}

func (s *TemplateService) toAttrLines(templateID int64, reqs []dto.TemplateAttributeLineReq) []*model.TemplateAttributeLine {
	out := make([]*model.TemplateAttributeLine, 0, len(reqs))
	tenantID := int64(0) // UpsertBatch 不依赖 tenantID;orm.TenantModel 会自动注入
	for i, r := range reqs {
		out = append(out, &model.TemplateAttributeLine{
			TenantID:    tenantID,
			TemplateID:  templateID,
			AttributeID: r.AttributeID,
			Sort:        i,
		})
	}
	_ = tenantID
	return out
}

func (s *TemplateService) attrsByIDs(ctx context.Context, ids []int64) map[int64]*model.Attribute {
	m := make(map[int64]*model.Attribute, len(ids))
	for _, id := range ids {
		if a, _ := s.attrs.GetByID(ctx, id); a != nil {
			m[id] = a
		}
	}
	return m
}

func (s *TemplateService) validateAttributeLines(ctx context.Context, lines []dto.TemplateAttributeLineReq) error {
	seen := make(map[int64]bool, len(lines))
	for _, l := range lines {
		if seen[l.AttributeID] {
			return badReq("区分属性 %d 重复", l.AttributeID)
		}
		seen[l.AttributeID] = true
		a, err := s.attrs.GetByID(ctx, l.AttributeID)
		if err != nil || a == nil {
			return badReq("属性 %d 不存在", l.AttributeID)
		}
		if !a.IsVariant {
			return badReq("属性 %s 不是区分属性(需在属性管理勾选)", a.Name)
		}
		if a.InputType != model.InputSelect {
			return badReq("属性 %s 必须是 select 类型才能驱动变体", a.Name)
		}
	}
	return nil
}

func validateTemplate(req *dto.TemplateSaveReq) (cleanName, cleanBasePrice, cleanDesc string, err error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", "", badReq("模板名称不能为空")
	}
	if len(name) > 128 {
		return "", "", "", badReq("模板名称最长 128 字符")
	}
	if req.CategoryID <= 0 {
		return "", "", "", badReq("分类必填")
	}
	if req.Code != "" {
		if len(req.Code) > 64 {
			return "", "", "", badReq("模板 code 最长 64 字符")
		}
		if !categoryCodePattern.MatchString(req.Code) {
			return "", "", "", badReq("模板 code 只允许字母数字下划线连字符")
		}
	}
	bp := strings.TrimSpace(req.BasePrice)
	if bp == "" {
		bp = "0"
	}
	if !decimalPattern.MatchString(bp) || strings.HasPrefix(bp, "-") {
		return "", "", "", badReq("基础售价必须为非负数字")
	}
	if len(req.Description) > 1024 {
		return "", "", "", badReq("模板描述最长 1024 字符")
	}
	return html.EscapeString(name), bp, html.EscapeString(req.Description), nil
}

func templateToDTO(t *model.ProductTemplate) *dto.TemplateDTO {
	return &dto.TemplateDTO{
		ID:          t.ID,
		Name:        t.Name,
		Code:        t.Code,
		CategoryID:  t.CategoryID,
		BaseUomID:   t.BaseUomID,
		BasePrice:   t.BasePrice,
		Description: t.Description,
		Status:      t.Status,
		CreateTime:  t.CreateTime.Format("2006-01-02 15:04:05"),
	}
}

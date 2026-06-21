package service

import (
	"context"
	"strings"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

// ProductService 产品应用服务。EAV 联动:主表 + product_attr_value;多单位:base_uom + product_uom。
type ProductService struct {
	products    repository.ProductRepository
	attrValues  repository.AttrValueRepository
	attrs       repository.AttributeRepository
	options     repository.AttributeOptionRepository
	cats        repository.CategoryRepository
	uoms        repository.UomRepository
	productUoms repository.ProductUomRepository
	templates   repository.TemplateRepository
	tx          *orm.TxManager
	auditor     Auditor
}

func NewProductService(
	products repository.ProductRepository,
	attrValues repository.AttrValueRepository,
	attrs repository.AttributeRepository,
	options repository.AttributeOptionRepository,
	cats repository.CategoryRepository,
	uoms repository.UomRepository,
	productUoms repository.ProductUomRepository,
	templates repository.TemplateRepository,
	tx *orm.TxManager,
) *ProductService {
	return &ProductService{
		products: products, attrValues: attrValues, attrs: attrs, options: options, cats: cats,
		uoms: uoms, productUoms: productUoms, templates: templates, tx: tx,
		auditor: nopAuditor{},
	}
}

// SetAuditor 注入 chatter 审计器。
func (s *ProductService) SetAuditor(a Auditor) {
	if a != nil {
		s.auditor = a
	}
}

func (s *ProductService) Create(ctx context.Context, req *dto.ProductSaveReq) (int64, error) {
	cleanName, cleanDesc, err := validateProduct(
		req.Name, req.Description, req.PicURL,
		req.PurchasePrice, req.SalePrice, req.Stock,
		req.CategoryID,
	)
	if err != nil {
		return 0, err
	}
	cat, err := s.cats.GetByID(ctx, req.CategoryID)
	if err != nil && !errIsNotFound(err) {
		return 0, err
	}
	if cat == nil {
		return 0, badReq("分类不存在")
	}
	// 拿到该分类(含继承链)的全部属性,用于 EAV 校验
	chain, _ := s.cats.AncestorChain(ctx, req.CategoryID)
	attrs, err := s.attrs.ListByCategoryIDs(ctx, chain)
	if err != nil {
		return 0, err
	}
	avs, err := buildAttrValues(ctx, attrs, req.AttrValues, s.options)
	if err != nil {
		return 0, err
	}

	uoms, err := buildProductUoms(req.Uoms)
	if err != nil {
		return 0, err
	}
	tolerancePct, err := validateUomConfig(req.UomMode, req.AuxUomID, req.NominalFactor, req.TolerancePct)
	if err != nil {
		return 0, err
	}

	tenantID := contextx.TenantID(ctx)
	// 浮动双计量产品初始库存恒为 0,后续只经出入库流水改动
	stock := req.Stock
	if req.UomMode == model.UomModeFloat {
		stock = "0"
	}
	// 默认按批次管理(沿用现状);固定产品该位无意义,统一存 true 不影响行为
	batchTracked := req.BatchTracked || req.UomMode != model.UomModeFloat
	p := &model.Product{
		TenantID:      tenantID,
		CategoryID:    req.CategoryID,
		TemplateID:    req.TemplateID,
		BaseUomID:     req.BaseUomID,
		UomMode:       req.UomMode,
		AuxUomID:      req.AuxUomID,
		NominalFactor: req.NominalFactor,
		TolerancePct:  tolerancePct,
		BatchTracked:  batchTracked,
		Code:          req.Code,
		Name:          cleanName,
		BarCode:       req.BarCode,
		PicURL:        req.PicURL,
		Description:   cleanDesc,
		PurchasePrice: req.PurchasePrice,
		SalePrice:     req.SalePrice,
		Stock:         stock,
		StockAux:      "0",
		Status:        req.Status,
		OwnerUserID:   req.OwnerUserID,
	}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.products.Create(ctx, p); err != nil {
			return wrapDuplicateError(err, "产品 code 已存在")
		}
		// 给 avs 补 TenantID(buildAttrValues 不知道租户上下文)
		for _, v := range avs {
			v.TenantID = tenantID
		}
		if err := s.attrValues.UpsertBatch(ctx, p.ID, avs); err != nil {
			return err
		}
		for _, u := range uoms {
			u.TenantID = tenantID
		}
		return s.productUoms.UpsertBatch(ctx, p.ID, uoms)
	})
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

func (s *ProductService) Update(ctx context.Context, req *dto.ProductSaveReq) error {
	if req.ID == 0 {
		return badReq("id 必填")
	}
	cur, err := s.products.GetByID(ctx, req.ID)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("产品不存在")
		}
		return err
	}
	cleanName, cleanDesc, err := validateProduct(
		req.Name, req.Description, req.PicURL,
		req.PurchasePrice, req.SalePrice, req.Stock,
		req.CategoryID,
	)
	if err != nil {
		return err
	}
	cat, err := s.cats.GetByID(ctx, req.CategoryID)
	if err != nil && !errIsNotFound(err) {
		return err
	}
	if cat == nil {
		return badReq("分类不存在")
	}
	chain, _ := s.cats.AncestorChain(ctx, req.CategoryID)
	attrs, err := s.attrs.ListByCategoryIDs(ctx, chain)
	if err != nil {
		return err
	}
	avs, err := buildAttrValues(ctx, attrs, req.AttrValues, s.options)
	if err != nil {
		return err
	}
	uoms, err := buildProductUoms(req.Uoms)
	if err != nil {
		return err
	}
	tolerancePct, err := validateUomConfig(req.UomMode, req.AuxUomID, req.NominalFactor, req.TolerancePct)
	if err != nil {
		return err
	}
	tenantID := contextx.TenantID(ctx)
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		batchTracked := req.BatchTracked || req.UomMode != model.UomModeFloat
		fields := map[string]any{
			"category_id":    req.CategoryID,
			"template_id":    req.TemplateID,
			"base_uom_id":    req.BaseUomID,
			"uom_mode":       req.UomMode,
			"aux_uom_id":     req.AuxUomID,
			"nominal_factor": req.NominalFactor,
			"tolerance_pct":  tolerancePct,
			"batch_tracked":  batchTracked,
			"code":           req.Code,
			"name":           cleanName,
			"bar_code":       req.BarCode,
			"pic_url":        req.PicURL,
			"description":    cleanDesc,
			"purchase_price": req.PurchasePrice,
			"sale_price":     req.SalePrice,
			"status":         req.Status,
			"owner_user_id":  req.OwnerUserID,
		}
		// 固定模式:库存可直接编辑;浮动模式:库存是流水投影,不接受表单覆盖
		if req.UomMode == model.UomModeFixed {
			fields["stock"] = req.Stock
		}
		if err := s.products.Update(ctx, req.ID, fields); err != nil {
			return wrapDuplicateError(err, "产品 code 已存在")
		}
		for _, v := range avs {
			v.TenantID = tenantID
		}
		if err := s.attrValues.UpsertBatch(ctx, req.ID, avs); err != nil {
			return err
		}
		for _, u := range uoms {
			u.TenantID = tenantID
		}
		return s.productUoms.UpsertBatch(ctx, req.ID, uoms)
	}); err != nil {
		return err
	}
	_ = s.auditor.TrackUpdate(ctx, "myerp_product", req.ID,
		map[string]any{
			"name": cur.Name, "code": cur.Code, "bar_code": cur.BarCode,
			"category_id": cur.CategoryID,
			"purchase_price": cur.PurchasePrice, "sale_price": cur.SalePrice, "stock": cur.Stock,
			"status": cur.Status, "owner_user_id": ptrInt64(cur.OwnerUserID), "description": cur.Description,
		},
		map[string]any{
			"name": cleanName, "code": req.Code, "bar_code": req.BarCode,
			"category_id": req.CategoryID,
			"purchase_price": req.PurchasePrice, "sale_price": req.SalePrice, "stock": req.Stock,
			"status": req.Status, "owner_user_id": ptrInt64(req.OwnerUserID), "description": cleanDesc,
		})
	return nil
}

// ptrInt64 安全取 *int64 的值,nil 返回 0。
func ptrInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func (s *ProductService) Delete(ctx context.Context, id int64) error {
	cur, err := s.products.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("产品不存在")
		}
		return err
	}
	_ = cur
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.attrValues.DeleteByProduct(ctx, id); err != nil {
			return err
		}
		if err := s.productUoms.DeleteByProduct(ctx, id); err != nil {
			return err
		}
		return s.products.DeleteByID(ctx, id)
	})
}

func (s *ProductService) Get(ctx context.Context, id int64) (*dto.ProductDTO, error) {
	p, err := s.products.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("产品不存在")
		}
		return nil, err
	}
	avs, _ := s.attrValues.ListByProductIDs(ctx, []int64{id})
	attrs := make(map[string]any, len(avs))
	for _, v := range avs {
		attrs[v.AttributeCode] = v.Value
	}
	d := productToDTO(p)
	d.Attrs = attrs
	// 基本/主计量单位名称
	if p.BaseUomID != nil {
		if u, _ := s.uoms.GetByID(ctx, *p.BaseUomID); u != nil {
			d.BaseUomName = u.Name
		}
	}
	// 所属模板名称
	if p.TemplateID != nil {
		if t, _ := s.templates.GetByID(ctx, *p.TemplateID); t != nil {
			d.TemplateName = t.Name
		}
	}
	// 辅计量单位名称(浮动双计量)
	if p.AuxUomID != nil {
		if u, _ := s.uoms.GetByID(ctx, *p.AuxUomID); u != nil {
			d.AuxUomName = u.Name
		}
	}
	// 多单位换算列表(回填单位名称/编码)
	pus, _ := s.productUoms.ListByProductIDs(ctx, []int64{id})
	if len(pus) > 0 {
		uomIDs := make([]int64, 0, len(pus))
		for _, pu := range pus {
			uomIDs = append(uomIDs, pu.UomID)
		}
		uomList, _ := s.uoms.ListByIDs(ctx, uomIDs)
		uomMap := make(map[int64]*model.Uom, len(uomList))
		for _, u := range uomList {
			uomMap[u.ID] = u
		}
		for _, pu := range pus {
			item := dto.ProductUomDTO{
				UomID: pu.UomID, Factor: pu.Factor,
				IsPurchase: pu.IsPurchase, IsSale: pu.IsSale,
			}
			if u := uomMap[pu.UomID]; u != nil {
				item.UomName = u.Name
				item.UomCode = u.Code
			}
			d.Uoms = append(d.Uoms, item)
		}
	}
	return d, nil
}

// buildProductUoms 校验并转换产品多单位换算请求项。factor 须为正数。
func buildProductUoms(reqs []dto.ProductUomReq) ([]*model.ProductUom, error) {
	out := make([]*model.ProductUom, 0, len(reqs))
	seen := make(map[int64]bool, len(reqs))
	for i, r := range reqs {
		if r.UomID <= 0 {
			return nil, badReq("多单位第 %d 行:请选择单位", i+1)
		}
		if seen[r.UomID] {
			return nil, badReq("多单位第 %d 行:单位重复", i+1)
		}
		seen[r.UomID] = true
		if !decimalPattern.MatchString(r.Factor) || strings.HasPrefix(r.Factor, "-") || r.Factor == "0" {
			return nil, badReq("多单位第 %d 行:换算系数必须为正数", i+1)
		}
		out = append(out, &model.ProductUom{
			UomID: r.UomID, Factor: r.Factor,
			IsPurchase: r.IsPurchase, IsSale: r.IsSale, Sort: i,
		})
	}
	return out, nil
}

// Page 列表(EAV 属性批量回填,防 N+1)。
func (s *ProductService) Page(ctx context.Context, q repository.ProductQuery) (*dto.Page[*dto.ProductDTO], error) {
	list, total, err := s.products.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.ProductDTO, 0, len(list))
	if len(list) == 0 {
		return &dto.Page[*dto.ProductDTO]{List: items, Total: total}, nil
	}
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
	// 批量回填基本单位名称(列表库存显示「100 颗」)+ 模板名(双视角)
	uomNameMap := s.baseUomNameMap(ctx, list)
	tplNameMap := s.templateNameMap(ctx, list)
	for _, p := range list {
		d := productToDTO(p)
		if m, ok := byProduct[p.ID]; ok {
			d.Attrs = m
		} else {
			d.Attrs = map[string]any{}
		}
		if p.BaseUomID != nil {
			d.BaseUomName = uomNameMap[*p.BaseUomID]
		}
		if p.TemplateID != nil {
			d.TemplateName = tplNameMap[*p.TemplateID]
		}
		items = append(items, d)
	}
	return &dto.Page[*dto.ProductDTO]{List: items, Total: total}, nil
}

// templateNameMap 批量取产品所属模板名,防 N+1。
func (s *ProductService) templateNameMap(ctx context.Context, list []*model.Product) map[int64]string {
	idSet := make(map[int64]bool)
	for _, p := range list {
		if p.TemplateID != nil {
			idSet[*p.TemplateID] = true
		}
	}
	if len(idSet) == 0 {
		return map[int64]string{}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	tList, _ := s.templates.ListByIDs(ctx, ids)
	m := make(map[int64]string, len(tList))
	for _, t := range tList {
		m[t.ID] = t.Name
	}
	return m
}

// baseUomNameMap 批量查产品基本单位名称,避免列表 N+1。
func (s *ProductService) baseUomNameMap(ctx context.Context, list []*model.Product) map[int64]string {
	idSet := make(map[int64]bool)
	for _, p := range list {
		if p.BaseUomID != nil {
			idSet[*p.BaseUomID] = true
		}
	}
	if len(idSet) == 0 {
		return map[int64]string{}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	uomList, _ := s.uoms.ListByIDs(ctx, ids)
	m := make(map[int64]string, len(uomList))
	for _, u := range uomList {
		m[u.ID] = u.Name
	}
	return m
}

// SearchByAttrFilter 按动态属性筛选产品。
// filters: map[attributeID]value (精确匹配)。多属性 AND。
func (s *ProductService) SearchByAttrFilter(
	ctx context.Context, q repository.ProductQuery, filters map[int64]string,
) (*dto.Page[*dto.ProductDTO], error) {
	if len(filters) == 0 || q.CategoryID == nil {
		return s.Page(ctx, q)
	}
	tenantID := contextx.TenantID(ctx)
	pids, err := s.attrValues.FindProductIDsByAttrFilters(ctx, tenantID, *q.CategoryID, filters)
	if err != nil {
		return nil, err
	}
	if len(pids) == 0 {
		return &dto.Page[*dto.ProductDTO]{List: []*dto.ProductDTO{}, Total: 0}, nil
	}
	// 把 pids 当成额外条件:用现有 Page 查全分类后,过滤命中的;
	// 简化做法:直接 Page 但分页可能与 pids 数量不一致。
	// 实际:用 categoryID 过滤完后,做一遍 ID 过滤,然后切分页。
	// 性能小数据集足够;真上量再改为单条 IN ? SQL。
	all, _, err := s.products.Page(ctx, repository.ProductQuery{
		PageNo: 1, PageSize: 10000, CategoryID: q.CategoryID,
		Name: q.Name, Code: q.Code, BarCode: q.BarCode, Status: q.Status,
	})
	if err != nil {
		return nil, err
	}
	hit := make(map[int64]bool, len(pids))
	for _, id := range pids {
		hit[id] = true
	}
	filtered := make([]*model.Product, 0, len(all))
	for _, p := range all {
		if hit[p.ID] {
			filtered = append(filtered, p)
		}
	}
	total := int64(len(filtered))
	// 内存分页
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	end := offset + q.PageSize
	if offset > len(filtered) {
		filtered = nil
	} else {
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[offset:end]
	}
	// 批量回填 EAV
	ids := make([]int64, 0, len(filtered))
	for _, p := range filtered {
		ids = append(ids, p.ID)
	}
	avs, _ := s.attrValues.ListByProductIDs(ctx, ids)
	byProduct := make(map[int64]map[string]any, len(filtered))
	for _, v := range avs {
		m := byProduct[v.ProductID]
		if m == nil {
			m = make(map[string]any)
			byProduct[v.ProductID] = m
		}
		m[v.AttributeCode] = v.Value
	}
	items := make([]*dto.ProductDTO, 0, len(filtered))
	for _, p := range filtered {
		d := productToDTO(p)
		if m, ok := byProduct[p.ID]; ok {
			d.Attrs = m
		} else {
			d.Attrs = map[string]any{}
		}
		items = append(items, d)
	}
	return &dto.Page[*dto.ProductDTO]{List: items, Total: total}, nil
}

// ResolveAttrFiltersFromQuery 把 ?attr_code=value 形式的 query 串转成
// 服务层期望的 map[attributeID]value。handler 调,可以独立单测。
//
// 防护:
//   - code 必须 ^[a-zA-Z][a-zA-Z0-9_]{0,31}$(否则视为非法,丢弃)
//   - 必须有 categoryID,否则不知道在哪个分类的属性集里找
//   - 属性必须属于该分类(含继承链)
func (s *ProductService) ResolveAttrFiltersFromQuery(
	ctx context.Context, categoryID int64, rawFilters map[string]string,
) (map[int64]string, error) {
	if len(rawFilters) == 0 || categoryID <= 0 {
		return nil, nil
	}
	chain, _ := s.cats.AncestorChain(ctx, categoryID)
	attrs, err := s.attrs.ListByCategoryIDs(ctx, chain)
	if err != nil {
		return nil, err
	}
	codeToID := make(map[string]int64, len(attrs))
	for _, a := range attrs {
		codeToID[a.Code] = a.ID
	}
	out := make(map[int64]string, len(rawFilters))
	for code, v := range rawFilters {
		if !attributeCodePattern.MatchString(code) {
			return nil, badReq("非法属性 code %q", code)
		}
		id, ok := codeToID[code]
		if !ok {
			return nil, badReq("属性 %s 不属于该分类", code)
		}
		out[id] = v
	}
	return out, nil
}

func productToDTO(p *model.Product) *dto.ProductDTO {
	return &dto.ProductDTO{
		ID:            p.ID,
		CategoryID:    p.CategoryID,
		TemplateID:    p.TemplateID,
		BaseUomID:     p.BaseUomID,
		UomMode:       p.UomMode,
		AuxUomID:      p.AuxUomID,
		NominalFactor: p.NominalFactor,
		TolerancePct:  p.TolerancePct,
		BatchTracked:  p.BatchTracked,
		Code:          p.Code,
		Name:          p.Name,
		BarCode:       p.BarCode,
		PicURL:        p.PicURL,
		Description:   p.Description,
		PurchasePrice: p.PurchasePrice,
		SalePrice:     p.SalePrice,
		Stock:         p.Stock,
		StockAux:      p.StockAux,
		Status:        p.Status,
		OwnerUserID:   p.OwnerUserID,
		CreateTime:    p.CreateTime.Format("2006-01-02 15:04:05"),
	}
}

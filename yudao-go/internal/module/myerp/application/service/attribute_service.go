package service

import (
	"context"
	"strings"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

const maxAttributeOptions = 200 // 单属性枚举上限,防 DoS

// AttributeService 属性应用服务。
type AttributeService struct {
	attrs   repository.AttributeRepository
	options repository.AttributeOptionRepository
	cats    repository.CategoryRepository
	tx      *orm.TxManager
	auditor Auditor
}

func NewAttributeService(
	a repository.AttributeRepository,
	o repository.AttributeOptionRepository,
	c repository.CategoryRepository,
	tx *orm.TxManager,
) *AttributeService {
	return &AttributeService{attrs: a, options: o, cats: c, tx: tx, auditor: nopAuditor{}}
}

// SetAuditor 注入 chatter 审计器。
func (s *AttributeService) SetAuditor(a Auditor) {
	if a != nil {
		s.auditor = a
	}
}

func (s *AttributeService) Create(ctx context.Context, req *dto.AttributeSaveReq) (int64, error) {
	if req.CategoryID <= 0 {
		return 0, badReq("categoryId 必填")
	}
	cat, err := s.cats.GetByID(ctx, req.CategoryID)
	if err != nil && !errIsNotFound(err) {
		return 0, err
	}
	if cat == nil {
		return 0, badReq("分类不存在")
	}
	cleanName, cleanDesc, cleanUnit, err := validateAttribute(attrFields{
		Name: req.Name, Code: req.Code, InputType: req.InputType,
		Description: req.Description, Unit: req.Unit,
	})
	if err != nil {
		return 0, err
	}
	maxLen := req.MaxLength
	if maxLen <= 0 || maxLen > 1024 {
		maxLen = 1024
	}
	a := &model.Attribute{
		CategoryID:   req.CategoryID,
		Code:         req.Code,
		Name:         cleanName,
		InputType:    model.InputType(req.InputType),
		Unit:         cleanUnit,
		Required:     req.Required,
		Searchable:   req.Searchable,
		ShowInList:   req.ShowInList,
		IsVariant:    req.IsVariant,
		MinValue:     req.MinValue,
		MaxValue:     req.MaxValue,
		MinLength:    req.MinLength,
		MaxLength:    maxLen,
		Regex:        req.Regex,
		DefaultValue: req.DefaultValue,
		Sort:         req.Sort,
		Status:       req.Status,
		Description:  cleanDesc,
	}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.attrs.Create(ctx, a); err != nil {
			return wrapDuplicateError(err, "属性 code 已在该分类下存在")
		}
		// select / multi_select 同事务写枚举
		if a.InputType == model.InputSelect || a.InputType == model.InputMultiSelect {
			if len(req.Options) > maxAttributeOptions {
				return badReq("枚举选项最多 %d 个", maxAttributeOptions)
			}
			return s.replaceOptions(ctx, a.ID, req.Options)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return a.ID, nil
}

func (s *AttributeService) Update(ctx context.Context, req *dto.AttributeSaveReq) error {
	cur, err := s.attrs.GetByID(ctx, req.ID)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("属性不存在")
		}
		return err
	}
	// 禁止改 input_type(避免历史 EAV 值类型不一致)
	if req.InputType != "" && req.InputType != string(cur.InputType) {
		return badReq("不允许修改属性类型(请删了重建)")
	}
	cleanName, cleanDesc, cleanUnit, err := validateAttribute(attrFields{
		Name: req.Name, Code: cur.Code, InputType: string(cur.InputType),
		Description: req.Description, Unit: req.Unit,
	})
	if err != nil {
		return err
	}
	maxLen := req.MaxLength
	if maxLen <= 0 || maxLen > 1024 {
		maxLen = 1024
	}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.attrs.Update(ctx, req.ID, map[string]any{
			"name":          cleanName,
			"unit":          cleanUnit,
			"required":      orm.Bit(req.Required),
			"searchable":    orm.Bit(req.Searchable),
			"show_in_list":  orm.Bit(req.ShowInList),
			"is_variant":    req.IsVariant,
			"min_value":     req.MinValue,
			"max_value":     req.MaxValue,
			"min_length":    req.MinLength,
			"max_length":    maxLen,
			"regex":         req.Regex,
			"default_value": req.DefaultValue,
			"sort":          req.Sort,
			"status":        req.Status,
			"description":   cleanDesc,
		}); err != nil {
			return err
		}
		// 枚举类型 + 提交了 options 才覆盖
		if (cur.InputType == model.InputSelect || cur.InputType == model.InputMultiSelect) && req.Options != nil {
			if len(req.Options) > maxAttributeOptions {
				return badReq("枚举选项最多 %d 个", maxAttributeOptions)
			}
			return s.replaceOptions(ctx, req.ID, req.Options)
		}
		return nil
	}); err != nil {
		return err
	}
	_ = s.auditor.TrackUpdate(ctx, "myerp_attribute", req.ID,
		map[string]any{
			"name": cur.Name, "unit": cur.Unit,
			"required": cur.Required, "searchable": cur.Searchable, "show_in_list": cur.ShowInList,
			"min_value": ptrStr(cur.MinValue), "max_value": ptrStr(cur.MaxValue),
			"regex": cur.Regex, "sort": cur.Sort, "status": cur.Status, "description": cur.Description,
		},
		map[string]any{
			"name": cleanName, "unit": cleanUnit,
			"required": req.Required, "searchable": req.Searchable, "show_in_list": req.ShowInList,
			"min_value": ptrStr(req.MinValue), "max_value": ptrStr(req.MaxValue),
			"regex": req.Regex, "sort": req.Sort, "status": req.Status, "description": cleanDesc,
		})
	return nil
}

// ptrStr 安全取 *string 的值,nil 返回空串(供 audit diff 使用)。
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *AttributeService) Delete(ctx context.Context, id int64) error {
	cur, err := s.attrs.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("属性不存在")
		}
		return err
	}
	_ = cur
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.options.DeleteByAttribute(ctx, id); err != nil {
			return err
		}
		return s.attrs.DeleteByID(ctx, id)
	})
}

func (s *AttributeService) Get(ctx context.Context, id int64) (*dto.AttributeDTO, error) {
	a, err := s.attrs.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("属性不存在")
		}
		return nil, err
	}
	d := attributeToDTO(a)
	if a.InputType == model.InputSelect || a.InputType == model.InputMultiSelect {
		opts, _ := s.options.ListByAttributeIDs(ctx, []int64{a.ID})
		d.Options = optionsToDTOs(opts)
	}
	return d, nil
}

func (s *AttributeService) Page(ctx context.Context, q repository.AttributeQuery) (*dto.Page[*dto.AttributeDTO], error) {
	list, total, err := s.attrs.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.AttributeDTO, 0, len(list))
	for _, a := range list {
		items = append(items, attributeToDTO(a))
	}
	return &dto.Page[*dto.AttributeDTO]{List: items, Total: total}, nil
}

// ListByCategory 列出该分类(含父分类继承)的所有启用属性 + 各属性的枚举值。
// 前端创建产品时调用,用于动态渲染表单。
func (s *AttributeService) ListByCategory(ctx context.Context, categoryID int64) ([]*dto.AttributeDTO, error) {
	if categoryID <= 0 {
		return nil, badReq("categoryId 必填")
	}
	chain, err := s.cats.AncestorChain(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	attrs, err := s.attrs.ListByCategoryIDs(ctx, chain)
	if err != nil {
		return nil, err
	}
	// 按 code 去重:深层(自己)优先,再父再爷
	// chain 顺序就是 自己 → 父 → 爷;repo 已按 category_id + sort 返回。
	// 简单做法:按 code 第一次出现即保留(因 attrs 内顺序未必稳)。
	// 严格做法:用 chain depth 做主排序。这里做严格版:
	depthOf := make(map[int64]int, len(chain))
	for i, id := range chain {
		depthOf[id] = i // 0 = 自己(最深),越大越浅
	}
	bestByCode := make(map[string]*model.Attribute, len(attrs))
	for _, a := range attrs {
		exist, ok := bestByCode[a.Code]
		if !ok || depthOf[a.CategoryID] < depthOf[exist.CategoryID] {
			bestByCode[a.Code] = a
		}
	}
	deduped := make([]*model.Attribute, 0, len(bestByCode))
	for _, a := range bestByCode {
		deduped = append(deduped, a)
	}
	// 收集枚举选项,一次性查
	var enumIDs []int64
	for _, a := range deduped {
		if a.InputType == model.InputSelect || a.InputType == model.InputMultiSelect {
			enumIDs = append(enumIDs, a.ID)
		}
	}
	opts, _ := s.options.ListByAttributeIDs(ctx, enumIDs)
	optsByAttr := make(map[int64][]dto.AttributeOptionDTO)
	for _, o := range opts {
		optsByAttr[o.AttributeID] = append(optsByAttr[o.AttributeID], dto.AttributeOptionDTO{
			Value: o.Value, PriceExtra: o.PriceExtra,
		})
	}
	items := make([]*dto.AttributeDTO, 0, len(deduped))
	for _, a := range deduped {
		d := attributeToDTO(a)
		if a.InputType == model.InputSelect || a.InputType == model.InputMultiSelect {
			d.Options = optsByAttr[a.ID]
		}
		items = append(items, d)
	}
	return items, nil
}

// replaceOptions 全量替换某属性的枚举选项(事务内调用,含 priceExtra)。
func (s *AttributeService) replaceOptions(ctx context.Context, attributeID int64, opts []dto.AttributeOptionReq) error {
	if err := s.options.DeleteByAttribute(ctx, attributeID); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	items := make([]*model.AttributeOption, 0, len(opts))
	for i, o := range opts {
		extra := strings.TrimSpace(o.PriceExtra)
		if extra == "" {
			extra = "0"
		}
		if !decimalPattern.MatchString(extra) {
			return badReq("选项 %s 的加价必须为数字", o.Value)
		}
		items = append(items, &model.AttributeOption{
			AttributeID: attributeID,
			Value:       o.Value,
			Sort:        i,
			PriceExtra:  extra,
		})
	}
	return s.options.CreateBatch(ctx, items)
}

func attributeToDTO(a *model.Attribute) *dto.AttributeDTO {
	return &dto.AttributeDTO{
		ID:           a.ID,
		CategoryID:   a.CategoryID,
		Code:         a.Code,
		Name:         a.Name,
		InputType:    string(a.InputType),
		Unit:         a.Unit,
		Required:     a.Required,
		Searchable:   a.Searchable,
		ShowInList:   a.ShowInList,
		IsVariant:    a.IsVariant,
		MinValue:     a.MinValue,
		MaxValue:     a.MaxValue,
		MinLength:    a.MinLength,
		MaxLength:    a.MaxLength,
		Regex:        a.Regex,
		DefaultValue: a.DefaultValue,
		Sort:         a.Sort,
		Status:       a.Status,
		Description:  a.Description,
		CreateTime:   a.CreateTime.Format("2006-01-02 15:04:05"),
	}
}

func optionsToDTOs(opts []*model.AttributeOption) []dto.AttributeOptionDTO {
	out := make([]dto.AttributeOptionDTO, 0, len(opts))
	for _, o := range opts {
		out = append(out, dto.AttributeOptionDTO{Value: o.Value, PriceExtra: o.PriceExtra})
	}
	return out
}

package service

import (
	"context"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

// CategoryService 分类应用服务。
type CategoryService struct {
	repo     repository.CategoryRepository
	products repository.ProductRepository
	tx       *orm.TxManager
	auditor  Auditor
}

func NewCategoryService(
	r repository.CategoryRepository,
	p repository.ProductRepository,
	tx *orm.TxManager,
) *CategoryService {
	return &CategoryService{repo: r, products: p, tx: tx, auditor: nopAuditor{}}
}

// SetAuditor 注入 chatter 审计器(组合根可选注入)。
func (s *CategoryService) SetAuditor(a Auditor) {
	if a != nil {
		s.auditor = a
	}
}

func (s *CategoryService) Create(ctx context.Context, req *dto.CategoryCreateReq) (int64, error) {
	name, code, desc, err := validateCategory(req.Name, req.Code, req.Description)
	if err != nil {
		return 0, err
	}
	// 父分类存在性
	if req.ParentID > 0 {
		p, err := s.repo.GetByID(ctx, req.ParentID)
		if err != nil && !errIsNotFound(err) {
			return 0, err
		}
		if p == nil {
			return 0, badReq("父分类不存在")
		}
	}
	// 应用层 code 唯一预检 + DB 唯一索引兜底
	if code != "" {
		if exist, _ := s.repo.GetByCode(ctx, code); exist != nil {
			return 0, badReq("分类编码已存在")
		}
	}
	c := &model.Category{
		Name:               name,
		ParentID:           req.ParentID,
		Code:               code,
		Sort:               req.Sort,
		Status:             req.Status,
		InheritParentAttrs: req.InheritParentAttrs,
		Description:        desc,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return 0, wrapDuplicateError(err, "分类编码已存在")
	}
	return c.ID, nil
}

func (s *CategoryService) Update(ctx context.Context, req *dto.CategoryUpdateReq) error {
	cur, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return err
	}
	if cur == nil {
		return badReq("分类不存在")
	}
	name, code, desc, err := validateCategory(req.Name, req.Code, req.Description)
	if err != nil {
		return err
	}
	// 防循环引用:新 parent_id 不能是自己,且不能是自己的子孙
	if req.ParentID > 0 {
		if req.ParentID == req.ID {
			return badReq("父分类不能为自己")
		}
		chain, _ := s.repo.AncestorChain(ctx, req.ParentID)
		for _, anc := range chain {
			if anc == req.ID {
				return badReq("不能将分类移到自己的子分类下(会形成环)")
			}
		}
	}
	if err := s.repo.Update(ctx, req.ID, map[string]any{
		"name":                 name,
		"parent_id":            req.ParentID,
		"code":                 code,
		"sort":                 req.Sort,
		"status":               req.Status,
		"inherit_parent_attrs": orm.Bit(req.InheritParentAttrs),
		"description":          desc,
	}); err != nil {
		return err
	}
	// 字段变更审计 → 右侧 chatter 时间线。失败不影响主流程。
	_ = s.auditor.TrackUpdate(ctx, "myerp_category", req.ID,
		map[string]any{
			"name": cur.Name, "code": cur.Code, "parent_id": cur.ParentID,
			"sort": cur.Sort, "status": cur.Status,
			"inherit_parent_attrs": cur.InheritParentAttrs, "description": cur.Description,
		},
		map[string]any{
			"name": name, "code": code, "parent_id": req.ParentID,
			"sort": req.Sort, "status": req.Status,
			"inherit_parent_attrs": req.InheritParentAttrs, "description": desc,
		})
	return nil
}

func (s *CategoryService) Delete(ctx context.Context, id int64) error {
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cur == nil {
		return badReq("分类不存在")
	}
	hasChild, err := s.repo.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if hasChild {
		return badReq("存在子分类,不能删除")
	}
	cnt, err := s.products.CountByCategory(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return badReq("该分类下有产品,不能删除")
	}
	return s.repo.DeleteByID(ctx, id)
}

func (s *CategoryService) Get(ctx context.Context, id int64) (*dto.CategoryDTO, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("分类不存在")
		}
		return nil, err
	}
	return categoryToDTO(c), nil
}

func (s *CategoryService) Page(ctx context.Context, q repository.CategoryQuery) (*dto.Page[*dto.CategoryDTO], error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.CategoryDTO, 0, len(list))
	for _, c := range list {
		items = append(items, categoryToDTO(c))
	}
	return &dto.Page[*dto.CategoryDTO]{List: items, Total: total}, nil
}

func (s *CategoryService) Tree(ctx context.Context) ([]*dto.CategoryDTO, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.CategoryDTO, 0, len(list))
	for _, c := range list {
		items = append(items, categoryToDTO(c))
	}
	return items, nil
}

func categoryToDTO(c *model.Category) *dto.CategoryDTO {
	return &dto.CategoryDTO{
		ID:                 c.ID,
		Name:               c.Name,
		ParentID:           c.ParentID,
		Code:               c.Code,
		Sort:               c.Sort,
		Status:             c.Status,
		InheritParentAttrs: c.InheritParentAttrs,
		Description:        c.Description,
		CreateTime:         c.CreateTime.Format("2006-01-02 15:04:05"),
	}
}

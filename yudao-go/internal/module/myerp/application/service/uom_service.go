package service

import (
	"context"
	"html"
	"strings"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

// UomService 单位字典应用服务。
type UomService struct {
	repo        repository.UomRepository
	productUoms repository.ProductUomRepository
	tx          *orm.TxManager
	auditor     Auditor
}

func NewUomService(
	r repository.UomRepository,
	pu repository.ProductUomRepository,
	tx *orm.TxManager,
) *UomService {
	return &UomService{repo: r, productUoms: pu, tx: tx, auditor: nopAuditor{}}
}

func (s *UomService) SetAuditor(a Auditor) {
	if a != nil {
		s.auditor = a
	}
}

func (s *UomService) Create(ctx context.Context, req *dto.UomSaveReq) (int64, error) {
	name, code, err := validateUom(req.Name, req.Code)
	if err != nil {
		return 0, err
	}
	if code != "" {
		if exist, _ := s.repo.GetByCode(ctx, code); exist != nil {
			return 0, badReq("单位编码已存在")
		}
	}
	u := &model.Uom{
		Name: name, Code: code, Category: req.Category,
		Sort: req.Sort, Status: req.Status, Description: html.EscapeString(req.Description),
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return 0, wrapDuplicateError(err, "单位编码已存在")
	}
	return u.ID, nil
}

func (s *UomService) Update(ctx context.Context, req *dto.UomSaveReq) error {
	cur, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("单位不存在")
		}
		return err
	}
	name, code, err := validateUom(req.Name, req.Code)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, req.ID, map[string]any{
		"name":        name,
		"code":        code,
		"category":    req.Category,
		"sort":        req.Sort,
		"status":      req.Status,
		"description": html.EscapeString(req.Description),
	}); err != nil {
		return err
	}
	_ = cur
	return nil
}

func (s *UomService) Delete(ctx context.Context, id int64) error {
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("单位不存在")
		}
		return err
	}
	_ = cur
	// 被产品多单位引用时禁删
	cnt, err := s.productUoms.CountByUom(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return badReq("该单位已被产品引用,不能删除")
	}
	return s.repo.DeleteByID(ctx, id)
}

func (s *UomService) Get(ctx context.Context, id int64) (*dto.UomDTO, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("单位不存在")
		}
		return nil, err
	}
	return uomToDTO(u), nil
}

func (s *UomService) Page(ctx context.Context, q repository.UomQuery) (*dto.Page[*dto.UomDTO], error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.UomDTO, 0, len(list))
	for _, u := range list {
		items = append(items, uomToDTO(u))
	}
	return &dto.Page[*dto.UomDTO]{List: items, Total: total}, nil
}

// ListAll 启用单位全量(产品多单位选择下拉用)。
func (s *UomService) ListAll(ctx context.Context) ([]*dto.UomDTO, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.UomDTO, 0, len(list))
	for _, u := range list {
		items = append(items, uomToDTO(u))
	}
	return items, nil
}

func validateUom(name, code string) (cleanName, cleanCode string, err error) {
	name = strings.TrimSpace(name)
	code = strings.TrimSpace(code)
	if name == "" {
		return "", "", badReq("单位名称不能为空")
	}
	if len(name) > 32 {
		return "", "", badReq("单位名称最长 32 字符")
	}
	if code != "" {
		if len(code) > 32 {
			return "", "", badReq("单位编码最长 32 字符")
		}
		if !categoryCodePattern.MatchString(code) {
			return "", "", badReq("单位编码只允许字母、数字、下划线、连字符")
		}
	}
	return html.EscapeString(name), code, nil
}

func uomToDTO(u *model.Uom) *dto.UomDTO {
	return &dto.UomDTO{
		ID:          u.ID,
		Name:        u.Name,
		Code:        u.Code,
		Category:    u.Category,
		Sort:        u.Sort,
		Status:      u.Status,
		Description: u.Description,
		CreateTime:  u.CreateTime.Format("2006-01-02 15:04:05"),
	}
}

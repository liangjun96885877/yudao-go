package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
	"yudao-go/internal/pkg/errcode"
)

// TemplateRepo 产品模板(SPU)仓储 GORM 实现。
type TemplateRepo struct{ tx *orm.TxManager }

func NewTemplateRepo(tx *orm.TxManager) *TemplateRepo { return &TemplateRepo{tx: tx} }

func (r *TemplateRepo) Create(ctx context.Context, t *model.ProductTemplate) error {
	po := toProductTemplatePO(t)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	t.ID = po.ID
	t.CreateTime = po.CreateTime
	return nil
}

func (r *TemplateRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	res := r.tx.DB(ctx).Model(&ProductTemplatePO{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

func (r *TemplateRepo) GetByID(ctx context.Context, id int64) (*model.ProductTemplate, error) {
	var po ProductTemplatePO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromProductTemplatePO(&po), nil
}

func (r *TemplateRepo) GetByCode(ctx context.Context, code string) (*model.ProductTemplate, error) {
	var po ProductTemplatePO
	err := r.tx.DB(ctx).Where("code = ?", code).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromProductTemplatePO(&po), nil
}

func (r *TemplateRepo) DeleteByID(ctx context.Context, id int64) error {
	return r.tx.DB(ctx).Delete(&ProductTemplatePO{}, id).Error
}

func (r *TemplateRepo) Page(ctx context.Context, q repository.TemplateQuery) ([]*model.ProductTemplate, int64, error) {
	db := r.tx.DB(ctx).Model(&ProductTemplatePO{})
	if q.CategoryID != nil {
		db = db.Where("category_id = ?", *q.CategoryID)
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Code != "" {
		db = db.Where("code = ?", q.Code)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []*ProductTemplatePO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.ProductTemplate, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductTemplatePO(po))
	}
	return out, total, nil
}

func (r *TemplateRepo) ListByIDs(ctx context.Context, ids []int64) ([]*model.ProductTemplate, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var pos []*ProductTemplatePO
	if err := r.tx.DB(ctx).Where("id IN ?", ids).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.ProductTemplate, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductTemplatePO(po))
	}
	return out, nil
}

// TemplateAttributeLineRepo 模板区分属性配置仓储 GORM 实现。
type TemplateAttributeLineRepo struct{ tx *orm.TxManager }

func NewTemplateAttributeLineRepo(tx *orm.TxManager) *TemplateAttributeLineRepo {
	return &TemplateAttributeLineRepo{tx: tx}
}

// UpsertBatch 全量替换某模板的区分属性配置(先删后插,事务内)。
func (r *TemplateAttributeLineRepo) UpsertBatch(ctx context.Context, templateID int64, items []*model.TemplateAttributeLine) error {
	db := r.tx.DB(ctx)
	if err := db.Where("template_id = ?", templateID).Delete(&TemplateAttributeLinePO{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	pos := make([]*TemplateAttributeLinePO, 0, len(items))
	for _, it := range items {
		it.TemplateID = templateID
		pos = append(pos, toTemplateAttributeLinePO(it))
	}
	return db.Create(&pos).Error
}

func (r *TemplateAttributeLineRepo) DeleteByTemplate(ctx context.Context, templateID int64) error {
	return r.tx.DB(ctx).Where("template_id = ?", templateID).Delete(&TemplateAttributeLinePO{}).Error
}

func (r *TemplateAttributeLineRepo) ListByTemplateIDs(ctx context.Context, templateIDs []int64) ([]*model.TemplateAttributeLine, error) {
	if len(templateIDs) == 0 {
		return nil, nil
	}
	var pos []*TemplateAttributeLinePO
	err := r.tx.DB(ctx).Where("template_id IN ?", templateIDs).Order("sort ASC, id ASC").Find(&pos).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.TemplateAttributeLine, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromTemplateAttributeLinePO(po))
	}
	return out, nil
}

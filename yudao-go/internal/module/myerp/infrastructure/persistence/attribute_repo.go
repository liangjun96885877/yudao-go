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

// AttributeRepo 是 AttributeRepository 的 GORM 实现。
type AttributeRepo struct{ tx *orm.TxManager }

func NewAttributeRepo(tx *orm.TxManager) *AttributeRepo { return &AttributeRepo{tx: tx} }

func (r *AttributeRepo) Create(ctx context.Context, a *model.Attribute) error {
	po := toAttributePO(a)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	a.ID = po.ID
	a.CreateTime = po.CreateTime
	return nil
}

func (r *AttributeRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	res := r.tx.DB(ctx).Model(&AttributePO{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

func (r *AttributeRepo) GetByID(ctx context.Context, id int64) (*model.Attribute, error) {
	var po AttributePO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromAttributePO(&po), nil
}

func (r *AttributeRepo) DeleteByID(ctx context.Context, id int64) error {
	return r.tx.DB(ctx).Delete(&AttributePO{}, id).Error
}

func (r *AttributeRepo) Page(ctx context.Context, q repository.AttributeQuery) ([]*model.Attribute, int64, error) {
	db := r.tx.DB(ctx).Model(&AttributePO{})
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
	var pos []*AttributePO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.Attribute, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromAttributePO(po))
	}
	return out, total, nil
}

func (r *AttributeRepo) ListByCategoryIDs(ctx context.Context, categoryIDs []int64) ([]*model.Attribute, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}
	var pos []*AttributePO
	err := r.tx.DB(ctx).
		Where("category_id IN ? AND status = 0", categoryIDs).
		Order("sort ASC, id ASC").Find(&pos).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Attribute, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromAttributePO(po))
	}
	return out, nil
}

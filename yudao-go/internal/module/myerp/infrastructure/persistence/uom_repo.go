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

// UomRepo 是 UomRepository 的 GORM 实现。
type UomRepo struct{ tx *orm.TxManager }

func NewUomRepo(tx *orm.TxManager) *UomRepo { return &UomRepo{tx: tx} }

func (r *UomRepo) Create(ctx context.Context, u *model.Uom) error {
	po := toUomPO(u)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	u.ID = po.ID
	u.CreateTime = po.CreateTime
	return nil
}

func (r *UomRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	res := r.tx.DB(ctx).Model(&UomPO{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

func (r *UomRepo) GetByID(ctx context.Context, id int64) (*model.Uom, error) {
	var po UomPO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromUomPO(&po), nil
}

func (r *UomRepo) GetByCode(ctx context.Context, code string) (*model.Uom, error) {
	var po UomPO
	err := r.tx.DB(ctx).Where("code = ?", code).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromUomPO(&po), nil
}

func (r *UomRepo) DeleteByID(ctx context.Context, id int64) error {
	return r.tx.DB(ctx).Delete(&UomPO{}, id).Error
}

func (r *UomRepo) Page(ctx context.Context, q repository.UomQuery) ([]*model.Uom, int64, error) {
	db := r.tx.DB(ctx).Model(&UomPO{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Code != "" {
		db = db.Where("code = ?", q.Code)
	}
	if q.Category != "" {
		db = db.Where("category = ?", q.Category)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []*UomPO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.Uom, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromUomPO(po))
	}
	return out, total, nil
}

func (r *UomRepo) ListByIDs(ctx context.Context, ids []int64) ([]*model.Uom, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var pos []*UomPO
	if err := r.tx.DB(ctx).Where("id IN ?", ids).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Uom, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromUomPO(po))
	}
	return out, nil
}

func (r *UomRepo) ListAll(ctx context.Context) ([]*model.Uom, error) {
	var pos []*UomPO
	if err := r.tx.DB(ctx).Where("status = 0").Order("sort ASC, id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Uom, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromUomPO(po))
	}
	return out, nil
}

// ProductUomRepo 是 ProductUomRepository 的 GORM 实现。
type ProductUomRepo struct{ tx *orm.TxManager }

func NewProductUomRepo(tx *orm.TxManager) *ProductUomRepo { return &ProductUomRepo{tx: tx} }

// UpsertBatch 全量替换某产品的多单位换算配置(先删后插,事务内)。
func (r *ProductUomRepo) UpsertBatch(ctx context.Context, productID int64, items []*model.ProductUom) error {
	db := r.tx.DB(ctx)
	if err := db.Where("product_id = ?", productID).Delete(&ProductUomPO{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	pos := make([]*ProductUomPO, 0, len(items))
	for _, it := range items {
		it.ProductID = productID
		pos = append(pos, toProductUomPO(it))
	}
	return db.Create(&pos).Error
}

func (r *ProductUomRepo) DeleteByProduct(ctx context.Context, productID int64) error {
	return r.tx.DB(ctx).Where("product_id = ?", productID).Delete(&ProductUomPO{}).Error
}

func (r *ProductUomRepo) ListByProductIDs(ctx context.Context, productIDs []int64) ([]*model.ProductUom, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	var pos []*ProductUomPO
	err := r.tx.DB(ctx).Where("product_id IN ?", productIDs).Order("sort ASC, id ASC").Find(&pos).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProductUom, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductUomPO(po))
	}
	return out, nil
}

func (r *ProductUomRepo) CountByUom(ctx context.Context, uomID int64) (int64, error) {
	var cnt int64
	err := r.tx.DB(ctx).Model(&ProductUomPO{}).Where("uom_id = ?", uomID).Count(&cnt).Error
	return cnt, err
}

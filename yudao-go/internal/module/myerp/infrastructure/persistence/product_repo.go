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

// ProductRepo 是 ProductRepository 的 GORM 实现。
type ProductRepo struct{ tx *orm.TxManager }

func NewProductRepo(tx *orm.TxManager) *ProductRepo { return &ProductRepo{tx: tx} }

func (r *ProductRepo) Create(ctx context.Context, p *model.Product) error {
	po := toProductPO(p)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	p.ID = po.ID
	p.CreateTime = po.CreateTime
	return nil
}

func (r *ProductRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	res := r.tx.DB(ctx).Model(&ProductPO{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

func (r *ProductRepo) GetByID(ctx context.Context, id int64) (*model.Product, error) {
	var po ProductPO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromProductPO(&po), nil
}

func (r *ProductRepo) GetByCode(ctx context.Context, code string) (*model.Product, error) {
	var po ProductPO
	err := r.tx.DB(ctx).Where("code = ?", code).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromProductPO(&po), nil
}

func (r *ProductRepo) DeleteByID(ctx context.Context, id int64) error {
	return r.tx.DB(ctx).Delete(&ProductPO{}, id).Error
}

func (r *ProductRepo) Page(ctx context.Context, q repository.ProductQuery) ([]*model.Product, int64, error) {
	db := r.tx.DB(ctx).Model(&ProductPO{})
	if q.CategoryID != nil {
		db = db.Where("category_id = ?", *q.CategoryID)
	}
	if q.TemplateID != nil {
		if *q.TemplateID < 0 {
			db = db.Where("template_id IS NULL")
		} else {
			db = db.Where("template_id = ?", *q.TemplateID)
		}
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Code != "" {
		db = db.Where("code = ?", q.Code)
	}
	if q.BarCode != "" {
		db = db.Where("bar_code = ?", q.BarCode)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []*ProductPO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.Product, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductPO(po))
	}
	return out, total, nil
}

func (r *ProductRepo) CountByCategory(ctx context.Context, categoryID int64) (int64, error) {
	var cnt int64
	err := r.tx.DB(ctx).Model(&ProductPO{}).Where("category_id = ?", categoryID).Count(&cnt).Error
	return cnt, err
}

func (r *ProductRepo) CountByTemplate(ctx context.Context, templateID int64) (int64, error) {
	var cnt int64
	err := r.tx.DB(ctx).Model(&ProductPO{}).Where("template_id = ?", templateID).Count(&cnt).Error
	return cnt, err
}

func (r *ProductRepo) ListByTemplate(ctx context.Context, templateID int64) ([]*model.Product, error) {
	var pos []*ProductPO
	if err := r.tx.DB(ctx).Where("template_id = ?", templateID).Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Product, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductPO(po))
	}
	return out, nil
}

// AddStock 原子增量:stock += deltaBase, stock_aux += deltaAux(MySQL DECIMAL 运算)。
func (r *ProductRepo) AddStock(ctx context.Context, id int64, deltaBase, deltaAux string) error {
	res := r.tx.DB(ctx).Model(&ProductPO{}).Where("id = ?", id).Updates(map[string]any{
		"stock":     gorm.Expr("stock + ?", deltaBase),
		"stock_aux": gorm.Expr("stock_aux + ?", deltaAux),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

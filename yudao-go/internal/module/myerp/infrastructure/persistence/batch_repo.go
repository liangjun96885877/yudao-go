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

// BatchRepo 是 BatchRepository 的 GORM 实现。
type BatchRepo struct{ tx *orm.TxManager }

func NewBatchRepo(tx *orm.TxManager) *BatchRepo { return &BatchRepo{tx: tx} }

func (r *BatchRepo) Create(ctx context.Context, b *model.ProductBatch) error {
	po := toProductBatchPO(b)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	b.ID = po.ID
	b.CreateTime = po.CreateTime
	return nil
}

func (r *BatchRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	res := r.tx.DB(ctx).Model(&ProductBatchPO{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

func (r *BatchRepo) GetByID(ctx context.Context, id int64) (*model.ProductBatch, error) {
	var po ProductBatchPO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromProductBatchPO(&po), nil
}

func (r *BatchRepo) GetByNo(ctx context.Context, productID int64, batchNo string) (*model.ProductBatch, error) {
	var po ProductBatchPO
	err := r.tx.DB(ctx).Where("product_id = ? AND batch_no = ?", productID, batchNo).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromProductBatchPO(&po), nil
}

func (r *BatchRepo) DeleteByID(ctx context.Context, id int64) error {
	return r.tx.DB(ctx).Delete(&ProductBatchPO{}, id).Error
}

func (r *BatchRepo) Page(ctx context.Context, q repository.BatchQuery) ([]*model.ProductBatch, int64, error) {
	db := r.tx.DB(ctx).Model(&ProductBatchPO{})
	if q.ProductID != nil {
		db = db.Where("product_id = ?", *q.ProductID)
	}
	if q.BatchNo != "" {
		db = db.Where("batch_no LIKE ?", "%"+q.BatchNo+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []*ProductBatchPO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.ProductBatch, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductBatchPO(po))
	}
	return out, total, nil
}

func (r *BatchRepo) CountByProduct(ctx context.Context, productID int64) (int64, error) {
	var cnt int64
	err := r.tx.DB(ctx).Model(&ProductBatchPO{}).Where("product_id = ?", productID).Count(&cnt).Error
	return cnt, err
}

// AddStock 原子增量:stock_base += deltaBase, stock_aux += deltaAux。
func (r *BatchRepo) AddStock(ctx context.Context, id int64, deltaBase, deltaAux string) error {
	res := r.tx.DB(ctx).Model(&ProductBatchPO{}).Where("id = ?", id).Updates(map[string]any{
		"stock_base": gorm.Expr("stock_base + ?", deltaBase),
		"stock_aux":  gorm.Expr("stock_aux + ?", deltaAux),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

// StockMoveRepo 是 StockMoveRepository 的 GORM 实现(append-only)。
type StockMoveRepo struct{ tx *orm.TxManager }

func NewStockMoveRepo(tx *orm.TxManager) *StockMoveRepo { return &StockMoveRepo{tx: tx} }

func (r *StockMoveRepo) Create(ctx context.Context, m *model.StockMove) error {
	po := toStockMovePO(m)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	m.ID = po.ID
	m.CreateTime = po.CreateTime
	return nil
}

func (r *StockMoveRepo) Page(ctx context.Context, q repository.StockMoveQuery) ([]*model.StockMove, int64, error) {
	db := r.tx.DB(ctx).Model(&StockMovePO{})
	if q.ProductID != nil {
		db = db.Where("product_id = ?", *q.ProductID)
	}
	if q.BatchID != nil {
		db = db.Where("batch_id = ?", *q.BatchID)
	}
	if q.MoveType != nil {
		db = db.Where("move_type = ?", *q.MoveType)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []*StockMovePO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.StockMove, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromStockMovePO(po))
	}
	return out, total, nil
}

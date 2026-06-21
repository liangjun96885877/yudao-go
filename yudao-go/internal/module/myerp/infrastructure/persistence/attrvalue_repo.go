package persistence

import (
	"context"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/domain/model"
)

// AttrValueRepo 是 AttrValueRepository 的 GORM 实现。
type AttrValueRepo struct{ tx *orm.TxManager }

func NewAttrValueRepo(tx *orm.TxManager) *AttrValueRepo { return &AttrValueRepo{tx: tx} }

// UpsertBatch 把某产品的全部 EAV 值替换为 items(先删后插,事务内)。
// 调用方须在 application service 包事务。
func (r *AttrValueRepo) UpsertBatch(ctx context.Context, productID int64, items []*model.ProductAttrValue) error {
	db := r.tx.DB(ctx)
	if err := db.Where("product_id = ?", productID).Delete(&ProductAttrValuePO{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	pos := make([]*ProductAttrValuePO, 0, len(items))
	for _, it := range items {
		it.ProductID = productID
		pos = append(pos, toProductAttrValuePO(it))
	}
	return db.Create(&pos).Error
}

func (r *AttrValueRepo) DeleteByProduct(ctx context.Context, productID int64) error {
	return r.tx.DB(ctx).Where("product_id = ?", productID).Delete(&ProductAttrValuePO{}).Error
}

func (r *AttrValueRepo) ListByProductIDs(
	ctx context.Context, productIDs []int64,
) ([]*model.ProductAttrValue, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	var pos []*ProductAttrValuePO
	err := r.tx.DB(ctx).Where("product_id IN ?", productIDs).Find(&pos).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProductAttrValue, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromProductAttrValuePO(po))
	}
	return out, nil
}

// FindProductIDsByAttrFilters 动态属性筛选:多个 (attributeID, expectedValue) 全部 AND 命中。
// 实现:对每个 attr 单独查一次产品 ID 集合,然后求交集。简单稳。
func (r *AttrValueRepo) FindProductIDsByAttrFilters(
	ctx context.Context, tenantID, categoryID int64, filters map[int64]string,
) ([]int64, error) {
	if len(filters) == 0 {
		return nil, nil
	}
	// 先按 categoryID 拿到候选产品 ID 集合作为种子
	var seed []int64
	if err := r.tx.DB(ctx).Model(&ProductPO{}).
		Where("category_id = ?", categoryID).
		Pluck("id", &seed).Error; err != nil {
		return nil, err
	}
	if len(seed) == 0 {
		return seed, nil
	}
	current := make(map[int64]bool, len(seed))
	for _, id := range seed {
		current[id] = true
	}
	// 对每个属性过滤一次,与现有集合做交集
	for attrID, val := range filters {
		var matched []int64
		if err := r.tx.DB(ctx).Model(&ProductAttrValuePO{}).
			Where("attribute_id = ? AND value = ?", attrID, val).
			Pluck("product_id", &matched).Error; err != nil {
			return nil, err
		}
		next := make(map[int64]bool, len(matched))
		for _, id := range matched {
			if current[id] {
				next[id] = true
			}
		}
		current = next
		if len(current) == 0 {
			break
		}
	}
	out := make([]int64, 0, len(current))
	for id := range current {
		out = append(out, id)
	}
	return out, nil
}

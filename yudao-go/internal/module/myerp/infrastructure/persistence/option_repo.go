package persistence

import (
	"context"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/domain/model"
)

// AttributeOptionRepo 是 AttributeOptionRepository 的 GORM 实现。
type AttributeOptionRepo struct{ tx *orm.TxManager }

func NewAttributeOptionRepo(tx *orm.TxManager) *AttributeOptionRepo {
	return &AttributeOptionRepo{tx: tx}
}

func (r *AttributeOptionRepo) CreateBatch(ctx context.Context, items []*model.AttributeOption) error {
	if len(items) == 0 {
		return nil
	}
	pos := make([]*AttributeOptionPO, 0, len(items))
	for _, it := range items {
		pos = append(pos, toAttributeOptionPO(it))
	}
	return r.tx.DB(ctx).Create(&pos).Error
}

func (r *AttributeOptionRepo) DeleteByAttribute(ctx context.Context, attributeID int64) error {
	// AttributeOptionPO 用 LightBase 无 deleted 字段,物理删除。
	return r.tx.DB(ctx).Where("attribute_id = ?", attributeID).Delete(&AttributeOptionPO{}).Error
}

func (r *AttributeOptionRepo) ListByAttributeIDs(
	ctx context.Context, attributeIDs []int64,
) ([]*model.AttributeOption, error) {
	if len(attributeIDs) == 0 {
		return nil, nil
	}
	var pos []*AttributeOptionPO
	err := r.tx.DB(ctx).
		Where("attribute_id IN ?", attributeIDs).
		Order("sort ASC, id ASC").Find(&pos).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.AttributeOption, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromAttributeOptionPO(po))
	}
	return out, nil
}

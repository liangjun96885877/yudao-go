package persistence

import (
	"context"

	"gorm.io/gorm/clause"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
)

// FollowerRepo 是 FollowerRepository 的 GORM 实现。
type FollowerRepo struct {
	tx *orm.TxManager
}

func NewFollowerRepo(tx *orm.TxManager) *FollowerRepo { return &FollowerRepo{tx: tx} }

// Add 幂等新增关注关系：唯一键冲突时不报错（已关注则保持原状）。
func (r *FollowerRepo) Add(ctx context.Context, f *model.Follower) error {
	po := toFollowerPO(f)
	return r.tx.DB(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"}, {Name: "biz_type"}, {Name: "biz_id"}, {Name: "user_id"},
		},
		DoNothing: true,
	}).Create(po).Error
}

func (r *FollowerRepo) Remove(ctx context.Context, ref model.BizRef, userID int64) error {
	// FollowerPO 无逻辑删除字段，执行物理删除。
	return r.tx.DB(ctx).
		Where("biz_type = ? AND biz_id = ? AND user_id = ?", ref.BizType, ref.BizID, userID).
		Delete(&FollowerPO{}).Error
}

func (r *FollowerRepo) ListByBiz(ctx context.Context, ref model.BizRef) ([]*model.Follower, error) {
	var pos []*FollowerPO
	if err := r.tx.DB(ctx).
		Where("biz_type = ? AND biz_id = ?", ref.BizType, ref.BizID).
		Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Follower, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromFollowerPO(po))
	}
	return out, nil
}

func (r *FollowerRepo) Exists(ctx context.Context, ref model.BizRef, userID int64) (bool, error) {
	var count int64
	err := r.tx.DB(ctx).Model(&FollowerPO{}).
		Where("biz_type = ? AND biz_id = ? AND user_id = ?", ref.BizType, ref.BizID, userID).
		Limit(1).Count(&count).Error
	return count > 0, err
}

// UpdateSettings 更新订阅事件类型与静音标记。借助 PO 的 serializer:json,
// 用 Select 强制更新两列（即便 muted=false 也写）。
func (r *FollowerRepo) UpdateSettings(ctx context.Context, ref model.BizRef, userID int64, subscribeTypes []string, muted bool) error {
	po := FollowerPO{SubscribeTypes: subscribeTypes, Muted: muted}
	return r.tx.DB(ctx).Model(&FollowerPO{}).
		Where("biz_type = ? AND biz_id = ? AND user_id = ?", ref.BizType, ref.BizID, userID).
		Select("subscribe_types", "muted").Updates(&po).Error
}

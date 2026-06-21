package persistence

import (
	"context"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
)

// AttachmentRepo 是 AttachmentRepository 的 GORM 实现。
type AttachmentRepo struct {
	tx *orm.TxManager
}

func NewAttachmentRepo(tx *orm.TxManager) *AttachmentRepo { return &AttachmentRepo{tx: tx} }

func (r *AttachmentRepo) CreateBatch(ctx context.Context, items []*model.Attachment) error {
	if len(items) == 0 {
		return nil
	}
	pos := make([]*AttachmentPO, 0, len(items))
	for _, a := range items {
		pos = append(pos, toAttachmentPO(a))
	}
	if err := r.tx.DB(ctx).Create(pos).Error; err != nil {
		return err
	}
	for i, po := range pos { // 回填主键
		items[i].ID = po.ID
		items[i].CreateTime = po.CreateTime
	}
	return nil
}

func (r *AttachmentRepo) ListByBiz(ctx context.Context, ref model.BizRef) ([]*model.Attachment, error) {
	var pos []*AttachmentPO
	if err := r.tx.DB(ctx).
		Where("biz_type = ? AND biz_id = ?", ref.BizType, ref.BizID).
		Order("id DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Attachment, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromAttachmentPO(po))
	}
	return out, nil
}

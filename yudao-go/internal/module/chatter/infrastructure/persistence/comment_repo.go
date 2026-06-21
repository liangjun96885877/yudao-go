package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/pkg/errcode"
)

// CommentRepo 是 CommentRepository 的 GORM 实现。
type CommentRepo struct {
	tx *orm.TxManager
}

func NewCommentRepo(tx *orm.TxManager) *CommentRepo { return &CommentRepo{tx: tx} }

func (r *CommentRepo) Create(ctx context.Context, c *model.Comment) error {
	po := toCommentPO(c)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	c.ID = po.ID
	c.CreateTime = po.CreateTime
	// 根评论的 root_id 指向自身，便于整条线程查询。
	if c.ParentID == 0 && c.RootID == 0 {
		if err := r.tx.DB(ctx).Model(&CommentPO{}).
			Where("id = ?", c.ID).Update("root_id", c.ID).Error; err != nil {
			return err
		}
		c.RootID = c.ID
	}
	return nil
}

// Update 基于乐观锁更新。c.Version 为递增后的新版本，旧版本为 c.Version-1。
func (r *CommentRepo) Update(ctx context.Context, c *model.Comment) error {
	res := r.tx.DB(ctx).Model(&CommentPO{}).
		Where("id = ? AND version = ?", c.ID, c.Version-1).
		Updates(map[string]any{
			"content":      c.Content,
			"content_html": c.ContentHTML,
			"edited_at":    c.EditedAt,
			"version":      c.Version,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.Conflict // 版本不匹配：并发修改冲突
	}
	return nil
}

func (r *CommentRepo) GetByID(ctx context.Context, id int64) (*model.Comment, error) {
	var po CommentPO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromCommentPO(&po), nil
}

func (r *CommentRepo) DeleteByID(ctx context.Context, id int64) error {
	// CommentPO 含逻辑删除字段，Delete 执行软删除。
	return r.tx.DB(ctx).Delete(&CommentPO{}, id).Error
}

// CountChildrenByParents 用 GROUP BY 一次拿到每个父评论的直接子评论数。
func (r *CommentRepo) CountChildrenByParents(
	ctx context.Context, parentIDs []int64,
) (map[int64]int, error) {
	out := make(map[int64]int, len(parentIDs))
	if len(parentIDs) == 0 {
		return out, nil
	}
	type row struct {
		ParentID int64
		Cnt      int
	}
	var rows []row
	if err := r.tx.DB(ctx).Model(&CommentPO{}).
		Select("parent_id, COUNT(*) AS cnt").
		Where("parent_id IN ?", parentIDs).
		Group("parent_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ParentID] = r.Cnt
	}
	return out, nil
}

func (r *CommentRepo) ListIDsByParent(ctx context.Context, parentID int64) ([]int64, error) {
	ids := make([]int64, 0)
	if parentID == 0 {
		return ids, nil
	}
	err := r.tx.DB(ctx).Model(&CommentPO{}).
		Where("parent_id = ?", parentID).
		Order("id ASC").Pluck("id", &ids).Error
	return ids, err
}

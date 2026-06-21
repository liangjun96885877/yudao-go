package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"yudao-go/internal/framework/orm"
)

// CRUD 是基于泛型的通用增删改查仓储，适用于 system 模块的单表实体。
// 逻辑删除：原版 deleted 为 bit(1)，读取统一过滤 deleted=0，删除置 deleted=1。
type CRUD[T any] struct {
	tx *orm.TxManager
}

func NewCRUD[T any](tx *orm.TxManager) *CRUD[T] { return &CRUD[T]{tx: tx} }

// Get 按主键查询。不存在返回 (nil, nil)。
func (c *CRUD[T]) Get(ctx context.Context, id int64) (*T, error) {
	var m T
	err := c.tx.DB(ctx).Where("deleted = 0").First(&m, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增记录，回写自增主键到 m。
func (c *CRUD[T]) Create(ctx context.Context, m *T) error {
	return c.tx.DB(ctx).Create(m).Error
}

// UpdateFields 按主键更新指定字段（用 map 以保证零值字段也能更新）。
func (c *CRUD[T]) UpdateFields(ctx context.Context, id int64, fields map[string]any) error {
	return c.tx.DB(ctx).Model(new(T)).
		Where("id = ? AND deleted = 0", id).Updates(fields).Error
}

// SoftDelete 逻辑删除若干记录。
func (c *CRUD[T]) SoftDelete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return c.tx.DB(ctx).Model(new(T)).
		Where("id IN ?", ids).Update("deleted", 1).Error
}

// List 按条件查询列表。apply 用于追加过滤与排序，可为 nil。
func (c *CRUD[T]) List(ctx context.Context, apply func(*gorm.DB) *gorm.DB) ([]*T, error) {
	q := c.tx.DB(ctx).Where("deleted = 0")
	if apply != nil {
		q = apply(q)
	}
	var list []*T
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Page 游标分页查询。apply 用于追加过滤与排序，可为 nil。
func (c *CRUD[T]) Page(
	ctx context.Context, offset, limit int, apply func(*gorm.DB) *gorm.DB,
) ([]*T, int64, error) {
	q := c.tx.DB(ctx).Model(new(T)).Where("deleted = 0")
	if apply != nil {
		q = apply(q)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*T
	if err := q.Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

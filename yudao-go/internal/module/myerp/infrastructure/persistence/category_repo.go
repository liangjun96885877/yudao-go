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

// CategoryRepo 是 CategoryRepository 的 GORM 实现。
type CategoryRepo struct {
	tx *orm.TxManager
}

func NewCategoryRepo(tx *orm.TxManager) *CategoryRepo { return &CategoryRepo{tx: tx} }

func (r *CategoryRepo) Create(ctx context.Context, c *model.Category) error {
	po := toCategoryPO(c)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	c.ID = po.ID
	c.CreateTime = po.CreateTime
	return nil
}

func (r *CategoryRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	res := r.tx.DB(ctx).Model(&CategoryPO{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errcode.NotFound
	}
	return nil
}

func (r *CategoryRepo) GetByID(ctx context.Context, id int64) (*model.Category, error) {
	var po CategoryPO
	err := r.tx.DB(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NotFound
	}
	if err != nil {
		return nil, err
	}
	return fromCategoryPO(&po), nil
}

func (r *CategoryRepo) GetByCode(ctx context.Context, code string) (*model.Category, error) {
	var po CategoryPO
	err := r.tx.DB(ctx).Where("code = ?", code).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromCategoryPO(&po), nil
}

func (r *CategoryRepo) DeleteByID(ctx context.Context, id int64) error {
	return r.tx.DB(ctx).Delete(&CategoryPO{}, id).Error
}

func (r *CategoryRepo) Page(ctx context.Context, q repository.CategoryQuery) ([]*model.Category, int64, error) {
	db := r.tx.DB(ctx).Model(&CategoryPO{})
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
	var pos []*CategoryPO
	offset := (q.PageNo - 1) * q.PageSize
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(q.PageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*model.Category, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromCategoryPO(po))
	}
	return out, total, nil
}

func (r *CategoryRepo) ListAll(ctx context.Context) ([]*model.Category, error) {
	var pos []*CategoryPO
	if err := r.tx.DB(ctx).Order("sort ASC, id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Category, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromCategoryPO(po))
	}
	return out, nil
}

func (r *CategoryRepo) HasChildren(ctx context.Context, id int64) (bool, error) {
	var cnt int64
	err := r.tx.DB(ctx).Model(&CategoryPO{}).Where("parent_id = ?", id).Limit(1).Count(&cnt).Error
	return cnt > 0, err
}

// AncestorChain 从 id 向上追到根,返回完整祖先链(含自身,根在最后)。
// 用于属性继承合并 + 循环引用检测。最多追 32 层防恶意构造死循环。
func (r *CategoryRepo) AncestorChain(ctx context.Context, id int64) ([]int64, error) {
	chain := make([]int64, 0, 8)
	current := id
	for depth := 0; depth < 32 && current != 0; depth++ {
		chain = append(chain, current)
		var po CategoryPO
		err := r.tx.DB(ctx).Select("parent_id").Where("id = ?", current).First(&po).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			break
		}
		if err != nil {
			return nil, err
		}
		current = po.ParentID
	}
	return chain, nil
}

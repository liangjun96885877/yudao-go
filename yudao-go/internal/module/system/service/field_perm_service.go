package service

import (
	"context"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/system/model"
)

// FieldPermService 解析与维护角色字段权限（横切能力 #14 · 完整档）。
type FieldPermService struct {
	tx *orm.TxManager
}

func NewFieldPermService(tx *orm.TxManager) *FieldPermService {
	return &FieldPermService{tx: tx}
}

// actionRank 用于多角色取最宽松：plain > mask > hide。
var actionRank = map[string]int{"hide": 0, "mask": 1, "plain": 2}

// Resolve 计算 userID 的有效字段权限。
// 字段脱敏对所有人（含超级管理员）一视同仁：要看明文须在「字段权限」中为对应角色配 plain。
func (s *FieldPermService) Resolve(ctx context.Context, userID int64) (*contextx.FieldPerm, error) {
	db := s.tx.DB(contextx.WithIgnoreDataPerm(ctx))
	var rows []struct {
		BizType string `gorm:"column:biz_type"`
		Field   string `gorm:"column:field"`
		Action  string `gorm:"column:action"`
	}
	db.Raw(`SELECT fp.biz_type, fp.field, fp.action FROM system_role_field_perm fp
		JOIN system_user_role ur ON ur.role_id = fp.role_id
		JOIN system_role r ON r.id = fp.role_id
		WHERE ur.user_id = ? AND fp.deleted = 0 AND r.status = 0 AND r.deleted = 0`,
		userID).Scan(&rows)
	actions := map[string]string{}
	for _, row := range rows {
		key := row.BizType + ":" + row.Field
		if cur := actions[key]; cur == "" || actionRank[row.Action] > actionRank[cur] {
			actions[key] = row.Action
		}
	}
	return &contextx.FieldPerm{Actions: actions}, nil
}

// ListByRole 返回某角色已配置的字段动作，键为 "bizType:field"。
func (s *FieldPermService) ListByRole(ctx context.Context, roleID int64) (map[string]string, error) {
	var rows []model.RoleFieldPerm
	err := s.tx.DB(ctx).Where("role_id = ?", roleID).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.BizType+":"+r.Field] = r.Action
	}
	return out, nil
}

// Save 全量重写某角色的字段权限。
func (s *FieldPermService) Save(ctx context.Context, roleID int64, items []model.RoleFieldPerm) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		db := s.tx.DB(ctx)
		if err := db.Unscoped().Where("role_id = ?", roleID).
			Delete(&model.RoleFieldPerm{}).Error; err != nil {
			return err
		}
		rows := make([]model.RoleFieldPerm, 0, len(items))
		for _, it := range items {
			if it.Action == "" || it.Action == "mask" {
				continue // 默认即 mask，不必落库
			}
			it.RoleID = roleID
			rows = append(rows, it)
		}
		if len(rows) == 0 {
			return nil
		}
		return db.Create(&rows).Error
	})
}

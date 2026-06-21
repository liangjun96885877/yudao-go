// Package repo 是 system 模块的仓储层。
package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/system/model"
)

// Repo 聚合 system 模块的数据访问。
type Repo struct {
	tx *orm.TxManager
}

func New(tx *orm.TxManager) *Repo { return &Repo{tx: tx} }

// notDeleted 过滤掉逻辑删除的行（原版 deleted 为 bit(1)）。
func notDeleted(db *gorm.DB) *gorm.DB { return db.Where("deleted = 0") }

func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// FindUserByUsername 按用户名查用户（租户由插件依据上下文自动过滤）。不存在返回 (nil, nil)。
func (r *Repo) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.tx.DB(ctx).Scopes(notDeleted).Where("username = ?", username).First(&u).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUser 按 ID 查用户。
func (r *Repo) GetUser(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.tx.DB(ctx).Scopes(notDeleted).First(&u, id).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListRoleIDsByUser 查询用户拥有的角色 ID。
func (r *Repo) ListRoleIDsByUser(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := r.tx.DB(ctx).Model(&model.UserRole{}).Scopes(notDeleted).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	return ids, err
}

// ListRolesByIDs 按 ID 批量查角色。
func (r *Repo) ListRolesByIDs(ctx context.Context, ids []int64) ([]*model.Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var roles []*model.Role
	err := r.tx.DB(ctx).Scopes(notDeleted).Where("id IN ?", ids).Find(&roles).Error
	return roles, err
}

// ListMenuIDsByRoles 查询角色关联的菜单 ID。
func (r *Repo) ListMenuIDsByRoles(ctx context.Context, roleIDs []int64) ([]int64, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var ids []int64
	err := r.tx.DB(ctx).Model(&model.RoleMenu{}).Scopes(notDeleted).
		Where("role_id IN ?", roleIDs).Pluck("menu_id", &ids).Error
	return ids, err
}

// ListAllEnabledMenus 查询所有启用的目录与菜单（不含按钮），按 sort 升序。
func (r *Repo) ListAllEnabledMenus(ctx context.Context) ([]*model.Menu, error) {
	var menus []*model.Menu
	err := r.tx.DB(ctx).Scopes(notDeleted).
		Where("status = 0 AND type IN (1, 2)").Order("sort ASC").Find(&menus).Error
	return menus, err
}

// ListMenusByIDs 按 ID 批量查启用的目录与菜单。
func (r *Repo) ListMenusByIDs(ctx context.Context, ids []int64) ([]*model.Menu, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var menus []*model.Menu
	err := r.tx.DB(ctx).Scopes(notDeleted).
		Where("id IN ? AND status = 0 AND type IN (1, 2)", ids).Order("sort ASC").Find(&menus).Error
	return menus, err
}

// ListPermissionsByRoles 查询角色拥有的全部权限码（含按钮 type=3），去重。
func (r *Repo) ListPermissionsByRoles(ctx context.Context, roleIDs []int64) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var perms []string
	err := r.tx.DB(ctx).Raw(`SELECT DISTINCT m.permission FROM system_menu m
		JOIN system_role_menu rm ON rm.menu_id = m.id
		WHERE rm.role_id IN ? AND rm.deleted = 0
		  AND m.permission <> '' AND m.status = 0 AND m.deleted = 0`, roleIDs).Scan(&perms).Error
	return perms, err
}

// FindTenantByName 按名称查租户。不存在返回 (nil, nil)。
func (r *Repo) FindTenantByName(ctx context.Context, name string) (*model.Tenant, error) {
	var t model.Tenant
	err := r.tx.DB(ctx).Scopes(notDeleted).Where("name = ?", name).First(&t).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateAccessToken 新增访问令牌。
func (r *Repo) CreateAccessToken(ctx context.Context, t *model.OAuth2AccessToken) error {
	return r.tx.DB(ctx).Create(t).Error
}

// FindAccessToken 按令牌串查询（忽略租户：鉴权时尚无租户上下文）。不存在返回 (nil, nil)。
func (r *Repo) FindAccessToken(ctx context.Context, token string) (*model.OAuth2AccessToken, error) {
	var t model.OAuth2AccessToken
	err := r.tx.DB(contextx.WithIgnoreTenant(ctx)).Scopes(notDeleted).
		Where("access_token = ?", token).First(&t).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindAccessTokenByRefresh 按刷新令牌查询（忽略租户）。不存在返回 (nil, nil)。
func (r *Repo) FindAccessTokenByRefresh(ctx context.Context, refreshToken string) (*model.OAuth2AccessToken, error) {
	var t model.OAuth2AccessToken
	err := r.tx.DB(contextx.WithIgnoreTenant(ctx)).Scopes(notDeleted).
		Where("refresh_token = ?", refreshToken).First(&t).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteAccessToken 物理删除访问令牌（登出）。
func (r *Repo) DeleteAccessToken(ctx context.Context, token string) error {
	return r.tx.DB(contextx.WithIgnoreTenant(ctx)).
		Where("access_token = ?", token).Delete(&model.OAuth2AccessToken{}).Error
}

// CreateLoginLog 写入一条登录日志。
func (r *Repo) CreateLoginLog(ctx context.Context, l *model.LoginLog) error {
	return r.tx.DB(ctx).Create(l).Error
}

// ListAllEnabledDictData 查询所有启用的字典数据。
func (r *Repo) ListAllEnabledDictData(ctx context.Context) ([]*model.DictData, error) {
	var data []*model.DictData
	err := r.tx.DB(ctx).Scopes(notDeleted).
		Where("status = 0").Order("dict_type ASC, sort ASC").Find(&data).Error
	return data, err
}

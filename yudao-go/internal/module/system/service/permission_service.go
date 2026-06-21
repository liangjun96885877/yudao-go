package service

import (
	"context"
	"sync"
	"time"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
)

// PermissionService 实现操作权限校验（横切能力 #7）：
// 判断当前用户是否拥有某权限码，并维护「已定义权限码」缓存。
type PermissionService struct {
	tx        *orm.TxManager
	mu        sync.RWMutex
	defined   map[string]bool
	definedAt time.Time
}

func NewPermissionService(tx *orm.TxManager) *PermissionService {
	return &PermissionService{tx: tx}
}

// IsDefinedPermission 判断 code 是否为系统中已定义的权限码（60s 缓存）。
func (s *PermissionService) IsDefinedPermission(code string) bool {
	s.mu.RLock()
	fresh := s.defined != nil && time.Since(s.definedAt) < 60*time.Second
	s.mu.RUnlock()
	if !fresh {
		s.reloadDefined()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defined[code]
}

func (s *PermissionService) reloadDefined() {
	ctx := contextx.WithIgnoreDataPerm(context.Background())
	var codes []string
	s.tx.DB(ctx).Raw(
		"SELECT DISTINCT permission FROM system_menu WHERE permission <> '' AND deleted = 0").
		Scan(&codes)
	m := make(map[string]bool, len(codes))
	for _, c := range codes {
		m[c] = true
	}
	s.mu.Lock()
	s.defined, s.definedAt = m, time.Now()
	s.mu.Unlock()
}

// HasPermission 判断当前用户是否拥有 code。超级管理员放行。
func (s *PermissionService) HasPermission(ctx context.Context, code string) bool {
	userID := contextx.UserID(ctx)
	if userID == 0 {
		return false
	}
	db := s.tx.DB(contextx.WithIgnoreDataPerm(ctx))
	// 超级管理员放行。
	var superCnt int64
	db.Raw(`SELECT COUNT(*) FROM system_role r
		JOIN system_user_role ur ON ur.role_id = r.id
		WHERE ur.user_id = ? AND r.code = 'super_admin' AND r.status = 0 AND r.deleted = 0`,
		userID).Scan(&superCnt)
	if superCnt > 0 {
		return true
	}
	// 普通用户：经 角色→角色菜单→菜单 查是否拥有该权限码。
	var cnt int64
	db.Raw(`SELECT COUNT(*) FROM system_menu m
		JOIN system_role_menu rm ON rm.menu_id = m.id
		JOIN system_user_role ur ON ur.role_id = rm.role_id
		JOIN system_role r ON r.id = rm.role_id
		WHERE ur.user_id = ? AND m.permission = ?
		  AND r.status = 0 AND r.deleted = 0 AND m.deleted = 0 AND m.status = 0`,
		userID, code).Scan(&cnt)
	return cnt > 0
}

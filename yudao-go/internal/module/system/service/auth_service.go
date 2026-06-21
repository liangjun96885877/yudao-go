package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/module/system/dto"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
)

// 用户类型常量（yudao 约定：会员=1，管理员=2）。
const userTypeAdmin int8 = 2

// 错误码（沿用原版 system 模块错误码段）。
var (
	errBadCredentials = errcode.New(1002003000, "登录失败，账号密码不正确")
	errUserDisabled   = errcode.New(1002003003, "登录失败，账号被禁用")
	errTenantNotFound = errcode.New(1004003000, "登录失败，租户不存在")
)

// AuthService 处理认证与权限信息。
type AuthService struct {
	repo  *repo.Repo
	token *TokenService
}

func NewAuthService(r *repo.Repo, token *TokenService) *AuthService {
	return &AuthService{repo: r, token: token}
}

// Login 账号密码登录。ip / userAgent 用于记录登录日志。
func (s *AuthService) Login(
	ctx context.Context, req *dto.LoginReq, ip, userAgent string,
) (*dto.LoginResp, error) {
	user, err := s.repo.FindUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errBadCredentials
	}
	// 校验 BCrypt 密码（兼容原版 Spring Security 加密的 $2a$ 哈希）。
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return nil, errBadCredentials
	}
	if user.Status != 0 {
		return nil, errUserDisabled
	}
	t, err := s.token.Create(ctx, user.ID, userTypeAdmin, user.TenantID)
	if err != nil {
		return nil, err
	}
	// 记录登录日志（log_type=100 账号密码登录，result=0 成功）；失败不影响登录。
	loginLog := &model.LoginLog{
		LogType: 100, TraceID: contextx.TraceID(ctx),
		UserID: user.ID, UserType: userTypeAdmin, Username: user.Username,
		Result: 0, UserIP: ip, UserAgent: userAgent,
	}
	loginLog.TenantID = user.TenantID
	_ = s.repo.CreateLoginLog(ctx, loginLog)

	return &dto.LoginResp{
		UserID:       user.ID,
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresTime:  t.ExpiresTime.UnixMilli(),
	}, nil
}

// RefreshToken 用刷新令牌换取新的访问令牌。
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*dto.LoginResp, error) {
	old, err := s.repo.FindAccessTokenByRefresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, errcode.Unauthorized.WithMsg("刷新令牌无效，请重新登录")
	}
	t, err := s.token.Create(ctx, old.UserID, old.UserType, old.TenantID)
	if err != nil {
		return nil, err
	}
	return &dto.LoginResp{
		UserID:       old.UserID,
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresTime:  t.ExpiresTime.UnixMilli(),
	}, nil
}

// Logout 注销当前令牌。
func (s *AuthService) Logout(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return nil
	}
	return s.token.Revoke(ctx, accessToken)
}

// TenantIDByName 按租户名称获取租户编号。
func (s *AuthService) TenantIDByName(ctx context.Context, name string) (int64, error) {
	t, err := s.repo.FindTenantByName(ctx, name)
	if err != nil {
		return 0, err
	}
	if t == nil {
		return 0, errTenantNotFound
	}
	return t.ID, nil
}

// GetPermissionInfo 返回当前用户的权限信息（用户、角色、权限、菜单树）。
func (s *AuthService) GetPermissionInfo(ctx context.Context) (*dto.PermissionInfoResp, error) {
	uid := contextx.UserID(ctx)
	if uid == 0 {
		return nil, errcode.Unauthorized
	}
	user, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errcode.Unauthorized
	}

	roleIDs, err := s.repo.ListRoleIDsByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	roles, err := s.repo.ListRolesByIDs(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	roleCodes := make([]string, 0, len(roles))
	superAdmin := false
	for _, r := range roles {
		roleCodes = append(roleCodes, r.Code)
		if r.Code == "super_admin" {
			superAdmin = true
		}
	}

	var menus []*model.Menu
	var permissions []string
	if superAdmin {
		// 超级管理员：全部菜单 + 通配权限。
		if menus, err = s.repo.ListAllEnabledMenus(ctx); err != nil {
			return nil, err
		}
		permissions = []string{"*:*:*"}
	} else {
		menuIDs, err := s.repo.ListMenuIDsByRoles(ctx, roleIDs)
		if err != nil {
			return nil, err
		}
		if menus, err = s.repo.ListMenusByIDs(ctx, menuIDs); err != nil {
			return nil, err
		}
		// 权限码须取自全部菜单（含按钮 type=3），不能只从目录/菜单的 menus 列表取。
		if permissions, err = s.repo.ListPermissionsByRoles(ctx, roleIDs); err != nil {
			return nil, err
		}
	}

	return &dto.PermissionInfoResp{
		User: dto.UserVO{
			ID: user.ID, Username: user.Username, Nickname: user.Nickname,
			Avatar: user.Avatar, DeptID: user.DeptID, Email: user.Email, Mobile: user.Mobile,
		},
		Roles:       roleCodes,
		Permissions: permissions,
		Menus:       buildMenuTree(menus),
	}, nil
}

// buildMenuTree 把扁平菜单列表组装成树（menus 已按 sort 升序）。
func buildMenuTree(menus []*model.Menu) []*dto.MenuVO {
	nodes := make(map[int64]*dto.MenuVO, len(menus))
	for _, m := range menus {
		nodes[m.ID] = &dto.MenuVO{
			ID: m.ID, ParentID: m.ParentID, Name: m.Name, Path: m.Path,
			Component: m.Component, ComponentName: m.ComponentName, Icon: m.Icon,
			Visible: m.Visible.Bool(), KeepAlive: m.KeepAlive.Bool(), AlwaysShow: m.AlwaysShow.Bool(),
		}
	}
	roots := make([]*dto.MenuVO, 0)
	for _, m := range menus {
		node := nodes[m.ID]
		if parent, ok := nodes[m.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node) // 父节点不在集合中 → 顶级
		}
	}
	return roots
}

// ListDictData 返回所有启用的字典数据。
func (s *AuthService) ListDictData(ctx context.Context) ([]*dto.DictDataVO, error) {
	data, err := s.repo.ListAllEnabledDictData(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.DictDataVO, 0, len(data))
	for _, d := range data {
		out = append(out, &dto.DictDataVO{
			DictType: d.DictType, Label: d.Label, Value: d.Value,
			ColorType: d.ColorType, CSSClass: d.CSSClass,
		})
	}
	return out, nil
}

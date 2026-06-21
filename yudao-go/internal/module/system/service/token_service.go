// Package service 是 system 模块的应用服务层。
package service

import (
	"context"
	"strings"
	"time"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/security"
	"yudao-go/internal/module/system/model"
	"yudao-go/internal/module/system/repo"
	"yudao-go/internal/pkg/errcode"
	"yudao-go/internal/pkg/idgen"
)

// accessTokenTTL 是访问令牌有效期。
const accessTokenTTL = 24 * time.Hour

// TokenService 负责 OAuth2 访问令牌的签发与校验。
// 实现 security.TokenValidator，供认证中间件使用。
type TokenService struct {
	repo *repo.Repo
}

func NewTokenService(r *repo.Repo) *TokenService { return &TokenService{repo: r} }

// token 生成无连字符的随机串。
func token() string { return strings.ReplaceAll(idgen.UUID(), "-", "") }

// Create 为用户签发新的访问令牌。
func (s *TokenService) Create(
	ctx context.Context, userID int64, userType int8, tenantID int64,
) (*model.OAuth2AccessToken, error) {
	t := &model.OAuth2AccessToken{
		UserID:       userID,
		UserType:     userType,
		UserInfo:     "{}",
		AccessToken:  token(),
		RefreshToken: token(),
		ClientID:     "default",
		Scopes:       "",
		ExpiresTime:  time.Now().Add(accessTokenTTL),
	}
	t.TenantID = tenantID
	if err := s.repo.CreateAccessToken(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Validate 校验访问令牌，返回登录用户。实现 security.TokenValidator。
func (s *TokenService) Validate(ctx context.Context, accessToken string) (*security.LoginUser, error) {
	t, err := s.repo.FindAccessToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errcode.Unauthorized
	}
	if time.Now().After(t.ExpiresTime) {
		return nil, errcode.Unauthorized.WithMsg("登录已过期，请重新登录")
	}
	// 加载用户名（忽略租户：鉴权阶段尚无租户上下文）。
	username := ""
	if u, _ := s.repo.GetUser(contextx.WithIgnoreTenant(ctx), t.UserID); u != nil {
		username = u.Username
	}
	return &security.LoginUser{
		ID:       t.UserID,
		TenantID: t.TenantID,
		UserType: t.UserType,
		Username: username,
	}, nil
}

// Revoke 注销访问令牌（登出）。
func (s *TokenService) Revoke(ctx context.Context, accessToken string) error {
	return s.repo.DeleteAccessToken(ctx, accessToken)
}

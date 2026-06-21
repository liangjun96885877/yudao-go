package service

import (
	"context"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/application/assembler"
	"yudao-go/internal/module/chatter/application/dto"
	"yudao-go/internal/module/chatter/domain/event"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/domain/repository"
	"yudao-go/internal/pkg/errcode"
	"yudao-go/internal/pkg/idgen"
)

// FollowerService 关注者应用服务。
type FollowerService struct {
	followers repository.FollowerRepository
	sink      EventSink
	tx        *orm.TxManager
	perm      Permission
}

func NewFollowerService(
	followers repository.FollowerRepository, sink EventSink, tx *orm.TxManager, perm Permission,
) *FollowerService {
	return &FollowerService{followers: followers, sink: sink, tx: tx, perm: perm}
}

// Follow 关注当前用户到某业务记录（幂等）。
func (s *FollowerService) Follow(ctx context.Context, req *dto.FollowReq) error {
	ref := bizRef(ctx, req.BizType, req.BizID)
	if !ref.Valid() {
		return errcode.BadRequest
	}
	if err := s.perm.CanRead(ctx, ref); err != nil {
		return err
	}
	actor := actorFromContext(ctx)
	f := &model.Follower{
		Ref:      ref,
		UserID:   actor.ID,
		UserName: actor.Name,
		Reason:   model.FollowManual,
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.followers.Add(ctx, f); err != nil {
			return err
		}
		return s.sink.Append(ctx, event.FollowerAdded{
			Base:   event.NewBase(idgen.UUID(), event.TopicFollowerAdded, "chatter_follower", actor.ID),
			Ref:    ref,
			Actor:  actor,
			UserID: actor.ID,
		})
	})
}

// Unfollow 取消当前用户的关注。
func (s *FollowerService) Unfollow(ctx context.Context, req *dto.FollowReq) error {
	ref := bizRef(ctx, req.BizType, req.BizID)
	if !ref.Valid() {
		return errcode.BadRequest
	}
	actor := actorFromContext(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.followers.Remove(ctx, ref, actor.ID)
	})
}

// UpdateSettings 更新当前用户的关注设置（订阅事件类型 + 静音）。
func (s *FollowerService) UpdateSettings(ctx context.Context, req *dto.FollowerSettingsReq) error {
	ref := bizRef(ctx, req.BizType, req.BizID)
	if !ref.Valid() {
		return errcode.BadRequest
	}
	actor := actorFromContext(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.followers.UpdateSettings(ctx, ref, actor.ID, req.SubscribeTypes, req.Muted)
	})
}

// List 列出某业务记录的关注者。
func (s *FollowerService) List(
	ctx context.Context, bizType string, bizID int64,
) ([]*dto.FollowerDTO, error) {
	ref := bizRef(ctx, bizType, bizID)
	if !ref.Valid() {
		return nil, errcode.BadRequest
	}
	if err := s.perm.CanRead(ctx, ref); err != nil {
		return nil, err
	}
	followers, err := s.followers.ListByBiz(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.FollowerDTO, 0, len(followers))
	for _, f := range followers {
		out = append(out, assembler.ToFollowerDTO(f))
	}
	return out, nil
}

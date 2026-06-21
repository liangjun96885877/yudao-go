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

// CommentService 评论应用服务。
type CommentService struct {
	comments repository.CommentRepository
	sink     EventSink
	tx       *orm.TxManager
	perm     Permission
}

func NewCommentService(
	comments repository.CommentRepository, sink EventSink, tx *orm.TxManager, perm Permission,
) *CommentService {
	return &CommentService{comments: comments, sink: sink, tx: tx, perm: perm}
}

// Add 新增评论。评论写入与事件发件箱在同一事务内原子提交。
func (s *CommentService) Add(ctx context.Context, req *dto.AddCommentReq) (*dto.CommentDTO, error) {
	ref := bizRef(ctx, req.BizType, req.BizID)
	if !ref.Valid() {
		return nil, errcode.BadRequest
	}
	if err := s.perm.CanRead(ctx, ref); err != nil {
		return nil, err
	}
	actor := actorFromContext(ctx)
	c := &model.Comment{
		Ref:            ref,
		ParentID:       req.ParentID,
		Content:        req.Content,
		ContentHTML:    renderHTML(req.Content),
		Author:         actor,
		MentionUserIDs: req.MentionUserIDs,
	}
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if c.ParentID != 0 { // 回复：继承父评论的顶层 ID
			parent, err := s.comments.GetByID(ctx, c.ParentID)
			if err != nil {
				return err
			}
			c.RootID = parent.RootID
		}
		if err := s.comments.Create(ctx, c); err != nil {
			return err
		}
		// 事务内写入发件箱：评论与事件原子提交，避免事件丢失。
		return s.sink.Append(ctx, event.CommentAdded{
			Base:      event.NewBase(idgen.UUID(), event.TopicCommentAdded, "chatter_comment", c.ID),
			Ref:       ref,
			Actor:     actor,
			CommentID: c.ID,
			Mentions:  req.MentionUserIDs,
		})
	})
	if err != nil {
		return nil, err
	}
	return assembler.ToCommentDTO(c), nil
}

// Update 编辑评论。仅作者可编辑；基于乐观锁版本防并发覆盖。
func (s *CommentService) Update(
	ctx context.Context, id int64, req *dto.UpdateCommentReq,
) (*dto.CommentDTO, error) {
	var result *model.Comment
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.comments.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if c.Author.ID != actorFromContext(ctx).ID {
			return errcode.Forbidden
		}
		if req.Version != c.Version {
			return errcode.Conflict
		}
		c.MarkEdited(req.Content, renderHTML(req.Content))
		if err := s.comments.Update(ctx, c); err != nil {
			return err
		}
		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return assembler.ToCommentDTO(result), nil
}

// Delete 删除评论（仅作者）。
func (s *CommentService) Delete(ctx context.Context, id int64) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.comments.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if c.Author.ID != actorFromContext(ctx).ID {
			return errcode.Forbidden
		}
		return s.comments.DeleteByID(ctx, id)
	})
}

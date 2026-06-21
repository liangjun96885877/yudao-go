package service

import (
	"context"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/application/assembler"
	"yudao-go/internal/module/chatter/application/dto"
	"yudao-go/internal/module/chatter/domain/model"
	"yudao-go/internal/module/chatter/domain/repository"
	"yudao-go/internal/pkg/errcode"
)

const defaultFeedLimit = 20
const maxFeedLimit = 50

// TimelineService 时间线查询应用服务。
type TimelineService struct {
	timelines repository.TimelineRepository
	comments  repository.CommentRepository
	tx        *orm.TxManager
	perm      Permission
}

func NewTimelineService(
	timelines repository.TimelineRepository,
	comments repository.CommentRepository,
	tx *orm.TxManager, perm Permission,
) *TimelineService {
	return &TimelineService{timelines: timelines, comments: comments, tx: tx, perm: perm}
}

// Feed 游标分页查询某业务记录的时间线，补齐字段变更明细与当前用户标记。
func (s *TimelineService) Feed(
	ctx context.Context, bizType string, bizID, cursor int64, limit int,
) (*dto.CursorPage[*dto.TimelineItemDTO], error) {
	ref := bizRef(ctx, bizType, bizID)
	if !ref.Valid() {
		return nil, errcode.BadRequest
	}
	if err := s.perm.CanRead(ctx, ref); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxFeedLimit {
		limit = defaultFeedLimit
	}

	timelines, err := s.timelines.PageByBiz(ctx, ref, cursor, limit)
	if err != nil {
		return nil, err
	}

	// 收集 update 类型条目，批量查询审计明细，避免 N+1。
	// 同时收集 comment 类条目的 refID(=comment.id),用于批量统计回复数(Axelor 风格的"replies (N)")。
	var updateIDs []int64
	var commentIDs []int64
	allIDs := make([]int64, 0, len(timelines))
	for _, t := range timelines {
		allIDs = append(allIDs, t.ID)
		if t.EventType == model.EventUpdate {
			updateIDs = append(updateIDs, t.ID)
		}
		if t.EventType == model.EventComment && t.RefID != 0 {
			commentIDs = append(commentIDs, t.RefID)
		}
	}
	logs, err := s.timelines.ListAuditLogs(ctx, updateIDs)
	if err != nil {
		return nil, err
	}
	logsByTimeline := make(map[int64][]*model.AuditLog, len(updateIDs))
	for _, l := range logs {
		logsByTimeline[l.TimelineID] = append(logsByTimeline[l.TimelineID], l)
	}
	replyCountByComment, err := s.comments.CountChildrenByParents(ctx, commentIDs)
	if err != nil {
		return nil, err
	}

	// 批量查询当前用户对这些条目的标记（已读 / 重要）。
	flagByTimeline := make(map[int64]*model.TimelineFlag)
	if uid := contextx.UserID(ctx); uid != 0 {
		flags, err := s.timelines.ListFlags(ctx, uid, allIDs)
		if err != nil {
			return nil, err
		}
		for _, f := range flags {
			flagByTimeline[f.TimelineID] = f
		}
	}

	items := make([]*dto.TimelineItemDTO, 0, len(timelines))
	var nextCursor int64
	for _, t := range timelines {
		item := assembler.ToTimelineItemDTO(t, logsByTimeline[t.ID])
		if f := flagByTimeline[t.ID]; f != nil {
			item.IsRead = f.IsRead
			item.IsImportant = f.IsImportant
		}
		if t.EventType == model.EventComment && t.RefID != 0 {
			item.ReplyCount = replyCountByComment[t.RefID]
		}
		items = append(items, item)
		nextCursor = t.ID // 列表按 id 降序，最后一条即下一页游标
	}
	if len(timelines) < limit {
		nextCursor = 0 // 无更多数据
	}
	return &dto.CursorPage[*dto.TimelineItemDTO]{List: items, NextCursor: nextCursor}, nil
}

// CommentReplies 返回某评论的所有直接子评论(对应的 timeline 条目),按时间升序。
// Axelor 风格:用户点击"replies (N)"按钮时调用,在父消息下方缩进展开。
func (s *TimelineService) CommentReplies(
	ctx context.Context, commentID int64,
) ([]*dto.TimelineItemDTO, error) {
	if commentID <= 0 {
		return nil, errcode.BadRequest
	}
	// 通过 comment.parent_id 找直接子评论的 id 列表
	childIDs, err := s.comments.ListIDsByParent(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if len(childIDs) == 0 {
		return []*dto.TimelineItemDTO{}, nil
	}
	// 再查这些 comment.id 对应的 timeline 条目
	timelines, err := s.timelines.ListByRefs(ctx, "comment", childIDs)
	if err != nil {
		return nil, err
	}
	// 权限:用第一条的 BizRef 做读权限校验(同一评论树必在同一业务记录上)。
	if len(timelines) > 0 {
		if err := s.perm.CanRead(ctx, timelines[0].Ref); err != nil {
			return nil, err
		}
	}
	// 当前用户标记 + 二级回复数(递归一层即可)。
	tIDs := make([]int64, 0, len(timelines))
	subCommentIDs := make([]int64, 0, len(timelines))
	for _, t := range timelines {
		tIDs = append(tIDs, t.ID)
		if t.RefID != 0 {
			subCommentIDs = append(subCommentIDs, t.RefID)
		}
	}
	flagByTimeline := make(map[int64]*model.TimelineFlag)
	if uid := contextx.UserID(ctx); uid != 0 {
		flags, err := s.timelines.ListFlags(ctx, uid, tIDs)
		if err != nil {
			return nil, err
		}
		for _, f := range flags {
			flagByTimeline[f.TimelineID] = f
		}
	}
	subReplyCount, err := s.comments.CountChildrenByParents(ctx, subCommentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.TimelineItemDTO, 0, len(timelines))
	for _, t := range timelines {
		item := assembler.ToTimelineItemDTO(t, nil)
		if f := flagByTimeline[t.ID]; f != nil {
			item.IsRead = f.IsRead
			item.IsImportant = f.IsImportant
		}
		item.ReplyCount = subReplyCount[t.RefID]
		out = append(out, item)
	}
	return out, nil
}

// SetFlag 设置当前用户对某条时间线的标记（已读 / 重要）。
func (s *TimelineService) SetFlag(
	ctx context.Context, timelineID int64, read, important bool,
) error {
	uid := contextx.UserID(ctx)
	if uid == 0 {
		return errcode.Unauthorized
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.timelines.UpsertFlag(ctx, timelineID, uid, read, important)
	})
}

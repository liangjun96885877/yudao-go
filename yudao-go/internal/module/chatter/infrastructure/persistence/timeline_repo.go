package persistence

import (
	"context"

	"gorm.io/gorm/clause"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/chatter/domain/model"
)

// TimelineRepo 是 TimelineRepository 的 GORM 实现。
type TimelineRepo struct {
	tx *orm.TxManager
}

func NewTimelineRepo(tx *orm.TxManager) *TimelineRepo { return &TimelineRepo{tx: tx} }

func (r *TimelineRepo) Save(ctx context.Context, t *model.Timeline) error {
	po := toTimelinePO(t)
	if err := r.tx.DB(ctx).Create(po).Error; err != nil {
		return err
	}
	t.ID = po.ID                 // 回填自增主键
	t.CreateTime = po.CreateTime // 回填创建时间
	return nil
}

func (r *TimelineRepo) SaveAuditLogs(ctx context.Context, logs []*model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	pos := make([]*AuditLogPO, 0, len(logs))
	for _, l := range logs {
		pos = append(pos, toAuditLogPO(l))
	}
	return r.tx.DB(ctx).Create(pos).Error // 批量插入
}

func (r *TimelineRepo) PageByBiz(
	ctx context.Context, ref model.BizRef, cursor int64, limit int,
) ([]*model.Timeline, error) {
	// 租户过滤由 ORM 多租户插件依据 context 自动追加。
	q := r.tx.DB(ctx).Model(&TimelinePO{}).
		Where("biz_type = ? AND biz_id = ?", ref.BizType, ref.BizID)
	if cursor > 0 { // 游标分页：取 id 更小的下一页
		q = q.Where("id < ?", cursor)
	}
	var pos []*TimelinePO
	if err := q.Order("id DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Timeline, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromTimelinePO(po))
	}
	return out, nil
}

func (r *TimelineRepo) ListAuditLogs(
	ctx context.Context, timelineIDs []int64,
) ([]*model.AuditLog, error) {
	if len(timelineIDs) == 0 {
		return nil, nil
	}
	var pos []*AuditLogPO
	if err := r.tx.DB(ctx).
		Where("timeline_id IN ?", timelineIDs).
		Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.AuditLog, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromAuditLogPO(po))
	}
	return out, nil
}

// ListFlags 查询某用户对一批时间线的标记。
func (r *TimelineRepo) ListFlags(
	ctx context.Context, userID int64, timelineIDs []int64,
) ([]*model.TimelineFlag, error) {
	if userID == 0 || len(timelineIDs) == 0 {
		return nil, nil
	}
	var pos []*TimelineFlagPO
	if err := r.tx.DB(ctx).
		Where("user_id = ? AND timeline_id IN ?", userID, timelineIDs).
		Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.TimelineFlag, 0, len(pos))
	for _, po := range pos {
		out = append(out, &model.TimelineFlag{
			TimelineID: po.TimelineID, UserID: po.UserID,
			IsRead: po.IsRead, IsImportant: po.IsImportant,
		})
	}
	return out, nil
}

// UpsertFlag 写入/更新某用户对某条时间线的标记。
func (r *TimelineRepo) UpsertFlag(
	ctx context.Context, timelineID, userID int64, read, important bool,
) error {
	po := &TimelineFlagPO{
		TimelineID: timelineID, UserID: userID, IsRead: read, IsImportant: important,
	}
	return r.tx.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "timeline_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"is_read", "is_important"}),
	}).Create(po).Error
}

func (r *TimelineRepo) ExistsByEventID(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := r.tx.DB(ctx).Model(&TimelinePO{}).
		Where("event_id = ?", eventID).Limit(1).Count(&count).Error
	return count > 0, err
}

// ListByRefs 按 ref_type + ref_id IN(...) 查询时间线条目,按 id 升序(回复链按时间顺序展示)。
func (r *TimelineRepo) ListByRefs(
	ctx context.Context, refType string, refIDs []int64,
) ([]*model.Timeline, error) {
	if len(refIDs) == 0 {
		return nil, nil
	}
	var pos []*TimelinePO
	if err := r.tx.DB(ctx).
		Where("ref_type = ? AND ref_id IN ?", refType, refIDs).
		Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Timeline, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromTimelinePO(po))
	}
	return out, nil
}

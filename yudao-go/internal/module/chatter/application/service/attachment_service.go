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

// AttachmentService 附件应用服务。
type AttachmentService struct {
	attachments repository.AttachmentRepository
	sink        EventSink
	tx          *orm.TxManager
	perm        Permission
}

func NewAttachmentService(
	attachments repository.AttachmentRepository, sink EventSink, tx *orm.TxManager, perm Permission,
) *AttachmentService {
	return &AttachmentService{attachments: attachments, sink: sink, tx: tx, perm: perm}
}

// Link 将已上传的文件关联到业务记录。附件写入与事件发件箱在同一事务内提交。
func (s *AttachmentService) Link(
	ctx context.Context, req *dto.LinkAttachmentReq,
) ([]*dto.AttachmentDTO, error) {
	ref := bizRef(ctx, req.BizType, req.BizID)
	if !ref.Valid() || len(req.Files) == 0 {
		return nil, errcode.BadRequest
	}
	if err := s.perm.CanRead(ctx, ref); err != nil {
		return nil, err
	}
	actor := actorFromContext(ctx)
	items := make([]*model.Attachment, 0, len(req.Files))
	for _, f := range req.Files {
		items = append(items, &model.Attachment{
			Ref:         ref,
			FileID:      f.FileID,
			FileName:    f.FileName,
			FileURL:     f.FileURL,
			FileSize:    f.FileSize,
			ContentType: f.ContentType,
			Uploader:    actor,
		})
	}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.attachments.CreateBatch(ctx, items); err != nil {
			return err
		}
		for _, it := range items {
			if err := s.sink.Append(ctx, event.AttachmentAdded{
				Base:         event.NewBase(idgen.UUID(), event.TopicAttachmentAdded, "chatter_attachment", it.ID),
				Ref:          ref,
				Actor:        actor,
				AttachmentID: it.ID,
				FileName:     it.FileName,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	out := make([]*dto.AttachmentDTO, 0, len(items))
	for _, it := range items {
		out = append(out, assembler.ToAttachmentDTO(it))
	}
	return out, nil
}

// List 列出某业务记录的附件。
func (s *AttachmentService) List(
	ctx context.Context, bizType string, bizID int64,
) ([]*dto.AttachmentDTO, error) {
	ref := bizRef(ctx, bizType, bizID)
	if !ref.Valid() {
		return nil, errcode.BadRequest
	}
	if err := s.perm.CanRead(ctx, ref); err != nil {
		return nil, err
	}
	list, err := s.attachments.ListByBiz(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.AttachmentDTO, 0, len(list))
	for _, a := range list {
		out = append(out, assembler.ToAttachmentDTO(a))
	}
	return out, nil
}

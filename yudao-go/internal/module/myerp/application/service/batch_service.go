package service

import (
	"context"
	"html"
	"strings"
	"time"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

// BatchService 产品批次应用服务(浮动双计量产品的库存载体)。
type BatchService struct {
	batches  repository.BatchRepository
	products repository.ProductRepository
	uoms     repository.UomRepository
	tx       *orm.TxManager
}

func NewBatchService(
	batches repository.BatchRepository,
	products repository.ProductRepository,
	uoms repository.UomRepository,
	tx *orm.TxManager,
) *BatchService {
	return &BatchService{batches: batches, products: products, uoms: uoms, tx: tx}
}

func (s *BatchService) Create(ctx context.Context, req *dto.BatchSaveReq) (int64, error) {
	batchNo := strings.TrimSpace(req.BatchNo)
	if batchNo == "" {
		return 0, badReq("批次号不能为空")
	}
	if len(batchNo) > 64 {
		return 0, badReq("批次号最长 64 字符")
	}
	p, err := s.products.GetByID(ctx, req.ProductID)
	if err != nil {
		if errIsNotFound(err) {
			return 0, badReq("产品不存在")
		}
		return 0, err
	}
	if p.UomMode != model.UomModeFloat {
		return 0, badReq("仅浮动双计量产品支持批次管理")
	}
	if !p.BatchTracked {
		return 0, badReq("该产品未启用批次管理(batch-less 模式)")
	}
	if exist, _ := s.batches.GetByNo(ctx, req.ProductID, batchNo); exist != nil {
		return 0, badReq("该产品下批次号已存在")
	}
	produceDate, err := parseDatePtr(req.ProduceDate)
	if err != nil {
		return 0, badReq("生产日期格式应为 2006-01-02")
	}
	expireDate, err := parseDatePtr(req.ExpireDate)
	if err != nil {
		return 0, badReq("到期日期格式应为 2006-01-02")
	}
	if err := validateFactorPtr(req.ActualFactor, "实测换算率"); err != nil {
		return 0, err
	}
	b := &model.ProductBatch{
		TenantID:     contextx.TenantID(ctx),
		ProductID:    req.ProductID,
		BatchNo:      batchNo,
		ActualFactor: req.ActualFactor,
		StockBase:    "0",
		StockAux:     "0",
		ProduceDate:  produceDate,
		ExpireDate:   expireDate,
		Status:       req.Status,
		Remark:       html.EscapeString(req.Remark),
	}
	if err := s.batches.Create(ctx, b); err != nil {
		return 0, wrapDuplicateError(err, "该产品下批次号已存在")
	}
	return b.ID, nil
}

func (s *BatchService) Update(ctx context.Context, req *dto.BatchSaveReq) error {
	cur, err := s.batches.GetByID(ctx, req.ID)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("批次不存在")
		}
		return err
	}
	batchNo := strings.TrimSpace(req.BatchNo)
	if batchNo == "" {
		return badReq("批次号不能为空")
	}
	if batchNo != cur.BatchNo {
		if exist, _ := s.batches.GetByNo(ctx, cur.ProductID, batchNo); exist != nil {
			return badReq("该产品下批次号已存在")
		}
	}
	produceDate, err := parseDatePtr(req.ProduceDate)
	if err != nil {
		return badReq("生产日期格式应为 2006-01-02")
	}
	expireDate, err := parseDatePtr(req.ExpireDate)
	if err != nil {
		return badReq("到期日期格式应为 2006-01-02")
	}
	if err := validateFactorPtr(req.ActualFactor, "实测换算率"); err != nil {
		return err
	}
	return s.batches.Update(ctx, req.ID, map[string]any{
		"batch_no":      batchNo,
		"actual_factor": req.ActualFactor,
		"produce_date":  produceDate,
		"expire_date":   expireDate,
		"status":        req.Status,
		"remark":        html.EscapeString(req.Remark),
	})
}

func (s *BatchService) Delete(ctx context.Context, id int64) error {
	cur, err := s.batches.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return badReq("批次不存在")
		}
		return err
	}
	// 有结存的批次禁删,否则产品库存投影会虚高
	if cmpDecimal(cur.StockBase, "0") != 0 || cmpDecimal(cur.StockAux, "0") != 0 {
		return badReq("该批次仍有结存,不能删除")
	}
	return s.batches.DeleteByID(ctx, id)
}

func (s *BatchService) Get(ctx context.Context, id int64) (*dto.BatchDTO, error) {
	b, err := s.batches.GetByID(ctx, id)
	if err != nil {
		if errIsNotFound(err) {
			return nil, badReq("批次不存在")
		}
		return nil, err
	}
	d := batchToDTO(b)
	s.fillBatchNames(ctx, []*dto.BatchDTO{d}, []*model.ProductBatch{b})
	return d, nil
}

func (s *BatchService) Page(ctx context.Context, q repository.BatchQuery) (*dto.Page[*dto.BatchDTO], error) {
	list, total, err := s.batches.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.BatchDTO, 0, len(list))
	for _, b := range list {
		items = append(items, batchToDTO(b))
	}
	s.fillBatchNames(ctx, items, list)
	return &dto.Page[*dto.BatchDTO]{List: items, Total: total}, nil
}

// fillBatchNames 批量回填产品名 + 主/辅单位名(防 N+1)。
func (s *BatchService) fillBatchNames(ctx context.Context, dtos []*dto.BatchDTO, batches []*model.ProductBatch) {
	if len(batches) == 0 {
		return
	}
	prodCache := make(map[int64]*model.Product)
	uomIDSet := make(map[int64]bool)
	for _, b := range batches {
		if _, ok := prodCache[b.ProductID]; !ok {
			p, _ := s.products.GetByID(ctx, b.ProductID)
			prodCache[b.ProductID] = p
			if p != nil {
				if p.BaseUomID != nil {
					uomIDSet[*p.BaseUomID] = true
				}
				if p.AuxUomID != nil {
					uomIDSet[*p.AuxUomID] = true
				}
			}
		}
	}
	uomIDs := make([]int64, 0, len(uomIDSet))
	for id := range uomIDSet {
		uomIDs = append(uomIDs, id)
	}
	uomList, _ := s.uoms.ListByIDs(ctx, uomIDs)
	uomName := make(map[int64]string, len(uomList))
	for _, u := range uomList {
		uomName[u.ID] = u.Name
	}
	for i, b := range batches {
		p := prodCache[b.ProductID]
		if p == nil {
			continue
		}
		dtos[i].ProductName = p.Name
		if p.BaseUomID != nil {
			dtos[i].BaseUomName = uomName[*p.BaseUomID]
		}
		if p.AuxUomID != nil {
			dtos[i].AuxUomName = uomName[*p.AuxUomID]
		}
	}
}

func batchToDTO(b *model.ProductBatch) *dto.BatchDTO {
	d := &dto.BatchDTO{
		ID:           b.ID,
		ProductID:    b.ProductID,
		BatchNo:      b.BatchNo,
		ActualFactor: b.ActualFactor,
		StockBase:    b.StockBase,
		StockAux:     b.StockAux,
		Status:       b.Status,
		Remark:       b.Remark,
		CreateTime:   b.CreateTime.Format("2006-01-02 15:04:05"),
	}
	if b.ProduceDate != nil {
		d.ProduceDate = b.ProduceDate.Format("2006-01-02")
	}
	if b.ExpireDate != nil {
		d.ExpireDate = b.ExpireDate.Format("2006-01-02")
	}
	return d
}

// parseDatePtr 解析 YYYY-MM-DD,空串返回 nil。
func parseDatePtr(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// validateFactorPtr 校验可空的换算率字符串(若非空必须为正数)。
func validateFactorPtr(f *string, label string) error {
	if f == nil {
		return nil
	}
	v := strings.TrimSpace(*f)
	if v == "" {
		return nil
	}
	if !decimalPattern.MatchString(v) || strings.HasPrefix(v, "-") || v == "0" {
		return badReq("%s必须为正数", label)
	}
	return nil
}

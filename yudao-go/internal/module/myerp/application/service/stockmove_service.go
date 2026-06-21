package service

import (
	"context"
	"html"
	"math"
	"strconv"
	"strings"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/application/dto"
	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
)

// StockMoveService 出入库流水(账本)。库存 = 流水之和;批次/产品结存是投影。
type StockMoveService struct {
	moves    repository.StockMoveRepository
	batches  repository.BatchRepository
	products repository.ProductRepository
	tx       *orm.TxManager
}

func NewStockMoveService(
	moves repository.StockMoveRepository,
	batches repository.BatchRepository,
	products repository.ProductRepository,
	tx *orm.TxManager,
) *StockMoveService {
	return &StockMoveService{moves: moves, batches: batches, products: products, tx: tx}
}

// Record 记一笔出入库:校验 → 算实际换算率 → 偏差校验 → 事务内写流水 + 更新批次/产品投影。
func (s *StockMoveService) Record(ctx context.Context, req *dto.StockMoveReq) (int64, error) {
	if req.MoveType != model.MoveIn && req.MoveType != model.MoveOut && req.MoveType != model.MoveAdjust {
		return 0, badReq("非法出入库类型")
	}
	p, err := s.products.GetByID(ctx, req.ProductID)
	if err != nil {
		if errIsNotFound(err) {
			return 0, badReq("产品不存在")
		}
		return 0, err
	}
	if p.UomMode != model.UomModeFloat {
		return 0, badReq("仅浮动双计量产品支持出入库记账")
	}

	// batch_tracked=true:批次必填(按批管理);=false:batch-less,batchId 必须空,直接产品级记账
	var batch *model.ProductBatch
	if p.BatchTracked {
		if req.BatchID == nil {
			return 0, badReq("请选择批次")
		}
		b, err := s.batches.GetByID(ctx, *req.BatchID)
		if err != nil {
			if errIsNotFound(err) {
				return 0, badReq("批次不存在")
			}
			return 0, err
		}
		if b.ProductID != req.ProductID {
			return 0, badReq("批次不属于该产品")
		}
		batch = b
	} else if req.BatchID != nil {
		return 0, badReq("该产品未启用批次管理,出入库不应带批次")
	}

	// 校验数量。入/出:正数;调整:带符号非零增量。
	allowNeg := req.MoveType == model.MoveAdjust
	if err := validateQty(req.QtyBase, "主计量数量", allowNeg); err != nil {
		return 0, err
	}
	if err := validateQty(req.QtyAux, "辅计量数量", allowNeg); err != nil {
		return 0, err
	}

	deltaBase := directedQty(req.MoveType, req.QtyBase)
	deltaAux := directedQty(req.MoveType, req.QtyAux)

	// 出库 / 负向调整:结存必须足够。按批次产品看批次结存,batch-less 看产品结存
	if cmpDecimal(deltaBase, "0") < 0 {
		var curBase, curAux, where string
		if batch != nil {
			curBase, curAux, where = batch.StockBase, batch.StockAux, "批次 "+batch.BatchNo
		} else {
			curBase, curAux, where = p.Stock, p.StockAux, "产品 "+p.Name
		}
		if cmpDecimal(addDecimal(curBase, deltaBase), "0") < 0 {
			return 0, badReq("%s 主计量结存不足(现存 %s)", where, curBase)
		}
		if cmpDecimal(addDecimal(curAux, deltaAux), "0") < 0 {
			return 0, badReq("%s 辅计量结存不足(现存 %s)", where, curAux)
		}
	}

	eff := computeEffFactor(req.QtyBase, req.QtyAux)
	if err := checkTolerance(p, eff); err != nil {
		return 0, err
	}

	m := &model.StockMove{
		TenantID:        contextx.TenantID(ctx),
		ProductID:       req.ProductID,
		BatchID:         req.BatchID,
		MoveType:        req.MoveType,
		QtyBase:         deltaBase,
		QtyAux:          deltaAux,
		EffectiveFactor: eff,
		Remark:          html.EscapeString(req.Remark),
		Creator:         contextx.UserName(ctx),
	}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.moves.Create(ctx, m); err != nil {
			return err
		}
		if batch != nil {
			if err := s.batches.AddStock(ctx, batch.ID, deltaBase, deltaAux); err != nil {
				return err
			}
		}
		if err := s.products.AddStock(ctx, req.ProductID, deltaBase, deltaAux); err != nil {
			return err
		}
		// 入库且批次尚未标定实测率 → 用本次实际换算率标定
		if batch != nil && req.MoveType == model.MoveIn && batch.ActualFactor == nil && eff != nil {
			return s.batches.Update(ctx, batch.ID, map[string]any{"actual_factor": *eff})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return m.ID, nil
}

func (s *StockMoveService) Page(ctx context.Context, q repository.StockMoveQuery) (*dto.Page[*dto.StockMoveDTO], error) {
	list, total, err := s.moves.Page(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.StockMoveDTO, 0, len(list))
	batchNoCache := make(map[int64]string)
	for _, m := range list {
		d := &dto.StockMoveDTO{
			ID:              m.ID,
			ProductID:       m.ProductID,
			BatchID:         m.BatchID,
			MoveType:        m.MoveType,
			QtyBase:         m.QtyBase,
			QtyAux:          m.QtyAux,
			EffectiveFactor: m.EffectiveFactor,
			Remark:          m.Remark,
			Creator:         m.Creator,
			CreateTime:      m.CreateTime.Format("2006-01-02 15:04:05"),
		}
		if m.BatchID != nil {
			no, ok := batchNoCache[*m.BatchID]
			if !ok {
				if b, _ := s.batches.GetByID(ctx, *m.BatchID); b != nil {
					no = b.BatchNo
				}
				batchNoCache[*m.BatchID] = no
			}
			d.BatchNo = no
		}
		items = append(items, d)
	}
	return &dto.Page[*dto.StockMoveDTO]{List: items, Total: total}, nil
}

// validateQty 校验数量字符串。allowNeg=false 时必须为正数。
func validateQty(qty, label string, allowNeg bool) error {
	qty = strings.TrimSpace(qty)
	if qty == "" || qty == "0" || qty == "-0" {
		return badReq("%s不能为空或 0", label)
	}
	if !decimalPattern.MatchString(qty) {
		return badReq("%s必须是数字", label)
	}
	if !allowNeg && strings.HasPrefix(qty, "-") {
		return badReq("%s必须为正数", label)
	}
	return nil
}

// directedQty 按出入库方向给数量定符号:出库取负,入库/调整原样。
func directedQty(moveType int8, qty string) string {
	qty = strings.TrimSpace(qty)
	if moveType == model.MoveOut {
		if qty == "0" || qty == "" || strings.HasPrefix(qty, "-") {
			return qty
		}
		return "-" + qty
	}
	return qty
}

// computeEffFactor 算本次实际换算率 |qtyAux/qtyBase|(float 足够,仅用于展示+偏差校验)。
func computeEffFactor(qtyBase, qtyAux string) *string {
	qb, e1 := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(qtyBase), "-"), 64)
	qa, e2 := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(qtyAux), "-"), 64)
	if e1 != nil || e2 != nil || qb == 0 {
		return nil
	}
	s := strconv.FormatFloat(qa/qb, 'f', 6, 64)
	return &s
}

// checkTolerance 浮动产品:实际换算率偏离名义率超过容差则拒。
func checkTolerance(p *model.Product, eff *string) error {
	if eff == nil || p.NominalFactor == nil {
		return nil
	}
	tol, err := strconv.ParseFloat(strings.TrimSpace(p.TolerancePct), 64)
	if err != nil || tol <= 0 {
		return nil
	}
	nf, err := strconv.ParseFloat(strings.TrimSpace(*p.NominalFactor), 64)
	if err != nil || nf <= 0 {
		return nil
	}
	ev, _ := strconv.ParseFloat(*eff, 64)
	dev := math.Abs(ev-nf) / nf * 100
	if dev > tol {
		return badReq("本次换算率 %s 偏离名义值 %s 达 %.2f%%,超过允许 %s%%", *eff, *p.NominalFactor, dev, p.TolerancePct)
	}
	return nil
}

// addDecimal 计算 a + b(可带负号),用 float 估算结存够不够(校验用,非记账)。
// 记账增量由 SQL DECIMAL 精确累加。
func addDecimal(a, b string) string {
	fa, _ := strconv.ParseFloat(strings.TrimSpace(a), 64)
	fb, _ := strconv.ParseFloat(strings.TrimSpace(b), 64)
	return strconv.FormatFloat(fa+fb, 'f', 6, 64)
}

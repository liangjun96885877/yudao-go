package persistence

import (
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/module/myerp/domain/model"
)

// === Category ===

func toCategoryPO(c *model.Category) *CategoryPO {
	po := &CategoryPO{
		Name:               c.Name,
		ParentID:           c.ParentID,
		Code:               c.Code,
		Sort:               c.Sort,
		Status:             c.Status,
		InheritParentAttrs: orm.Bit(c.InheritParentAttrs),
		Description:        c.Description,
	}
	po.ID = c.ID
	po.TenantID = c.TenantID
	po.Creator = c.Creator
	po.Updater = c.Updater
	po.CreateTime = c.CreateTime
	po.UpdateTime = c.UpdateTime
	return po
}

func fromCategoryPO(po *CategoryPO) *model.Category {
	return &model.Category{
		ID:                 po.ID,
		TenantID:           po.TenantID,
		Name:               po.Name,
		ParentID:           po.ParentID,
		Code:               po.Code,
		Sort:               po.Sort,
		Status:             po.Status,
		InheritParentAttrs: bool(po.InheritParentAttrs),
		Description:        po.Description,
		Creator:            po.Creator,
		Updater:            po.Updater,
		CreateTime:         po.CreateTime,
		UpdateTime:         po.UpdateTime,
	}
}

// === Attribute ===

func toAttributePO(a *model.Attribute) *AttributePO {
	po := &AttributePO{
		CategoryID:   a.CategoryID,
		Code:         a.Code,
		Name:         a.Name,
		InputType:    string(a.InputType),
		Unit:         a.Unit,
		Required:     orm.Bit(a.Required),
		Searchable:   orm.Bit(a.Searchable),
		ShowInList:   orm.Bit(a.ShowInList),
		IsVariant:    a.IsVariant,
		MinValue:     a.MinValue,
		MaxValue:     a.MaxValue,
		MinLength:    a.MinLength,
		MaxLength:    a.MaxLength,
		Regex:        a.Regex,
		DefaultValue: a.DefaultValue,
		Sort:         a.Sort,
		Status:       a.Status,
		Description:  a.Description,
	}
	po.ID = a.ID
	po.TenantID = a.TenantID
	po.Creator = a.Creator
	po.Updater = a.Updater
	po.CreateTime = a.CreateTime
	po.UpdateTime = a.UpdateTime
	return po
}

func fromAttributePO(po *AttributePO) *model.Attribute {
	return &model.Attribute{
		ID:           po.ID,
		TenantID:     po.TenantID,
		CategoryID:   po.CategoryID,
		Code:         po.Code,
		Name:         po.Name,
		InputType:    model.InputType(po.InputType),
		Unit:         po.Unit,
		Required:     bool(po.Required),
		Searchable:   bool(po.Searchable),
		ShowInList:   bool(po.ShowInList),
		IsVariant:    po.IsVariant,
		MinValue:     po.MinValue,
		MaxValue:     po.MaxValue,
		MinLength:    po.MinLength,
		MaxLength:    po.MaxLength,
		Regex:        po.Regex,
		DefaultValue: po.DefaultValue,
		Sort:         po.Sort,
		Status:       po.Status,
		Description:  po.Description,
		Creator:      po.Creator,
		Updater:      po.Updater,
		CreateTime:   po.CreateTime,
		UpdateTime:   po.UpdateTime,
	}
}

// === AttributeOption ===

func toAttributeOptionPO(o *model.AttributeOption) *AttributeOptionPO {
	return &AttributeOptionPO{
		LightBase: LightBase{
			ID: o.ID, TenantID: o.TenantID, CreateTime: o.CreateTime,
		},
		AttributeID: o.AttributeID,
		Value:       o.Value,
		Sort:        o.Sort,
		PriceExtra:  o.PriceExtra,
	}
}

func fromAttributeOptionPO(po *AttributeOptionPO) *model.AttributeOption {
	return &model.AttributeOption{
		ID:          po.ID,
		TenantID:    po.TenantID,
		AttributeID: po.AttributeID,
		Value:       po.Value,
		Sort:        po.Sort,
		PriceExtra:  po.PriceExtra,
		CreateTime:  po.CreateTime,
	}
}

// === Product ===

func toProductPO(p *model.Product) *ProductPO {
	po := &ProductPO{
		CategoryID:    p.CategoryID,
		TemplateID:    p.TemplateID,
		BaseUomID:     p.BaseUomID,
		UomMode:       p.UomMode,
		AuxUomID:      p.AuxUomID,
		NominalFactor: p.NominalFactor,
		TolerancePct:  p.TolerancePct,
		BatchTracked:  p.BatchTracked,
		Code:          p.Code,
		Name:          p.Name,
		BarCode:       p.BarCode,
		PicURL:        p.PicURL,
		Description:   p.Description,
		PurchasePrice: p.PurchasePrice,
		SalePrice:     p.SalePrice,
		Stock:         p.Stock,
		StockAux:      p.StockAux,
		Status:        p.Status,
		OwnerUserID:   p.OwnerUserID,
	}
	po.ID = p.ID
	po.TenantID = p.TenantID
	po.Creator = p.Creator
	po.Updater = p.Updater
	po.CreateTime = p.CreateTime
	po.UpdateTime = p.UpdateTime
	return po
}

func fromProductPO(po *ProductPO) *model.Product {
	return &model.Product{
		ID:            po.ID,
		TenantID:      po.TenantID,
		CategoryID:    po.CategoryID,
		TemplateID:    po.TemplateID,
		BaseUomID:     po.BaseUomID,
		UomMode:       po.UomMode,
		AuxUomID:      po.AuxUomID,
		NominalFactor: po.NominalFactor,
		TolerancePct:  po.TolerancePct,
		BatchTracked:  po.BatchTracked,
		Code:          po.Code,
		Name:          po.Name,
		BarCode:       po.BarCode,
		PicURL:        po.PicURL,
		Description:   po.Description,
		PurchasePrice: po.PurchasePrice,
		SalePrice:     po.SalePrice,
		Stock:         po.Stock,
		StockAux:      po.StockAux,
		Status:        po.Status,
		OwnerUserID:   po.OwnerUserID,
		Creator:       po.Creator,
		Updater:       po.Updater,
		CreateTime:    po.CreateTime,
		UpdateTime:    po.UpdateTime,
	}
}

// === ProductBatch ===

func toProductBatchPO(b *model.ProductBatch) *ProductBatchPO {
	po := &ProductBatchPO{
		ProductID:    b.ProductID,
		BatchNo:      b.BatchNo,
		ActualFactor: b.ActualFactor,
		StockBase:    b.StockBase,
		StockAux:     b.StockAux,
		ProduceDate:  b.ProduceDate,
		ExpireDate:   b.ExpireDate,
		Status:       b.Status,
		Remark:       b.Remark,
	}
	po.ID = b.ID
	po.TenantID = b.TenantID
	po.Creator = b.Creator
	po.Updater = b.Updater
	po.CreateTime = b.CreateTime
	po.UpdateTime = b.UpdateTime
	return po
}

func fromProductBatchPO(po *ProductBatchPO) *model.ProductBatch {
	return &model.ProductBatch{
		ID:           po.ID,
		TenantID:     po.TenantID,
		ProductID:    po.ProductID,
		BatchNo:      po.BatchNo,
		ActualFactor: po.ActualFactor,
		StockBase:    po.StockBase,
		StockAux:     po.StockAux,
		ProduceDate:  po.ProduceDate,
		ExpireDate:   po.ExpireDate,
		Status:       po.Status,
		Remark:       po.Remark,
		Creator:      po.Creator,
		Updater:      po.Updater,
		CreateTime:   po.CreateTime,
		UpdateTime:   po.UpdateTime,
	}
}

// === StockMove ===

func toStockMovePO(m *model.StockMove) *StockMovePO {
	po := &StockMovePO{
		ProductID:       m.ProductID,
		BatchID:         m.BatchID,
		MoveType:        m.MoveType,
		QtyBase:         m.QtyBase,
		QtyAux:          m.QtyAux,
		EffectiveFactor: m.EffectiveFactor,
		BizType:         m.BizType,
		BizID:           m.BizID,
		Remark:          m.Remark,
		Creator:         m.Creator,
	}
	po.ID = m.ID
	po.TenantID = m.TenantID
	po.CreateTime = m.CreateTime
	return po
}

func fromStockMovePO(po *StockMovePO) *model.StockMove {
	return &model.StockMove{
		ID:              po.ID,
		TenantID:        po.TenantID,
		ProductID:       po.ProductID,
		BatchID:         po.BatchID,
		MoveType:        po.MoveType,
		QtyBase:         po.QtyBase,
		QtyAux:          po.QtyAux,
		EffectiveFactor: po.EffectiveFactor,
		BizType:         po.BizType,
		BizID:           po.BizID,
		Remark:          po.Remark,
		Creator:         po.Creator,
		CreateTime:      po.CreateTime,
	}
}

// === ProductTemplate ===

func toProductTemplatePO(t *model.ProductTemplate) *ProductTemplatePO {
	po := &ProductTemplatePO{
		Name:        t.Name,
		Code:        t.Code,
		CategoryID:  t.CategoryID,
		BaseUomID:   t.BaseUomID,
		BasePrice:   t.BasePrice,
		Description: t.Description,
		Status:      t.Status,
	}
	po.ID = t.ID
	po.TenantID = t.TenantID
	po.Creator = t.Creator
	po.Updater = t.Updater
	po.CreateTime = t.CreateTime
	po.UpdateTime = t.UpdateTime
	return po
}

func fromProductTemplatePO(po *ProductTemplatePO) *model.ProductTemplate {
	return &model.ProductTemplate{
		ID:          po.ID,
		TenantID:    po.TenantID,
		Name:        po.Name,
		Code:        po.Code,
		CategoryID:  po.CategoryID,
		BaseUomID:   po.BaseUomID,
		BasePrice:   po.BasePrice,
		Description: po.Description,
		Status:      po.Status,
		Creator:     po.Creator,
		Updater:     po.Updater,
		CreateTime:  po.CreateTime,
		UpdateTime:  po.UpdateTime,
	}
}

// === TemplateAttributeLine ===

func toTemplateAttributeLinePO(l *model.TemplateAttributeLine) *TemplateAttributeLinePO {
	return &TemplateAttributeLinePO{
		LightBase: LightBase{ID: l.ID, TenantID: l.TenantID, CreateTime: l.CreateTime},
		TemplateID:  l.TemplateID,
		AttributeID: l.AttributeID,
		Sort:        l.Sort,
	}
}

func fromTemplateAttributeLinePO(po *TemplateAttributeLinePO) *model.TemplateAttributeLine {
	return &model.TemplateAttributeLine{
		ID:          po.ID,
		TenantID:    po.TenantID,
		TemplateID:  po.TemplateID,
		AttributeID: po.AttributeID,
		Sort:        po.Sort,
		CreateTime:  po.CreateTime,
	}
}

// === Uom ===

func toUomPO(u *model.Uom) *UomPO {
	po := &UomPO{
		Name:        u.Name,
		Code:        u.Code,
		Category:    u.Category,
		Sort:        u.Sort,
		Status:      u.Status,
		Description: u.Description,
	}
	po.ID = u.ID
	po.TenantID = u.TenantID
	po.Creator = u.Creator
	po.Updater = u.Updater
	po.CreateTime = u.CreateTime
	po.UpdateTime = u.UpdateTime
	return po
}

func fromUomPO(po *UomPO) *model.Uom {
	return &model.Uom{
		ID:          po.ID,
		TenantID:    po.TenantID,
		Name:        po.Name,
		Code:        po.Code,
		Category:    po.Category,
		Sort:        po.Sort,
		Status:      po.Status,
		Description: po.Description,
		Creator:     po.Creator,
		Updater:     po.Updater,
		CreateTime:  po.CreateTime,
		UpdateTime:  po.UpdateTime,
	}
}

// === ProductUom ===

func toProductUomPO(v *model.ProductUom) *ProductUomPO {
	po := &ProductUomPO{
		ProductID:  v.ProductID,
		UomID:      v.UomID,
		Factor:     v.Factor,
		IsPurchase: v.IsPurchase,
		IsSale:     v.IsSale,
		Sort:       v.Sort,
	}
	po.ID = v.ID
	po.TenantID = v.TenantID
	po.CreateTime = v.CreateTime
	return po
}

func fromProductUomPO(po *ProductUomPO) *model.ProductUom {
	return &model.ProductUom{
		ID:         po.ID,
		TenantID:   po.TenantID,
		ProductID:  po.ProductID,
		UomID:      po.UomID,
		Factor:     po.Factor,
		IsPurchase: po.IsPurchase,
		IsSale:     po.IsSale,
		Sort:       po.Sort,
		CreateTime: po.CreateTime,
	}
}

// === ProductAttrValue ===

func toProductAttrValuePO(v *model.ProductAttrValue) *ProductAttrValuePO {
	po := &ProductAttrValuePO{
		LightBase: LightBase{
			ID: v.ID, TenantID: v.TenantID, CreateTime: v.CreateTime,
		},
		ProductID:     v.ProductID,
		AttributeID:   v.AttributeID,
		AttributeCode: v.AttributeCode,
		Value:         v.Value,
		ValueDecimal:  v.ValueDecimal,
		ValueDate:     v.ValueDate,
	}
	if v.ValueBool != nil {
		b := orm.Bit(*v.ValueBool)
		po.ValueBool = &b
	}
	return po
}

func fromProductAttrValuePO(po *ProductAttrValuePO) *model.ProductAttrValue {
	v := &model.ProductAttrValue{
		ID:            po.ID,
		TenantID:      po.TenantID,
		ProductID:     po.ProductID,
		AttributeID:   po.AttributeID,
		AttributeCode: po.AttributeCode,
		Value:         po.Value,
		ValueDecimal:  po.ValueDecimal,
		ValueDate:     po.ValueDate,
		CreateTime:    po.CreateTime,
	}
	if po.ValueBool != nil {
		b := bool(*po.ValueBool)
		v.ValueBool = &b
	}
	return v
}

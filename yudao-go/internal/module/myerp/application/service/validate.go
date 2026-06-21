package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"yudao-go/internal/module/myerp/domain/model"
	"yudao-go/internal/module/myerp/domain/repository"
	"yudao-go/internal/pkg/errcode"
)

var (
	categoryCodePattern  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	attributeCodePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)
	httpURLPattern       = regexp.MustCompile(`^https?://`)

	// 数字字符串校验:支持负号 + 整数 + 小数,但不允许指数/字母。
	decimalPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

// badReq 业务错误简写。
func badReq(format string, args ...any) error {
	if len(args) == 0 {
		return errcode.BadRequest.WithMsg(format)
	}
	return errcode.BadRequest.WithMsgf(format, args...)
}

// === Category 校验 ===

func validateCategory(name, code, description string) (cleanName, cleanCode, cleanDesc string, err error) {
	name = strings.TrimSpace(name)
	code = strings.TrimSpace(code)
	if name == "" {
		return "", "", "", badReq("分类名称不能为空")
	}
	if len(name) > 64 {
		return "", "", "", badReq("分类名称最长 64 字符")
	}
	if code != "" {
		if len(code) > 32 {
			return "", "", "", badReq("分类编码最长 32 字符")
		}
		if !categoryCodePattern.MatchString(code) {
			return "", "", "", badReq("分类编码只允许字母、数字、下划线、连字符")
		}
	}
	return html.EscapeString(name), code, html.EscapeString(description), nil
}

// === Attribute 校验 ===

func validateAttribute(req attrFields) (cleanName, cleanDesc, cleanUnit string, err error) {
	name := strings.TrimSpace(req.Name)
	code := strings.TrimSpace(req.Code)
	if name == "" {
		return "", "", "", badReq("属性名称不能为空")
	}
	if len(name) > 64 {
		return "", "", "", badReq("属性名称最长 64 字符")
	}
	if code == "" {
		return "", "", "", badReq("属性 code 不能为空")
	}
	if !attributeCodePattern.MatchString(code) {
		return "", "", "", badReq("属性 code 必须字母开头,仅含字母数字下划线,最长 32")
	}
	if !model.ValidInputTypes[model.InputType(req.InputType)] {
		return "", "", "", badReq("非法属性类型 %q", req.InputType)
	}
	return html.EscapeString(name), html.EscapeString(req.Description), html.EscapeString(req.Unit), nil
}

// attrFields 用于复用 validateAttribute,避免引 application/dto 造成循环依赖。
type attrFields struct {
	Name        string
	Code        string
	InputType   string
	Description string
	Unit        string
}

// === Product 校验 ===

func validateProduct(name, description, picURL, purchase, sale, stock string, categoryID int64) (cleanName, cleanDesc string, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", badReq("产品名称不能为空")
	}
	if len(name) > 128 {
		return "", "", badReq("产品名称最长 128 字符")
	}
	if len(description) > 1024 {
		return "", "", badReq("产品描述最长 1024 字符")
	}
	if categoryID <= 0 {
		return "", "", badReq("categoryId 必填")
	}
	// 价格/库存:非负数字字符串
	for _, pair := range []struct{ label, val string }{
		{"采购价", purchase}, {"销售价", sale}, {"库存", stock},
	} {
		if pair.val == "" {
			continue
		}
		if !decimalPattern.MatchString(pair.val) {
			return "", "", badReq("%s 必须是数字", pair.label)
		}
		if strings.HasPrefix(pair.val, "-") {
			return "", "", badReq("%s 不能为负", pair.label)
		}
	}
	if picURL != "" && !httpURLPattern.MatchString(picURL) {
		return "", "", badReq("pic_url 必须是 http/https")
	}
	return html.EscapeString(name), html.EscapeString(description), nil
}

// validateUomConfig 校验双计量配置。固定模式无需额外字段;浮动模式必须有辅单位 + 名义率。
// 返回规整后的 tolerancePct(空 → "0")。
func validateUomConfig(uomMode int8, auxUomID *int64, nominalFactor *string, tolerancePct string) (string, error) {
	tolerancePct = strings.TrimSpace(tolerancePct)
	if tolerancePct == "" {
		tolerancePct = "0"
	}
	if uomMode == model.UomModeFixed {
		return "0", nil
	}
	if uomMode != model.UomModeFloat {
		return "", badReq("非法换算方式")
	}
	// 浮动双计量
	if auxUomID == nil || *auxUomID <= 0 {
		return "", badReq("浮动双计量需选择辅计量单位")
	}
	if nominalFactor == nil || strings.TrimSpace(*nominalFactor) == "" {
		return "", badReq("浮动双计量需填名义换算率")
	}
	nf := strings.TrimSpace(*nominalFactor)
	if !decimalPattern.MatchString(nf) || strings.HasPrefix(nf, "-") || nf == "0" {
		return "", badReq("名义换算率必须为正数")
	}
	if !decimalPattern.MatchString(tolerancePct) || strings.HasPrefix(tolerancePct, "-") {
		return "", badReq("允许偏差%% 必须为非负数")
	}
	return tolerancePct, nil
}

// === EAV 值构建(产品创建/更新的核心安全函数) ===
//
// 给定分类(含继承链)的属性集 attrs + 用户提交的 attrValues,
// 按 input_type 分支校验 + 类型化(value/value_decimal/value_date/value_bool 冗余列填充)。
//
// 安全护栏:
//   - 必填属性缺失 → 拒
//   - 属性不属于该分类 → 拒(防 EAV 跨分类污染)
//   - value 长度 > attribute.max_length → 拒(防 DoS)
//   - number 范围 / select 枚举 / date 格式 / regex 一一校验
//   - 字符串值统一 html.EscapeString(防 XSS)
func buildAttrValues(
	ctx context.Context,
	attrs []*model.Attribute,
	input map[string]any,
	optRepo repository.AttributeOptionRepository,
) ([]*model.ProductAttrValue, error) {
	codeToAttr := make(map[string]*model.Attribute, len(attrs))
	for _, a := range attrs {
		codeToAttr[a.Code] = a
	}
	// 必填校验
	for _, a := range attrs {
		if a.Required && a.Status == 0 {
			if _, ok := input[a.Code]; !ok {
				return nil, badReq("属性 %s 必填", a.Name)
			}
		}
	}
	// 收集所有 select/multi_select 属性的枚举一次性查(避免每 attr 一次)
	var enumAttrIDs []int64
	for _, a := range attrs {
		if a.InputType == model.InputSelect || a.InputType == model.InputMultiSelect {
			enumAttrIDs = append(enumAttrIDs, a.ID)
		}
	}
	enumOpts, _ := optRepo.ListByAttributeIDs(ctx, enumAttrIDs)
	enumByAttr := make(map[int64]map[string]bool, len(enumAttrIDs))
	for _, o := range enumOpts {
		set := enumByAttr[o.AttributeID]
		if set == nil {
			set = make(map[string]bool)
			enumByAttr[o.AttributeID] = set
		}
		set[o.Value] = true
	}

	out := make([]*model.ProductAttrValue, 0, len(input))
	for code, raw := range input {
		a, ok := codeToAttr[code]
		if !ok {
			return nil, badReq("属性 %s 不属于该分类", code)
		}
		if a.Status != 0 {
			continue // 停用属性跳过(数据保留但不接收新值)
		}
		v := &model.ProductAttrValue{
			AttributeID:   a.ID,
			AttributeCode: a.Code,
		}
		valStr := toString(raw)
		if a.MaxLength > 0 && len(valStr) > a.MaxLength {
			return nil, badReq("属性 %s 值长度超出 %d", a.Name, a.MaxLength)
		}
		valStr = html.EscapeString(valStr)

		switch a.InputType {
		case model.InputText, model.InputURL, model.InputColor:
			if a.Regex != "" {
				re, err := regexp.Compile(a.Regex)
				if err == nil && !re.MatchString(valStr) {
					return nil, badReq("属性 %s 格式不符", a.Name)
				}
			}
			v.Value = valStr

		case model.InputNumber:
			if !decimalPattern.MatchString(valStr) {
				return nil, badReq("属性 %s 必须是数字", a.Name)
			}
			if a.MinValue != nil && lessThan(valStr, *a.MinValue) {
				return nil, badReq("属性 %s 不能小于 %s", a.Name, *a.MinValue)
			}
			if a.MaxValue != nil && greaterThan(valStr, *a.MaxValue) {
				return nil, badReq("属性 %s 不能大于 %s", a.Name, *a.MaxValue)
			}
			v.Value = valStr
			vc := valStr
			v.ValueDecimal = &vc

		case model.InputBool:
			b := valStr == "true" || valStr == "1"
			if b {
				v.Value = "true"
			} else {
				v.Value = "false"
			}
			v.ValueBool = &b

		case model.InputDate, model.InputDateTime:
			layout := "2006-01-02"
			if a.InputType == model.InputDateTime {
				layout = "2006-01-02 15:04:05"
			}
			t, err := time.ParseInLocation(layout, valStr, time.Local)
			if err != nil {
				return nil, badReq("属性 %s 时间格式应为 %s", a.Name, layout)
			}
			v.Value = t.Format(layout)
			v.ValueDate = &t

		case model.InputSelect:
			if !enumByAttr[a.ID][valStr] {
				return nil, badReq("属性 %s 值 %s 不在选项内", a.Name, valStr)
			}
			v.Value = valStr

		case model.InputMultiSelect:
			arr, ok := raw.([]any)
			if !ok {
				return nil, badReq("属性 %s 应传数组", a.Name)
			}
			vals := make([]string, 0, len(arr))
			for _, item := range arr {
				s := html.EscapeString(toString(item))
				if !enumByAttr[a.ID][s] {
					return nil, badReq("属性 %s 值 %s 不在选项内", a.Name, s)
				}
				vals = append(vals, s)
			}
			b, _ := json.Marshal(vals)
			v.Value = string(b)

		default:
			return nil, badReq("不支持的属性类型 %s", a.InputType)
		}
		out = append(out, v)
	}
	return out, nil
}

// === 辅助函数 ===

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON 数字默认是 float64,这里转字符串保留精度(整数形式优先)
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.9f", x), "0"), ".")
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// lessThan / greaterThan 用 decimal 字符串语义比较(避免 float 精度坑)。
// 实现:对齐小数位后按字符串比较;支持负号。
func lessThan(a, b string) bool  { return cmpDecimal(a, b) < 0 }
func greaterThan(a, b string) bool { return cmpDecimal(a, b) > 0 }

func cmpDecimal(a, b string) int {
	negA := strings.HasPrefix(a, "-")
	negB := strings.HasPrefix(b, "-")
	if negA && !negB {
		return -1
	}
	if !negA && negB {
		return 1
	}
	if negA {
		// 都负:绝对值大的更小,反转结果
		return -cmpAbsDecimal(strings.TrimPrefix(a, "-"), strings.TrimPrefix(b, "-"))
	}
	return cmpAbsDecimal(a, b)
}

func cmpAbsDecimal(a, b string) int {
	intA, fracA := splitDecimal(a)
	intB, fracB := splitDecimal(b)
	// 去前导 0
	intA = strings.TrimLeft(intA, "0")
	intB = strings.TrimLeft(intB, "0")
	if intA == "" {
		intA = "0"
	}
	if intB == "" {
		intB = "0"
	}
	if len(intA) != len(intB) {
		if len(intA) < len(intB) {
			return -1
		}
		return 1
	}
	if intA != intB {
		if intA < intB {
			return -1
		}
		return 1
	}
	// 整数部分相同,补齐小数比
	maxLen := len(fracA)
	if len(fracB) > maxLen {
		maxLen = len(fracB)
	}
	fracA += strings.Repeat("0", maxLen-len(fracA))
	fracB += strings.Repeat("0", maxLen-len(fracB))
	if fracA == fracB {
		return 0
	}
	if fracA < fracB {
		return -1
	}
	return 1
}

func splitDecimal(s string) (intPart, fracPart string) {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// wrapDuplicateError 把 MySQL 1062 Duplicate entry 错误包装为业务错。
func wrapDuplicateError(err error, msg string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "Duplicate entry") {
		return badReq(msg)
	}
	return err
}

// errIsNotFound 判断是否项目内 NotFound。
func errIsNotFound(err error) bool {
	return errors.Is(err, errcode.NotFound)
}

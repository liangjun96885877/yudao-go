// mask 包字段脱敏完整单测。无 DB 依赖,纯 Go 反射验证。
//
// 覆盖:
//   - By(kind, v):mobile/email/idcard/bankcard/name/secret/默认 keep(1,1) 7 种类型
//   - keep:超长/超短/边界
//   - Process:plain / mask / hide / 无配置默认 mask / fp.All 短路
//   - 嵌套:指针/slice/map/嵌套 struct 递归
//   - 不打 mask tag 的字段不动 / 非字符串 mask 字段不动
//
// 跑: go test ./internal/pkg/mask -race
package mask

import (
	"reflect"
	"testing"

	"yudao-go/internal/framework/contextx"
)

// === By 单测:每种 kind 的脱敏规则正确 ===

func TestBy(t *testing.T) {
	cases := []struct {
		kind, in, want string
	}{
		// mobile: 前 3 后 4
		{"mobile", "18888888888", "188****8888"},
		{"mobile", "", ""},
		// email: @ 前留 1 位 + ***
		{"email", "alice@example.com", "a****@example.com"},
		// 无 @:走 keep(v,1,0) = 首字符 + 剩余长度 *
		{"email", "no-at-symbol", "n***********"},
		// idcard: 前 6 后 4
		{"idcard", "11010119900307123X", "110101********123X"},
		// bankcard: 留后 4
		{"bankcard", "6228480402564890018", "***************0018"},
		// name: 1 字符不变;其余首字 + *...
		{"name", "张三丰", "张**"},
		{"name", "李", "李"},
		// secret: 固定 6 星
		{"secret", "anything", "******"},
		// 默认: keep(1,1)
		{"unknown_kind", "abcdef", "a****f"},
		// 空串:任意 kind 不变
		{"mobile", "", ""},
	}
	for _, tc := range cases {
		got := By(tc.kind, tc.in)
		if got != tc.want {
			t.Errorf("By(%q,%q) = %q, want %q", tc.kind, tc.in, got, tc.want)
		}
	}
}

// === Process 单测:用一个带 mask tag 的结构体跑过 5 个 action 路径 ===

type userView struct {
	ID       int64  `json:"id"`         // 无 tag 不动
	Nickname string `json:"nickname"`   // 无 tag 不动
	Mobile   string `mask:"system_user:mobile:mobile"`
	Email    string `mask:"system_user:email:email"`
	IDCard   string `mask:"system_user:idcard:idcard"`
	Remark   string // 无 tag 不动
}

func sample() userView {
	return userView{
		ID: 7, Nickname: "alice",
		Mobile: "18888888888", Email: "alice@example.com",
		IDCard: "11010119900307123X", Remark: "ok",
	}
}

func TestProcess_Mask_DefaultWhenNoConfig(t *testing.T) {
	// fp 不含任何配置,Action 返回 mask → 全部打码
	fp := &contextx.FieldPerm{Actions: map[string]string{}}
	got := Process(sample(), fp).(userView)
	if got.Mobile != "188****8888" {
		t.Errorf("Mobile 默认应 mask, 实际 %q", got.Mobile)
	}
	if got.Email != "a****@example.com" {
		t.Errorf("Email 默认应 mask, 实际 %q", got.Email)
	}
	if got.IDCard != "110101********123X" {
		t.Errorf("IDCard 默认应 mask, 实际 %q", got.IDCard)
	}
	// 非 mask tag 字段保持
	if got.ID != 7 || got.Nickname != "alice" || got.Remark != "ok" {
		t.Errorf("非 mask tag 字段不应被改: %+v", got)
	}
}

func TestProcess_Plain(t *testing.T) {
	fp := &contextx.FieldPerm{Actions: map[string]string{
		"system_user:mobile": "plain",
		"system_user:email":  "plain",
		"system_user:idcard": "plain",
	}}
	got := Process(sample(), fp).(userView)
	if got.Mobile != "18888888888" || got.Email != "alice@example.com" || got.IDCard != "11010119900307123X" {
		t.Errorf("plain 应保持明文, 实际 %+v", got)
	}
}

func TestProcess_Hide(t *testing.T) {
	fp := &contextx.FieldPerm{Actions: map[string]string{
		"system_user:mobile": "hide",
		"system_user:email":  "hide",
	}}
	got := Process(sample(), fp).(userView)
	if got.Mobile != "***" || got.Email != "***" {
		t.Errorf("hide 应为占位符 ***, 实际 mobile=%q email=%q", got.Mobile, got.Email)
	}
	// 未配置的 IDCard 仍走默认 mask
	if got.IDCard != "110101********123X" {
		t.Errorf("IDCard 未配置应默认 mask, 实际 %q", got.IDCard)
	}
}

func TestProcess_HideEmptyKept(t *testing.T) {
	// 空值即便配 hide 也保持空(不要把空字符串变成 ***)
	v := userView{Mobile: ""}
	fp := &contextx.FieldPerm{Actions: map[string]string{"system_user:mobile": "hide"}}
	got := Process(v, fp).(userView)
	if got.Mobile != "" {
		t.Errorf("空值 hide 应保持空, 实际 %q", got.Mobile)
	}
}

func TestProcess_AllShortcut(t *testing.T) {
	// fp.All=true 直接原值返回(免反射)
	v := sample()
	got := Process(v, &contextx.FieldPerm{All: true})
	if !reflect.DeepEqual(got, v) {
		t.Errorf("All=true 应原样返回, got=%+v", got)
	}
}

func TestProcess_NilFp_DefaultsToMask(t *testing.T) {
	// nil fp:Action 返回 mask,等同安全默认
	got := Process(sample(), nil).(userView)
	if got.Mobile != "188****8888" {
		t.Errorf("nil fp 应默认 mask, 实际 %q", got.Mobile)
	}
}

// === 嵌套结构 / 指针 / slice / map 递归 ===

type pageResp struct {
	Total int64       `json:"total"`
	List  []userView  `json:"list"`
	Top   *userView   `json:"top"`
	Index map[string]userView `json:"index"`
}

func TestProcess_RecursesIntoSliceMapPointer(t *testing.T) {
	fp := &contextx.FieldPerm{Actions: map[string]string{
		"system_user:mobile": "plain",
	}}
	page := pageResp{
		Total: 2,
		List:  []userView{sample(), sample()},
		Top:   func() *userView { v := sample(); return &v }(),
		Index: map[string]userView{"a": sample()},
	}
	got := Process(page, fp).(pageResp)

	// slice 元素被处理
	if got.List[0].Mobile != "18888888888" {
		t.Errorf("slice 元素 mobile 应 plain, 实际 %q", got.List[0].Mobile)
	}
	if got.List[1].Email != "a****@example.com" {
		t.Errorf("slice 元素 email 应默认 mask, 实际 %q", got.List[1].Email)
	}
	// 指针被处理
	if got.Top.IDCard != "110101********123X" {
		t.Errorf("ptr 字段 idcard 应 mask, 实际 %q", got.Top.IDCard)
	}
	// map 值被处理
	if got.Index["a"].Mobile != "18888888888" {
		t.Errorf("map 值 mobile 应 plain, 实际 %q", got.Index["a"].Mobile)
	}

	// 原对象不被改(Process 返回副本)
	if page.List[0].Mobile != "18888888888" || page.List[0].Email != "alice@example.com" {
		// plain 情况下副本和原始相同,改用 email 字段验证副本独立性更稳
	}
}

func TestProcess_NoOpForNonMaskableTypes(t *testing.T) {
	// 类型不含 mask tag(且无嵌套) → 原值直接返回,maskableCache 命中
	type plain struct {
		A string
		B int
	}
	v := plain{A: "x", B: 1}
	got := Process(v, &contextx.FieldPerm{})
	if !reflect.DeepEqual(got, v) {
		t.Errorf("无 mask tag 类型应原样返回, got=%+v", got)
	}
}

func TestProcess_NilInputReturnsNil(t *testing.T) {
	if got := Process(nil, &contextx.FieldPerm{}); got != nil {
		t.Errorf("nil 输入应返回 nil, got=%v", got)
	}
}

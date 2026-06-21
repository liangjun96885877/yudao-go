// 权限码派生纯单测 —— derivePermCode 是 RequirePermissionByPath 的核心,
// table-driven 覆盖所有归一化规则。无 DB 依赖。
package security

import "testing"

func TestDerivePermCode(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// 标准三段
		{"/admin-api/system/user/create", "system:user:create"},
		{"/admin-api/system/user/update", "system:user:update"},
		{"/admin-api/system/user/delete", "system:user:delete"},

		// 查询类归一为 query
		{"/admin-api/system/user/page", "system:user:query"},
		{"/admin-api/system/user/list", "system:user:query"},
		{"/admin-api/system/user/get", "system:user:query"},
		{"/admin-api/system/user/simple-list", "system:user:query"},
		{"/admin-api/system/user/list-all-simple", "system:user:query"},
		{"/admin-api/system/user/export-excel", "system:user:query"},

		// 批删归一为 delete
		{"/admin-api/system/user/delete-list", "system:user:delete"},

		// 多段(infra/codegen/preview) → infra:codegen:preview
		{"/admin-api/infra/codegen/preview", "infra:codegen:preview"},

		// chatter 模块多段嵌套
		{"/admin-api/chatter/comment/create", "chatter:comment:create"},

		// 段数 < 2 应返回空
		{"/admin-api/health", ""},
		{"/admin-api/", ""},

		// 非 admin-api 前缀返回空(不强制)
		{"/infra/ws", ""},
		{"/foo/bar/baz", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := derivePermCode(tc.path)
		if got != tc.want {
			t.Errorf("derivePermCode(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

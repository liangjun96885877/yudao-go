// Package dto 定义 system 模块的请求/响应对象。
package dto

// LoginReq 登录请求。
type LoginReq struct {
	Username            string `json:"username" binding:"required"`
	Password            string `json:"password" binding:"required"`
	CaptchaVerification string `json:"captchaVerification"`
}

// LoginResp 登录响应。
type LoginResp struct {
	UserID       int64  `json:"userId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresTime  int64  `json:"expiresTime"` // epoch 毫秒
}

// MenuVO 菜单树节点，字段与前端 AppCustomRouteRecordRaw 对齐。
type MenuVO struct {
	ID            int64     `json:"id"`
	ParentID      int64     `json:"parentId"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Component     string    `json:"component"`
	ComponentName string    `json:"componentName"`
	Icon          string    `json:"icon"`
	Visible       bool      `json:"visible"`
	KeepAlive     bool      `json:"keepAlive"`
	AlwaysShow    bool      `json:"alwaysShow"`
	Children      []*MenuVO `json:"children,omitempty"`
}

// UserVO 当前登录用户信息。
type UserVO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	DeptID   int64  `json:"deptId"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
}

// PermissionInfoResp 权限信息响应。
type PermissionInfoResp struct {
	User        UserVO    `json:"user"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	Menus       []*MenuVO `json:"menus"`
}

// DictDataVO 字典数据项。
type DictDataVO struct {
	DictType  string `json:"dictType"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	ColorType string `json:"colorType"`
	CSSClass  string `json:"cssClass"`
}

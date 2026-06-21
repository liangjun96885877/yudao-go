// Package model 是 system 模块的持久化模型。
// 说明：system 为对原版的忠实移植，PO 直接作为模型/VO 使用（务实取舍）。
package model

import (
	"time"

	"yudao-go/internal/framework/orm"
)

// Bit 是 MySQL bit(1) 列适配类型（统一定义在 framework/orm）。
type Bit = orm.Bit

// Base 是 system 表公共字段。不映射 deleted（原版 bit(1)），由仓储显式过滤/软删。
type Base struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Creator    string    `gorm:"column:creator" json:"creator"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	Updater    string    `gorm:"column:updater" json:"updater"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

// TenantBase 在 Base 基础上增加租户字段，多租户插件据此自动过滤。
type TenantBase struct {
	Base
	TenantID int64 `gorm:"column:tenant_id" json:"tenantId"`
}

// User 对应 system_users。
type User struct {
	TenantBase
	Username  string     `gorm:"column:username" json:"username"`
	Password  string     `gorm:"column:password" json:"-"`
	Nickname  string     `gorm:"column:nickname" json:"nickname"`
	Remark    string     `gorm:"column:remark" json:"remark"`
	DeptID    int64      `gorm:"column:dept_id" json:"deptId"`
	PostIDs   string     `gorm:"column:post_ids" json:"-"`
	Email     string     `gorm:"column:email" json:"email" mask:"system_user:email:email"`
	Mobile    string     `gorm:"column:mobile" json:"mobile" mask:"system_user:mobile:mobile"`
	Sex       int8       `gorm:"column:sex" json:"sex"`
	Avatar    string     `gorm:"column:avatar" json:"avatar"`
	Status    int8       `gorm:"column:status" json:"status"`
	LoginIP   string     `gorm:"column:login_ip" json:"loginIp"`
	LoginDate *time.Time `gorm:"column:login_date" json:"loginDate"`
}

func (User) TableName() string { return "system_users" }

// Role 对应 system_role。
type Role struct {
	TenantBase
	Name      string `gorm:"column:name" json:"name"`
	Code      string `gorm:"column:code" json:"code"`
	Sort      int    `gorm:"column:sort" json:"sort"`
	DataScope        int8           `gorm:"column:data_scope" json:"dataScope"`
	DataScopeDeptIDs orm.Int64Array `gorm:"column:data_scope_dept_ids" json:"dataScopeDeptIds"`
	Status           int8           `gorm:"column:status" json:"status"`
	Type             int8   `gorm:"column:type" json:"type"`
	Remark           string `gorm:"column:remark" json:"remark"`
}

func (Role) TableName() string { return "system_role" }

// Menu 对应 system_menu（全局表，无租户字段）。
type Menu struct {
	Base
	Name          string `gorm:"column:name" json:"name"`
	Permission    string `gorm:"column:permission" json:"permission"`
	Type          int8   `gorm:"column:type" json:"type"`
	Sort          int    `gorm:"column:sort" json:"sort"`
	ParentID      int64  `gorm:"column:parent_id" json:"parentId"`
	Path          string `gorm:"column:path" json:"path"`
	Icon          string `gorm:"column:icon" json:"icon"`
	Component     string `gorm:"column:component" json:"component"`
	ComponentName string `gorm:"column:component_name" json:"componentName"`
	Status        int8   `gorm:"column:status" json:"status"`
	Visible       Bit    `gorm:"column:visible" json:"visible"`
	KeepAlive     Bit    `gorm:"column:keep_alive" json:"keepAlive"`
	AlwaysShow    Bit    `gorm:"column:always_show" json:"alwaysShow"`
}

func (Menu) TableName() string { return "system_menu" }

// Dept 对应 system_dept。
type Dept struct {
	TenantBase
	Name         string `gorm:"column:name" json:"name"`
	ParentID     int64  `gorm:"column:parent_id" json:"parentId"`
	Sort         int    `gorm:"column:sort" json:"sort"`
	LeaderUserID int64  `gorm:"column:leader_user_id" json:"leaderUserId"`
	Phone        string `gorm:"column:phone" json:"phone"`
	Email        string `gorm:"column:email" json:"email"`
	Status       int8   `gorm:"column:status" json:"status"`
}

func (Dept) TableName() string { return "system_dept" }

// Post 对应 system_post。
type Post struct {
	TenantBase
	Code   string `gorm:"column:code" json:"code"`
	Name   string `gorm:"column:name" json:"name"`
	Sort   int    `gorm:"column:sort" json:"sort"`
	Status int8   `gorm:"column:status" json:"status"`
	Remark string `gorm:"column:remark" json:"remark"`
}

func (Post) TableName() string { return "system_post" }

// UserRole 对应 system_user_role。
type UserRole struct {
	TenantBase
	UserID int64 `gorm:"column:user_id" json:"userId"`
	RoleID int64 `gorm:"column:role_id" json:"roleId"`
}

func (UserRole) TableName() string { return "system_user_role" }

// RoleMenu 对应 system_role_menu。
type RoleMenu struct {
	TenantBase
	RoleID int64 `gorm:"column:role_id" json:"roleId"`
	MenuID int64 `gorm:"column:menu_id" json:"menuId"`
}

func (RoleMenu) TableName() string { return "system_role_menu" }

// Tenant 对应 system_tenant（租户注册表，本身无租户字段）。
type Tenant struct {
	Base
	Name          string     `gorm:"column:name" json:"name"`
	ContactUserID int64      `gorm:"column:contact_user_id" json:"contactUserId"`
	ContactName   string     `gorm:"column:contact_name" json:"contactName"`
	ContactMobile string     `gorm:"column:contact_mobile" json:"contactMobile"`
	Status        int8       `gorm:"column:status" json:"status"`
	Website       string     `gorm:"column:websites" json:"website"`
	PackageID     int64      `gorm:"column:package_id" json:"packageId"`
	ExpireTime    *time.Time `gorm:"column:expire_time" json:"expireTime"`
	AccountCount  int        `gorm:"column:account_count" json:"accountCount"`
}

func (Tenant) TableName() string { return "system_tenant" }

// TenantPackage 对应 system_tenant_package。
type TenantPackage struct {
	Base
	Name    string `gorm:"column:name" json:"name"`
	Status  int8   `gorm:"column:status" json:"status"`
	Remark  string `gorm:"column:remark" json:"remark"`
	MenuIDs string `gorm:"column:menu_ids" json:"-"`
}

func (TenantPackage) TableName() string { return "system_tenant_package" }

// LoginLog 对应 system_login_log。
type LoginLog struct {
	TenantBase
	LogType   int8   `gorm:"column:log_type" json:"logType"`
	TraceID   string `gorm:"column:trace_id" json:"traceId"`
	UserID    int64  `gorm:"column:user_id" json:"userId"`
	UserType  int8   `gorm:"column:user_type" json:"userType"`
	Username  string `gorm:"column:username" json:"username"`
	Result    int8   `gorm:"column:result" json:"result"`
	UserIP    string `gorm:"column:user_ip" json:"userIp"`
	UserAgent string `gorm:"column:user_agent" json:"userAgent"`
}

func (LoginLog) TableName() string { return "system_login_log" }

// OperateLog 对应 system_operate_log。
type OperateLog struct {
	TenantBase
	TraceID       string `gorm:"column:trace_id" json:"traceId"`
	UserID        int64  `gorm:"column:user_id" json:"userId"`
	UserType      int8   `gorm:"column:user_type" json:"userType"`
	Type          string `gorm:"column:type" json:"type"`
	SubType       string `gorm:"column:sub_type" json:"subType"`
	BizID         int64  `gorm:"column:biz_id" json:"bizId"`
	Action        string `gorm:"column:action" json:"action"`
	Success       Bit    `gorm:"column:success" json:"success"`
	Extra         string `gorm:"column:extra" json:"extra"`
	RequestMethod string `gorm:"column:request_method" json:"requestMethod"`
	RequestURL    string `gorm:"column:request_url" json:"requestUrl"`
	UserIP        string `gorm:"column:user_ip" json:"userIp"`
	UserAgent     string `gorm:"column:user_agent" json:"userAgent"`
}

func (OperateLog) TableName() string { return "system_operate_log" }

// OAuth2AccessToken 对应 system_oauth2_access_token。
type OAuth2AccessToken struct {
	TenantBase
	UserID       int64     `gorm:"column:user_id" json:"userId"`
	UserType     int8      `gorm:"column:user_type" json:"userType"`
	UserInfo     string    `gorm:"column:user_info" json:"-"`
	AccessToken  string    `gorm:"column:access_token" json:"accessToken"`
	RefreshToken string    `gorm:"column:refresh_token" json:"refreshToken"`
	ClientID     string    `gorm:"column:client_id" json:"clientId"`
	Scopes       string    `gorm:"column:scopes" json:"-"`
	ExpiresTime  time.Time `gorm:"column:expires_time" json:"expiresTime"`
}

func (OAuth2AccessToken) TableName() string { return "system_oauth2_access_token" }

// OAuth2Client 对应 system_oauth2_client（OAuth2 客户端 / 应用）。
type OAuth2Client struct {
	Base
	ClientID                    string        `gorm:"column:client_id" json:"clientId"`
	Secret                      string        `gorm:"column:secret" json:"secret"`
	Name                        string        `gorm:"column:name" json:"name"`
	Logo                        string        `gorm:"column:logo" json:"logo"`
	Description                 string        `gorm:"column:description" json:"description"`
	Status                      int8          `gorm:"column:status" json:"status"`
	AccessTokenValiditySeconds  int           `gorm:"column:access_token_validity_seconds" json:"accessTokenValiditySeconds"`
	RefreshTokenValiditySeconds int           `gorm:"column:refresh_token_validity_seconds" json:"refreshTokenValiditySeconds"`
	RedirectURIs                orm.StrArray  `gorm:"column:redirect_uris" json:"redirectUris"`
	AuthorizedGrantTypes        orm.StrArray  `gorm:"column:authorized_grant_types" json:"authorizedGrantTypes"`
	Scopes                      orm.StrArray  `gorm:"column:scopes" json:"scopes"`
	AutoApproveScopes           orm.StrArray  `gorm:"column:auto_approve_scopes" json:"autoApproveScopes"`
	Authorities                 orm.StrArray  `gorm:"column:authorities" json:"authorities"`
	ResourceIDs                 orm.StrArray  `gorm:"column:resource_ids" json:"resourceIds"`
	AdditionalInformation       string        `gorm:"column:additional_information" json:"additionalInformation"`
}

func (OAuth2Client) TableName() string { return "system_oauth2_client" }

// DictType 对应 system_dict_type。
type DictType struct {
	Base
	Name   string `gorm:"column:name" json:"name"`
	Type   string `gorm:"column:type" json:"type"`
	Status int8   `gorm:"column:status" json:"status"`
	Remark string `gorm:"column:remark" json:"remark"`
}

func (DictType) TableName() string { return "system_dict_type" }

// DictData 对应 system_dict_data。
type DictData struct {
	Base
	Sort      int    `gorm:"column:sort" json:"sort"`
	Label     string `gorm:"column:label" json:"label"`
	Value     string `gorm:"column:value" json:"value"`
	DictType  string `gorm:"column:dict_type" json:"dictType"`
	Status    int8   `gorm:"column:status" json:"status"`
	ColorType string `gorm:"column:color_type" json:"colorType"`
	CSSClass  string `gorm:"column:css_class" json:"cssClass"`
	Remark    string `gorm:"column:remark" json:"remark"`
}

func (DictData) TableName() string { return "system_dict_data" }

// RoleFieldPerm 对应 system_role_field_perm（角色字段权限）。
// action：plain 明文 / mask 打码 / hide 占位符。
type RoleFieldPerm struct {
	TenantBase
	RoleID  int64  `gorm:"column:role_id" json:"roleId"`
	BizType string `gorm:"column:biz_type" json:"bizType"`
	Field   string `gorm:"column:field" json:"field"`
	Action  string `gorm:"column:action" json:"action"`
}

func (RoleFieldPerm) TableName() string { return "system_role_field_perm" }

// NotifyTemplate 对应 system_notify_template（站内信模板）。
type NotifyTemplate struct {
	Base
	Name     string `gorm:"column:name" json:"name"`
	Code     string `gorm:"column:code" json:"code"`
	Nickname string `gorm:"column:nickname" json:"nickname"`
	Content  string `gorm:"column:content" json:"content"`
	Type     int8   `gorm:"column:type" json:"type"`
	Params   string `gorm:"column:params" json:"-"`
	Status   int8   `gorm:"column:status" json:"status"`
	Remark   string `gorm:"column:remark" json:"remark"`
}

func (NotifyTemplate) TableName() string { return "system_notify_template" }

// NotifyMessage 对应 system_notify_message（站内信，用户收件箱）。
type NotifyMessage struct {
	TenantBase
	UserID           int64      `gorm:"column:user_id" json:"userId"`
	UserType         int8       `gorm:"column:user_type" json:"userType"`
	TemplateID       int64      `gorm:"column:template_id" json:"templateId"`
	TemplateCode     string     `gorm:"column:template_code" json:"templateCode"`
	TemplateNickname string     `gorm:"column:template_nickname" json:"templateNickname"`
	TemplateContent  string     `gorm:"column:template_content" json:"templateContent"`
	TemplateType     int        `gorm:"column:template_type" json:"templateType"`
	TemplateParams   string     `gorm:"column:template_params" json:"templateParams"`
	ReadStatus       Bit        `gorm:"column:read_status" json:"readStatus"`
	ReadTime         *time.Time `gorm:"column:read_time" json:"readTime"`
}

func (NotifyMessage) TableName() string { return "system_notify_message" }

// MailAccount 对应 system_mail_account（邮箱账号）。
type MailAccount struct {
	Base
	Mail          string `gorm:"column:mail" json:"mail"`
	Username      string `gorm:"column:username" json:"username"`
	Password      string `gorm:"column:password" json:"password"`
	Host          string `gorm:"column:host" json:"host"`
	Port          int    `gorm:"column:port" json:"port"`
	SSLEnable     Bit    `gorm:"column:ssl_enable" json:"sslEnable"`
	StarttlsEnable Bit   `gorm:"column:starttls_enable" json:"starttlsEnable"`
}

func (MailAccount) TableName() string { return "system_mail_account" }

// MailTemplate 对应 system_mail_template（邮件模板）。
type MailTemplate struct {
	Base
	Name      string `gorm:"column:name" json:"name"`
	Code      string `gorm:"column:code" json:"code"`
	AccountID int64  `gorm:"column:account_id" json:"accountId"`
	Nickname  string `gorm:"column:nickname" json:"nickname"`
	Title     string `gorm:"column:title" json:"title"`
	Content   string `gorm:"column:content" json:"content"`
	Params    string `gorm:"column:params" json:"-"`
	Status    int8   `gorm:"column:status" json:"status"`
	Remark    string `gorm:"column:remark" json:"remark"`
}

func (MailTemplate) TableName() string { return "system_mail_template" }

// MailLog 对应 system_mail_log（邮件发送日志）。
type MailLog struct {
	Base
	UserID           int64      `gorm:"column:user_id" json:"userId"`
	UserType         int8       `gorm:"column:user_type" json:"userType"`
	ToMails          string     `gorm:"column:to_mails" json:"toMails"`
	CcMails          string     `gorm:"column:cc_mails" json:"ccMails"`
	BccMails         string     `gorm:"column:bcc_mails" json:"bccMails"`
	AccountID        int64      `gorm:"column:account_id" json:"accountId"`
	FromMail         string     `gorm:"column:from_mail" json:"fromMail"`
	TemplateID       int64      `gorm:"column:template_id" json:"templateId"`
	TemplateCode     string     `gorm:"column:template_code" json:"templateCode"`
	TemplateNickname string     `gorm:"column:template_nickname" json:"templateNickname"`
	TemplateTitle    string     `gorm:"column:template_title" json:"templateTitle"`
	TemplateContent  string     `gorm:"column:template_content" json:"templateContent"`
	TemplateParams   string     `gorm:"column:template_params" json:"templateParams"`
	SendStatus       int8       `gorm:"column:send_status" json:"sendStatus"`
	SendTime         *time.Time `gorm:"column:send_time" json:"sendTime"`
	SendMessageID    string     `gorm:"column:send_message_id" json:"sendMessageId"`
	SendException    string     `gorm:"column:send_exception" json:"sendException"`
}

func (MailLog) TableName() string { return "system_mail_log" }

// SmsChannel 对应 system_sms_channel（短信渠道）。
type SmsChannel struct {
	Base
	Signature   string `gorm:"column:signature" json:"signature"`
	Code        string `gorm:"column:code" json:"code"`
	Status      int8   `gorm:"column:status" json:"status"`
	Remark      string `gorm:"column:remark" json:"remark"`
	APIKey      string `gorm:"column:api_key" json:"apiKey"`
	APISecret   string `gorm:"column:api_secret" json:"apiSecret"`
	CallbackURL string `gorm:"column:callback_url" json:"callbackUrl"`
}

func (SmsChannel) TableName() string { return "system_sms_channel" }

// SmsTemplate 对应 system_sms_template（短信模板）。
type SmsTemplate struct {
	Base
	Type          int8   `gorm:"column:type" json:"type"`
	Status        int8   `gorm:"column:status" json:"status"`
	Code          string `gorm:"column:code" json:"code"`
	Name          string `gorm:"column:name" json:"name"`
	Content       string `gorm:"column:content" json:"content"`
	Params        string `gorm:"column:params" json:"-"`
	Remark        string `gorm:"column:remark" json:"remark"`
	APITemplateID string `gorm:"column:api_template_id" json:"apiTemplateId"`
	ChannelID     int64  `gorm:"column:channel_id" json:"channelId"`
	ChannelCode   string `gorm:"column:channel_code" json:"channelCode"`
}

func (SmsTemplate) TableName() string { return "system_sms_template" }

// SmsLog 对应 system_sms_log（短信发送日志）。
type SmsLog struct {
	Base
	ChannelID       int64      `gorm:"column:channel_id" json:"channelId"`
	ChannelCode     string     `gorm:"column:channel_code" json:"channelCode"`
	TemplateID      int64      `gorm:"column:template_id" json:"templateId"`
	TemplateCode    string     `gorm:"column:template_code" json:"templateCode"`
	TemplateType    int8       `gorm:"column:template_type" json:"templateType"`
	TemplateContent string     `gorm:"column:template_content" json:"templateContent"`
	TemplateParams  string     `gorm:"column:template_params" json:"templateParams"`
	APITemplateID   string     `gorm:"column:api_template_id" json:"apiTemplateId"`
	Mobile          string     `gorm:"column:mobile" json:"mobile"`
	UserID          int64      `gorm:"column:user_id" json:"userId"`
	UserType        int8       `gorm:"column:user_type" json:"userType"`
	SendStatus      int8       `gorm:"column:send_status" json:"sendStatus"`
	SendTime        *time.Time `gorm:"column:send_time" json:"sendTime"`
	APISendCode     string     `gorm:"column:api_send_code" json:"apiSendCode"`
	APISendMsg      string     `gorm:"column:api_send_msg" json:"apiSendMsg"`
	APIRequestID    string     `gorm:"column:api_request_id" json:"apiRequestId"`
	APISerialNo     string     `gorm:"column:api_serial_no" json:"apiSerialNo"`
	ReceiveStatus   int8       `gorm:"column:receive_status" json:"receiveStatus"`
	ReceiveTime     *time.Time `gorm:"column:receive_time" json:"receiveTime"`
	APIReceiveCode  string     `gorm:"column:api_receive_code" json:"apiReceiveCode"`
	APIReceiveMsg   string     `gorm:"column:api_receive_msg" json:"apiReceiveMsg"`
}

func (SmsLog) TableName() string { return "system_sms_log" }

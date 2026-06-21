package orm

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// BaseModel 是所有持久化对象（PO）的公共基类。
// 移植标准：所有 PO 嵌入 BaseModel；多租户 PO 改嵌入 TenantModel。
type BaseModel struct {
	ID         int64                 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Creator    string                `gorm:"column:creator" json:"creator"`
	CreateTime time.Time             `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	Updater    string                `gorm:"column:updater" json:"updater"`
	UpdateTime time.Time             `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	// Deleted 为基于标志位的逻辑删除（与 yudao 表结构 deleted bit 兼容）。
	Deleted soft_delete.DeletedAt `gorm:"column:deleted;softDelete:flag" json:"-"`
}

// TenantModel 在 BaseModel 基础上增加租户字段。
// 多租户插件依据是否存在 TenantID 字段自动注入/过滤。
type TenantModel struct {
	BaseModel
	TenantID int64 `gorm:"column:tenant_id;index" json:"tenantId"`
}

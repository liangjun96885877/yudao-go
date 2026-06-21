// Package model 是 chatter 模块的领域模型层。
// 移植标准：domain 层为纯业务，禁止 import 任何框架 / ORM 包。
package model

import "time"

// BizRef 多态业务引用：将 chatter 挂载到任意业务实体。
type BizRef struct {
	TenantID int64
	BizType  string
	BizID    int64
}

// Valid 校验引用是否完整。
func (r BizRef) Valid() bool { return r.BizType != "" && r.BizID > 0 }

// EventType 时间线事件类型。
type EventType string

const (
	EventCreate       EventType = "create"
	EventUpdate       EventType = "update"
	EventComment      EventType = "comment"
	EventApproval     EventType = "approval"
	EventStatusChange EventType = "status_change"
	EventAttachment   EventType = "attachment"
	EventFollow       EventType = "follow"
	EventSystem       EventType = "system"
	EventAISummary    EventType = "ai_summary"
)

// ActorType 操作者类型。
type ActorType int8

const (
	ActorUser   ActorType = 1 // 用户
	ActorSystem ActorType = 2 // 系统
	ActorAI     ActorType = 3 // AI Agent（预留）
)

// Actor 操作者。
type Actor struct {
	Type ActorType
	ID   int64
	Name string
}

// SystemActor 返回系统操作者。
func SystemActor() Actor { return Actor{Type: ActorSystem, Name: "系统"} }

// Visibility 时间线条目可见性。
type Visibility int8

const (
	VisibilityPublic   Visibility = 1 // 公开
	VisibilityInternal Visibility = 2 // 内部
)

// ValueType 字段变更的值类型，用于前端选择展示方式。
type ValueType string

const (
	ValueString ValueType = "string"
	ValueInt    ValueType = "int"
	ValueDate   ValueType = "date"
	ValueEnum   ValueType = "enum" // 需 OldDisplay/NewDisplay 提供枚举展示值
	ValueRef    ValueType = "ref"  // 外键，需 OldDisplay/NewDisplay 提供关联展示值
)

// FieldChange 单个字段变更（值对象）。
type FieldChange struct {
	Field      string    // 字段名
	Label      string    // 展示名
	OldValue   string    // 原始旧值
	NewValue   string    // 原始新值
	OldDisplay string    // 旧值展示（枚举 / 外键解析后）
	NewDisplay string    // 新值展示
	ValueType  ValueType // 值类型
}

// nowPtr 返回当前时间指针，供可空时间字段使用。
func nowPtr() *time.Time { t := time.Now(); return &t }

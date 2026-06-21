package orm

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// TxManager 管理事务边界。移植标准：事务边界在 application 层，repository 用 DB(ctx) 取连接。
type TxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *TxManager { return &TxManager{db: db} }

// Do 在事务中执行 fn。若 ctx 已处于事务中则复用（支持嵌套调用，不会开启新事务）。
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx) // 已在事务内，复用
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DB 返回当前应使用的 *gorm.DB：事务内返回事务连接，否则返回根连接。
// 始终绑定 ctx，确保多租户/审计插件能读到上下文。
func (m *TxManager) DB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return m.db.WithContext(ctx)
}

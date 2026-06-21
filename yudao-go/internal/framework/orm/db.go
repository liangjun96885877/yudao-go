// Package orm 封装 GORM：连接、基类、多租户与审计插件、事务管理。
package orm

import (
	"time"

	gormtracing "gorm.io/plugin/opentelemetry/tracing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"yudao-go/internal/framework/config"
)

// Open 根据配置打开数据库连接并配置连接池。
func Open(cfg config.Database) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		// 跳过默认事务包装，提升写入性能；事务边界由 TxManager 显式控制。
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeSec > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)
	}
	return db, nil
}

// RegisterPlugins 注册多租户、审计字段填充、链路追踪插件。
// 须在 Open 之后、使用 db 之前调用一次。
func RegisterPlugins(db *gorm.DB) error {
	if err := RegisterTenantPlugin(db); err != nil {
		return err
	}
	if err := RegisterAuditFillPlugin(db); err != nil {
		return err
	}
	if err := RegisterDataPermPlugin(db); err != nil {
		return err
	}
	// 链路追踪：为每个 SQL 操作生成 span（仅追踪，不采集 metrics）。
	return db.Use(gormtracing.NewPlugin(gormtracing.WithoutMetrics()))
}

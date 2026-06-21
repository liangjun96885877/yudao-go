package rest

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/web"
)

// MonitorHandler 提供 Redis 监控、MySQL 监控接口。
type MonitorHandler struct {
	rdb *redis.Client
	tx  *orm.TxManager
}

func NewMonitorHandler(rdb *redis.Client, tx *orm.TxManager) *MonitorHandler {
	return &MonitorHandler{rdb: rdb, tx: tx}
}

func (h *MonitorHandler) Register(g *gin.RouterGroup) {
	g.GET("/infra/redis/get-monitor-info", h.redisInfo)
	g.GET("/infra/db/get-monitor-info", h.dbInfo)
}

// ===== Redis 监控 =====

// redisInfo 返回 Redis 的 INFO 指标、Key 数量与命令统计。
func (h *MonitorHandler) redisInfo(c *gin.Context) {
	ctx := c.Request.Context()
	raw, err := h.rdb.Info(ctx, "all").Result()
	if err != nil {
		web.FailErr(c, err)
		return
	}
	info, cmdStats := parseRedisInfo(raw)
	dbSize, _ := h.rdb.DBSize(ctx).Result()
	web.Success(c, gin.H{
		"info":         info,
		"dbSize":       dbSize,
		"commandStats": cmdStats,
	})
}

// parseRedisInfo 解析 Redis INFO 文本：普通指标进 info，cmdstat_* 进命令统计。
func parseRedisInfo(raw string) (map[string]string, []gin.H) {
	info := map[string]string{}
	cmdStats := make([]gin.H, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			continue
		}
		key, val := line[:idx], line[idx+1:]
		if strings.HasPrefix(key, "cmdstat_") {
			var calls, usec int64
			for _, kv := range strings.Split(val, ",") {
				p := strings.SplitN(kv, "=", 2)
				if len(p) != 2 {
					continue
				}
				switch p[0] {
				case "calls":
					calls, _ = strconv.ParseInt(p[1], 10, 64)
				case "usec":
					usec, _ = strconv.ParseInt(p[1], 10, 64)
				}
			}
			cmdStats = append(cmdStats, gin.H{
				"command": strings.TrimPrefix(key, "cmdstat_"),
				"calls":   calls, "usec": usec,
			})
		} else {
			info[key] = val
		}
	}
	return info, cmdStats
}

// ===== MySQL 监控 =====

// dbStatusKeys 是 SHOW GLOBAL STATUS 中需要展示的指标白名单。
var dbStatusKeys = map[string]bool{
	"Uptime": true, "Threads_connected": true, "Threads_running": true,
	"Connections": true, "Aborted_connects": true, "Max_used_connections": true,
	"Queries": true, "Questions": true, "Slow_queries": true,
	"Com_select": true, "Com_insert": true, "Com_update": true, "Com_delete": true,
	"Bytes_received": true, "Bytes_sent": true, "Table_locks_waited": true,
	"Innodb_buffer_pool_reads": true, "Innodb_buffer_pool_read_requests": true,
	"Innodb_rows_read": true, "Innodb_rows_inserted": true,
}

// dbInfo 返回数据库连接池状态与 MySQL 服务器运行指标。
func (h *MonitorHandler) dbInfo(c *gin.Context) {
	gormDB := h.tx.DB(c.Request.Context())
	sqlDB, err := gormDB.DB()
	if err != nil {
		web.FailErr(c, err)
		return
	}
	// 连接池（database/sql）统计
	st := sqlDB.Stats()
	pool := gin.H{
		"maxOpenConnections": st.MaxOpenConnections,
		"openConnections":    st.OpenConnections,
		"inUse":              st.InUse,
		"idle":               st.Idle,
		"waitCount":          st.WaitCount,
		"waitDurationMs":     st.WaitDuration.Milliseconds(),
		"maxIdleClosed":      st.MaxIdleClosed,
		"maxLifetimeClosed":  st.MaxLifetimeClosed,
	}
	// MySQL 服务器运行指标
	server := map[string]string{}
	rows, err := gormDB.Raw("SHOW GLOBAL STATUS").Rows()
	if err != nil {
		web.FailErr(c, err)
		return
	}
	for rows.Next() {
		var name, value string
		if rows.Scan(&name, &value) == nil && dbStatusKeys[name] {
			server[name] = value
		}
	}
	_ = rows.Close()
	// 版本与最大连接数
	var version string
	gormDB.Raw("SELECT VERSION()").Scan(&version)
	server["version"] = version
	var maxConn struct {
		Name  string `gorm:"column:Variable_name"`
		Value string `gorm:"column:Value"`
	}
	if gormDB.Raw("SHOW VARIABLES LIKE 'max_connections'").Scan(&maxConn).Error == nil {
		server["max_connections"] = maxConn.Value
	}
	web.Success(c, gin.H{"pool": pool, "server": server})
}

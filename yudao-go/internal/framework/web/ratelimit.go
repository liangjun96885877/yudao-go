package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit 返回限流中间件（横切能力 #10）：基于 Redis 固定窗口计数，按 IP 维度。
// perMinute <= 0 时不限流。Redis 异常时放行（不因限流组件故障阻断业务）。
func RateLimit(rdb *redis.Client, perMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if perMinute <= 0 {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		window := time.Now().Unix() / 60
		key := fmt.Sprintf("ratelimit:%s:%d", c.ClientIP(), window)
		n, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			rdb.Expire(ctx, key, 70*time.Second)
		}
		if n > int64(perMinute) {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code": 429, "msg": "请求过于频繁，请稍后再试", "data": nil,
			})
			return
		}
		c.Next()
	}
}

// Idempotent 返回幂等中间件（横切能力 #9）：
// 写请求携带 Idempotency-Key 头时，用 Redis SETNX 防重复提交；不带该头则不拦截。
func Idempotent(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		ok, err := rdb.SetNX(c.Request.Context(), "idempotent:"+key, "1", 5*time.Minute).Result()
		if err != nil {
			c.Next()
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code": 900, "msg": "请求重复，请勿重复提交", "data": nil,
			})
			return
		}
		c.Next()
	}
}

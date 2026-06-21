// Package redisx 封装 Redis：客户端、缓存、分布式锁。移植标准：业务不直连 redis.Client。
package redisx

import (
	"context"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"yudao-go/internal/framework/config"
)

// Client 是 Redis 客户端封装。go-redis 的 *redis.Client 本身并发安全。
type Client struct {
	rdb *redis.Client
}

// New 创建并校验 Redis 连接。
func New(cfg config.Redis) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	// 链路追踪：为每个 Redis 命令生成 span。
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

// Raw 返回底层 go-redis 客户端，供需要原生能力的封装层使用。
func (c *Client) Raw() *redis.Client { return c.rdb }

func (c *Client) Close() error { return c.rdb.Close() }

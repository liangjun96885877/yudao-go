package redisx

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Cache 是带前缀的 JSON 缓存。移植标准：缓存 key 统一前缀 {模块}:{业务}:{标识}。
type Cache struct {
	client *Client
	prefix string
	sf     singleflight.Group // 防缓存击穿：同一 key 并发回源只执行一次
}

// NewCache 创建缓存实例，prefix 形如 "chatter:timeline"。
func NewCache(client *Client, prefix string) *Cache {
	return &Cache{client: client, prefix: prefix}
}

func (c *Cache) fullKey(key string) string { return c.prefix + ":" + key }

// GetOrLoad 读取缓存；未命中时经 singleflight 回源 loader 并写回。
// 泛型为包级函数（Go 方法不支持类型参数）。
func GetOrLoad[T any](
	ctx context.Context, c *Cache, key string, ttl time.Duration,
	loader func(ctx context.Context) (T, error),
) (T, error) {
	var zero T
	full := c.fullKey(key)

	// 1. 查缓存
	raw, err := c.client.rdb.Get(ctx, full).Bytes()
	if err == nil {
		var v T
		if jsonErr := json.Unmarshal(raw, &v); jsonErr == nil {
			return v, nil
		}
		// 反序列化失败则当作未命中，回源覆盖
	} else if err != redis.Nil {
		return zero, err // Redis 故障，向上抛出
	}

	// 2. 回源（同 key 并发合并为一次）
	result, err, _ := c.sf.Do(full, func() (any, error) {
		v, loadErr := loader(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		if data, mErr := json.Marshal(v); mErr == nil {
			_ = c.client.rdb.Set(ctx, full, data, ttl).Err() // 写回失败不阻断主流程
		}
		return v, nil
	})
	if err != nil {
		return zero, err
	}
	return result.(T), nil
}

// Delete 删除缓存 key。
func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.rdb.Del(ctx, c.fullKey(key)).Err()
}

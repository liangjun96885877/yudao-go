package redisx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"yudao-go/internal/pkg/errcode"
	"yudao-go/internal/pkg/idgen"
)

// unlockScript 仅在锁仍归当前持有者（token 匹配）时删除，避免误删他人在 TTL 过期后重新获取的锁。
var unlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

// Lock 表示一把已持有的分布式锁。
type Lock struct {
	client *Client
	key    string
	token  string
}

// Release 释放锁。并发安全：通过 token 校验只释放自己持有的锁。
func (l *Lock) Release(ctx context.Context) error {
	return unlockScript.Run(ctx, l.client.rdb, []string{l.key}, l.token).Err()
}

// AcquireLock 尝试获取分布式锁。返回 ok=false 表示锁已被占用。ttl 必须为正值。
func (c *Client) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*Lock, bool, error) {
	token := idgen.UUID()
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return &Lock{client: c, key: key, token: token}, true, nil
}

// WithLock 持锁执行 fn。获取失败返回 errcode.RepeatedRequest。
// 释放锁使用脱离取消的 context，避免请求结束后锁无法释放。
func (c *Client) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	lock, ok, err := c.AcquireLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !ok {
		return errcode.RepeatedRequest
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = lock.Release(releaseCtx)
	}()
	return fn()
}

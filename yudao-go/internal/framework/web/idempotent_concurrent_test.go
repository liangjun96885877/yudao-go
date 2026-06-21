// Idempotent 中间件并发安全集成测试。
//
// 用真实 Redis(deploy/docker-compose.yml 的 yudao-go-redis:16381),不可用时 Skip。
// 跑: CGO_ENABLED=1 go test -race -count=1 ./internal/framework/web -run TestIdempotent
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"yudao-go/internal/pkg/idgen"
)

const idempTestRedisAddr = "127.0.0.1:16381"

func openIdempotentTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr: idempTestRedisAddr, DialTimeout: 800 * time.Millisecond,
		ReadTimeout: 800 * time.Millisecond, WriteTimeout: 800 * time.Millisecond,
	})
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		t.Skipf("跳过 Idempotent 集成测试: Redis 不可用 (%v)", err)
	}
	return rdb
}

// newIdempotentRouter 起一个仅挂 Idempotent 中间件的最小 router,
// /poke 路由直接 200,Idempotency-Key 重复时被中间件拦截返回 code:900。
func newIdempotentRouter(rdb *redis.Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(Idempotent(rdb))
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
	}
	r.POST("/poke", handler)
	r.GET("/poke", handler)
	return r
}

// TestIdempotent_SameKeyConcurrent 验证同一 Idempotency-Key 并发 50 个 POST
// → 仅 1 个 code:0,其余 49 个 code:900。
func TestIdempotent_SameKeyConcurrent(t *testing.T) {
	rdb := openIdempotentTestRedis(t)
	defer rdb.Close()
	r := newIdempotentRouter(rdb)

	key := "test-idemp-" + idgen.UUID()
	defer rdb.Del(context.Background(), "idempotent:"+key)

	const N = 50
	var (
		passCnt atomic.Int64
		dupCnt  atomic.Int64
	)
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/poke", nil)
			req.Header.Set("Idempotency-Key", key)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			body := w.Body.String()
			switch {
			case strings.Contains(body, `"code":0`):
				passCnt.Add(1)
			case strings.Contains(body, `"code":900`):
				dupCnt.Add(1)
			default:
				t.Errorf("非预期响应: %s", body)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}

	if passCnt.Load() != 1 {
		t.Fatalf("幂等失效: 期望 pass=1, 实际 %d (重复 %d)", passCnt.Load(), dupCnt.Load())
	}
	if dupCnt.Load() != N-1 {
		t.Fatalf("拦截数量不对: 期望 %d, 实际 %d", N-1, dupCnt.Load())
	}
}

// TestIdempotent_DifferentKeysAllPass 不同 key 之间互不影响,N 个不同 key 全部通过。
func TestIdempotent_DifferentKeysAllPass(t *testing.T) {
	rdb := openIdempotentTestRedis(t)
	defer rdb.Close()
	r := newIdempotentRouter(rdb)

	const N = 20
	keys := make([]string, N)
	for i := 0; i < N; i++ {
		keys[i] = "test-idemp-uniq-" + idgen.UUID()
	}
	defer func() {
		for _, k := range keys {
			rdb.Del(context.Background(), "idempotent:"+k)
		}
	}()

	var pass atomic.Int64
	done := make(chan struct{}, N)
	for _, k := range keys {
		go func(k string) {
			req := httptest.NewRequest(http.MethodPost, "/poke", nil)
			req.Header.Set("Idempotency-Key", k)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if strings.Contains(w.Body.String(), `"code":0`) {
				pass.Add(1)
			}
			done <- struct{}{}
		}(k)
	}
	for i := 0; i < N; i++ {
		<-done
	}
	if pass.Load() != N {
		t.Fatalf("不同 key 应全部通过: 期望 %d, 实际 %d", N, pass.Load())
	}
}

// TestIdempotent_GETBypassed GET 请求即便带 Idempotency-Key 也不应被拦截。
func TestIdempotent_GETBypassed(t *testing.T) {
	rdb := openIdempotentTestRedis(t)
	defer rdb.Close()
	r := newIdempotentRouter(rdb)

	key := "test-idemp-get-" + idgen.UUID()
	defer rdb.Del(context.Background(), "idempotent:"+key)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/poke", nil)
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), `"code":0`) {
			t.Fatalf("GET 不应被幂等拦截, 第 %d 次响应: %s", i+1, w.Body.String())
		}
	}
}

// TestIdempotent_NoKeyBypassed 不带 Idempotency-Key 的写请求不应被中间件干预。
func TestIdempotent_NoKeyBypassed(t *testing.T) {
	rdb := openIdempotentTestRedis(t)
	defer rdb.Close()
	r := newIdempotentRouter(rdb)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/poke", nil)
		// 不设 Idempotency-Key
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), `"code":0`) {
			t.Fatalf("无 key 不应拦截, 第 %d 次响应: %s", i+1, w.Body.String())
		}
	}
}

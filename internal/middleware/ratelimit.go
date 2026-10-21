package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	lru "github.com/hashicorp/golang-lru/v2"
)

// Limiter 按任意 key（IP、用户 ID、邮箱）计数：每个 window 内最多 limit 次。
// shortcut: 计数存在进程内存里，部署多个实例时要换成 Redis
type Limiter struct {
	limit   int
	window  time.Duration
	mu      sync.Mutex
	buckets *lru.Cache[string, *bucket]
}

type bucket struct {
	count int
	start time.Time
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	buckets, _ := lru.New[string, *bucket](10000) // 只在 size<=0 时报错
	return &Limiter{limit: limit, window: window, buckets: buckets}
}

// Allow 记一次并返回是否仍在限额内
func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets.Get(key)
	if !ok || now.Sub(b.start) >= l.window {
		b = &bucket{start: now}
		l.buckets.Add(key, b)
	}
	b.count++
	return b.count <= l.limit
}

// RateLimit 按客户端 IP 限制：每个 window 内最多 limit 次请求，超出返回 429。
// 每次调用返回一个独立计数的中间件，所以每个接口各算各的。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	l := NewLimiter(limit, window)
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.String(http.StatusTooManyRequests, "操作太频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

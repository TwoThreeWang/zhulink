package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	lru "github.com/hashicorp/golang-lru/v2"
)

// RateLimit 按客户端 IP 限制：每个 window 内最多 limit 次请求，超出返回 429。
// 每次调用返回一个独立计数的中间件，所以每个接口各算各的。
// shortcut: 计数存在进程内存里，部署多个实例时要换成 Redis
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		count int
		start time.Time
	}
	buckets, _ := lru.New[string, *bucket](10000) // 只在 size<=0 时报错
	var mu sync.Mutex

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := buckets.Get(ip)
		if !ok || now.Sub(b.start) >= window {
			b = &bucket{start: now}
			buckets.Add(ip, b)
		}
		b.count++
		over := b.count > limit
		mu.Unlock()

		if over {
			c.String(http.StatusTooManyRequests, "操作太频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders 给所有响应加上基础安全头。
// 页面里有大量内联脚本和第三方广告/统计脚本，暂时没法用 script-src 限制脚本来源，
// 这里只加不会影响现有功能的几项
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")                  // 不让浏览器猜 Content-Type
		h.Set("X-Frame-Options", "SAMEORIGIN")                      // 禁止被外站 iframe 嵌入（点击劫持）
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin") // 跨站只带域名，不带完整路径
		h.Set("Content-Security-Policy", "base-uri 'self'; object-src 'none'; frame-ancestors 'self'")
		c.Next()
	}
}

// BodyLimit 限制请求体大小：uploadPath 用 uploadMax，其余用 defaultMax。
// 超出后表单解析失败，各 handler 按"没填"处理
func BodyLimit(defaultMax int64, uploadPath string, uploadMax int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			max := defaultMax
			if c.Request.URL.Path == uploadPath {
				max = uploadMax
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}

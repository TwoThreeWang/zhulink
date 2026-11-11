package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "SAMEORIGIN",
		"Content-Security-Policy": "frame-ancestors 'self'",
	} {
		if got := w.Header().Get(header); !strings.Contains(got, want) {
			t.Errorf("%s = %q，应包含 %q", header, got, want)
		}
	}
}

func TestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(64, "/upload", 1024))
	echo := func(c *gin.Context) { c.String(http.StatusOK, c.PostForm("v")) }
	r.POST("/form", echo)
	r.POST("/upload", echo)

	post := func(path string, size int) string {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("v="+strings.Repeat("a", size)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Body.String()
	}

	if got := post("/form", 10); len(got) != 10 {
		t.Errorf("限额内的表单应正常解析，得到 %d 字节", len(got))
	}
	if got := post("/form", 500); got != "" {
		t.Errorf("超出限额的表单不应被解析，得到 %d 字节", len(got))
	}
	if got := post("/upload", 500); len(got) != 500 {
		t.Errorf("上传路径应使用更大的限额，得到 %d 字节", len(got))
	}
}

func TestLimiterSizeDedupes(t *testing.T) {
	l := NewLimiterSize(1, time.Hour, 100)
	if !l.Allow("1.2.3.4|abc") {
		t.Fatal("第一次应放行")
	}
	if l.Allow("1.2.3.4|abc") {
		t.Fatal("同一个 key 在窗口内不应重复放行")
	}
	if !l.Allow("1.2.3.4|other") || !l.Allow("5.6.7.8|abc") {
		t.Fatal("不同帖子、不同 IP 应各自计数")
	}
}

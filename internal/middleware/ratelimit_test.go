package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", RateLimit(2, time.Minute), func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := []int{}
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("got %v, want [200 200 429]", codes)
	}

	// 其他 IP 不受影响
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.2:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("other ip got %d", w.Code)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(1, 50*time.Millisecond)
	if !l.Allow("a") || l.Allow("a") {
		t.Fatal("key a: want first allowed, second denied")
	}
	if !l.Allow("b") {
		t.Fatal("key b should be counted separately")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("key a should be allowed again after the window")
	}
}

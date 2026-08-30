package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"zhulink/internal/middleware"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestStartUserSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := cookie.NewStore(bytes.Repeat([]byte{1}, 64), bytes.Repeat([]byte{2}, 32))
	store.Options(middleware.SessionCookieOptions(middleware.SessionIdleMaxAge))

	router := gin.New()
	router.Use(sessions.Sessions("zhulink_session", store))
	router.GET("/start", func(c *gin.Context) {
		sessions.Default(c).Options(middleware.SessionCookieOptions(-1))
		if err := startUserSession(c, 42); err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/read", func(c *gin.Context) {
		session := sessions.Default(c)
		if session.Get(middleware.SessionUserIDKey) != uint(42) {
			c.Status(http.StatusUnauthorized)
			return
		}
		if _, ok := session.Get(middleware.SessionIssuedKey).(int64); !ok {
			c.Status(http.StatusUnauthorized)
			return
		}
		if _, ok := session.Get(middleware.SessionRenewedKey).(int64); !ok {
			c.Status(http.StatusUnauthorized)
			return
		}
		if _, ok := session.Get(middleware.SessionExpiresKey).(int64); !ok {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusNoContent)
	})

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodGet, "/start", nil)
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusNoContent {
		t.Fatalf("start session status = %d, want %d", startRecorder.Code, http.StatusNoContent)
	}
	cookies := startRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("start session cookies = %d, want 1", len(cookies))
	}
	if cookies[0].MaxAge != middleware.SessionIdleMaxAge || !cookies[0].HttpOnly {
		t.Fatalf("unexpected session cookie options: MaxAge=%d HttpOnly=%v", cookies[0].MaxAge, cookies[0].HttpOnly)
	}

	readRecorder := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, "/read", nil)
	readRequest.AddCookie(cookies[0])
	router.ServeHTTP(readRecorder, readRequest)
	if readRecorder.Code != http.StatusNoContent {
		t.Fatalf("read session status = %d, want %d", readRecorder.Code, http.StatusNoContent)
	}
}

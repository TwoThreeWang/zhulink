package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"zhulink/internal/middleware"
	"zhulink/internal/router"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// 未登录访客的请求不会查数据库，可以不连库验证路由和中间件的接线
func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.BodyLimit(2<<20, "/api/upload", 12<<20))
	store := cookie.NewStore(bytes.Repeat([]byte{1}, 64), bytes.Repeat([]byte{2}, 32))
	store.Options(middleware.SessionCookieOptions(middleware.SessionIdleMaxAge))
	r.Use(sessions.Sessions("zhulink_session", store))
	r.HTMLRender = loadTemplates("./web/templates")
	r.Use(middleware.LoadUser())
	router.RegisterRoutes(r)
	return r
}

func TestStateChangingRoutesAreNotGET(t *testing.T) {
	r := newTestEngine()
	do := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		return w
	}

	// 退出登录：GET 不存在，POST 才生效
	if w := do(http.MethodGet, "/logout"); w.Code != http.StatusNotFound {
		t.Errorf("GET /logout = %d，期望 404（帖子里的图片地址不能触发退出）", w.Code)
	}
	if w := do(http.MethodPost, "/logout"); w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Errorf("POST /logout = %d -> %q，期望 302 到 /", w.Code, w.Header().Get("Location"))
	}

	// 管理员恢复帖子：GET、POST 都要登录
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if w := do(method, "/admin/post/abcdefgh/restore-ai-block"); w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
			t.Errorf("%s restore-ai-block 未登录 = %d -> %q，期望跳转登录", method, w.Code, w.Header().Get("Location"))
		}
	}

	// 已废弃的绑定回调地址不再存在
	if w := do(http.MethodGet, "/dashboard/settings/bind-google/callback"); w.Code != http.StatusNotFound {
		t.Errorf("旧的绑定回调 = %d，期望 404", w.Code)
	}

	// 安全头对错误页也生效
	w := do(http.MethodGet, "/no-such-page")
	if w.Code != http.StatusNotFound || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("404 页缺少安全头: %d %v", w.Code, w.Header())
	}
}

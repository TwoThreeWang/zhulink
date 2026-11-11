package main

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"zhulink/internal/models"

	"github.com/gin-gonic/gin"
)

// 模板和静态文件路径相对于仓库根目录
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// 所有模板都能解析，并且带危险外链的帖子在各个列表页里渲染不出可执行的地址
func TestTemplatesRenderAndFilterDangerousURL(t *testing.T) {
	renderer := loadTemplates("./web/templates")

	posts := []models.Post{
		{ID: 1, Pid: "aaaaaaaa", Title: "危险链接", URL: "javascript:alert(document.domain)//"},
		{ID: 2, Pid: "bbbbbbbb", Title: "正常链接", URL: "https://example.com/a?b=1&c=2"},
	}
	render := func(name string, data gin.H) string {
		t.Helper()
		data["SiteURL"] = "https://zhulink.vip"
		data["CurrentPath"] = "/"
		w := httptest.NewRecorder()
		if err := renderer.Instance(name, data).Render(w); err != nil {
			t.Fatalf("渲染 %s 失败: %v", name, err)
		}
		return w.Body.String()
	}

	pages := map[string]string{
		"story/list.html":  render("story/list.html", gin.H{"Posts": posts, "Active": "top", "Title": "热门", "CurrentPage": 1, "TotalPages": 1}),
		"search.html":      render("search.html", gin.H{"Posts": posts, "Query": "x", "Total": int64(2), "CurrentPage": 1, "TotalPages": 1}),
		"user/public.html": render("user/public.html", gin.H{"User": models.User{ID: 1, Username: "u"}, "Posts": posts, "BookmarkedPosts": posts, "ActiveTab": "posts", "IsOwner": true}),
	}
	for name, html := range pages {
		if strings.Contains(html, "javascript:alert") {
			t.Errorf("%s 输出了 javascript: 链接", name)
		}
		if strings.Contains(html, "window.open('") {
			t.Errorf("%s 仍把链接直接拼进 window.open 的字符串参数", name)
		}
		if !strings.Contains(html, `data-ext-url="https://example.com/a?b=1&amp;c=2"`) {
			t.Errorf("%s 没有输出正常的外链", name)
		}
		if !strings.Contains(html, `href="/p/aaaaaaaa"`) {
			t.Errorf("%s 危险链接的帖子应退化为普通站内链接", name)
		}
	}

	// 退出登录必须是 POST 表单，不能再有 GET 链接
	loggedIn := render("story/list.html", gin.H{"Posts": posts, "CurrentUser": &models.User{ID: 1, Username: "u"}, "UnreadCount": 0, "CurrentPage": 1, "TotalPages": 1})
	if strings.Contains(loggedIn, `href="/logout"`) || !strings.Contains(loggedIn, `<form action="/logout" method="POST">`) {
		t.Error("导航栏的退出登录不是 POST 表单")
	}

	confirm := render("admin/restore_confirm.html", gin.H{"Post": models.Post{Pid: "cccccccc", Title: "<b>被拦截</b>", User: models.User{ID: 2, Username: "author"}}})
	if !strings.Contains(confirm, `<form action="/admin/post/cccccccc/restore-ai-block" method="POST"`) || strings.Contains(confirm, "<b>被拦截</b>") {
		t.Error("恢复确认页没有 POST 表单，或标题未转义")
	}
}

package utils

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBlocksInjection(t *testing.T) {
	for _, src := range []string{
		`https://www.bilibili.com/video/x"onload="alert(1)`,
		`https://www.youtube.com/watch?v=x"onload="alert(1)`,
		`https://youtu.be/x"onload="alert(1)`,
		`https://b23.tv/x"onmouseover="alert(1)`,
	} {
		if out := string(RenderMarkdown(src)); strings.Contains(out, `onload="`) || strings.Contains(out, `onmouseover="`) {
			t.Errorf("注入未被拦截: %s\n=> %s", src, out)
		}
	}

	if out := SanitizeHTML(`<div style="position:fixed" class="fixed inset-0"><iframe style="position:fixed" class="fixed" src="https://a.example"></iframe></div>`); strings.Contains(out, "style=") || strings.Contains(out, "class=") {
		t.Errorf("style/class 未被过滤: %s", out)
	}

	if out := string(RenderMarkdown("https://www.bilibili.com/video/BV1xx411c7mD")); !strings.Contains(out, "bvid=BV1xx411c7mD") {
		t.Errorf("正常视频链接未嵌入: %s", out)
	}
}

func TestImageAltKeepsCJKPunctuation(t *testing.T) {
	out := string(RenderMarkdown("![微信图片（1），封面：首页](https://a.example/x.png)"))
	if !strings.Contains(out, `alt="微信图片（1），封面：首页"`) {
		t.Fatalf("alt dropped: %s", out)
	}
}

func TestRenderArticleHeadingIDsAndTOC(t *testing.T) {
	src := "## 背景 介绍\n\n正文\n\n### Install **安装**\n\n### comments\n\n## 背景 介绍\n\n#### 不进目录\n\n## ！！！\n"
	out, toc := RenderArticle(src)
	html := string(out)

	for _, want := range []string{
		`id="toc-背景-介绍"`, `id="toc-install-安装"`, `id="toc-comments"`,
		`id="toc-背景-介绍-2"`, `id="toc-不进目录"`, `id="toc-section"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("缺少 %s\n=> %s", want, html)
		}
	}
	if strings.Contains(html, `id="comments"`) {
		t.Errorf("标题 id 未加前缀，会与页面锚点冲突: %s", html)
	}

	want := []TOCItem{
		{"toc-背景-介绍", "背景 介绍", 0},
		{"toc-install-安装", "Install 安装", 1},
		{"toc-comments", "comments", 1},
		{"toc-背景-介绍-2", "背景 介绍", 0},
		{"toc-section", "！！！", 0},
	}
	if len(toc) != len(want) {
		t.Fatalf("目录项数量 %d，期望 %d: %+v", len(toc), len(want), toc)
	}
	for i := range want {
		if toc[i] != want[i] {
			t.Errorf("目录第 %d 项 %+v，期望 %+v", i, toc[i], want[i])
		}
	}
}

func TestRenderArticleTOCThreshold(t *testing.T) {
	if _, toc := RenderArticle("## 一\n\n## 二\n"); toc != nil {
		t.Errorf("标题不足 3 个时不应生成目录: %+v", toc)
	}
	if _, toc := RenderArticle("# 一\n\n## 二\n\n### 三\n"); len(toc) != 3 || toc[0].Depth != 0 || toc[2].Depth != 2 {
		t.Errorf("h1~h3 应全部进入目录: %+v", toc)
	}
}

func TestRenderMarkdownSnippetHasNoHeadingIDs(t *testing.T) {
	if out := string(RenderMarkdown("## comments")); strings.Contains(out, "id=") {
		t.Errorf("评论标题不应带 id: %s", out)
	}
}

func TestImageSrcRestrictedToExternalOrSiteImages(t *testing.T) {
	t.Setenv("SITE_URL", "https://zhulink.vip")

	// 指向站内页面的"图片"会让每个浏览者带着 Cookie 去请求那个地址，必须去掉
	for _, src := range []string{
		"![x](/logout)",
		"![x](/admin/post/abcdefgh/restore-ai-block)",
		"![x](https://zhulink.vip/logout)",
		"![x](https://ZhuLink.vip/dashboard/settings/bind-google)",
		"![x](//zhulink.vip/logout)",
		"![x](logout)",
		"![x](/img/../logout)",
		"![x](https://zhulink.vip/img/../logout)",
	} {
		if out := string(RenderMarkdown(src)); strings.Contains(out, "<img") {
			t.Errorf("站内地址的图片未被去掉: %s\n=> %s", src, out)
		}
	}

	for _, src := range []string{
		"![x](https://a.example/x.png)",
		"![x](http://a.example/x.png)",
		"![x](/img/AbCd123.jpg)",
		"![x](/static/img/logo.svg)",
		"![x](https://zhulink.vip/img/AbCd123.jpg)",
	} {
		if out := string(RenderMarkdown(src)); !strings.Contains(out, "<img") {
			t.Errorf("正常图片被误删: %s\n=> %s", src, out)
		}
	}
}

func TestRenderMarkdownEmptyAfterSanitize(t *testing.T) {
	// 原始 HTML 会被整个丢掉，结果应该是空串，而不是 <html><head></head><body></body></html>
	for _, src := range []string{"<script>alert(1)</script>", "<iframe src=\"https://a.example\"></iframe>", ""} {
		if out := string(RenderMarkdown(src)); strings.Contains(out, "<html") || strings.Contains(out, "<body") || strings.Contains(out, "<script") || strings.Contains(out, "<iframe") {
			t.Errorf("净化后应为空: %q => %q", src, out)
		}
	}
}

func TestFirstBlocks(t *testing.T) {
	html := string(RenderMarkdown("> 引用里的段落\n\n第二段\n\n- a\n- b\n\n第四段"))
	out := FirstBlocks(html, 3)
	if strings.Contains(out, "第四段") {
		t.Errorf("超出的块未被截掉: %s", out)
	}
	for _, tag := range []string{"blockquote", "p", "ul", "li"} {
		if open, closed := strings.Count(out, "<"+tag+">"), strings.Count(out, "</"+tag+">"); open == 0 || open != closed {
			t.Errorf("<%s> 标签不成对（%d 开 %d 闭）: %s", tag, open, closed, out)
		}
	}
	if FirstBlocks("", 3) != "" {
		t.Error("空内容应返回空串")
	}
}

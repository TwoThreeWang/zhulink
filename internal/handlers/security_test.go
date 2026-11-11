package handlers

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"zhulink/internal/models"
	"zhulink/internal/utils"

	"github.com/gin-gonic/gin"
)

func TestValidatePostInput(t *testing.T) {
	if title, url, msg := validatePostInput("  标题  ", " https://example.com/a ", "正文"); msg != "" || title != "标题" || url != "https://example.com/a" {
		t.Fatalf("正常输入被拒绝或未整理: %q %q %q", title, url, msg)
	}
	if _, _, msg := validatePostInput("标题", "", ""); msg != "" {
		t.Fatalf("链接和正文可以留空: %q", msg)
	}
	bad := []struct{ title, url, content string }{
		{"   ", "", ""},
		{strings.Repeat("长", utils.MaxTitleRunes+1), "", ""},
		{"标题", "javascript:alert(document.domain)//", ""},
		{"标题", "data:text/html,x", ""},
		{"标题", "example.com", ""},
		{"标题", "", strings.Repeat("字", utils.MaxPostRunes+1)},
	}
	for _, c := range bad {
		if _, _, msg := validatePostInput(c.title, c.url, c.content); msg == "" {
			t.Errorf("应拒绝: title=%.10q url=%q content长度=%d", c.title, c.url, len(c.content))
		}
	}
}

func TestPostSafeURL(t *testing.T) {
	if got := (models.Post{URL: "https://example.com/a"}).SafeURL(); got != "https://example.com/a" {
		t.Errorf("正常链接被丢掉: %q", got)
	}
	// 修复前已经存进库里的危险链接，输出时也要挡住
	for _, u := range []string{"javascript:alert(1)//", "data:text/html,x", "/logout"} {
		if got := (models.Post{URL: u}).SafeURL(); got != "" {
			t.Errorf("危险链接未被过滤: %q => %q", u, got)
		}
	}
}

// RSS 的正文放在 CDATA 里：用户内容不能跳出 CDATA，整个 feed 必须还是合法 XML
func TestRSSItemContentCannotBreakOutOfCDATA(t *testing.T) {
	payloads := []string{
		`]]><x:script xmlns:x="http://www.w3.org/1999/xhtml">alert(document.domain)</x:script><![CDATA[`,
		"正文 ]]> 结尾",
		"`]]>`",
		"```\n]]><script>alert(1)</script>\n```",
		"<p><img src=x onerror=alert(1)></p>",
		"控制字符\x00\x08\x0b 也不能把 feed 弄坏",
	}
	for _, p := range payloads {
		content := cdataSafe(rssItemContent(p, "https://zhulink.vip"))
		doc := `<rss><item><description><![CDATA[` + content + `]]></description></item></rss>`

		var parsed struct {
			Items []struct {
				Description string `xml:"description"`
				Extra       []struct {
					XMLName xml.Name
				} `xml:",any"`
			} `xml:"item"`
		}
		if err := xml.Unmarshal([]byte(doc), &parsed); err != nil {
			t.Errorf("feed 不是合法 XML: %q\n=> %s\n%v", p, doc, err)
			continue
		}
		if len(parsed.Items) != 1 || len(parsed.Items[0].Extra) != 0 {
			t.Errorf("内容跳出了 CDATA，注入了额外的 XML 节点: %q\n=> %s", p, doc)
		}
		for _, bad := range []string{"<script", "<x:script", "onerror"} {
			if strings.Contains(parsed.Items[0].Description, bad) {
				t.Errorf("RSS 正文里残留 %s: %q\n=> %s", bad, p, parsed.Items[0].Description)
			}
		}
	}
}

func TestRSSItemContentIsRenderedHTMLWithAbsoluteURLs(t *testing.T) {
	out := rssItemContent("## 标题\n\n**加粗** 和 ![图](/img/AbCd123.jpg)\n\n第三块\n\n第四块", "https://zhulink.vip")
	for _, want := range []string{"<h2>", "<strong>加粗</strong>", `src="https://zhulink.vip/img/AbCd123.jpg"`, "第三块"} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少 %s\n=> %s", want, out)
		}
	}
	if strings.Contains(out, "第四块") || strings.Contains(out, "## ") {
		t.Errorf("应只保留前 3 个块，且不输出 Markdown 原文: %s", out)
	}
}

func TestEscapeXMLStripsControlChars(t *testing.T) {
	if got := escapeXML("a\x00b<c>\x0b"); got != "ab&lt;c&gt;" {
		t.Errorf("escapeXML = %q", got)
	}
}

// 图片代理：非法的 id 或扩展名必须在请求上游之前就拒绝
func TestImageProxyRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/img/:id", NewImageHandler().Proxy)

	for _, path := range []string{
		"/img/..%2F..%2Fgallery.html",
		"/img/abc.html",
		"/img/AbCd123.svg",
		"/img/a.jpg",
		"/img/AbCd123%3Fx=1.jpg",
		"/img/" + strings.Repeat("a", 40) + ".jpg",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s 状态码 = %d，期望 404", path, w.Code)
		}
	}
}

func TestMarkdownLinkTextEscaper(t *testing.T) {
	if got := markdownLinkTextEscaper.Replace(`a](http://evil)[b\`); got != `a\](http://evil)\[b\\` {
		t.Errorf("转义结果 = %q", got)
	}
	// 用户名里带 ] 也不能把回复引用的链接改指到别处
	out := string(utils.RenderMarkdown("↳ 回复 [#1 @" + markdownLinkTextEscaper.Replace("x](https://evil.example/)") + "](#comment-5)\n\n正文"))
	if strings.Contains(out, `href="https://evil.example/"`) || !strings.Contains(out, `href="#comment-5"`) {
		t.Errorf("回复引用的链接被用户名改写: %s", out)
	}
}

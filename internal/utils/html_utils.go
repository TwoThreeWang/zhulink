package utils

import (
	"html/template"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// EnhanceHTMLContent 为 HTML 中的图片增加安全和优化属性
func EnhanceHTMLContent(htmlStr string) template.HTML {
	if htmlStr == "" {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return template.HTML(htmlStr)
	}

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		s.SetAttr("referrerpolicy", "no-referrer")
		s.SetAttr("rel", "noopener")
		s.SetAttr("loading", "lazy")
		s.SetAttr("onerror", "this.onerror=null; this.src='/static/img/imgerr.svg'")
	})

	// goquery renders full document tags if missing, we just want the body content
	html, _ := doc.Find("body").Html()
	if html == "" {
		html, _ = doc.Html()
	}

	return template.HTML(html)
}

package utils

import (
	"html/template"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// EnhanceHTMLContent 为 HTML 中的图片增加安全和优化属性,并转换视频链接为嵌入式播放器
func EnhanceHTMLContent(htmlStr string) template.HTML {
	if htmlStr == "" {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return template.HTML(htmlStr)
	}

	// 增强图片属性
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		s.SetAttr("referrerpolicy", "no-referrer")
		s.SetAttr("rel", "noopener")
		s.SetAttr("loading", "lazy")
		s.SetAttr("onerror", "this.onerror=null; this.src='/static/img/imgerr.svg'")
	})

	// 转换视频链接为嵌入式播放器
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())

		// 检查是否为单独的视频链接
		if strings.HasPrefix(text, "http") && !strings.Contains(text, " ") {
			var embedHTML string

			// Bilibili 视频
			if strings.Contains(text, "bilibili.com/video/") {
				// 提取 BV 号
				parts := strings.Split(text, "/video/")
				if len(parts) > 1 {
					bvid := strings.Split(parts[1], "?")[0]
					bvid = strings.TrimSuffix(bvid, "/")
					embedHTML = `<div class="video-container"><iframe src="https://player.bilibili.com/player.html?bvid=` + bvid + `&high_quality=1&autoplay=0" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`
				}
			} else if strings.Contains(text, "b23.tv/") {
				// Bilibili 短链接,保留原链接提示用户
				embedHTML = `<p class="text-stone-500 text-sm">⚠️ 请使用完整的 Bilibili 视频链接 (bilibili.com/video/BV...)</p><p><a href="` + text + `" target="_blank" rel="noopener noreferrer">` + text + `</a></p>`
			} else if strings.Contains(text, "youtube.com/watch?v=") {
				// YouTube 视频
				parts := strings.Split(text, "v=")
				if len(parts) > 1 {
					videoID := strings.Split(parts[1], "&")[0]
					embedHTML = `<div class="video-container"><iframe src="https://www.youtube.com/embed/` + videoID + `" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`
				}
			} else if strings.Contains(text, "youtu.be/") {
				// YouTube 短链接
				parts := strings.Split(text, "youtu.be/")
				if len(parts) > 1 {
					videoID := strings.Split(parts[1], "?")[0]
					embedHTML = `<div class="video-container"><iframe src="https://www.youtube.com/embed/` + videoID + `" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`
				}
			}

			// 替换原链接为嵌入式播放器
			if embedHTML != "" {
				s.ReplaceWithHtml(embedHTML)
			}
		}
	})

	// goquery renders full document tags if missing, we just want the body content
	html, _ := doc.Find("body").Html()
	if html == "" {
		html, _ = doc.Html()
	}

	return template.HTML(html)
}

package utils

import (
	"html"
	"html/template"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// SanitizeHTML 使用 bluemonday 消毒 HTML，移除 script/style 等危险标签
func SanitizeHTML(htmlStr string) string {
	if htmlStr == "" {
		return ""
	}
	return policy.Sanitize(htmlStr)
}

// videoIDPattern 视频 ID 只允许这些字符，防止拼进 iframe src 时注入属性
var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
		// 浏览器加载图片会带着登录 Cookie 发 GET，指向站内页面的"图片"等于替每个浏览者发请求
		if src, _ := s.Attr("src"); !allowedImageSrc(src) {
			s.Remove()
			return
		}
		s.SetAttr("referrerpolicy", "no-referrer")
		s.SetAttr("rel", "noopener")
		s.SetAttr("loading", "lazy")
		s.SetAttr("onerror", "this.onerror=null; this.src='/static/img/imgerr.svg'")
	})

	// 增强链接属性: 设置 noopener(安全) 和 nofollow(SEO)
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		// 只处理外部链接，忽略内部锚点或相对路径
		if strings.HasPrefix(href, "http") {
			s.SetAttr("rel", "noopener nofollow")
		}
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
					if videoIDPattern.MatchString(bvid) {
						embedHTML = `<div class="video-container"><iframe src="https://player.bilibili.com/player.html?bvid=` + bvid + `&high_quality=1&autoplay=0" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`
					}
				}
			} else if strings.Contains(text, "b23.tv/") {
				// Bilibili 短链接,保留原链接提示用户
				embedHTML = `<p class="text-stone-500 text-sm">⚠️ 请使用完整的 Bilibili 视频链接 (bilibili.com/video/BV...)</p><p><a href="` + html.EscapeString(text) + `" target="_blank" rel="noopener noreferrer">` + html.EscapeString(text) + `</a></p>`
			} else if strings.Contains(text, "youtube.com/watch?v=") {
				// YouTube 视频
				parts := strings.Split(text, "v=")
				if len(parts) > 1 {
					videoID := strings.Split(parts[1], "&")[0]
					if videoIDPattern.MatchString(videoID) {
						embedHTML = `<div class="video-container"><iframe src="https://www.youtube.com/embed/` + videoID + `" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`
					}
				}
			} else if strings.Contains(text, "youtu.be/") {
				// YouTube 短链接
				parts := strings.Split(text, "youtu.be/")
				if len(parts) > 1 {
					videoID := strings.Split(parts[1], "?")[0]
					if videoIDPattern.MatchString(videoID) {
						embedHTML = `<div class="video-container"><iframe src="https://www.youtube.com/embed/` + videoID + `" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`
					}
				}
			}

			// 替换原链接为嵌入式播放器
			if embedHTML != "" {
				s.ReplaceWithHtml(embedHTML)
			}
		}
	})

	// goquery 会补全 <html><body>，这里只取 body 里的内容（净化后为空就返回空串）
	html, _ := doc.Find("body").Html()
	return template.HTML(html)
}

// allowedImageSrc 图片地址只允许外站的 http/https 地址，以及本站的图片代理和静态文件
func allowedImageSrc(src string) bool {
	src = strings.TrimSpace(src)
	if isSiteImagePath(src) {
		return !strings.Contains(src, "..")
	}
	u, err := url.Parse(src)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	if site := siteHost(); site != "" && strings.EqualFold(u.Host, site) {
		return isSiteImagePath(u.Path) && !strings.Contains(u.Path, "..")
	}
	return true
}

func isSiteImagePath(p string) bool {
	return strings.HasPrefix(p, "/img/") || strings.HasPrefix(p, "/static/")
}

// siteHost 本站域名（取自 SITE_URL），用来识别写成绝对地址的站内链接
func siteHost() string {
	raw := os.Getenv("SITE_URL")
	if raw == "" {
		raw = "https://zhulink.vip"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// FirstBlocks 取 HTML 片段的前 n 个顶层块（段落、标题、列表等），保证标签成对
func FirstBlocks(htmlStr string, n int) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var b strings.Builder
	doc.Find("body").Children().EachWithBreak(func(i int, s *goquery.Selection) bool {
		if i >= n {
			return false
		}
		if out, err := goquery.OuterHtml(s); err == nil {
			b.WriteString(out)
			b.WriteByte('\n')
		}
		return true
	})
	return b.String()
}
